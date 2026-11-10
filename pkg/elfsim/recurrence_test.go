package elfsim

import (
	"context"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sarchlab/poros/pkg/linuxuser"
)

// recurrenceCyclesPerUpdate runs one committed recurrence probe at 16 and 32
// outer trips and returns the modelled cycles per reported update.
func recurrenceCyclesPerUpdate(t *testing.T, image []byte, definition, name string) float64 {
	t.Helper()
	index, updates := 0, 0.0
	for i, line := range strings.Split(strings.TrimSpace(definition), "\n") {
		fields := strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "PROBE("), ")"), ", ")
		if len(fields) == 4 && fields[0] == name {
			index = i + 1
			updates, _ = strconv.ParseFloat(fields[2], 64)
		}
	}
	if index == 0 || updates == 0 {
		t.Fatalf("probe %s is not declared", name)
	}
	var cycles [2]uint64
	for i, trips := range []string{"16", "32"} {
		process, err := linuxuser.Parse(image, linuxuser.Options{Args: []string{"program", strconv.Itoa(index), trips}, ExecName: "program"})
		if err != nil {
			t.Fatal(err)
		}
		result, err := Run(context.Background(), process, Options{Limits: linuxuser.RunOptions{MaxInstructions: 4000000, MaxOutput: 64, Timeout: 30 * time.Second}})
		if err != nil || result.ExitStatus == nil || *result.ExitStatus != 0 || result.Timing.Retired != result.Instructions {
			t.Fatalf("%s at %s trips: %v %+v", name, trips, err, result.Result)
		}
		cycles[i] = result.Timing.Cycles
	}
	return float64(cycles[1]-cycles[0]) / (16 * updates)
}

// The committed probes exercise fusion through the real loader, layout and
// pipeline: the measured forms cost one cycle per update only when adjacent
// in one 64-byte line, and a stream of pairs is held to one per cycle.
func TestRecurrenceProbesShowModelledFusion(t *testing.T) {
	image, err := os.ReadFile("../../evaluation/microbenchmarks/recurrence/recurrence.elf")
	if err != nil {
		t.Fatal(err)
	}
	definition, err := os.ReadFile("../../evaluation/microbenchmarks/recurrence/cases.def")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]float64{
		"sel32_rr_adj":       1,    // adjacent register pair selecting the compared registers
		"sel32_rr_sep":       2,    // one instruction between
		"sel32_rr_adj_p8":    1,    // second layout without a straddle
		"sel32_rr_adj_p4":    1.25, // every fourth pair straddles a line
		"sel32_rr_swap_adj":  1,    // compared registers selected in the other order
		"sel32_rr_dst_adj":   1,    // third destination register
		"sel32_ri_same_adj":  1,    // immediate compare, select uses the compared register
		"sel32_rr_other_adj": 2,    // select of two registers that were not compared
		"sel32_ri_adj":       2,    // immediate compare, select of two other registers
		"sel32_ri16_adj":     2,    // immediate 16, select of two other registers
		"sel32_shift_adj":    2,    // shifted immediate, select of two other registers
		"sel64_rr_adj":       2,    // 64-bit pairs keep separate timing
		"sel32_csinc_adj":    2,    // conditional increment
		"thr32_mix_adj":      1,    // independent pairs: one fusion pipe
		"tight_max_reg":      1,    // one pair per taken branch
	} {
		if got := recurrenceCyclesPerUpdate(t, image, string(definition), name); math.Abs(got-want) > 0.01 {
			t.Errorf("%s costs %.4f cycles per update, want %.2f", name, got, want)
		}
	}
	// Separated independent pairs are ordinary integer work on all six pipes.
	if got := recurrenceCyclesPerUpdate(t, image, string(definition), "thr32_mix_sep"); got < 0.66 || got > 0.70 {
		t.Errorf("thr32_mix_sep costs %.4f cycles per update, want about 0.68", got)
	}
	// The Nussinov loop replica is faster only with the pair adjacent.
	adjacent := recurrenceCyclesPerUpdate(t, image, string(definition), "nuss_const")
	separated := recurrenceCyclesPerUpdate(t, image, string(definition), "nuss_sep")
	if adjacent > 2.05 || separated < 2.9 {
		t.Errorf("kernel replica: adjacent %.4f, separated %.4f cycles per trip", adjacent, separated)
	}
}
