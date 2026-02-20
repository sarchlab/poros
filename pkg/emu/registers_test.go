package emu

import (
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

func TestReadWriteX(t *testing.T) {
	r := NewRegisters()
	r.WriteX(0, 42)
	if got := r.ReadX(0); got != 42 {
		t.Errorf("ReadX(0) = %d, want 42", got)
	}
	r.WriteX(30, 0xDEADBEEF)
	if got := r.ReadX(30); got != 0xDEADBEEF {
		t.Errorf("ReadX(30) = %#x, want 0xDEADBEEF", got)
	}
}

func TestXZR(t *testing.T) {
	r := NewRegisters()
	r.WriteX(31, 999) // should be discarded
	if got := r.ReadX(31); got != 0 {
		t.Errorf("XZR should always read 0, got %d", got)
	}
}

func TestReadWriteW(t *testing.T) {
	r := NewRegisters()
	r.WriteW(5, 0xABCD1234)
	if got := r.ReadW(5); got != 0xABCD1234 {
		t.Errorf("ReadW(5) = %#x, want 0xABCD1234", got)
	}
	// W write should zero-extend to 64 bits
	if got := r.ReadX(5); got != 0xABCD1234 {
		t.Errorf("after WriteW, ReadX(5) = %#x, want 0xABCD1234 (zero-extended)", got)
	}
}

func TestWZR(t *testing.T) {
	r := NewRegisters()
	r.WriteW(31, 999)
	if got := r.ReadW(31); got != 0 {
		t.Errorf("WZR should always read 0, got %d", got)
	}
}

func TestReadWriteFP(t *testing.T) {
	r := NewRegisters()
	r.WriteD(0, 3.14)
	if got := r.ReadD(0); got != 3.14 {
		t.Errorf("ReadD(0) = %f, want 3.14", got)
	}
	r.WriteS(1, 2.5)
	if got := r.ReadS(1); got != 2.5 {
		t.Errorf("ReadS(1) = %f, want 2.5", got)
	}
}

func TestEvalCond(t *testing.T) {
	r := NewRegisters()

	// Test EQ (Z=1)
	r.Z = true
	eq := arm64asm.Cond{Value: 0, Invert: false} // EQ
	if !r.EvalCond(eq) {
		t.Error("EQ should be true when Z=1")
	}

	ne := arm64asm.Cond{Value: 1, Invert: false} // NE
	if r.EvalCond(ne) {
		t.Error("NE should be false when Z=1")
	}

	// Test GE (N==V)
	r.N = true
	r.V = true
	ge := arm64asm.Cond{Value: 10, Invert: false} // GE
	if !r.EvalCond(ge) {
		t.Error("GE should be true when N==V")
	}
}

func TestSPReadWrite(t *testing.T) {
	r := NewRegisters()
	r.SP = 0x7FFF0000
	if r.SP != 0x7FFF0000 {
		t.Errorf("SP = %#x, want 0x7FFF0000", r.SP)
	}
}
