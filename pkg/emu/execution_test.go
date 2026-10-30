package emu

import (
	"golang.org/x/arch/arm64/arm64asm"
	"reflect"
	"testing"
)

func TestPeekPreservesArchitecturalStateAndDataEvents(t *testing.T) {
	em := NewEmulator()
	em.Mem.TrackAccesses = true
	em.Mem.WriteU32(0x1000, 0xd2800540)
	em.Regs.PC = 0x2000
	em.Mem.Accesses = []MemAccess{{Addr: 0x3456, Size: 8, IsWrite: true}}
	before := *em.Regs
	events := append([]MemAccess(nil), em.Mem.Accesses...)
	inst, err := em.Peek(0x1000)
	if err != nil || inst.Op != arm64asm.MOV {
		t.Fatalf("bad peek: %v %v", inst, err)
	}
	if *em.Regs != before || !reflect.DeepEqual(em.Mem.Accesses, events) {
		t.Fatal("Peek mutated registers or data accesses")
	}
}
