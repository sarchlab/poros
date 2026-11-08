// Package timed assembles Akita's timing memory components. Their storage is
// deliberately separate from the functional executor: requests model traffic,
// not speculative architectural values.
package timed

import (
	"fmt"
	"math"
	"reflect"

	akitamem "github.com/sarchlab/akita/v5/mem"
	"github.com/sarchlab/akita/v5/mem/cache/writeback"
	"github.com/sarchlab/akita/v5/mem/simplebankedmemory"
	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/messaging"
	"github.com/sarchlab/akita/v5/sim/messaging/direct"
	"github.com/sarchlab/akita/v5/sim/messaging/twowaybuffered"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"
)

// Config is assembly configuration, not a component Spec. It is emitted in
// every result so that queue capacities and latencies are reproducible.
type Config struct {
	Backend      string                  `json:"backend"`
	L1           writeback.Spec          `json:"l1"`
	L2           writeback.Spec          `json:"l2"`
	Memory       simplebankedmemory.Spec `json:"memory"`
	PortCapacity int                     `json:"port_capacity"`
}

func DefaultConfig(freq timing.Freq) Config {
	// Four internal ticks per CPU cycle resolve Akita's directory/bank/port
	// stages. This is model resolution, not a measured physical cache clock.
	cpuFreq := freq
	factor := timing.Freq(4)
	if freq*factor > timing.Freq(1e12) {
		factor = timing.Freq(math.Floor(1e12 / float64(freq)))
	}
	freq *= factor
	l1 := writeback.DefaultSpec()
	l1.Freq, l1.NumReqPerCycle = freq, 1
	l1.NumSets, l1.NumWays, l1.Log2BlockSize = 256, 8, 6
	l1.DirLatency, l1.BankLatency = 5, 0
	l1.DirectoryWidth, l1.BankWidth, l1.NumBanks = 4, 4, 1
	l1.NumMSHREntries, l1.WriteBufferCapacity = 24, 32
	l1.MaxInFlightFetches, l1.MaxInFlightEvictions = 24, 16
	l1.Bottom.Destinations = []messaging.RemotePort{"L2.Top"}
	l2 := writeback.DefaultSpec()
	l2.Freq, l2.NumReqPerCycle = freq, 1
	l2.NumSets, l2.NumWays, l2.Log2BlockSize = 8192, 16, 7
	l2.DirLatency, l2.BankLatency = 1, 64
	l2.DirectoryWidth, l2.BankWidth, l2.NumBanks = 4, 4, 4
	l2.NumMSHREntries, l2.WriteBufferCapacity = 64, 64
	l2.MaxInFlightFetches, l2.MaxInFlightEvictions = 64, 32
	l2.Bottom.Destinations = []messaging.RemotePort{"Memory.Top"}
	d := simplebankedmemory.DefaultSpec()
	d.Freq = timing.Freq(math.Round(float64(cpuFreq) * 500000000 / 3490000000))
	d.NumBanks, d.BankPipelineWidth = 1, 1
	d.BankPipelineDepth, d.StageLatency = 1, 50
	d.PostPipelineBufSize = 4
	d.BankSelectorLog2InterleaveSize = 7
	return Config{Backend: "akita", L1: l1, L2: l2, Memory: d, PortCapacity: 16}
}

