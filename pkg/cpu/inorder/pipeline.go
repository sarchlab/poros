// Package inorder models the five-stage in-order CPU pipeline.
package inorder

import (
	"github.com/sarchlab/akita/v5/sim/modeling/ticking"
	"github.com/sarchlab/akita/v5/sim/timing"
	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
	"golang.org/x/arch/arm64/arm64asm"
)

// Spec is immutable timing configuration.
type Spec struct {
	Freq        timing.Freq `json:"freq"`
	LoadLatency int         `json:"load_latency"`
}

func DefaultSpec() Spec { return Spec{Freq: 3490 * timing.MHz, LoadLatency: 4} }

// Resources supplies the functional execution backend and optional synchronous cache.
// These live objects are not included in component checkpoints.
type Resources struct {
	Emu       *emu.Emulator
	Execution emu.Execution
	Cache     *mem.Cache
}
type Ports struct{}
type Middlewares struct{ Pipeline *middleware }
type Comp = ticking.Component[Spec, State, Resources, Ports, Middlewares]

// State owns all pipeline timing data. Instructions retain their fetched encoding
// and its decoded operand metadata, so stalled stages do not decode repeatedly.
type State struct {
	Fetch, Decode, Execute, Memory, Writeback stageReg
	FetchPC                                   uint64
	GprPending, FpPending                     [32]int
	Cycles, InstDone                          uint64
	Done                                      bool
	Err                                       string
}
type stageReg struct {
	Valid                              bool
	PC                                 uint64
	Word                               uint32
	Op                                 arm64asm.Op
	SourceGPRs, SourceFPs              []int
	DestGPR, DestFP, DestGPR2, DestFP2 int
	CyclesLeft                         int
	Executed, Branched                 bool
	MemLatency                         int
}

func (s *State) Initialize(c *Comp) {
	if (c.Resources.Emu == nil) == (c.Resources.Execution == nil) {
		panic("inorder: exactly one emulator or execution resource is required")
	}
	if c.Spec.LoadLatency < 1 {
		panic("inorder: load latency must be positive")
	}
	if c.Resources.Emu != nil && !c.Resources.Emu.Mem.TrackAccesses {
		panic("inorder: enable emulator memory access tracking before Build")
	}
	s.FetchPC = c.Resources.backend().CPURegisters().PC
}
func (m *Middlewares) Initialize(c *Comp) { m.Pipeline = &middleware{comp: c} }

type middleware struct{ comp *Comp }

// CPI returns cycles per instruction.
func (s *State) CPI() float64 {
	if s.InstDone == 0 {
		return 0
	}
	return float64(s.Cycles) / float64(s.InstDone)
}

// Handle advances the pipeline by one cycle.
// Returns true if the pipeline should continue ticking.
func (p *middleware) Handle(_ timing.Event) bool {
	if p.comp.State.Done {
		return false
	}

	p.comp.State.Cycles++

	// Process stages in reverse order to avoid overwriting.
	p.stageWriteback()
	p.stageMemory()
	stall := p.stageExecute()
	if !stall {
		stall = p.stageDecode()
	}
	if !stall {
		p.stageFetch()
	}

	// If all stages are empty, the pipeline has drained.
	if !p.comp.State.Fetch.Valid && !p.comp.State.Decode.Valid && !p.comp.State.Execute.Valid &&
		!p.comp.State.Memory.Valid && !p.comp.State.Writeback.Valid {
		p.comp.State.Done = true
	}

	return !p.comp.State.Done
}

// --- Stage implementations ---

func (p *middleware) stageWriteback() {
	if !p.comp.State.Writeback.Valid {
		return
	}

	// Clear scoreboard for destination registers.
	gpr, fp := p.comp.State.Writeback.DestGPR, p.comp.State.Writeback.DestFP
	if gpr >= 0 {
		p.comp.State.GprPending[gpr]--
	}
	if fp >= 0 {
		p.comp.State.FpPending[fp]--
	}
	if gpr2, fp2 := p.comp.State.Writeback.DestGPR2, p.comp.State.Writeback.DestFP2; gpr2 >= 0 || fp2 >= 0 {
		if gpr2 >= 0 {
			p.comp.State.GprPending[gpr2]--
		}
		if fp2 >= 0 {
			p.comp.State.FpPending[fp2]--
		}
	}

	p.comp.State.InstDone++
	p.comp.State.Writeback = stageReg{}
}

func (p *middleware) stageMemory() {
	if !p.comp.State.Memory.Valid {
		return
	}

	if p.comp.State.Memory.CyclesLeft > 0 {
		p.comp.State.Memory.CyclesLeft--
		return
	}

	// Move to writeback if writeback is empty.
	if p.comp.State.Writeback.Valid {
		return // structural hazard — stall
	}

	p.comp.State.Writeback = p.comp.State.Memory
	p.comp.State.Memory = stageReg{}
}

