package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func vmArgs(t *testing.T, m *usermem.AddressSpace, hint uint64) emu.Registers {
	t.Helper()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], hint)
	if err := m.Write(0x1000, b[:]); err != nil {
		t.Fatal(err)
	}
	r := emu.Registers{C: true}
	r.GPR[16] = 0xfffffff1
	r.GPR[0] = 0x103
	r.GPR[1] = 0x1000
	r.GPR[2] = 1
	r.GPR[4] = 0x3c000001
	r.GPR[5] = 3
	return r
}
func TestMachMapAnonymousRoundZeroPrivateAndReserve(t *testing.T) {
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 16, usermem.Read|usermem.Write, nil)
	k := kernelState{taskSendRefs: 1, reserved: []addressSpan{{0x100000000, 0x100040000}}}
	r := vmArgs(t, m, 0x100000000)
	if !k.trap(&r, m) || r.GPR[0] != 0 || !r.C {
		t.Fatal(r.GPR[:6])
	}
	var out [8]byte
	m.Read(0x1000, out[:])
	addr := binary.LittleEndian.Uint64(out[:])
	if addr != 0x100040000 || k.ownedBytes != vmPage {
		t.Fatal(addr, k.ownedBytes)
	}
	b := make([]byte, vmPage)
	if err := m.Read(addr, b); err != nil {
		t.Fatal(err)
	}
	for _, v := range b {
		if v != 0 {
			t.Fatal("uninitialized anonymous bytes")
		}
	}
	if err := m.Write(addr, []byte{99}); err != nil {
		t.Fatal(err)
	}
	r = vmArgs(t, m, 0x100000000)
	if !k.trap(&r, m) || r.GPR[0] != 0 {
		t.Fatal(r)
	}
	m.Read(0x1000, out[:])
	next := binary.LittleEndian.Uint64(out[:])
	if next != addr+vmPage {
		t.Fatal("allocation overlap", next)
	}
	m.Read(next, out[:])
	if out[0] != 0 {
		t.Fatal("private anonymous pages shared")
	}
}
func TestMachMapCopyoutFailureRetainsMapping(t *testing.T) {
	m := &usermem.AddressSpace{}
	m.MapProtected(0x1000, 16, usermem.Read|usermem.Write, usermem.Read|usermem.Write, nil)
	r := vmArgs(t, m, 0x100000000)
	m.Protect(0x1000, 16, usermem.Read, false)
	k := kernelState{taskSendRefs: 1}
	if !k.trap(&r, m) || r.GPR[0] != 10 || k.ownedBytes != vmPage {
		t.Fatal(r, k.ownedBytes)
	}
	if err := m.Write(0x100000000, []byte{1}); err != nil {
		t.Fatal("copyout failure rolled back mapping", err)
	}
}
func TestMachMapErrorsAndUnsupportedFlags(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*emu.Registers)
		want    uint64
		handled bool
	}{
		{"task", func(r *emu.Registers) { r.GPR[0] = 0 }, 0x10000003, true},
		{"bad-pointer", func(r *emu.Registers) { r.GPR[1] = 0 }, 10, true},
		{"zero-size", func(r *emu.Registers) { r.GPR[2] = 0 }, 4, true},
		{"overflow", func(r *emu.Registers) { r.GPR[2] = ^uint64(0) }, 4, true},
		{"protection", func(r *emu.Registers) { r.GPR[5] = 8 }, 4, true},
		{"capacity", func(r *emu.Registers) { r.GPR[2] = maxOwnedMappingBytes + 1 }, 3, true},
		{"unsupported-flags", func(r *emu.Registers) { r.GPR[4] = 2 }, 0, false},
		{"executable", func(r *emu.Registers) { r.GPR[5] = 5 }, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &usermem.AddressSpace{}
			m.Map(0x1000, 16, usermem.Read|usermem.Write, nil)
			r := vmArgs(t, m, 0x100000000)
			tc.mutate(&r)
			before := r
			k := kernelState{taskSendRefs: 1}
			handled := k.trap(&r, m)
			if handled != tc.handled || (handled && r.GPR[0] != tc.want) || (!handled && r != before) || len(m.Mappings()) != 1 || k.ownedBytes != 0 {
				t.Fatal(handled, r.GPR[:6], len(m.Mappings()))
			}
		})
	}
}
func TestMachMapFixedCollisionAndAlignmentMask(t *testing.T) {
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 16, usermem.Read|usermem.Write, nil)
	k := kernelState{taskSendRefs: 1, reserved: []addressSpan{{0x100000000, 0x100040000}}}
	r := vmArgs(t, m, 0x100000000)
	r.GPR[4] = 0
	if !k.trap(&r, m) || r.GPR[0] != 3 || k.ownedBytes != 0 {
		t.Fatal("fixed overwrite accepted")
	}
	r = vmArgs(t, m, 0x100000000)
	r.GPR[3] = (1 << 20) - 1
	if !k.trap(&r, m) || r.GPR[0] != 0 {
		t.Fatal(r)
	}
	var b [8]byte
	m.Read(0x1000, b[:])
	if binary.LittleEndian.Uint64(b[:]) != 0x100100000 {
		t.Fatal("alignment mask ignored")
	}
}

