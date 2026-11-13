package elfsim

import (
	"context"
	"fmt"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/poros/pkg/cpu"
	"github.com/sarchlab/poros/pkg/linuxuser"
)

type MultiCoreResult struct {
	Cores             []Result `json:"cores"`
	TotalCycles       uint64   `json:"total_cycles"`
	TotalInstructions uint64   `json:"total_instructions"`
	Policy            string   `json:"policy"`
}

// RunMultiCore runs independent processes on OoO cores sharing a modeled L2.
// This preserves the old independent-copy benchmark experiment, not Linux
// threads, shared guest memory, or validated physical address-space isolation.
// The runner owns execution deadlines and simulation termination. Each deadline
// measures host elapsed time, including time spent simulating the other cores.
// Unlike Run, this API returns a nonnil error when a core stops without a guest
// exit (including instruction limits, deadlines, cancellation or execution faults).
// Callers must inspect the error as well as any returned per-core results and counters.
func RunMultiCore(ctx context.Context, processes []*linuxuser.Process, o Options) (result MultiCoreResult, err error) {
	if len(processes) < 1 || len(processes) > 64 || (o.Model != "" && o.Model != "ooo") {
		return result, fmt.Errorf("invalid multicore process count/model/frequency")
	}
	device, err := configure(&o)
	if err != nil {
		return result, err
	}
	seen := map[*linuxuser.Process]bool{}
	for _, p := range processes {
		if p == nil || seen[p] {
			return result, fmt.Errorf("each core requires a distinct nonnil process")
		}
		seen[p] = true
	}
	executions := make([]*linuxuser.Execution, len(processes))
	configs := make([]cpu.CoreConfig, len(processes))
	defer func() {
		for _, x := range executions {
			if x != nil {
				x.Close()
			}
		}
	}()
	for i, p := range processes {
		x, e := p.NewExecution(ctx, o.Limits)
		if e != nil {
			return result, e
		}
		executions[i] = x
		configs[i] = cpu.CoreConfig{Execution: cpu.NewELFTimingExecution(x)}
	}
	simulation := sim.MakeBuilder().Build()
	defer func() {
		if e := simulation.Terminate(); err == nil {
			err = e
		}
	}()
	mc := cpu.NewMultiCorePipelineFor("CPU", simulation, device.OoO, device.Cache, configs)
	if err = simulation.Initialize(); err != nil {
		return result, err
	}
	runErr := mc.Run()
	result.Policy = "independent ELF process copies; private L1/shared modeled L2; identical virtual addresses are shared cache tags; no guest threads or calibrated isolation model"
	result.TotalCycles = mc.TotalCycles
	result.TotalInstructions = mc.TotalInstructions
	for i, core := range mc.Cores {
		r := executions[i].Result()
		stats := Statistics{Model: "ooo", Target: device.Identity, AssumedHz: o.FrequencyHz, Cycles: core.State.Cycles, Retired: core.State.InstDone, CPI: core.State.CPI(), PredictedUserSeconds: float64(core.State.Cycles) / o.FrequencyHz, Cache: core.Resources.Cache.Stats(), CacheConfig: core.Resources.Cache.Config(), Configuration: core.Spec, BranchMetricsAvailable: true, BranchPredictions: core.State.Predictor.Predictions, BranchDirectionHits: core.State.Predictor.Hits, Policy: result.Policy}
		result.Cores = append(result.Cores, Result{Result: r, Timing: stats})
		if !executions[i].Stopped() || stats.Retired != r.Instructions {
			return result, fmt.Errorf("core%d did not drain to architectural boundary: %s", i, core.State.Err)
		}
	}
	return result, runErr
}