// stageExecute returns true if downstream is stalled (memory stage busy).
func (p *middleware) stageExecute() bool {
	if !p.comp.State.Execute.Valid {
		return false
	}

	// First cycle: call the emulator for functional execution.
	if !p.comp.State.Execute.Executed {
		if p.comp.Resources.Execution != nil && p.comp.State.Execute.Op == arm64asm.SVC && (p.comp.State.Memory.Valid || p.comp.State.Writeback.Valid) {
			return true
		}

		inst, branched, err := p.comp.Resources.backend().StepDetailed()
		if err != nil {
			p.comp.State.Err = err.Error()
			if p.comp.Resources.Emu != nil {
				p.comp.State.Done = true
			}
			p.comp.State.Execute = stageReg{}
			p.flush()
			return true
		}
		_ = inst // The fetched encoding is already stored in the execute stage.
		p.comp.State.Execute.Executed = true
		p.comp.State.Execute.Branched = branched
		p.comp.State.Execute.MemLatency = p.computeMemLatency()

		// Handle taken branch: flush fetch and decode stages.
		// This also covers halt (RET to haltPC): flush + pipeline drains.
		if branched || (p.comp.Resources.Execution != nil && (inst.Op == arm64asm.SVC || p.comp.Resources.backend().Stopped())) {
			p.flush()
		}
	}

	if p.comp.State.Execute.CyclesLeft > 0 {
		p.comp.State.Execute.CyclesLeft--
		return false
	}

	// Move to memory if memory is empty.
	if p.comp.State.Memory.Valid {
		return true // stall
	}

	p.comp.State.Memory = p.comp.State.Execute
	p.comp.State.Memory.CyclesLeft = p.comp.State.Memory.MemLatency
	p.comp.State.Execute = stageReg{}
	return false
}

// stageDecode returns true if decode is stalled (data hazard).
func (p *middleware) stageDecode() bool {
	if !p.comp.State.Decode.Valid {
		return false
	}

	// Check scoreboard for source register hazards.
	for _, idx := range p.comp.State.Decode.SourceGPRs {
		if p.comp.State.GprPending[idx] > 0 {
			return true // stall: data hazard
		}
	}
	for _, idx := range p.comp.State.Decode.SourceFPs {
		if p.comp.State.FpPending[idx] > 0 {
			return true // stall: data hazard
		}
	}

	// Move to execute if execute is empty.
	if p.comp.State.Execute.Valid {
		return true // structural hazard
	}

	// Set execute latency.
	p.comp.State.Decode.CyclesLeft = instinfo.ExecuteLatency(p.comp.State.Decode.Op) - 1

	// Mark destination registers as pending in scoreboard.
	gpr, fp := p.comp.State.Decode.DestGPR, p.comp.State.Decode.DestFP
	if gpr >= 0 {
		p.comp.State.GprPending[gpr]++
	}
	if fp >= 0 {
		p.comp.State.FpPending[fp]++
	}
	if gpr2, fp2 := p.comp.State.Decode.DestGPR2, p.comp.State.Decode.DestFP2; gpr2 >= 0 || fp2 >= 0 {
		if gpr2 >= 0 {
			p.comp.State.GprPending[gpr2]++
		}
		if fp2 >= 0 {
			p.comp.State.FpPending[fp2]++
		}
	}

	p.comp.State.Execute = p.comp.State.Decode
	p.comp.State.Decode = stageReg{}
	return false
}

func (p *middleware) stageFetch() {
	if p.comp.State.Decode.Valid {
		return // decode occupied — stall fetch
	}

	// Read 4 bytes at fetchPC from emulator memory.
	if p.comp.Resources.Execution != nil && p.comp.Resources.backend().Stopped() {
		return
	}
	inst, err := p.comp.Resources.backend().Peek(p.comp.State.FetchPC)
	if err != nil {
		// Speculative wrong-path fetch faults are discarded. At the current
		// architectural PC, resolve only once older pipeline work has drained.
		if p.comp.Resources.Execution != nil && !p.comp.State.Execute.Valid && !p.comp.State.Memory.Valid && !p.comp.State.Writeback.Valid && p.comp.State.FetchPC == p.comp.Resources.backend().CPURegisters().PC {
			_, _, err = p.comp.Resources.backend().StepDetailed()
			if err != nil {
				p.comp.State.Err = err.Error()
			}
		}
		return
	}

	gpr, fp := instinfo.DestRegs(inst)
	gpr2, fp2 := instinfo.DestRegs2(inst)
	p.comp.State.Decode = stageReg{
		Valid:      true,
		PC:         p.comp.State.FetchPC,
		Word:       inst.Enc,
		Op:         inst.Op,
		SourceGPRs: instinfo.SourceGPRs(inst), SourceFPs: instinfo.SourceFPs(inst),
		DestGPR: gpr, DestFP: fp, DestGPR2: gpr2, DestFP2: fp2,
	}
	p.comp.State.FetchPC += 4 // predict not-taken
}

// computeMemLatency determines the memory stage latency for the instruction
// currently in the execute stage, based on tracked memory accesses and cache.
func (p *middleware) computeMemLatency() int {
	if p.comp.Resources.Cache == nil {
		if instinfo.IsLoad(p.comp.State.Execute.Op) {
			return p.comp.Spec.LoadLatency - 1
		}
		return 0
	}

	accesses := p.comp.Resources.backend().MemoryAccesses()
	if len(accesses) == 0 {
		return 0
	}

	worstLevel := mem.HitL1
	for _, acc := range accesses {
		level := p.comp.Resources.Cache.Lookup(acc.Addr, acc.Size, acc.IsWrite)
		if level > worstLevel {
			worstLevel = level
		}
	}

	cfg := p.comp.Resources.Cache.Config()
	switch worstLevel {
	case mem.HitL2:
		return cfg.L2Latency - 1
	case mem.HitDRAM:
		return cfg.DRAMLatency - 1
	default:
		return cfg.L1Latency - 1
	}
}

// flush clears fetch and decode stages after a taken branch.
func (p *middleware) flush() {
	p.comp.State.Fetch = stageReg{}
	p.comp.State.Decode = stageReg{}
	p.comp.State.FetchPC = p.comp.Resources.backend().CPURegisters().PC
}

func (r Resources) backend() emu.Execution {
	if r.Execution != nil {
		return r.Execution
	}
	return r.Emu
}
