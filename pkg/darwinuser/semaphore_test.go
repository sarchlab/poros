package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func semaphoreRequest(t *testing.T, before []byte, reply, size, receive uint32) (*kernelState, *usermem.AddressSpace, emu.Registers) {
	k, m, r := clockRequest(t, before, reply, size, receive)
	r.GPR[3] = uint64(reply)<<32 | 0x103
	r.GPR[4] = 3418 << 32
	return k, m, r
}
func TestSemaphoreCreateNativeReplies(t *testing.T) {
	var native struct {
		Cases []struct {
			Name          string
			Policy, Value int32
			SendSize      uint32 `json:"send_size"`
			ReceiveSize   uint32 `json:"receive_size"`
			Reply         uint32 `json:"reply_port"`
			X0, X1        uint64
			Port          uint32 `json:"semaphore_port"`
			Refs          uint32
			Before        string `json:"before_hex"`
			After         string `json:"after_hex"`
			Input         uint64 `json:"input_nzcv"`
			Output        uint64 `json:"output_nzcv"`
		}
	}
	kernelNativeResult(t, "semaphore-create-message-probe.c", &native)
	if len(native.Cases) != 20 {
		t.Fatal(len(native.Cases))
	}
	for _, c := range native.Cases {
		t.Run(c.Name, func(t *testing.T) {
			before, e := hex.DecodeString(c.Before)
			if e != nil {
				t.Fatal(e)
			}
			after, e := hex.DecodeString(c.After)
			if e != nil {
				t.Fatal(e)
			}
			k, m, r := semaphoreRequest(t, before, c.Reply, c.SendSize, c.ReceiveSize)
			old := r
			ok := k.trap(&r, m)
			got := make([]byte, len(before))
			m.Read(0x1000, got)
			if c.Name == "ndr-noncanonical" {
				if ok || r != old || !bytes.Equal(got, before) || len(k.semaphores) != 0 {
					t.Fatal("unmodeled representation mutated state")
				}
				return
			}
			if c.Port != 0 {
				if len(k.semaphores) != 1 || c.Refs != 1 {
					t.Fatal("missing semaphore")
				}
				for name, semaphore := range k.semaphores {
					binary.LittleEndian.PutUint32(after[28:32], name)
					if semaphore.policy != uint32(c.Policy) || semaphore.value != c.Value || semaphore.sendRefs != 1 {
						t.Fatal(semaphore)
					}
				}
			} else if len(k.semaphores) != 0 {
				t.Fatal("undelivered reply retained semaphore")
			}
			old.GPR[0] = c.X0
			if !ok || r != old || r.GPR[1] != c.X1 || !bytes.Equal(got, after) || c.Input != 0x50000000 || c.Output != c.Input {
				t.Fatalf("native semaphore mismatch supported=%v x0=%x expected=%x\ngot=%x\nwant=%x", ok, r.GPR[0], c.X0, got, after)
			}
		})
	}
}

func TestSemaphoreCreateUnsupportedAndCopyinBoundaries(t *testing.T) {
	for _, kind := range []string{"target", "descriptor", "options", "size", "partial", "names-bound", "copyin"} {
		t.Run(kind, func(t *testing.T) {
			request := make([]byte, 128)
			copy(request[24:32], nativeNDR[:])
			k, m, r := semaphoreRequest(t, request, 0x11f, 40, 128)
			switch kind {
			case "target":
				r.GPR[3]++
			case "descriptor":
				r.GPR[5]++
			case "options":
				r.GPR[1]++
			case "size":
				r.GPR[2] = 48<<32 | 0x1513
			case "partial":
				m.Protect(0x1020, 96, usermem.Read, false)
			case "names-bound":
				k.nextPort = 0x10000
			case "copyin":
				r.GPR[0] = 0x9000
			}
			old := r
			next := k.nextPort
			ok := k.trap(&r, m)
			if kind == "copyin" {
				old.GPR[0] = 0x10000002
				if !ok || r != old {
					t.Fatal(r)
				}
				return
			}
			got := make([]byte, 128)
			m.Read(0x1000, got)
			if ok || r != old || len(k.semaphores) != 0 || next != k.nextPort || !bytes.Equal(got, request) {
				t.Fatal("unsupported mutated state", kind)
			}
		})
	}
}

