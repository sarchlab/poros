package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func clockRequest(t *testing.T, before []byte, reply, size, receive uint32) (*kernelState, *usermem.AddressSpace, emu.Registers) {
	t.Helper()
	m := &usermem.AddressSpace{}
	if e := m.Map(0x1000, uint64(len(before)), usermem.Read|usermem.Write, before); e != nil {
		t.Fatal(e)
	}
	k := &kernelState{taskSendRefs: 1, hostSendRefs: 1, receivePorts: map[uint32]receivePort{reply: {}}}
	r := emu.Registers{Z: true, V: true}
	r.GPR[0] = 0x1000
	r.GPR[1] = 0x200000003
	r.GPR[2] = uint64(size)<<32 | 0x1513
	r.GPR[3] = uint64(reply)<<32 | 0x107
	r.GPR[4] = 206 << 32
	r.GPR[5] = uint64(reply) << 32
	r.GPR[6] = uint64(receive)
	r.GPR[16] = uint64(uint32(0xffffffd1))
	return k, m, r
}

func TestClockServiceNativeReplies(t *testing.T) {
	var native struct {
		Cases []struct {
			Name        string
			ClockID     uint32 `json:"clock_id"`
			SendSize    uint32 `json:"send_size"`
			ReceiveSize uint32 `json:"receive_size"`
			Reply       uint32 `json:"reply_port"`
			X0, X1      uint64
			Received    uint32 `json:"received_port"`
			Before      string `json:"before_hex"`
			After       string `json:"after_hex"`
			Input       uint64 `json:"input_nzcv"`
			Output      uint64 `json:"output_nzcv"`
		}
	}
	kernelNativeResult(t, "clock-service-message-probe.c", &native)
	if len(native.Cases) != 14 {
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
			k, m, r := clockRequest(t, before, c.Reply, c.SendSize, c.ReceiveSize)
			old := r
			ok := k.trap(&r, m)
			got := make([]byte, len(before))
			m.Read(0x1000, got)
			if c.Name == "ndr-noncanonical" {
				if ok || r != old || !bytes.Equal(got, before) {
					t.Fatal("unsupported representation mutated state")
				}
				return
			}
			if c.X0 == 0 && c.ClockID < 2 && c.SendSize == 36 {
				if k.clocks[c.ClockID].sendRefs != 1 {
					t.Fatal("missing send right")
				}
				binary.LittleEndian.PutUint32(after[28:32], k.clocks[c.ClockID].name)
			} else if k.clocks != [2]clockService{} {
				t.Fatal("undelivered reply installed right")
			}
			old.GPR[0] = c.X0
			if !ok || r != old || r.GPR[1] != c.X1 || !bytes.Equal(got, after) || c.Input != 0x50000000 || c.Output != c.Input {
				t.Fatalf("native reply mismatch supported=%v x0=%x expected=%x\ngot=%x\nwant=%x", ok, r.GPR[0], c.X0, got, after)
			}
		})
	}
}

func TestClockServiceNativeCopyoutAuthority(t *testing.T) {
	var native struct {
		Cases []struct {
			Name         string
			Reply        uint32 `json:"reply_port"`
			Clock        uint32 `json:"clock_port"`
			Before       string `json:"before_hex"`
			After        string `json:"after_hex"`
			X0           uint64
			RefsBefore   uint32 `json:"refs_before"`
			RefsReturned uint32 `json:"refs_returned"`
		}
	}
	kernelNativeResult(t, "clock-service-copyout-probe.c", &native)
	if len(native.Cases) != 3 {
		t.Fatal(len(native.Cases))
	}
	for _, c := range native.Cases {
		t.Run(c.Name, func(t *testing.T) {
			before, _ := hex.DecodeString(c.Before)
			after, _ := hex.DecodeString(c.After)
			k, m, r := clockRequest(t, before, c.Reply, 36, 128)
			k.clocks[0] = clockService{c.Clock, c.RefsBefore}
			prefix := uint64(0)
			if c.Name == "writable-prefix16" {
				prefix = 16
			}
			if c.Name == "writable-prefix32" {
				prefix = 32
			}
			if e := m.Protect(0x1000+prefix, uint64(len(before))-prefix, usermem.Read, false); e != nil {
				t.Fatal(e)
			}
			old := r
			ok := k.trap(&r, m)
			got := make([]byte, len(before))
			m.Read(0x1000, got)
			if prefix != 0 {
				if ok || r != old || !bytes.Equal(got, before) || k.clocks[0].sendRefs != c.RefsBefore {
					t.Fatal("unsupported partial copyout mutated state")
				}
				return
			}
			old.GPR[0] = c.X0
			if !ok || r != old || !bytes.Equal(got, after) || k.clocks[0].sendRefs != c.RefsReturned {
				t.Fatal("readonly receive must retain copied out authority", r, k.clocks)
			}
		})
	}
}

