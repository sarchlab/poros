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
	bits := []uint64{0xbff0000000000000, 0x8000000000000000, 0, 1, 0x0010000000000000, 0x3fe0000000000000, 0x3ff0000000000000, 0x3ff0000000000001, 0x4000000000000000, 0x4085e00000000000, 0xc087480000000000, 0x40862e42fefa39ef, 0x4086300000000000, 0x7ff0000000000000, 0x7ff8000000001234}
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
			if err != nil || r.Termination != "exit" || r.ExitStatus == nil || *r.ExitStatus != 0 || len(r.Stdout) != len(bits)*24 {
				t.Fatalf("guest math incomplete: %v %+v", err, r)
			}
			for i, b := range bits {
				v := math.Float64frombits(b)
				for j, fn := range []func(float64) float64{math.Log, math.Exp, math.Sqrt} {
					got := math.Float64frombits(binary.LittleEndian.Uint64(r.Stdout[(i*3+j)*8:]))
					if err := checkGuestMathValue(b, j, got, fn(v)); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func checkGuestMathValue(input uint64, operation int, got, want float64) error {
	if input == 0xc087480000000000 && operation == 1 {
		// exp(-745) = 2.8223507...e-324, above half the smallest subnormal.
		if math.Float64bits(got) != 1 {
			return fmt.Errorf("exp(-745): got bits%x want1", math.Float64bits(got))
		}
		return nil
	}
	switch {
	case math.IsNaN(want):
		if !math.IsNaN(got) {
			return fmt.Errorf("expected NaN")
		}
	case math.IsInf(want, 0) || want == 0:
		if math.Float64bits(got) != math.Float64bits(want) {
			return fmt.Errorf("signed zero/infinity mismatch")
		}
	default:
		if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > 4e-15*math.Abs(want)+2*math.SmallestNonzeroFloat64 {
			return fmt.Errorf("input%x op%d got%.17g want%.17g", input, operation, got, want)
		}
	}
	return nil
}
func TestGuestMathRejectsFlushedEdge(t *testing.T) {
	if checkGuestMathValue(0xc087480000000000, 1, 0, 0) == nil {
		t.Fatal("accepted flushed exp(-745)")
	}
	if err := checkGuestMathValue(0xc087480000000000, 1, math.SmallestNonzeroFloat64, 0); err != nil {
		t.Fatal(err)
	}
}
