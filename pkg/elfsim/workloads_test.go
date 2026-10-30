package elfsim

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/sarchlab/poros/pkg/linuxuser"
)

// Large sizes are mandatory in the 24-case accuracy job. This short regression
// exercises complete MVT ELF startup/output in all three execution modes.
func TestMVTCompleteELF(t *testing.T) {
	input, err := os.ReadFile("../../tests/usertime/mvt.elf")
	if err != nil {
		t.Fatal(err)
	}
	const n = 8
	var reference linuxuser.Result
	for _, mode := range []string{"functional", "inorder", "ooo"} {
		t.Run(mode, func(t *testing.T) {
			p, err := linuxuser.Parse(input, linuxuser.Options{Args: []string{"program", "8"}})
			if err != nil {
				t.Fatal(err)
			}
			limits := linuxuser.RunOptions{MaxInstructions: 100000, Timeout: 5 * time.Second}
			var result linuxuser.Result
			if mode == "functional" {
				result, err = p.Run(context.Background(), limits)
			} else {
				var r Result
				r, err = Run(context.Background(), p, Options{Model: mode, Limits: limits})
				result = r.Result
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Termination != "exit" || result.ExitStatus == nil || *result.ExitStatus != 0 || len(result.Stdout) != 16*n || len(result.Stderr) != 0 {
				t.Fatalf("incomplete result %+v", result)
			}
			for half := 0; half < 2; half++ {
				for i := 0; i < n; i++ {
					numerator := (i + half) % n * n
					for j := 0; j < n; j++ {
						numerator += (i * j) % n * ((j + 3 + half) % n)
					}
					want := float64(numerator) / (n * n)
					got := math.Float64frombits(binary.LittleEndian.Uint64(result.Stdout[8*(half*n+i):]))
					if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > 1e-11+1e-12*math.Abs(want) {
						t.Fatalf("output[%d]=%g want %g", half*n+i, got, want)
					}
				}
			}
			if mode == "functional" {
				reference = result
			} else if !reflect.DeepEqual(reference, result) {
				t.Fatal("architectural result differs between modes")
			}
		})
	}
}

func TestWorkloadInputBounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"mvt", nil}, {"mvt", []string{"0"}}, {"mvt", []string{"1537"}}, {"mvt", []string{"18446744073709551616"}}, {"mvt", []string{"-1"}}, {"mvt", []string{""}},
		{"integer", []string{"10000001"}}, {"branch", []string{"1x"}},
		{"memory", []string{"10"}}, {"memory", []string{"10", "7"}}, {"memory", []string{"10", "24"}}, {"memory", []string{"10", "67108864"}},
	} {
		t.Run(tc.name+"/"+stringArgs(tc.args), func(t *testing.T) {
			input, err := os.ReadFile("../../tests/usertime/" + tc.name + ".elf")
			if err != nil {
				t.Fatal(err)
			}
			p, err := linuxuser.Parse(input, linuxuser.Options{Args: append([]string{"program"}, tc.args...)})
			if err != nil {
				t.Fatal(err)
			}
			r, err := p.Run(context.Background(), linuxuser.RunOptions{MaxInstructions: 10000, Timeout: 5 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			if r.Termination != "exit" || r.ExitStatus == nil || *r.ExitStatus != 2 || len(r.Stdout) != 0 {
				t.Fatalf("invalid input accepted: %+v", r)
			}
		})
	}
}
func stringArgs(args []string) string {
	out := ""
	for _, s := range args {
		out += s + "_"
	}
	return out
}
