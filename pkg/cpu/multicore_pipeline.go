package cpu

import (
	"fmt"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
)

// CoreConfig defines the configuration for a single core in a multi-core
// simulation.
type CoreConfig struct {
	Emulator       *emu.Emulator
	FuncHookCycles map[uint64]int // optional cycle costs for hooked functions
}

// MultiCorePipeline wraps N independent OoOPipeline instances with per-core
// private L1 caches and a shared L2 cache. All cores are driven by a single
// Akita SerialEngine, so no locks are needed.
type MultiCorePipeline struct {
	Cores    []*OoOPipeline
	SharedL2 *mem.SharedL2
	Engine   sim.Engine

	// Aggregate results (populated after Run).
	TotalCycles       uint64 // max(core.Cycles)
	TotalInstructions uint64 // sum(core.InstDone)
}

// NewMultiCorePipeline creates a multi-core pipeline with len(configs) cores.
// Each core gets a private L1 cache and all share a single L2 cache.
func NewMultiCorePipeline(
	name string,
	engine sim.Engine,
	freq sim.Freq,
	configs []CoreConfig,
) *MultiCorePipeline {
	cfg := mem.DefaultCacheConfig()
	sharedL2 := mem.NewSharedL2(cfg)

	mc := &MultiCorePipeline{
		Cores:    make([]*OoOPipeline, len(configs)),
		SharedL2: sharedL2,
		Engine:   engine,
	}

	for i, cc := range configs {
		cache := mem.NewPrivateL1WithSharedL2(cfg, sharedL2)

		coreName := fmt.Sprintf("%s.Core%d", name, i)
		p := NewOoOPipeline(coreName, engine, freq, cc.Emulator)
		p.Cache = cache
		if cc.FuncHookCycles != nil {
			p.FuncHookCycles = cc.FuncHookCycles
		}
		mc.Cores[i] = p
	}

	return mc
}

// WarmCaches warms each core's private L1 + shared L2 with the given data
// regions. Each core gets the same set of regions warmed.
func (mc *MultiCorePipeline) WarmCaches(regions []struct{ Addr uint64; Size int }) {
	for _, core := range mc.Cores {
		for _, r := range regions {
			lineSize := uint64(core.Cache.Config().L1LineSize)
			for addr := r.Addr; addr < r.Addr+uint64(r.Size); addr += lineSize {
				core.Cache.Lookup(addr, int(lineSize), false)
			}
			core.Cache.ResetStats()
		}
	}
}

// Run starts all cores and runs the engine to completion. Returns the first
// pipeline error encountered, if any.
func (mc *MultiCorePipeline) Run() error {
	for _, core := range mc.Cores {
		core.TickLater()
	}

	if err := mc.Engine.Run(); err != nil {
		return err
	}

	// Compute aggregate stats.
	mc.TotalCycles = 0
	mc.TotalInstructions = 0
	for _, core := range mc.Cores {
		if core.Err != nil {
			return core.Err
		}
		mc.TotalInstructions += core.InstDone
		if core.Cycles > mc.TotalCycles {
			mc.TotalCycles = core.Cycles
		}
	}

	return nil
}
