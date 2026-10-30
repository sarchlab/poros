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
	k := kernelState{reserved: []addressSpan{{0x100000000, 0x100040000}}}
	r := vmArgs(t, m, 0x100000000)
	if !k.trap(&r, m) || r.GPR[0] != 0 || !r.C {
		t.Fatal(r.GPR[:6])
	}
	var out [8]byte
	m.Read(0x1000, out[:])
	addr := binary.LittleEndian.Uint64(out[:])
	if addr != 0x100040000 || k.anonymousBytes != vmPage {
		t.Fatal(addr, k.anonymousBytes)
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
	k := kernelState{}
	if !k.trap(&r, m) || r.GPR[0] != 10 || k.anonymousBytes != vmPage {
		t.Fatal(r, k.anonymousBytes)
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
		{"capacity", func(r *emu.Registers) { r.GPR[2] = maxAnonymousBytes + 1 }, 3, true},
		{"unsupported-flags", func(r *emu.Registers) { r.GPR[4] = 2 }, 0, false},
		{"executable", func(r *emu.Registers) { r.GPR[5] = 5 }, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &usermem.AddressSpace{}
			m.Map(0x1000, 16, usermem.Read|usermem.Write, nil)
			r := vmArgs(t, m, 0x100000000)
			tc.mutate(&r)
			before := r
			k := kernelState{}
			handled := k.trap(&r, m)
			if handled != tc.handled || (handled && r.GPR[0] != tc.want) || (!handled && r != before) || len(m.Mappings()) != 1 || k.anonymousBytes != 0 {
				t.Fatal(handled, r.GPR[:6], len(m.Mappings()))
			}
		})
	}
}
func TestMachMapFixedCollisionAndAlignmentMask(t *testing.T) {
	m := &usermem.AddressSpace{}
	m.Map(0x1000, 16, usermem.Read|usermem.Write, nil)
	k := kernelState{reserved: []addressSpan{{0x100000000, 0x100040000}}}
	r := vmArgs(t, m, 0x100000000)
	r.GPR[4] = 0
	if !k.trap(&r, m) || r.GPR[0] != 3 || k.anonymousBytes != 0 {
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