func TestMachDeallocateRoundingReuseAndAccounting(t *testing.T) {
	m := &usermem.AddressSpace{}
	if e := m.Map(0x1000, 16, usermem.Read|usermem.Write, nil); e != nil {
		t.Fatal(e)
	}
	k := kernelState{taskSendRefs: 1}
	r := vmArgs(t, m, 0x100000000)
	r.GPR[2] = 3 * vmPage
	if !k.trap(&r, m) || r.GPR[0] != 0 {
		t.Fatal(r)
	}
	var b [8]byte
	m.Read(0x1000, b[:])
	addr := binary.LittleEndian.Uint64(b[:])
	m.Write(addr+vmPage, []byte{99})
	r = emu.Registers{C: true}
	r.GPR[16] = 0xfffffff4
	r.GPR[0] = 0x103
	r.GPR[1] = addr + vmPage + 1
	r.GPR[2] = 1
	if !k.trap(&r, m) || r.GPR[0] != 0 || !r.C {
		t.Fatal(r)
	}
	if k.ownedBytes != 2*vmPage || k.ownedMappings != 2 {
		t.Fatal(k)
	}
	if e := m.Read(addr+vmPage, b[:]); e == nil {
		t.Fatal("removed page readable")
	}
	if e := m.Read(addr, b[:]); e != nil {
		t.Fatal(e)
	}
	if e := m.Read(addr+2*vmPage, b[:]); e != nil {
		t.Fatal(e)
	}
	r = vmArgs(t, m, addr+vmPage)
	r.GPR[4] = 0
	if !k.trap(&r, m) || r.GPR[0] != 0 {
		t.Fatal(r)
	}
	m.Read(addr+vmPage, b[:])
	if b != [8]byte{} {
		t.Fatal("reallocated page not zero", b)
	}
	// Removing an image reservation releases its virtual address for fixed maps.
	k.reserved = []addressSpan{{0x200000000, 0x200008000}}
	r.GPR[16] = 0xfffffff4
	r.GPR[0] = 0x103
	r.GPR[1] = 0x200004000
	r.GPR[2] = vmPage
	if !k.trap(&r, m) || len(k.reserved) != 1 || k.reserved[0].hi != 0x200004000 {
		t.Fatal(k.reserved)
	}
}
func TestMachDeallocateErrorsAndProtectedRegions(t *testing.T) {
	for _, tc := range []struct {
		task, start, size, want uint64
		handled                 bool
	}{
		{0, 0, 1, 0x10000003, true}, {0x103, ^uint64(0), 0, 0, true}, {0x103, ^uint64(0), 2, 4, true}, {0x103, 0x2000, 1, 0, true}, {0x103, 0x4000, 1, 0, false},
	} {
		m := &usermem.AddressSpace{}
		m.Map(0x4000, vmPage, usermem.Read, nil)
		k := kernelState{taskSendRefs: 1, protected: []addressSpan{{0x4000, 0x8000}}}
		r := emu.Registers{}
		r.GPR[16] = 0xfffffff4
		r.GPR[0] = tc.task
		r.GPR[1] = tc.start
		r.GPR[2] = tc.size
		before := r
		got := k.trap(&r, m)
		if got != tc.handled || (got && r.GPR[0] != tc.want) || (!got && r != before) {
			t.Fatal(tc, r, got)
		}
		if len(m.Mappings()) != 1 {
			t.Fatal("error changed mappings")
		}
	}
}

func TestMunmapProtectedRangeAndAccounting(t *testing.T) {
	const base uint64 = 0x200000000
	m := &usermem.AddressSpace{}
	m.Map(base, 3*vmPage, usermem.Read|usermem.Write, nil)
	k := kernelState{ownedRanges: []addressSpan{{base, base + 3*vmPage}}, ownedBytes: 3 * vmPage, ownedMappings: 1, reserved: []addressSpan{{base, base + 3*vmPage}}, protected: []addressSpan{{base + vmPage, base + 2*vmPage}}}
	r := emu.Registers{C: true}
	r.GPR[16] = 73
	r.GPR[0] = base
	r.GPR[1] = 3 * vmPage
	old := r
	if k.trap(&r, m) || r != old || k.ownedBytes != 3*vmPage || len(m.Mappings()) != 1 {
		t.Fatal("unsupported protected unmap mutated state")
	}
	k.protected = nil
	r.GPR[0] = base + vmPage
	r.GPR[1] = 1
	r.GPR[2] = 0x80 // stale third register is not an advice argument
	if !k.trap(&r, m) || r.GPR[0] != 0 || r.GPR[1] != 0 || r.C || k.ownedBytes != 2*vmPage || k.ownedMappings != 2 || len(k.reserved) != 2 || len(m.Mappings()) != 2 {
		t.Fatal(r, k)
	}
	if m.Check(base+vmPage, 1, usermem.Read, "removed") == nil {
		t.Fatal("middle page remained mapped")
	}
}
