// Package inorder models the five-stage in-order CPU pipeline.
package inorder

import (
	"encoding/binary"

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
	Emu   *emu.Emulator
	Cache *mem.Cache
}
type Ports struct{}
type Middlewares struct{ Pipeline *middleware }
type Comp = ticking.Component[Spec, State, Resources, Ports, Middlewares]

// State owns all pipeline timing data. Instructions retain their fetched encoding.
type State struct {
	Fetch, Decode, Execute, Memory, Writeback stageReg
	FetchPC                                   uint64
	GprPending, FpPending                     [32]int
	Cycles, InstDone                          uint64
	Done                                      bool
	Err                                       string
}
type stageReg struct {
	Valid              bool
	PC                 uint64
	Word               uint32
	CyclesLeft         int
	Executed, Branched bool
	MemLatency         int
}

func (s stageReg) instruction() arm64asm.Inst {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], s.Word)
	i, err := arm64asm.Decode(b[:])
	if err != nil {
		panic(err)
	}
	return i
}
func (s *State) Initialize(c *Comp) {
	if c.Resources.Emu == nil {
		panic("inorder: emulator is required")
	}
	if c.Spec.LoadLatency < 1 {
		panic("inorder: load latency must be positive")
	}
	if !c.Resources.Emu.Mem.TrackAccesses {
		panic("inorder: enable emulator memory access tracking before Build")
	}
	s.FetchPC = c.Resources.Emu.Regs.PC
}
func (m *Middlewares) Initialize(c *Comp) { m.Pipeline = &middleware{Comp: c} }

type middleware struct{ *Comp }

// CPI returns cycles per instruction.
func (s *State) CPI() float64 {
	if s.InstDone == 0 {
		return 0
	}
	return float64(s.Cycles) / float64(s.InstDone)
}

// Tick advances the pipeline by one cycle.
// Returns true if the pipeline should continue ticking.
func (p *middleware) Handle(_ timing.Event) bool {
	if p.State.Done {
		return false
	}

	p.State.Cycles++

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
	if !p.State.Fetch.Valid && !p.State.Decode.Valid && !p.State.Execute.Valid &&
		!p.State.Memory.Valid && !p.State.Writeback.Valid {
		p.State.Done = true
	}

	return !p.State.Done
}

// --- Stage implementations ---

func (p *middleware) stageWriteback() {
	if !p.State.Writeback.Valid {
		return
	}

	// Clear scoreboard for destination registers.
	i := p.State.Writeback.instruction()
	gpr, fp := instinfo.DestRegs(i)
	if gpr >= 0 {
		p.State.GprPending[gpr]--
	}
	if fp >= 0 {
		p.State.FpPending[fp]--
	}
	if gpr2, fp2 := instinfo.DestRegs2(i); gpr2 >= 0 || fp2 >= 0 {
		if gpr2 >= 0 {
			p.State.GprPending[gpr2]--
		}
		if fp2 >= 0 {
			p.State.FpPending[fp2]--
		}
	}

	p.State.InstDone++
	p.State.Writeback = stageReg{}
}

func (p *middleware) stageMemory() {
	if !p.State.Memory.Valid {
		return
	}

	if p.State.Memory.CyclesLeft > 0 {
		p.State.Memory.CyclesLeft--
		return
	}

	// Move to writeback if writeback is empty.
	if p.State.Writeback.Valid {
		return // structural hazard — stall
	}

	p.State.Writeback = p.State.Memory
	p.State.Memory = stageReg{}
}

