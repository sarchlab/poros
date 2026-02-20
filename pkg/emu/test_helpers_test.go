package emu

import (
	"encoding/binary"
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

// newTestEmulator creates an emulator for testing with code loaded at address 0.
func newTestEmulator(code ...uint32) *Emulator {
	em := NewEmulator()
	buf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(buf[i*4:], inst)
	}
	em.LoadProgram(0, buf)
	return em
}

// stepN executes N instructions.
func stepN(t *testing.T, em *Emulator, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := em.Step(); err != nil {
			t.Fatalf("step %d failed: %v", i+1, err)
		}
	}
}

// assertX asserts a 64-bit register value.
func assertX(t *testing.T, em *Emulator, reg int, want uint64) {
	t.Helper()
	got := em.Regs.ReadX(reg)
	if got != want {
		t.Errorf("X%d = %#x, want %#x", reg, got, want)
	}
}

// assertW asserts a 32-bit register value.
func assertW(t *testing.T, em *Emulator, reg int, want uint32) {
	t.Helper()
	got := em.Regs.ReadW(reg)
	if got != want {
		t.Errorf("W%d = %#x, want %#x", reg, got, want)
	}
}

// assertPC asserts the program counter value.
func assertPC(t *testing.T, em *Emulator, want uint64) {
	t.Helper()
	if em.Regs.PC != want {
		t.Errorf("PC = %#x, want %#x", em.Regs.PC, want)
	}
}

// assertFlags asserts NZCV flags.
func assertFlags(t *testing.T, em *Emulator, n, z, c, v bool) {
	t.Helper()
	if em.Regs.N != n || em.Regs.Z != z || em.Regs.C != c || em.Regs.V != v {
		t.Errorf("flags N=%v Z=%v C=%v V=%v, want N=%v Z=%v C=%v V=%v",
			em.Regs.N, em.Regs.Z, em.Regs.C, em.Regs.V, n, z, c, v)
	}
}

// verifyDecode checks that a uint32 decodes to the expected opcode.
func verifyDecode(t *testing.T, enc uint32, wantOp arm64asm.Op) {
	t.Helper()
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], enc)
	inst, err := arm64asm.Decode(buf[:])
	if err != nil {
		t.Fatalf("decode %#08x: %v", enc, err)
	}
	if inst.Op != wantOp {
		t.Fatalf("decode %#08x: got %s, want %s", enc, inst.Op, wantOp)
	}
}
