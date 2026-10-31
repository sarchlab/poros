package elfsim

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"math"
	"os"
	"testing"
	"time"
)

func TestGuestMath(t *testing.T) {
	input, err := os.ReadFile("../../tests/benchmarks/support/math.elf")
	if err != nil {
		t.Fatal(err)
	}
	// Generated independently with scripts/math-oracle.py at 100 and 200 decimal digits.
	// No host math library participates in the expected results.
	cases := []struct {
		input uint64
		want  [3]uint64 // log, exp, sqrt
	}{
		{0xbff0000000000000, [3]uint64{0x7ff8000000000000, 0x3fd78b56362cef38, 0x7ff8000000000000}},
		{0x8000000000000000, [3]uint64{0xfff0000000000000, 0x3ff0000000000000, 0x8000000000000000}},
		{0x0000000000000000, [3]uint64{0xfff0000000000000, 0x3ff0000000000000, 0x0000000000000000}},
		{0x0000000000000001, [3]uint64{0xc0874385446d71c3, 0x3ff0000000000000, 0x1e60000000000000}},
		{0x0010000000000000, [3]uint64{0xc086232bdd7abcd2, 0x3ff0000000000000, 0x2000000000000000}},
		{0x3fe0000000000000, [3]uint64{0xbfe62e42fefa39ef, 0x3ffa61298e1e069c, 0x3fe6a09e667f3bcd}},
		{0x3ff0000000000000, [3]uint64{0x0000000000000000, 0x4005bf0a8b145769, 0x3ff0000000000000}},
		{0x3ff0000000000001, [3]uint64{0x3cafffffffffffff, 0x4005bf0a8b14576b, 0x3ff0000000000000}},
		{0x4000000000000000, [3]uint64{0x3fe62e42fefa39ef, 0x401d8e64b8d4ddae, 0x3ff6a09e667f3bcd}},
		{0x4085e00000000000, [3]uint64{0x401a344e6741ebab, 0x7f0d945df4f8ec8e, 0x403a751f9447b724}},
		{0xc087480000000000, [3]uint64{0x7ff8000000000000, 0x0000000000000001, 0x7ff8000000000000}},
		{0x40862e42fefa39ef, [3]uint64{0x401a428494faf1b1, 0x7fefffffffffff2a, 0x403aa4499161cd47}},
		{0x4086300000000000, [3]uint64{0x401a42d4d1ff8154, 0x7ff0000000000000, 0x403aa554ccb17776}},
		{0x7ff0000000000000, [3]uint64{0x7ff0000000000000, 0x7ff0000000000000, 0x7ff0000000000000}},
		{0x7ff8000000001234, [3]uint64{0x7ff8000000000000, 0x7ff8000000000000, 0x7ff8000000000000}},
	}
	for _, mode := range []string{"functional", "inorder", "ooo"} {
		t.Run(mode, func(t *testing.T) {
			p, err := linuxuser.Parse(input, linuxuser.Options{Args: []string{"math"}})
			if err != nil {
				t.Fatal(err)
			}
			limits := linuxuser.RunOptions{MaxInstructions: 100000, Timeout: 5 * time.Second}
			var r linuxuser.Result
			if mode == "functional" {
				r, err = p.Run(context.Background(), limits)
			} else {
				var result Result
				result, err = Run(context.Background(), p, Options{Model: mode, Limits: limits})
				r = result.Result
			}
			if err != nil || r.Termination != "exit" || r.ExitStatus == nil || *r.ExitStatus != 0 || len(r.Stdout) != len(cases)*24 {
				t.Fatalf("guest math incomplete: %v %+v", err, r)
			}
			for i, c := range cases {
				for j, want := range c.want {
					got := math.Float64frombits(binary.LittleEndian.Uint64(r.Stdout[(i*3+j)*8:]))
					if err := checkGuestMathValue(c.input, j, got, want); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func checkGuestMathValue(input uint64, operation int, got float64, wantBits uint64) error {
	want := math.Float64frombits(wantBits)
	gotBits := math.Float64bits(got)
	mismatch := func() error {
		return fmt.Errorf("input %#x op %d got %#x want %#x", input, operation, gotBits, wantBits)
	}
	switch {
	case math.IsNaN(want):
		if !math.IsNaN(got) {
			return mismatch()
		}
	case math.IsInf(want, 0) || math.Abs(want) < 0x1p-1022:
		// Includes signed zero and every subnormal: flushing must not pass.
		if gotBits != wantBits {
			return mismatch()
		}
	default:
		if math.IsNaN(got) || math.IsInf(got, 0) || math.Signbit(got) != math.Signbit(want) {
			return mismatch()
		}
		distance := gotBits - wantBits
		if gotBits < wantBits {
			distance = wantBits - gotBits
		}
		if distance > 2 {
			return mismatch()
		}
	}
	return nil
}

func TestGuestMathRejectsIncorrectEdges(t *testing.T) {
	for _, tc := range []struct {
		input     uint64
		operation int
		got       float64
		want      uint64
	}{
		{0xc087480000000000, 1, 0, 1},                   // exp(-745) must round to the minimum subnormal.
		{1, 0, -709.08956571282408, 0xc0874385446d71c3}, // log(2^-1074) = -1074 ln(2).
		{0x8000000000000000, 2, 0, 0x8000000000000000},  // sqrt(-0) preserves its sign.
	} {
		if checkGuestMathValue(tc.input, tc.operation, tc.got, tc.want) == nil {
			t.Fatal("accepted incorrect edge", tc)
		}
		if err := checkGuestMathValue(tc.input, tc.operation, math.Float64frombits(tc.want), tc.want); err != nil {
			t.Fatal(err)
		}
	}
}
