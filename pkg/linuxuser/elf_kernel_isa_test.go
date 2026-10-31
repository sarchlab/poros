package linuxuser

import (
	"github.com/sarchlab/poros/pkg/emu"
	"math"
	"testing"
)

func TestCheckedScalarIntegerBitsConversion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		op     uint32
		bits   uint64
		want   float64
		single bool
		dst    int
	}{
		{"unsigned64", 0x7e61d820, ^uint64(0), float64(^uint64(0)), false, 0},
		{"signed64", 0x5e61d820, 1 << 63, float64(-9223372036854775808.0), false, 0},
		{"unsigned32", 0x7e21d820, 0xffffffff, float64(float32(uint32(0xffffffff))), true, 0},
		{"signed32", 0x5e21d820, 0x80000000, float64(-2147483648), true, 0},
		{"alias-rounding", 0x7e61d821, 1<<53 | 1, float64(uint64(1<<53 | 1)), false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := emu.Registers{PC: 0x1000}
			r.WriteQ(1, tc.bits, ^uint64(0))
			r.WriteQ(0, 17, ^uint64(0))
			if _, err := emu.StepChecked(&r, instructionMemory(tc.op)); err != nil {
				t.Fatal(err)
			}
			got := r.ReadD(tc.dst)
			if tc.single {
				got = float64(r.ReadS(tc.dst))
			}
			if got != tc.want {
				t.Fatalf("got %g want %g", got, tc.want)
			}
			_, hi := r.ReadQ(tc.dst)
			if hi != 0 {
				t.Fatal("scalar destination did not clear upper bits")
			}
		})
	}
}
func TestCheckedExtractAndNegate(t *testing.T) {
	for _, tc := range []struct {
		op          uint32
		input, want uint64
	}{
		{0xd36dcc20, ^uint64(0), 127}, {0x936dcc20, ^uint64(0), ^uint64(0)},
		{0x53052c20, 0xffffffe0, 127}, {0x13052c20, 0xffffffe0, 0xffffffff},
	} {
		r := emu.Registers{PC: 0x1000}
		r.WriteX(1, tc.input)
		if _, err := emu.StepChecked(&r, instructionMemory(tc.op)); err != nil {
			t.Fatal(err)
		}
		if r.ReadX(0) != tc.want {
			t.Fatalf("%x got%x want%x", tc.op, r.ReadX(0), tc.want)
		}
	}
	for _, tc := range []struct {
		op          uint32
		input, want uint64
		n, z, c, v  bool
	}{
		{0x6b0103e0, 0, 0, false, true, true, false},
		{0x6b0103e0, 1, 0xffffffff, true, false, false, false},
		{0x6b0103e0, 0x80000000, 0x80000000, true, false, false, true},
		{0xeb0103e0, 1 << 63, 1 << 63, true, false, false, true},
	} {
		r := emu.Registers{PC: 0x1000}
		r.WriteX(1, tc.input)
		if _, err := emu.StepChecked(&r, instructionMemory(tc.op)); err != nil {
			t.Fatal(err)
		}
		if r.ReadX(0) != tc.want || r.N != tc.n || r.Z != tc.z || r.C != tc.c || r.V != tc.v {
			t.Fatalf("NEGS %x: %+v", tc.input, r)
		}
	}
}
func TestCheckedVectorSignBits(t *testing.T) {
	r := emu.Registers{PC: 0x1000}
	r.WriteQ(0, ^uint64(0), ^uint64(0))
	if _, err := emu.StepChecked(&r, instructionMemory(0x4f000400)); err != nil {
		t.Fatal(err)
	}
	if lo, hi := r.ReadQ(0); lo != 0 || hi != 0 {
		t.Fatal("MOVI zero")
	}
	// FNEG V0.2D,V1.2D must preserve payloads and toggle both signs.
	r.PC = 0x1000
	r.WriteQ(1, 0x7ff8000000000123, 1<<63)
	if _, err := emu.StepChecked(&r, instructionMemory(0x6ee0f820)); err != nil {
		t.Fatal(err)
	}
	if lo, hi := r.ReadQ(0); lo != 0xfff8000000000123 || hi != 0 {
		t.Fatalf("FNEG bits %x %x", lo, hi)
	}
	r.PC = 0x1000
	r.WriteQ(0, 0x55, 0x8000)
	r.WriteQ(1, 0xaa, 0x40)
	// ORR V0.16B,V0.16B,V1.16B exercises destination/source aliasing.
	if _, err := emu.StepChecked(&r, instructionMemory(0x4ea11c00)); err != nil {
		t.Fatal(err)
	}
	if lo, hi := r.ReadQ(0); lo != 0xff || hi != 0x8040 {
		t.Fatalf("ORR bits %x %x", lo, hi)
	}
	r.PC = 0x1000
	before := r
	if _, err := emu.StepChecked(&r, instructionMemory(0x4f000420)); err == nil || r != before {
		t.Fatal("nonzero vector MOVI must still reject atomically")
	}
}
func TestCheckedFCMPE(t *testing.T) {
	for _, tc := range []struct {
		a, b       float64
		n, z, c, v bool
	}{{1, 2, true, false, false, false}, {2, 1, false, false, true, false}, {1, 1, false, true, true, false}, {math.NaN(), 1, false, false, true, true}} {
		r := emu.Registers{PC: 0x1000}
		r.WriteD(0, tc.a)
		r.WriteD(1, tc.b)
		before := r.FP
		if _, err := emu.StepChecked(&r, instructionMemory(0x1e612010)); err != nil {
			t.Fatal(err)
		}
		if r.N != tc.n || r.Z != tc.z || r.C != tc.c || r.V != tc.v || r.FP != before {
			t.Fatal("FCMPE flags/register preservation")
		}
	}
}