func TestSemaphoreCreateNativeCopyoutAuthority(t *testing.T) {
	var native struct {
		Cases []struct {
			Name        string
			ReceiveSize uint32 `json:"receive_size"`
			Reply       uint32 `json:"reply_port"`
			X0, X1      uint64
			NewNames    int    `json:"new_send_names"`
			NewRefs     uint32 `json:"new_port_refs"`
			Before      string `json:"before_hex"`
			After       string `json:"after_hex"`
			Queued      int    `json:"queued_replies"`
			Status      int    `json:"reply_status_result"`
		}
	}
	kernelNativeResult(t, "semaphore-create-copyout-probe.c", &native)
	if len(native.Cases) != 8 {
		t.Fatal(len(native.Cases))
	}
	for _, c := range native.Cases {
		t.Run(c.Name, func(t *testing.T) {
			before, _ := hex.DecodeString(c.Before)
			after, _ := hex.DecodeString(c.After)
			k, m, r := semaphoreRequest(t, before, c.Reply, 40, c.ReceiveSize)
			prefix := uint64(len(before))
			switch c.Name {
			case "readonly":
				prefix = 0
			case "writable-prefix16":
				prefix = 16
			case "writable-prefix32":
				prefix = 32
			}
			if prefix < uint64(len(before)) {
				if e := m.Protect(0x1000+prefix, uint64(len(before))-prefix, usermem.Read, false); e != nil {
					t.Fatal(e)
				}
			}
			old := r
			ok := k.trap(&r, m)
			got := make([]byte, len(before))
			m.Read(0x1000, got)
			if prefix == 16 || prefix == 32 {
				if ok || r != old || len(k.semaphores) != 0 || !bytes.Equal(got, before) {
					t.Fatal("unsupported partial output mutated state")
				}
				return
			}
			old.GPR[0] = c.X0
			if c.X0 == 0 && c.NewNames == 1 {
				for name := range k.semaphores {
					binary.LittleEndian.PutUint32(after[28:32], name)
				}
			}
			if !ok || r != old || r.GPR[1] != c.X1 || len(k.semaphores) != c.NewNames || !bytes.Equal(got, after) || c.Status != 0 || c.Queued != 0 {
				t.Fatal("copyout contract mismatch", c.Name, r, k.semaphores)
			}
			for name, s := range k.semaphores {
				if s.sendRefs != c.NewRefs {
					t.Fatal(s)
				}
				release := emu.Registers{C: true}
				release.GPR[0] = 0x103
				release.GPR[1] = uint64(name)
				if !k.deallocatePort(&release) || release.GPR[0] != 0 || len(k.semaphores) != 0 {
					t.Fatal("last send did not remove object")
				}
				release.GPR[0] = 0x103
				if !k.deallocatePort(&release) || release.GPR[0] != 15 {
					t.Fatal("stale right retained authority")
				}
			}
		})
	}
}

func TestSemaphoreObjectsHaveIndependentNamesAndValues(t *testing.T) {
	request := make([]byte, 128)
	copy(request[24:32], nativeNDR[:])
	k, m, base := semaphoreRequest(t, request, 0x11f, 40, 48)
	var names [2]uint32
	for i := range names {
		binary.LittleEndian.PutUint32(request[36:40], uint32(i+1))
		m.Write(0x1000, request)
		r := base
		if !k.trap(&r, m) || r.GPR[0] != 0 {
			t.Fatal(r)
		}
		var result [48]byte
		m.Read(0x1000, result[:])
		names[i] = binary.LittleEndian.Uint32(result[28:32])
	}
	if names[0] == names[1] || len(k.semaphores) != 2 || k.semaphores[names[0]].value != 1 || k.semaphores[names[1]].value != 2 {
		t.Fatal(k.semaphores)
	}
	r := emu.Registers{}
	r.GPR[0] = 0x103
	r.GPR[1] = uint64(names[0])
	if !k.deallocatePort(&r) || len(k.semaphores) != 1 || k.semaphores[names[1]].value != 2 {
		t.Fatal(k.semaphores)
	}
}

func TestSemaphoreNativeSendLifetime(t *testing.T) {
	var native struct {
		Cases []struct {
			Name       string
			Result     uint64 `json:"operation_result"`
			RefsResult int    `json:"refs_result"`
			Refs       uint32
		}
	}
	kernelNativeResult(t, "semaphore-lifetime-probe.c", &native)
	if len(native.Cases) != 8 {
		t.Fatal(len(native.Cases))
	}
	request := make([]byte, 128)
	copy(request[24:32], nativeNDR[:])
	k, m, base := semaphoreRequest(t, request, 0x11f, 40, 48)
	var first, last uint32
	for _, c := range native.Cases {
		switch c.Name {
		case "create", "new-object":
			m.Write(0x1000, request)
			r := base
			if !k.trap(&r, m) || r.GPR[0] != c.Result {
				t.Fatal(c.Name, r)
			}
			var response [48]byte
			m.Read(0x1000, response[:])
			last = binary.LittleEndian.Uint32(response[28:32])
			if first == 0 {
				first = last
			} else if first == last {
				t.Fatal("stale semaphore name reused")
			}
		case "copy-send":
			r := emu.Registers{}
			r.GPR[0] = 0x103
			r.GPR[1] = uint64(first)
			r.GPR[2] = 0
			r.GPR[3] = 1
			if !k.modifyPortRefs(&r) || r.GPR[0] != c.Result {
				t.Fatal(r)
			}
		case "release-first", "release-last", "release-stale":
			r := emu.Registers{}
			r.GPR[0] = 0x103
			r.GPR[1] = uint64(first)
			if !k.deallocatePort(&r) || r.GPR[0] != c.Result {
				t.Fatal(c.Name, r)
			}
		case "old-name-after-new":
			if _, ok := k.semaphores[first]; ok {
				t.Fatal("stale name retained")
			}
			continue
		case "destroy-new":
			// Explicit semaphore_destroy is not exercised by the current guest and
			// remains unsupported. Its native cleanup observation is retained, but
			// does not stand in for an implemented destroy RPC.
			if c.Result != 0 || c.RefsResult != 15 {
				t.Fatal(c)
			}
			continue
		}
		name := first
		if c.Name == "new-object" {
			name = last
		}
		object, exists := k.semaphores[name]
		if c.RefsResult == 0 {
			if !exists || object.sendRefs != c.Refs {
				t.Fatal(c.Name, k.semaphores)
			}
		} else if exists {
			t.Fatal("last send retained object", c.Name)
		}
	}
}
