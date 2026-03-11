package emu

import (
	"math"
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

// ── FCCMP tests ─────────────────────────────────────────────────────────

func TestFCCMP_D_CondTrue(t *testing.T) {
	// Set up: CMP X1, X1 (sets Z=1 => EQ true), then FCCMP D1, D2, #0, EQ
	// CMP X1, X1 => SUBS XZR, X1, X1 => 0xEB01003F
	// FCCMP D1, D2, #0, EQ => 0x1E620420
	em := newTestEmulator(0xEB01003F, 0x1E620420)
	em.Regs.WriteX(1, 42)
	em.Regs.WriteD(1, 3.0)
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 2)
	// Condition EQ is true (X1==X1), so FCCMP compares D1(3.0) vs D2(5.0)
	// 3.0 < 5.0 => N=1 Z=0 C=0 V=0
	assertFlags(t, em, true, false, false, false)
}

func TestFCCMP_D_CondFalse(t *testing.T) {
	// Set up: CMP X1, X2 (X1!=X2, Z=0 => EQ false), then FCCMP D1, D2, #0x5, EQ
	// CMP X1, X2 => SUBS XZR, X1, X2 => 0xEB02003F
	// FCCMP D1, D2, #5, EQ => 0x1E620425
	em := newTestEmulator(0xEB02003F, 0x1E620425)
	em.Regs.WriteX(1, 10)
	em.Regs.WriteX(2, 20)
	em.Regs.WriteD(1, 3.0)
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 2)
	// Condition EQ is false (X1!=X2), so NZCV = nzcv immediate = 5 = 0b0101
	// N=0, Z=1, C=0, V=1
	assertFlags(t, em, false, true, false, true)
}

func TestFCCMP_D_CondTrue_Equal(t *testing.T) {
	// FCCMP with condition true, and D1==D2
	// CMP X1, X1 => 0xEB01003F  (sets EQ=true)
	// FCCMP D1, D2, #0, EQ => 0x1E620420
	em := newTestEmulator(0xEB01003F, 0x1E620420)
	em.Regs.WriteX(1, 1)
	em.Regs.WriteD(1, 7.0)
	em.Regs.WriteD(2, 7.0)
	stepN(t, em, 2)
	// Equal: N=0 Z=1 C=1 V=0
	assertFlags(t, em, false, true, true, false)
}

func TestFCCMP_S_CondTrue(t *testing.T) {
	// CMP X1, X1 => 0xEB01003F
	// FCCMP S1, S2, #0, EQ => 0x1E220420
	em := newTestEmulator(0xEB01003F, 0x1E220420)
	em.Regs.WriteX(1, 1)
	em.Regs.WriteS(1, 10.0)
	em.Regs.WriteS(2, 5.0)
	stepN(t, em, 2)
	// 10.0 > 5.0: N=0 Z=0 C=1 V=0
	assertFlags(t, em, false, false, true, false)
}

func TestFCCMP_D_CondFalse_NZCVImm(t *testing.T) {
	// Test that nzcv=0xF sets all flags
	// CMP X1, X2 => 0xEB02003F (NE, so EQ is false)
	// FCCMP D0, D3, #0xF, EQ => 0x1E63042F ... let me compute
	// FCCMP encoding: 00011110 01 1 Rm(5) cond(4) 01 Rn(5) 0 nzcv(4)
	// Rm=D3=00011, cond=EQ=0000, Rn=D0=00000, nzcv=1111
	// 00011110 01100011 00000100 00001111
	// = 1E 63 04 0F
	verifyDecode(t, 0x1E63040F, arm64asm.FCCMP)
	em := newTestEmulator(0xEB02003F, 0x1E63040F)
	em.Regs.WriteX(1, 1)
	em.Regs.WriteX(2, 2)
	stepN(t, em, 2)
	// EQ is false => flags = 0xF = N=1 Z=1 C=1 V=1
	assertFlags(t, em, true, true, true, true)
}

// ── FMAXNM tests ────────────────────────────────────────────────────────

func TestFMAXNM_D_Normal(t *testing.T) {
	// FMAXNM D0, D1, D2 => 0x1E626820
	enc := uint32(0x1E626820)
	verifyDecode(t, enc, arm64asm.FMAXNM)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 3.0)
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 5.0 {
		t.Errorf("D0 = %f, want 5.0", got)
	}
}

