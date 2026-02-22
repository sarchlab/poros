package cpu

import (
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/m2sim3/pkg/emu"
	"github.com/sarchlab/m2sim3/pkg/mem"
	"golang.org/x/arch/arm64/arm64asm"
)

// stageReg holds one instruction flowing through the pipeline.
type stageReg struct {
	valid      bool
	pc         uint64
	inst       arm64asm.Inst
	cyclesLeft int  // remaining cycles in current stage
	executed   bool // emulator has executed this instruction
	branched   bool // taken branch detected at Execute
	memLatency int  // precomputed memory stage latency
}

// Pipeline is a 5-stage in-order pipeline TickingComponent.
//
// Stages: Fetch → Decode → Execute → Memory → Writeback
// Each Tick advances the pipeline by one cycle.
type Pipeline struct {
	*sim.TickingComponent

	Emu   *emu.Emulator
	Cache *mem.Cache // optional cache model (nil = fixed latency)

	// Pipeline stage registers.
	fetch     stageReg
	decode    stageReg
	execute   stageReg
	memory    stageReg
	writeback stageReg

	// Fetch PC maintained by the pipeline (may differ from emu.PC
	// while instructions are in-flight).
	fetchPC uint64

	// Scoreboard: count of pending writes per register.
	gprPending [32]int
	fpPending  [32]int

	// Statistics.
	Cycles   uint64
	InstDone uint64

	// Configuration.
	LoadLatency int // memory stage latency for loads (default 4)

	// Internal state.
	done bool
	Err  error
}

// NewPipeline creates a 5-stage pipeline wrapping the given emulator.
func NewPipeline(
	name string,
	engine sim.Engine,
	freq sim.Freq,
	emulator *emu.Emulator,
) *Pipeline {
	emulator.Mem.TrackAccesses = true
	p := &Pipeline{
		Emu:         emulator,
		fetchPC:     emulator.Regs.PC,
		LoadLatency: 4,
	}
	p.TickingComponent = sim.NewTickingComponent(name, engine, freq, p)
	return p
}

// CPI returns cycles per instruction.
func (p *Pipeline) CPI() float64 {
	if p.InstDone == 0 {
		return 0
	}
	return float64(p.Cycles) / float64(p.InstDone)
}

// Tick advances the pipeline by one cycle.
// Returns true if the pipeline should continue ticking.
func (p *Pipeline) Tick() bool {
	if p.done {
		return false
	}

	p.Cycles++

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
	if !p.fetch.valid && !p.decode.valid && !p.execute.valid &&
		!p.memory.valid && !p.writeback.valid {
		p.done = true
	}

	return !p.done
}

// --- Stage implementations ---

func (p *Pipeline) stageWriteback() {
	if !p.writeback.valid {
		return
	}

	// Clear scoreboard for destination registers.
	gpr, fp := destRegs(p.writeback.inst)
	if gpr >= 0 {
		p.gprPending[gpr]--
	}
	if fp >= 0 {
		p.fpPending[fp]--
	}
	if gpr2, fp2 := destRegs2(p.writeback.inst); gpr2 >= 0 || fp2 >= 0 {
		if gpr2 >= 0 {
			p.gprPending[gpr2]--
		}
		if fp2 >= 0 {
			p.fpPending[fp2]--
		}
	}

	p.InstDone++
	p.writeback = stageReg{}
}

func (p *Pipeline) stageMemory() {
	if !p.memory.valid {
		return
	}

	if p.memory.cyclesLeft > 0 {
		p.memory.cyclesLeft--
		return
	}

	// Move to writeback if writeback is empty.
	if p.writeback.valid {
		return // structural hazard — stall
	}

	p.writeback = p.memory
	p.memory = stageReg{}
}

// stageExecute returns true if downstream is stalled (memory stage busy).
func (p *Pipeline) stageExecute() bool {
	if !p.execute.valid {
		return false
	}

	// First cycle: call the emulator for functional execution.
	if !p.execute.executed {
		p.Emu.Mem.ClearAccesses()
		inst, branched, err := p.Emu.StepDetailed()
		if err != nil {
			p.Err = err
			p.done = true
			return true
		}
		_ = inst // already have it in execute.inst
		p.execute.executed = true
		p.execute.branched = branched
		p.execute.memLatency = p.computeMemLatency()

		// Handle taken branch: flush fetch and decode stages.
		// This also covers halt (RET to haltPC): flush + pipeline drains.
		if branched {
			p.flush()
		}
	}

	if p.execute.cyclesLeft > 0 {
		p.execute.cyclesLeft--
		return false
	}

	// Move to memory if memory is empty.
	if p.memory.valid {
		return true // stall
	}

	p.memory = p.execute
	p.memory.cyclesLeft = p.memory.memLatency
	p.execute = stageReg{}
	return false
}

// stageDecode returns true if decode is stalled (data hazard).
func (p *Pipeline) stageDecode() bool {
	if !p.decode.valid {
		return false
	}

	// Check scoreboard for source register hazards.
	for _, idx := range sourceGPRs(p.decode.inst) {
		if p.gprPending[idx] > 0 {
			return true // stall: data hazard
		}
	}
	for _, idx := range sourceFPs(p.decode.inst) {
		if p.fpPending[idx] > 0 {
			return true // stall: data hazard
		}
	}

	// Move to execute if execute is empty.
	if p.execute.valid {
		return true // structural hazard
	}

	// Set execute latency.
	p.decode.cyclesLeft = ExecuteLatency(p.decode.inst.Op) - 1

	// Mark destination registers as pending in scoreboard.
	gpr, fp := destRegs(p.decode.inst)
	if gpr >= 0 {
		p.gprPending[gpr]++
	}
	if fp >= 0 {
		p.fpPending[fp]++
	}
	if gpr2, fp2 := destRegs2(p.decode.inst); gpr2 >= 0 || fp2 >= 0 {
		if gpr2 >= 0 {
			p.gprPending[gpr2]++
		}
		if fp2 >= 0 {
			p.fpPending[fp2]++
		}
	}

	p.execute = p.decode
	p.decode = stageReg{}
	return false
}

func (p *Pipeline) stageFetch() {
	if p.decode.valid {
		return // decode occupied — stall fetch
	}

	// Read 4 bytes at fetchPC from emulator memory.
	var buf [4]byte
	for i := range buf {
		buf[i] = p.Emu.Mem.ReadU8(p.fetchPC + uint64(i))
	}

	inst, err := arm64asm.Decode(buf[:])
	if err != nil {
		// Speculative fetch past program end — insert bubble.
		// The pipeline will flush or halt before this matters.
		return
	}

	p.decode = stageReg{
		valid: true,
		pc:    p.fetchPC,
		inst:  inst,
	}
	p.fetchPC += 4 // predict not-taken
}

// computeMemLatency determines the memory stage latency for the instruction
// currently in the execute stage, based on tracked memory accesses and cache.
func (p *Pipeline) computeMemLatency() int {
	if p.Cache == nil {
		if IsLoad(p.execute.inst.Op) {
			return p.LoadLatency - 1
		}
		return 0
	}

	accesses := p.Emu.Mem.Accesses
	if len(accesses) == 0 {
		return 0
	}

	worstLevel := mem.HitL1
	for _, acc := range accesses {
		level := p.Cache.Lookup(acc.Addr, acc.Size, acc.IsWrite)
		if level > worstLevel {
			worstLevel = level
		}
	}

	cfg := p.Cache.Config()
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
func (p *Pipeline) flush() {
	p.fetch = stageReg{}
	p.decode = stageReg{}
	p.fetchPC = p.Emu.Regs.PC
}

