package emu

import (
	"math"
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

func TestFADD_D(t *testing.T) {
	// FADD D0, D1, D2 => 0x1E622820
	enc := uint32(0x1E622820)
	verifyDecode(t, enc, arm64asm.FADD)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 3.0)
	em.Regs.WriteD(2, 4.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 7.0 {
		t.Errorf("D0 = %f, want 7.0", got)
	}
}

func TestFSUB_D(t *testing.T) {
	// FSUB D0, D1, D2 => 0x1E623820
	enc := uint32(0x1E623820)
	verifyDecode(t, enc, arm64asm.FSUB)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 10.0)
	em.Regs.WriteD(2, 3.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 7.0 {
		t.Errorf("D0 = %f, want 7.0", got)
	}
}

func TestFMUL_D(t *testing.T) {
	// FMUL D0, D1, D2 => 0x1E620820
	enc := uint32(0x1E620820)
	verifyDecode(t, enc, arm64asm.FMUL)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 3.0)
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 15.0 {
		t.Errorf("D0 = %f, want 15.0", got)
	}
}

func TestFDIV_D(t *testing.T) {
	// FDIV D0, D1, D2 => 0x1E621820
	enc := uint32(0x1E621820)
	verifyDecode(t, enc, arm64asm.FDIV)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 15.0)
	em.Regs.WriteD(2, 3.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 5.0 {
		t.Errorf("D0 = %f, want 5.0", got)
	}
}

func TestFSQRT_D(t *testing.T) {
	// FSQRT D0, D1 => 0x1E61C020
	enc := uint32(0x1E61C020)
	verifyDecode(t, enc, arm64asm.FSQRT)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 25.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 5.0 {
		t.Errorf("D0 = %f, want 5.0", got)
	}
}

func TestFNEG_D(t *testing.T) {
	// FNEG D0, D1 => 0x1E614020
	enc := uint32(0x1E614020)
	verifyDecode(t, enc, arm64asm.FNEG)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 3.14)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != -3.14 {
		t.Errorf("D0 = %f, want -3.14", got)
	}
}

func TestFABS_D(t *testing.T) {
	// FABS D0, D1 => 0x1E60C020
	enc := uint32(0x1E60C020)
	verifyDecode(t, enc, arm64asm.FABS)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, -42.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 42.0 {
		t.Errorf("D0 = %f, want 42.0", got)
	}
}

func TestFMADD_D(t *testing.T) {
	// FMADD D0, D1, D2, D3 => 0x1F420C20 (type=1 for double)
	enc := uint32(0x1F420C20)
	verifyDecode(t, enc, arm64asm.FMADD)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 2.0)
	em.Regs.WriteD(2, 3.0)
	em.Regs.WriteD(3, 10.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 16.0 { // 10 + 2*3 = 16
		t.Errorf("D0 = %f, want 16.0", got)
	}
}

func TestFMSUB_D(t *testing.T) {
	// FMSUB D0, D1, D2, D3 => 0x1F428C20 (type=1 for double)
	enc := uint32(0x1F428C20)
	verifyDecode(t, enc, arm64asm.FMSUB)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 2.0)
	em.Regs.WriteD(2, 3.0)
	em.Regs.WriteD(3, 10.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 4.0 { // 10 - 2*3 = 4
		t.Errorf("D0 = %f, want 4.0", got)
	}
}

func TestFCMP_D_EQ(t *testing.T) {
	// FCMP D1, D2 => 0x1E622020
	enc := uint32(0x1E622020)
	verifyDecode(t, enc, arm64asm.FCMP)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 5.0)
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 1)
	// Equal: N=0 Z=1 C=1 V=0
	assertFlags(t, em, false, true, true, false)
}

func TestFCMP_D_LT(t *testing.T) {
	enc := uint32(0x1E622020)
	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 3.0)
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 1)
	// a < b: N=1 Z=0 C=0 V=0
	assertFlags(t, em, true, false, false, false)
}

func TestFCMP_D_NaN(t *testing.T) {
	enc := uint32(0x1E622020)
	em := newTestEmulator(enc)
	em.Regs.WriteD(1, math.NaN())
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 1)
	// NaN: N=0 Z=0 C=1 V=1
	assertFlags(t, em, false, false, true, true)
}

func TestSCVTF_D_X(t *testing.T) {
	// SCVTF D0, X1 => 0x9E620020
	enc := uint32(0x9E620020)
	verifyDecode(t, enc, arm64asm.SCVTF)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, ^uint64(42)+1) // -42
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != -42.0 {
		t.Errorf("D0 = %f, want -42.0", got)
	}
}

func TestUCVTF_D_X(t *testing.T) {
	// UCVTF D0, X1 => 0x9E630020
	enc := uint32(0x9E630020)
	verifyDecode(t, enc, arm64asm.UCVTF)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, 100)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 100.0 {
		t.Errorf("D0 = %f, want 100.0", got)
	}
}

func TestFMOV_D_D(t *testing.T) {
	// FMOV D0, D1 => 0x1E604020
	enc := uint32(0x1E604020)
	verifyDecode(t, enc, arm64asm.FMOV)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 3.14)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 3.14 {
		t.Errorf("D0 = %f, want 3.14", got)
	}
}

func TestFMOV_D_X(t *testing.T) {
	// FMOV D0, X1 => 0x9E670020
	enc := uint32(0x9E670020)
	verifyDecode(t, enc, arm64asm.FMOV)

	em := newTestEmulator(enc)
	em.Regs.WriteX(1, math.Float64bits(2.5))
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 2.5 {
		t.Errorf("D0 = %f, want 2.5", got)
	}
}

func TestFCSEL_D(t *testing.T) {
	// CMP X1, X2            => 0xEB02003F
	// FCSEL D0, D3, D4, EQ  => 0x1E640C60
	em := newTestEmulator(0xEB02003F, 0x1E640C60)
	em.Regs.WriteX(1, 42)
	em.Regs.WriteX(2, 42)
	em.Regs.WriteD(3, 100.0)
	em.Regs.WriteD(4, 200.0)
	stepN(t, em, 2)
	if got := em.Regs.ReadD(0); got != 100.0 {
		t.Errorf("D0 = %f, want 100.0 (EQ taken)", got)
	}
}

func TestFADD_S(t *testing.T) {
	// FADD S0, S1, S2 => 0x1E222820
	enc := uint32(0x1E222820)
	verifyDecode(t, enc, arm64asm.FADD)

	em := newTestEmulator(enc)
	em.Regs.WriteS(1, 1.5)
	em.Regs.WriteS(2, 2.5)
	stepN(t, em, 1)
	if got := em.Regs.ReadS(0); got != 4.0 {
		t.Errorf("S0 = %f, want 4.0", got)
	}
}