func (c Config) Validate(cpuFreq timing.Freq) error {
	for _, freq := range []timing.Freq{cpuFreq, c.L1.Freq, c.L2.Freq, c.Memory.Freq} {
		if math.IsNaN(float64(freq)) || math.IsInf(float64(freq), 0) || math.Trunc(float64(freq)) != float64(freq) {
			return fmt.Errorf("memory frequencies must be finite integer Hz")
		}
	}
	if c.Backend != "akita" || c.PortCapacity < 1 || c.PortCapacity > 4096 || cpuFreq <= 0 || c.L1.Freq < cpuFreq || c.L1.Freq > 8*cpuFreq || c.L1.Freq > timing.Freq(1e12) || uint64(c.L1.Freq)%uint64(cpuFreq) != 0 || c.L1.Freq != c.L2.Freq || c.Memory.Freq < 1 || c.Memory.Freq > 8*cpuFreq || c.Memory.Freq > timing.Freq(1e12) {
		return fmt.Errorf("invalid memory backend, port capacity or internal tick frequency")
	}
	defaults := DefaultConfig(cpuFreq)
	for i, v := range []writeback.Spec{c.L1, c.L2} {
		if v.NumSets < 1 || v.NumSets > 1<<20 || v.NumWays < 1 || v.NumWays > 64 || v.Log2BlockSize < 4 || v.Log2BlockSize > 12 || v.ByteSize() > 512<<20 || v.NumBanks < 1 || v.NumBanks > 64 || v.NumReqPerCycle < 1 || v.NumReqPerCycle > 64 || v.DirectoryWidth < 1 || v.DirectoryWidth > 64 || v.BankWidth < 2 || v.BankWidth > 64 || v.DirLatency < 0 || v.DirLatency > 10000 || v.BankLatency < 0 || v.BankLatency > 10000 || v.NumMSHREntries < 1 || v.NumMSHREntries > 4096 || v.WriteBufferCapacity < 1 || v.WriteBufferCapacity > 4096 || v.MaxInFlightFetches < 1 || v.MaxInFlightFetches > 4096 || v.MaxInFlightEvictions < 1 || v.MaxInFlightEvictions > 4096 {
			return fmt.Errorf("invalid L%d cache geometry, queue or pipeline", i+1)
		}
		want := defaults.L1.Bottom
		if i == 1 {
			want = defaults.L2.Bottom
		}
		if !reflect.DeepEqual(v.Bottom, want) {
			return fmt.Errorf("L%d routing must match the assembled hierarchy", i+1)
		}
	}
	if c.L1.Log2BlockSize > c.L2.Log2BlockSize {
		return fmt.Errorf("L2 line must cover an L1 fetch")
	}
	m := c.Memory
	if m.NumBanks < 1 || m.NumBanks > 64 || m.BankPipelineWidth < 1 || m.BankPipelineWidth > 64 || m.BankPipelineDepth < 1 || m.BankPipelineDepth > 64 || m.StageLatency < 1 || m.StageLatency > 10000 || m.PostPipelineBufSize < 1 || m.PostPipelineBufSize > 4096 || m.BankSelectorLog2InterleaveSize < 4 || m.BankSelectorLog2InterleaveSize > 32 || !reflect.DeepEqual(m.BankAddressConversion, defaults.Memory.BankAddressConversion) {
		return fmt.Errorf("invalid banked memory geometry or pipeline")
	}
	return nil
}

type Hierarchy struct {
	L1, L2 *writeback.Comp
	Memory *simplebankedmemory.Comp
}

// Build wires a single CPU data port to private L1/L2 and banked backing memory.
// All components run in the caller's simulation; no nested event loop exists.
func Build(s *sim.Simulation, cpu messaging.Port, cfg Config) (*Hierarchy, error) {
	// Validate structural bounds also for callers that assemble components
	// directly. The CPU runner separately checks the CPU/cache clock ratio.
	if err := cfg.Validate(cfg.L1.Freq); err != nil {
		return nil, err
	}
	ports := twowaybuffered.MakeBuilder().WithSimulation(s).WithSpec(twowaybuffered.Spec{IncomingCapacity: cfg.PortCapacity, OutgoingCapacity: cfg.PortCapacity})
	cache := func(name string, spec writeback.Spec) *writeback.Comp {
		c := modeling.MakeBuilder[*writeback.Comp](spec).WithSimulation(s).WithResources(writeback.Resources{
			Storage: akitamem.MakeStorageBuilder().WithCapacity(spec.ByteSize()).WithSimulation(s).Build(name + ".Storage"),
		}).Build(name)
		c.BindPort("Top", ports.Build())
		c.BindPort("Bottom", ports.Build())
		c.BindPort("Control", ports.Build())
		return c
	}
	h := &Hierarchy{L1: cache("L1", cfg.L1), L2: cache("L2", cfg.L2)}
	h.Memory = modeling.MakeBuilder[*simplebankedmemory.Comp](cfg.Memory).WithSimulation(s).WithResources(simplebankedmemory.Resources{
		Storage: akitamem.MakeStorageBuilder().WithCapacity(1 << 48).WithSimulation(s).Build("Memory.Storage"),
	}).Build("Memory")
	h.Memory.BindPort("Top", ports.Build())
	h.Memory.BindPort("Control", ports.Build())
	connect := func(name string, a, b messaging.Port) {
		c := direct.NewConnection(name, s, cfg.L1.Freq)
		c.BindPort(a)
		c.BindPort(b)
	}
	connect("CPUToL1", cpu, h.L1.Ports.Top)
	connect("L1ToL2", h.L1.Ports.Bottom, h.L2.Ports.Top)
	connect("L2ToMemory", h.L2.Ports.Bottom, h.Memory.Ports.Top)
	return h, nil
}