func TestClockServiceNativeLifetime(t *testing.T) {
	var native struct {
		ClockID uint32 `json:"clock_id"`
		Cases   []struct {
			Name       string
			Result     uint64 `json:"operation_result"`
			Port, Refs uint32
			RefsResult int `json:"refs_result"`
		}
	}
	kernelNativeResult(t, "clock-service-lifetime-probe.c", &native)
	if len(native.Cases) != 8 || native.ClockID != 1 {
		t.Fatal(native)
	}
	request := make([]byte, 128)
	copy(request[24:32], nativeNDR[:])
	binary.LittleEndian.PutUint32(request[32:36], 1)
	k, m, base := clockRequest(t, request, 0x11f, 36, 128)
	var first, last uint32
	for _, c := range native.Cases {
		if c.Name == "first" || c.Name == "second" || c.Name == "reacquire" {
			m.Write(0x1000, request)
			r := base
			if !k.trap(&r, m) || r.GPR[0] != c.Result {
				t.Fatal(c.Name, r)
			}
			last = k.clocks[1].name
			if first == 0 {
				first = last
			}
			if c.Name == "second" && last != first {
				t.Fatal("live right renamed")
			}
			if c.Name == "reacquire" && last == first {
				t.Fatal("stale name revived")
			}
		} else if c.Name != "old-name-after-reacquire" {
			name := first
			if c.Name == "release-reacquired" {
				name = last
			}
			r := emu.Registers{C: true}
			r.GPR[0] = 0x103
			r.GPR[1] = uint64(name)
			old := r
			if !k.deallocatePort(&r) || r.GPR[0] != c.Result {
				t.Fatal(c.Name, r)
			}
			old.GPR[0] = c.Result
			if r != old {
				t.Fatal("Mach ABI mutated")
			}
		}
		if c.RefsResult == 0 && k.clocks[1].sendRefs != c.Refs {
			t.Fatal(c.Name, k.clocks)
		}
		if c.RefsResult == 15 && c.Name != "old-name-after-reacquire" && c.Name != "release-stale" && k.clocks[1].sendRefs != 0 {
			t.Fatal("released right retained authority", c.Name)
		}
		if c.Name == "old-name-after-reacquire" && k.clocks[1].name == first {
			t.Fatal("old name alive")
		}
	}
}

func TestClockServiceUnsupportedBeforeMutation(t *testing.T) {
	for _, kind := range []string{"wrong-target", "missing-host", "partial", "refs-bound", "names-bound", "size", "copyin"} {
		t.Run(kind, func(t *testing.T) {
			request := make([]byte, 128)
			copy(request[24:32], nativeNDR[:])
			k, m, r := clockRequest(t, request, 0x11f, 36, 128)
			switch kind {
			case "wrong-target":
				r.GPR[3]++
			case "missing-host":
				k.hostSendRefs = 0
			case "partial":
				m.Protect(0x1020, 96, usermem.Read, false)
			case "refs-bound":
				k.clocks[0] = clockService{0x123, 65535}
			case "names-bound":
				k.nextPort = 0x10000
			case "size":
				r.GPR[2] = 48<<32 | 0x1513
			case "copyin":
				r.GPR[0] = 0x9000
			}
			old := r
			clocks := k.clocks
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
			if ok || r != old || clocks != k.clocks || next != k.nextPort || !bytes.Equal(got, request) {
				t.Fatal("unsupported mutation", kind, r)
			}
		})
	}
}

func TestClockShortReplyDoesNotAccessDiscardedBody(t *testing.T) {
	request := make([]byte, 128)
	copy(request[24:32], nativeNDR[:])
	k, m, r := clockRequest(t, request, 0x11f, 36, 47)
	if e := m.Protect(0x1020, 96, usermem.Read, false); e != nil {
		t.Fatal(e)
	}
	if !k.trap(&r, m) || r.GPR[0] != 0x10004004 || k.clocks != [2]clockService{} {
		t.Fatal(r, k.clocks)
	}
	got := make([]byte, 128)
	m.Read(0x1000, got)
	if binary.LittleEndian.Uint32(got[:4]) != 0x80001200 || binary.LittleEndian.Uint32(got[4:8]) != 24 || !bytes.Equal(got[32:], request[32:]) {
		t.Fatal("short reply touched discarded body")
	}
}
