package darwinuser

import (
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestNativeHostSendRightLifetime(t *testing.T) {
	var native struct {
		Cases []struct {
			Name   string
			X0, X1 uint64
			Carry  bool
			Input  uint64 `json:"input_nzcv"`
			Output uint64 `json:"output_nzcv"`
			Port   uint32
			Lookup int
			Refs   uint32 `json:"send_refs"`
		}
	}
	kernelNativeResult(t, "host-right-probe.c", &native)
	if len(native.Cases) != 11 {
		t.Fatal(len(native.Cases))
	}
	k := &kernelState{taskSendRefs: 1}
	m := &usermem.AddressSpace{}
	names := map[uint32]uint32{}
	first := uint32(0)
	for _, c := range native.Cases {
		if c.Name == "old-name-after-reacquire" {
			if c.Lookup != 15 || names[c.Port] == k.hostRightName() {
				t.Fatal("stale host name revived")
			}
			continue
		}
		r := emu.Registers{N: c.Input>>31&1 != 0, Z: c.Input>>30&1 != 0, C: c.Input>>29&1 != 0, V: c.Input>>28&1 != 0}
		r.GPR[16] = uint64(uint32(0xffffffee))
		r.GPR[0] = 0x103
		r.GPR[1] = c.X1
		acquire := c.Name == "acquire-first" || c.Name == "acquire-second" || c.Name == "reacquire"
		if acquire {
			r.GPR[16] = uint64(uint32(0xffffffe3))
			r.GPR[0] = 0x1234
		} else if n, ok := names[uint32(c.X1)]; ok {
			r.GPR[1] = c.X1&0xffffffff00000000 | uint64(n)
		}
		if c.Name == "release-high-bits" {
			r.GPR[0] |= 1 << 32
		}
		if c.Name == "invalid-task" {
			r.GPR[0] = 0
		}
		old := r
		if !k.trap(&r, m) {
			t.Fatal("unsupported", c.Name)
		}
		if c.Input != c.Output || r.C != c.Carry {
			t.Fatal("native flags", c.Name)
		}
		if acquire {
			if n, ok := names[c.Port]; ok && n != uint32(r.GPR[0]) {
				t.Fatal("live host name changed")
			}
			names[c.Port] = uint32(r.GPR[0])
			old.GPR[0] = r.GPR[0]
			if c.Name == "acquire-first" {
				first = uint32(r.GPR[0])
			}
			if c.Name == "reacquire" && uint32(r.GPR[0]) == first {
				t.Fatal("stale name resurrected")
			}
		} else {
			old.GPR[0] = c.X0
		}
		if old != r {
			t.Fatal("ABI mutation", c.Name, r, old)
		}
		if acquire || c.Name == "release-first" || c.Name == "release-second" || c.Name == "release-high-bits" {
			if k.hostSendRefs != c.Refs {
				t.Fatal("ref count", c.Name, k.hostSendRefs, c.Refs)
			}
		}
	}
	// A subsequent reply-port allocation cannot collide with any current host name.
	if k.allocateReceivePort(receivePort{}) == k.hostRightName() {
		t.Fatal("name collision")
	}
}

func TestPinnedOrReceivePortDeallocationRemainsExplicit(t *testing.T) {
	for _, name := range []uint32{0x103, 0x10b, 0x10f} {
		k := &kernelState{taskSendRefs: 1, threadSendRefs: 1, receivePorts: map[uint32]receivePort{0x10f: {}}}
		r := emu.Registers{C: true}
		r.GPR[0] = 0x103
		r.GPR[1] = uint64(name)
		r.GPR[16] = uint64(uint32(0xffffffee))
		old := r
		if k.trap(&r, &usermem.AddressSpace{}) || r != old || k.taskSendRefs != 1 || k.threadSendRefs != 1 || len(k.receivePorts) != 1 {
			t.Fatal("unsupported deallocation mutated state")
		}
	}
}
