package elfsim

import (
	"context"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/sarchlab/poros/pkg/linuxuser"
)

// On M5 the one-cycle store-to-load path follows the address operand: the
// same-address increment through a second base register pays the plain
// forward. Chains whose value changes at every update, or never, keep the
// path.
func TestM5RecurrenceProbesBypassByAddressOperand(t *testing.T) {
	image, err := os.ReadFile("../../evaluation/microbenchmarks/recurrence/recurrence.elf")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../evaluation/microbenchmarks/recurrence/cases.def")
	if err != nil {
		t.Fatal(err)
	}
	definition := string(raw)
	for name, want := range map[string]float64{
		"mem64_add":       2, // one base register
		"mem64_add_disp":  2, // one base register, displacement 8
		"mem64_add_alias": 7, // a second base register holding the same address
		"mem32_max_chg":   2, // the stored value changes at one address
	} {
		if got := recurrenceCyclesPerUpdateOn(t, "m5-max", image, definition, name); math.Abs(got-want) > 0.01 {
			t.Errorf("%s costs %.4f cycles per update on M5, want %.2f", name, got, want)
		}
	}
	if got := recurrenceCyclesPerUpdateOn(t, "m2", image, definition, "mem64_add_alias"); math.Abs(got-2) > 0.01 {
		t.Errorf("mem64_add_alias costs %.4f cycles per update on M2, want 2.00", got)
	}
	// The kernel replica raises its maximum at every trip, or never: neither
	// reload builds a run of equal values and then breaks it.
	constant := recurrenceCyclesPerUpdateOn(t, "m5-max", image, definition, "nuss_const")
	if got := recurrenceCyclesPerUpdateOn(t, "m5-max", image, definition, "nuss_chg"); got != constant || got > 2.05 {
		t.Errorf("nuss_chg costs %.4f cycles per trip on M5, nuss_const %.4f", got, constant)
	}
}

// The Nussinov kernel reloads a destination's running maximum at every inner
// iteration. The maximum stays put for long runs and then changes, within a
// destination or at the next one, so on M5 the reload loses the one-cycle
// path: seven to eight cycles per inner iteration at this size against about
// three with the path. M2 keeps its timing.
func TestM5NussinovReloadLosesTheBypass(t *testing.T) {
	image, err := os.ReadFile("../../tests/benchmarks/nussinov.elf")
	if err != nil {
		t.Fatal(err)
	}
	const n, iterations = 96, 96 * 95 * 94 / 6
	perIteration := func(device string) float64 {
		process, err := linuxuser.Parse(image, linuxuser.Options{Args: []string{"program", strconv.Itoa(n)}, ExecName: "program"})
		if err != nil {
			t.Fatal(err)
		}
		result, err := Run(context.Background(), process, Options{Target: device, Limits: linuxuser.RunOptions{MaxInstructions: 4000000, MaxOutput: 4 * n * n, Timeout: 60 * time.Second}})
		if err != nil || result.ExitStatus == nil || *result.ExitStatus != 0 || result.Timing.Retired != result.Instructions {
			t.Fatalf("nussinov on %s: %v %+v", device, err, result.Result)
		}
		return float64(result.Timing.Cycles) / iterations
	}
	if got := perIteration("m5-max"); got < 7 || got > 8 {
		t.Errorf("M5 takes %.2f cycles per inner iteration, want 7 to 8", got)
	}
	if got := perIteration("m2"); got < 3 || got > 3.3 {
		t.Errorf("M2 takes %.2f cycles per inner iteration, want 3.0 to 3.3", got)
	}
}
