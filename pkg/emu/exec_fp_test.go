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

func TestFCVT_S_to_D(t *testing.T) {
	// FCVT D0, S1 => 0x1E22C020
	enc := uint32(0x1E22C020)
	verifyDecode(t, enc, arm64asm.FCVT)

	em := newTestEmulator(enc)
	em.Regs.WriteS(1, 3.5)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != 3.5 {
		t.Errorf("D0 = %f, want 3.5", got)
	}
}

func TestFCVT_D_to_S(t *testing.T) {
	// FCVT S0, D1 => 0x1E624020
	enc := uint32(0x1E624020)
	verifyDecode(t, enc, arm64asm.FCVT)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 2.25)
	stepN(t, em, 1)
	if got := em.Regs.ReadS(0); got != 2.25 {
		t.Errorf("S0 = %f, want 2.25", got)
	}
}

func TestSXTL_2D_2S(t *testing.T) {
	// SXTL V0.2D, V1.2S => 0x0F20A420
	enc := uint32(0x0F20A420)
	verifyDecode(t, enc, arm64asm.SXTL)

	em := newTestEmulator(enc)
	// Pack two int32 values into V1: lane0=-5 (0xFFFFFFFB), lane1=42 (0x2A)
	var neg5 int32 = -5
	var pos42 int32 = 42
	lane0 := uint64(uint32(neg5))  // 0xFFFFFFFB
	lane1 := uint64(uint32(pos42)) // 0x0000002A
	em.Regs.WriteQ(1, lane0|(lane1<<32), 0)
	stepN(t, em, 1)

	lo, hi := em.Regs.ReadQ(0)
	// Lane 0: int64(-5) = 0xFFFFFFFFFFFFFFFB
	wantLo := uint64(int64(neg5))
	if lo != wantLo {
		t.Errorf("V0 lane0 = %#x, want %#x", lo, wantLo)
	}
	// Lane 1: int64(42) = 0x000000000000002A
	wantHi := uint64(int64(pos42))
	if hi != wantHi {
		t.Errorf("V0 lane1 = %#x, want %#x", hi, wantHi)
	}
}

func TestFNMUL_D(t *testing.T) {
	// FNMUL D0, D1, D2 => 0x1E628820
	enc := uint32(0x1E628820)
	verifyDecode(t, enc, arm64asm.FNMUL)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 3.0)
	em.Regs.WriteD(2, 5.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadD(0); got != -15.0 {
		t.Errorf("D0 = %f, want -15.0", got)
	}
}

func TestFNMUL_S(t *testing.T) {
	// FNMUL S0, S1, S2 => 0x1E228820
	enc := uint32(0x1E228820)
	verifyDecode(t, enc, arm64asm.FNMUL)

	em := newTestEmulator(enc)
	em.Regs.WriteS(1, 2.0)
	em.Regs.WriteS(2, 4.0)
	stepN(t, em, 1)
	if got := em.Regs.ReadS(0); got != -8.0 {
		t.Errorf("S0 = %f, want -8.0", got)
	}
}

func TestFNMADD_D(t *testing.T) {
	// FNMADD D0, D1, D2, D3 => 0x1F620C20
	enc := uint32(0x1F620C20)
	verifyDecode(t, enc, arm64asm.FNMADD)

	em := newTestEmulator(enc)
	em.Regs.WriteD(1, 2.0)
	em.Regs.WriteD(2, 3.0)
	em.Regs.WriteD(3, 10.0)
	stepN(t, em, 1)
	// FNMADD: -(Rn*Rm + Ra) = -(2*3 + 10) = -16
	if got := em.Regs.ReadD(0); got != -16.0 {
		t.Errorf("D0 = %f, want -16.0", got)
	}
}

func TestMOVI_2D_Zero(t *testing.T) {
	// MOVI V0.2D, #0 => 0x6F00E400
	enc := uint32(0x6F00E400)
	verifyDecode(t, enc, arm64asm.MOVI)

	em := newTestEmulator(enc)
	// Pre-fill V0 with non-zero data
	em.Regs.WriteQ(0, 0xDEADBEEF, 0xCAFEBABE)
	stepN(t, em, 1)

	lo, hi := em.Regs.ReadQ(0)
	if lo != 0 || hi != 0 {
		t.Errorf("V0 = (%#x, %#x), want (0, 0)", lo, hi)
	}
}