func TestFMAXNM_D_NaN_A(t *testing.T) {
	enc := uint32(0x1E626820)
	em := newTestEmulator(enc)
	em.Regs.WriteD(1, math.NaN())
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 1)
	// NaN vs number: return number
	if got := em.Regs.ReadD(0); got != 5.0 {
		t.Errorf("D0 = %f, want 5.0", got)
	}
}

func TestFMAXNM_D_NaN_B(t *testing.T) {
	enc := uint32(0x1E626820)
	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 3.0)
	em.Regs.WriteD(2, math.NaN())
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 3.0 {
		t.Errorf("D0 = %f, want 3.0", got)
	}
}

func TestFMAXNM_D_BothNaN(t *testing.T) {
	enc := uint32(0x1E626820)
	em := newTestEmulator(enc)
	em.Regs.WriteD(1, math.NaN())
	em.Regs.WriteD(2, math.NaN())
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); !math.IsNaN(got) {
		t.Errorf("D0 = %f, want NaN", got)
	}
}

func TestFMAXNM_S_Normal(t *testing.T) {
	// FMAXNM S0, S1, S2 => 0x1E226820
	enc := uint32(0x1E226820)
	verifyDecode(t, enc, arm64asm.FMAXNM)

	em := newTestEmulator(enc)
	em.Regs.WriteS(1, 3.0)
	em.Regs.WriteS(2, 5.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadS(0); got != 5.0 {
		t.Errorf("S0 = %f, want 5.0", got)
	}
}

func TestFMAXNM_S_NaN(t *testing.T) {
	enc := uint32(0x1E226820)
	em := newTestEmulator(enc)
	em.Regs.WriteS(1, float32(math.NaN()))
	em.Regs.WriteS(2, 7.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadS(0); got != 7.0 {
		t.Errorf("S0 = %f, want 7.0", got)
	}
}

// ── FMINNM tests ────────────────────────────────────────────────────────

func TestFMINNM_D_Normal(t *testing.T) {
	// FMINNM D0, D1, D2 => 0x1E627820
	enc := uint32(0x1E627820)
	verifyDecode(t, enc, arm64asm.FMINNM)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 3.0)
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 3.0 {
		t.Errorf("D0 = %f, want 3.0", got)
	}
}

func TestFMINNM_D_NaN_A(t *testing.T) {
	enc := uint32(0x1E627820)
	em := newTestEmulator(enc)
	em.Regs.WriteD(1, math.NaN())
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 5.0 {
		t.Errorf("D0 = %f, want 5.0", got)
	}
}

func TestFMINNM_D_NaN_B(t *testing.T) {
	enc := uint32(0x1E627820)
	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 3.0)
	em.Regs.WriteD(2, math.NaN())
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 3.0 {
		t.Errorf("D0 = %f, want 3.0", got)
	}
}

func TestFMINNM_D_BothNaN(t *testing.T) {
	enc := uint32(0x1E627820)
	em := newTestEmulator(enc)
	em.Regs.WriteD(1, math.NaN())
	em.Regs.WriteD(2, math.NaN())
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); !math.IsNaN(got) {
		t.Errorf("D0 = %f, want NaN", got)
	}
}

func TestFMINNM_S_Normal(t *testing.T) {
	// FMINNM S0, S1, S2 => 0x1E227820
	enc := uint32(0x1E227820)
	verifyDecode(t, enc, arm64asm.FMINNM)

	em := newTestEmulator(enc)
	em.Regs.WriteS(1, 3.0)
	em.Regs.WriteS(2, 5.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadS(0); got != 3.0 {
		t.Errorf("S0 = %f, want 3.0", got)
	}
}

func TestFMINNM_S_NaN(t *testing.T) {
	enc := uint32(0x1E227820)
	em := newTestEmulator(enc)
	em.Regs.WriteS(1, float32(math.NaN()))
	em.Regs.WriteS(2, 7.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadS(0); got != 7.0 {
		t.Errorf("S0 = %f, want 7.0", got)
	}
}
