package emu

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestLegacySingleQAccessStream(t *testing.T) {
	em := NewEmulator()
	code := make([]byte, 8)
	binary.LittleEndian.PutUint32(code, 0x3dc00020)     // ldr q0,[x1]
	binary.LittleEndian.PutUint32(code[4:], 0x3d800040) // str q0,[x2]
	em.LoadProgram(0x1000, code)
	em.Regs.PC = 0x1000
	em.Regs.GPR[1] = 0x2000
	em.Regs.GPR[2] = 0x3000
	em.Mem.WriteU64(0x2000, 0x123456789abcdef0)
	em.Mem.WriteU64(0x2008, 0xfedcba9876543210)
	em.Mem.TrackAccesses = true
	if _, _, err := em.StepDetailed(); err != nil {
		t.Fatal(err)
	}
	lo, hi := em.Regs.ReadQ(0)
	if lo != 0x123456789abcdef0 || hi != 0xfedcba9876543210 {
		t.Fatal("LDR Q lost a half")
	}
	if !reflect.DeepEqual(em.Mem.Accesses, []MemAccess{{0x2000, 8, false}, {0x2008, 8, false}}) {
		t.Fatalf("load accesses: %+v", em.Mem.Accesses)
	}
	if _, _, err := em.StepDetailed(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(em.Mem.Accesses, []MemAccess{{0x3000, 8, true}, {0x3008, 8, true}}) {
		t.Fatalf("store accesses: %+v", em.Mem.Accesses)
	}
	em.Mem.TrackAccesses = false
	if em.Mem.ReadU64(0x3000) != lo || em.Mem.ReadU64(0x3008) != hi {
		t.Fatal("STR Q lost a half")
	}
}
func TestLegacyLiteralLoadAccessStream(t *testing.T) {
	em := NewEmulator()
	code := make([]byte, 16)
	binary.LittleEndian.PutUint32(code, 0x58000040) // ldr x0,PC+8
	binary.LittleEndian.PutUint64(code[8:], 0x0123456789abcdef)
	em.LoadProgram(0x1000, code)
	em.Regs.PC = 0x1000
	em.Mem.TrackAccesses = true
	if _, _, err := em.StepDetailed(); err != nil {
		t.Fatal(err)
	}
	if em.Regs.GPR[0] != 0x0123456789abcdef {
		t.Fatal("literal load used wrong address")
	}
	if !reflect.DeepEqual(em.Mem.Accesses, []MemAccess{{0x1008, 8, false}}) {
		t.Fatalf("literal accesses: %+v", em.Mem.Accesses)
	}
}
