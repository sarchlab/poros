package darwinuser

import (
	"bytes"
	"encoding/hex"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestHostPriorityMessageNativeContract(t *testing.T) {
	var native struct {
		Direct struct {
			Status, Count int
			Priorities    [8]int32
		}
		Cases []struct {
			Name          string
			Flavor, Count uint32
			SendSize      uint32 `json:"send_size"`
			ReceiveSize   uint32 `json:"receive_size"`
			Reply         uint32 `json:"reply_port"`
			X0, X1        uint64
			Carry         bool
			Destroy       int
			Before        string `json:"before_hex"`
			After         string `json:"after_hex"`
		}
		HostRelease int `json:"host_release"`
	}
	kernelNativeResult(t, "host-info-message-probe.c", &native)
	if native.Direct.Status != 0 || native.Direct.Count != 8 || native.HostRelease != 0 || len(native.Cases) != 14 {
		t.Fatal(native)
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
			m := &usermem.AddressSpace{}
			if e = m.Map(0x1000, uint64(len(before)), usermem.Read|usermem.Write, before); e != nil {
				t.Fatal(e)
			}
			k := &kernelState{hostSendRefs: 1, hostPriorities: &native.Direct.Priorities, receivePorts: map[uint32]receivePort{c.Reply: {}}}
			r := emu.Registers{C: true}
			r.GPR[0] = 0x1000
			r.GPR[1] = 0x200000003
			r.GPR[2] = uint64(c.SendSize)<<32 | 0x1513
			r.GPR[3] = uint64(c.Reply)<<32 | 0x107
			r.GPR[4] = 200 << 32
			r.GPR[5] = uint64(c.Reply) << 32
			r.GPR[6] = uint64(c.ReceiveSize)
			r.GPR[16] = uint64(uint32(0xffffffd1))
			old := r
			supported := k.trap(&r, m)
			got := make([]byte, len(before))
			m.Read(0x1000, got)
			if c.Name == "unknown-flavor" || c.Name == "ndr-big-endian" {
				if supported || r != old || !bytes.Equal(got, before) {
					t.Fatal("unmodeled service or representation mutated state")
				}
				return
			}
			if !supported || r.GPR[0] != c.X0 || r.GPR[1] != c.X1 || !bytes.Equal(got, after) || c.Destroy != 0 {
				t.Fatalf("reply mismatch: supported=%v x0=%x want=%x\ngot=%x\nwant=%x", supported, r.GPR[0], c.X0, got, after)
			}
			old.GPR[0] = c.X0
			if r != old {
				t.Fatal("unexpected architectural change")
			}
		})
	}
}

func TestHostPriorityUnsupportedAndFaultBoundaries(t *testing.T) {
	for _, kind := range []string{"missing", "target", "unowned", "reply", "descriptor", "options", "partial", "copyin", "readonly"} {
		t.Run(kind, func(t *testing.T) {
			k, m, r, _ := mapMessage(t)
			k.hostSendRefs = 1
			k.hostPriorities = &[8]int32{1, 2, 3, 4, 5, 6, 7, 8}
			reply := uint32(r.GPR[3] >> 32)
			request := make([]byte, 40)
			copy(request[24:32], nativeNDR[:])
			request[32] = 5
			request[36] = 8
			m.Write(0x1000, request)
			r.GPR[2] = 40<<32 | 0x1513
			r.GPR[3] = uint64(reply)<<32 | 0x107
			r.GPR[4] = 200 << 32
			r.GPR[5] = uint64(reply) << 32
			r.GPR[6] = 80
			switch kind {
			case "missing":
				k.hostPriorities = nil
			case "target":
				r.GPR[3]++
			case "unowned":
				k.hostSendRefs = 0
			case "reply":
				r.GPR[5] += 4 << 32
				r.GPR[3] += 4 << 32
			case "descriptor":
				r.GPR[5]++
			case "options":
				r.GPR[1]++
			case "partial":
				m.Protect(0x1028, 88, usermem.Read, false)
			case "copyin":
				r.GPR[0] = 0x9000
			case "readonly":
				m.Protect(0x1000, 128, usermem.Read, false)
			}
			old := r
			got := make([]byte, 128)
			m.Read(0x1000, got)
			supported := k.trap(&r, m)
			expected := uint64(0)
			if kind == "copyin" {
				expected = 0x10000002
			}
			if kind == "readonly" {
				expected = 0x10004008
			}
			if expected != 0 {
				old.GPR[0] = expected
				if !supported || r != old {
					t.Fatal("fault contract", r, supported)
				}
			} else if supported || r != old {
				t.Fatal("unsupported mutated state")
			}
			after := make([]byte, 128)
			m.Read(0x1000, after)
			if !bytes.Equal(got, after) || k.ownedBytes != 0 {
				t.Fatal("unexpected memory mutation")
			}
		})
	}
}
