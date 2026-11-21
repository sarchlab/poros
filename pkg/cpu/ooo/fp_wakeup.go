package ooo

import (
	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"golang.org/x/arch/arm64/arm64asm"
)

type faddProducer struct {
	Position uint64
	Fraction uint64
	Valid    bool
}

// Position counts effective slots in program order. Each live FP producer
// holds its position and the uncharged fraction inherited along its chain.
type faddWakeupState struct {
	Position  uint64
	Producers [32]faddProducer
}

func scalarFAdd64(inst arm64asm.Inst) bool {
	dest, ok := inst.Args[0].(arm64asm.Reg)
	return inst.Op == arm64asm.FADD && ok && dest >= arm64asm.D0 && dest <= arm64asm.D31
}

func (p *middleware) faddOperandReady(state *faddWakeupState, register int, ready uint64) (uint64, uint64) {
	producer := state.Producers[register]
	if !producer.Valid {
		return ready, 0
	}
	spec := p.comp.Spec
	width := uint64(spec.FetchWidth)
	distance := state.Position - producer.Position
	// Sparse chains already give the ordinary result enough dispatch
	// distance. The measured extra cost applies only to nearby consumers.
	latency := uint64(spec.Latencies.FPArithmetic)
	if latency == 0 {
		latency = uint64(instinfo.ExecuteLatency(arm64asm.FADD))
	}
	if distance >= width*latency {
		return ready, 0
	}
	fraction := producer.Fraction + min(distance, uint64(spec.Latencies.FAddWakeupSpan))
	return ready + fraction/width, fraction % width
}

func (p *middleware) advanceFAddWakeup(state *faddWakeupState, inst arm64asm.Inst, changedPC bool) {
	if !instinfo.IsLoad(inst.Op) {
		// Adding independent ready loads had little effect on the measured
		// carry chain. Data availability still goes through the load path.
		state.Position++
	}
	taken := changedPC
	if !taken && inst.Op == arm64asm.B {
		// StepDetailed exposes changes of PC. A branch to the sequential PC
		// can still be taken and changes the measured wakeup cost.
		taken = true
		for _, arg := range inst.Args {
			if condition, ok := arg.(arm64asm.Cond); ok {
				taken = p.comp.Resources.backend().CPURegisters().EvalCond(condition)
				break
			}
		}
	}
	if taken {
		state.Position += uint64(p.comp.Spec.Latencies.FAddWakeupTaken)
	}
}