// stageExecute returns true if downstream is stalled (memory stage busy).
func (p *middleware) stageExecute() bool {
	if !p.State.Execute.Valid {
		return false
	}

	// First cycle: call the emulator for functional execution.
	if !p.State.Execute.Executed {
		p.Resources.Emu.Mem.ClearAccesses()
		inst, branched, err := p.Resources.Emu.StepDetailed()
		if err != nil {
			p.State.Err = err.Error()
			p.State.Done = true
			return true
		}
		_ = inst // The fetched encoding is already stored in the execute stage.
		p.State.Execute.Executed = true
		p.State.Execute.Branched = branched
		p.State.Execute.MemLatency = p.computeMemLatency()

		// Handle taken branch: flush fetch and decode stages.
		// This also covers halt (RET to haltPC): flush + pipeline drains.
		if branched {
			p.flush()
		}
	}

	if p.State.Execute.CyclesLeft > 0 {
		p.State.Execute.CyclesLeft--
		return false
	}

	// Move to memory if memory is empty.
	if p.State.Memory.Valid {
		return true // stall
	}

	p.State.Memory = p.State.Execute
	p.State.Memory.CyclesLeft = p.State.Memory.MemLatency
	p.State.Execute = stageReg{}
	return false
}

// stageDecode returns true if decode is stalled (data hazard).
func (p *middleware) stageDecode() bool {
	if !p.State.Decode.Valid {
		return false
	}

	// Check scoreboard for source register hazards.
	i := p.State.Decode.instruction()
	for _, idx := range instinfo.SourceGPRs(i) {
		if p.State.GprPending[idx] > 0 {
			return true // stall: data hazard
		}
	}
	for _, idx := range instinfo.SourceFPs(i) {
		if p.State.FpPending[idx] > 0 {
			return true // stall: data hazard
		}
	}

	// Move to execute if execute is empty.
	if p.State.Execute.Valid {
		return true // structural hazard
	}

	// Set execute latency.
	p.State.Decode.CyclesLeft = instinfo.ExecuteLatency(i.Op) - 1

	// Mark destination registers as pending in scoreboard.
	gpr, fp := instinfo.DestRegs(i)
	if gpr >= 0 {
		p.State.GprPending[gpr]++
	}
	if fp >= 0 {
		p.State.FpPending[fp]++
	}
	if gpr2, fp2 := instinfo.DestRegs2(i); gpr2 >= 0 || fp2 >= 0 {
		if gpr2 >= 0 {
			p.State.GprPending[gpr2]++
		}
		if fp2 >= 0 {
			p.State.FpPending[fp2]++
		}
	}

	p.State.Execute = p.State.Decode
	p.State.Decode = stageReg{}
	return false
}

func (p *middleware) stageFetch() {
	if p.State.Decode.Valid {
		return // decode occupied — stall fetch
	}

	// Read 4 bytes at fetchPC from emulator memory.
	var buf [4]byte
	for i := range buf {
		buf[i] = p.Resources.Emu.Mem.ReadU8(p.State.FetchPC + uint64(i))
	}

	inst, err := arm64asm.Decode(buf[:])
	if err != nil {
		// Speculative fetch past program end — insert bubble.
		// The pipeline will flush or halt before this matters.
		return
	}

	p.State.Decode = stageReg{
		Valid: true,
		PC:    p.State.FetchPC,
		Word:  inst.Enc,
	}
	p.State.FetchPC += 4 // predict not-taken
}

// computeMemLatency determines the memory stage latency for the instruction
// currently in the execute stage, based on tracked memory accesses and cache.
func (p *middleware) computeMemLatency() int {
	if p.Resources.Cache == nil {
		if instinfo.IsLoad(p.State.Execute.instruction().Op) {
			return p.Spec.LoadLatency - 1
		}
		return 0
	}

	accesses := p.Resources.Emu.Mem.Accesses
	if len(accesses) == 0 {
		return 0
	}

	worstLevel := mem.HitL1
	for _, acc := range accesses {
		level := p.Resources.Cache.Lookup(acc.Addr, acc.Size, acc.IsWrite)
		if level > worstLevel {
			worstLevel = level
		}
	}

	cfg := p.Resources.Cache.Config()
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
	p.State.Fetch = stageReg{}
	p.State.Decode = stageReg{}
	p.State.FetchPC = p.Resources.Emu.Regs.PC
}
