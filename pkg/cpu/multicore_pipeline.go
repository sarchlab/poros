package cpu

import (
	"fmt"
	"sort"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
)

// CoreConfig defines the configuration for a single core in a multi-core
// simulation.
type CoreConfig struct {
	Emulator       *emu.Emulator
	FuncHookCycles map[uint64]int // optional cycle costs for hooked functions
}

// MultiCorePipeline wraps N independent ooo.Comp instances with per-core
// private L1 caches and a shared L2 cache. All cores are driven by a single
// Akita SerialEngine, so no locks are needed.
type MultiCorePipeline struct {
	Cores      []*ooo.Comp
	SharedL2   *mem.SharedL2
	Simulation *sim.Simulation

	// Aggregate results (populated after Run).
	TotalCycles       uint64 // max(core.State.Cycles)
	TotalInstructions uint64 // sum(core.State.InstDone)
}

// NewMultiCorePipeline creates a multi-core pipeline with len(configs) cores.
// Each core gets a private L1 cache and all share a single L2 cache.
func NewMultiCorePipeline(
	name string,
	simulation *sim.Simulation,
	freq timing.Freq,
	configs []CoreConfig,
) *MultiCorePipeline {
	cfg := mem.DefaultCacheConfig()
	sharedL2 := mem.NewSharedL2(cfg)

	mc := &MultiCorePipeline{
		Cores:      make([]*ooo.Comp, len(configs)),
		SharedL2:   sharedL2,
		Simulation: simulation,
	}

	for i, cc := range configs {
		cache := mem.NewPrivateL1WithSharedL2(cfg, sharedL2)

		coreName := fmt.Sprintf("%s.Core%d", name, i)
		cc.Emulator.Mem.TrackAccesses = true
		spec := ooo.DefaultSpec()
		spec.Freq = freq
		spec.FuncHookCycles = HookCycleSpec(cc.FuncHookCycles)
		p := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(simulation).
			WithResources(ooo.Resources{Emu: cc.Emulator, Cache: cache}).Build(coreName)
		mc.Cores[i] = p
	}

	return mc
}

// WarmCaches warms each core's private L1 + shared L2 with the given data
// regions. Each core gets the same set of regions warmed.
func (mc *MultiCorePipeline) WarmCaches(regions []struct {
	Addr uint64
	Size int
}) {
	for _, core := range mc.Cores {
		for _, r := range regions {
			lineSize := uint64(core.Resources.Cache.Config().L1LineSize)
			for addr := r.Addr; addr < r.Addr+uint64(r.Size); addr += lineSize {
				core.Resources.Cache.Lookup(addr, int(lineSize), false)
			}
			core.Resources.Cache.ResetStats()
		}
	}
}

// Run starts all cores and runs the simulation to completion. Returns the first
// pipeline error encountered, if any. Initialize the shared Simulation after all
// components have been built and before calling Run.
func (mc *MultiCorePipeline) Run() error {
	for _, core := range mc.Cores {
		core.TickLater()
	}

	if err := mc.Simulation.Run(); err != nil {
		return err
	}

	// Compute aggregate stats.
	mc.TotalCycles = 0
	mc.TotalInstructions = 0
	for _, core := range mc.Cores {
		if core.State.Err != "" {
			return fmt.Errorf("%s: %s", core.Name(), core.State.Err)
		}
		mc.TotalInstructions += core.State.InstDone
		if core.State.Cycles > mc.TotalCycles {
			mc.TotalCycles = core.State.Cycles
		}
	}

	return nil
}

// HookCycleSpec converts the benchmark map into deterministic immutable configuration.
func HookCycleSpec(cycles map[uint64]int) []ooo.HookCycles {
	result := make([]ooo.HookCycles, 0, len(cycles))
	for address, n := range cycles {
		result = append(result, ooo.HookCycles{Address: address, Cycles: n})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Address < result[j].Address })
	return result
}