// Exact expected bits derive from binary arithmetic, not a second math.FMA call.
// Native M2 FMADD/FMSUB/FNMADD instructions also reproduce these results.
func TestFusedDoubleRounding(t *testing.T) {
	cases := []struct {
		name          string
		n, m, a       float64
		add, sub, neg uint64
	}{
		// (1+2^-27)*(1-2^-27) = 1-2^-54 exactly; separate rounding loses the residual.
		{"cancellation", 0x1.0000002p0, 0x1.ffffffcp-1, -1, 0xbc90000000000000, 0x3c90000000000000, 0x3c90000000000000},
		// The exact product overflows binary64, but the fused sum is +/-MaxFloat64.
		{"intermediate-overflow", math.MaxFloat64, 2, -math.MaxFloat64, 0x7fefffffffffffff, 0xffefffffffffffff, 0xffefffffffffffff},
		{"exact-zero", 1, 1, -1, 0, 0, 0},
		{"signed-zero", 0, 1, 0, 0, 0x8000000000000000, 0x8000000000000000},
	}
	for _, c := range cases {
		for _, op := range []struct {
			name string
			enc  uint32
			a    float64
			want uint64
		}{
			{"FMADD", 0x1f420c20, c.a, c.add},
			{"FMSUB", 0x1f428c20, -c.a, c.sub},
			{"FNMADD", 0x1f620c20, c.a, c.neg},
			{"FNMSUB", 0x1f628c20, -c.a, c.add},
		} {
			t.Run(c.name+"/"+op.name, func(t *testing.T) {
				em := newTestEmulator(op.enc)
				em.Regs.WriteD(1, c.n)
				em.Regs.WriteD(2, c.m)
				em.Regs.WriteD(3, op.a)
				stepN(t, em, 1)
				if got := math.Float64bits(em.Regs.ReadD(0)); got != op.want {
					t.Fatalf("got %016x, want %016x", got, op.want)
				}
			})
		}
	}
}

func TestFNMSUBSingleAndSignedZero(t *testing.T) {
	for _, tc := range []struct {
		enc    uint32
		single bool
	}{{0x1f228c20, true}, {0x1f628c20, false}} {
		verifyDecode(t, tc.enc, arm64asm.FNMSUB)
		em := newTestEmulator(tc.enc)
		if tc.single {
			em.Regs.WriteS(1, 2)
			em.Regs.WriteS(2, 3)
			em.Regs.WriteS(3, 4)
		} else {
			em.Regs.WriteD(1, 2)
			em.Regs.WriteD(2, 3)
			em.Regs.WriteD(3, 4)
		}
		stepN(t, em, 1)
		if tc.single && em.Regs.ReadS(0) != 2 || !tc.single && em.Regs.ReadD(0) != 2 {
			t.Fatal("FNMSUB must compute product minus accumulator")
		}
	}
	em := newTestEmulator(0x1f628c20)
	em.Regs.WriteD(1, math.Copysign(0, -1))
	em.Regs.WriteD(2, 1)
	em.Regs.WriteD(3, 0)
	stepN(t, em, 1)
	if got := math.Float64bits(em.Regs.ReadD(0)); got != 0x8000000000000000 {
		t.Fatalf("signed zero: %x", got)
	}
}

func TestFNMSUBSingleAvoidsDoubleRounding(t *testing.T) {
	em := newTestEmulator(0x1f228c20)
	// (1+2^-12)^2 is exactly halfway between adjacent binary32 values;
	// subtracting -2^-100 nudges it above the tie, which binary64 rounding loses.
	em.Regs.WriteS(1, 0x1.001p0)
	em.Regs.WriteS(2, 0x1.001p0)
	em.Regs.WriteS(3, -0x1p-100)
	stepN(t, em, 1)
	if got := math.Float32bits(em.Regs.ReadS(0)); got != 0x3f801001 {
		t.Fatalf("double rounded %08x", got)
	}
}

func TestFNMSUBSingleSpecialValues(t *testing.T) {
	for _, tc := range []struct {
		name    string
		n, m, a float32
		bits    uint32
		nan     bool
	}{
		{"negative zero", math.Float32frombits(0x80000000), 1, 0, 0x80000000, false},
		{"positive zero", 0, 1, math.Float32frombits(0x80000000), 0, false},
		{"infinity", float32(math.Inf(1)), 2, 1, 0x7f800000, false},
		{"infinity cancellation", float32(math.Inf(1)), 1, float32(math.Inf(1)), 0, true},
		{"zero times infinity", 0, float32(math.Inf(1)), 1, 0, true},
		{"nan", float32(math.NaN()), 1, 1, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			em := newTestEmulator(0x1f228c20)
			em.Regs.WriteS(1, tc.n)
			em.Regs.WriteS(2, tc.m)
			em.Regs.WriteS(3, tc.a)
			stepN(t, em, 1)
			got := em.Regs.ReadS(0)
			if tc.nan {
				if !math.IsNaN(float64(got)) {
					t.Fatalf("expected NaN, got %g", got)
				}
			} else if math.Float32bits(got) != tc.bits {
				t.Fatalf("got %08x want %08x", math.Float32bits(got), tc.bits)
			}
		})
	}
}
