package ooo

import (
	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"golang.org/x/arch/arm64/arm64asm"
)

const (
	// A fused pair must lie within one instruction line of this many bytes.
	fusionLineBytes = 64
	// The largest compare immediate in a fused pair.
	fusionImmediateLimit = 15
)

// fusedSelect links an admitted compare to the select that completes its
// pair. Both are dispatched in one cycle, so it never outlives one
// fetchAndProcess call and is not part of State.
type fusedSelect struct {
	pc       uint64 // address of the select
	complete uint64 // cycle at which the pair's flags and result are ready
	pending  bool
}

// fusedBranch links an admitted compare to the conditional branch that
// completes its pair (Spec.CompareBranchFusion). Like fusedSelect it lives
// within one fetchAndProcess call.
type fusedBranch struct {
	pc       uint64 // address of the branch
	issue    uint64 // cycle in which the pair issued; the branch resolves then
	complete uint64 // cycle at which both halves complete
	pending  bool
}

// compareSelectFuses reports whether a compare and the select after it have a
// fusible form: a 32-bit CMP of two plain registers, or of a register and an
// unshifted immediate up to 15, followed by a 32-bit CSEL, where the two
// instructions together read at most two different registers. The measured
// instances are a register compare with a select between its operands, in
// either order and into any destination, and a compare with 15 followed by a
// select between the compared register and one other. 64-bit pairs, shifted
// or extended compares, larger immediates, other conditional selects, the zero
// register and a select of registers that were not compared keep separate
// timing; see evaluation/microbenchmarks/recurrence/README.md.
func compareSelectFuses(compare, selection arm64asm.Inst) bool {
	if compare.Op != arm64asm.CMP || selection.Op != arm64asm.CSEL {
		return false
	}
	n, ok := wordRegister(compare.Args[0])
	if !ok {
		return false
	}
	m := n
	if register, unmodified := instinfo.UnmodifiedReg(compare.Args[1]); unmodified {
		if m, ok = wordRegister(register); !ok {
			return false
		}
	} else if immediate, unshifted := instinfo.UnshiftedImm(compare.Args[1]); !unshifted || immediate > fusionImmediateLimit {
		return false
	}
	_, destination := wordRegister(selection.Args[0])
	first, firstWord := wordRegister(selection.Args[1])
	second, secondWord := wordRegister(selection.Args[2])
	if !destination || !firstWord || !secondWord {
		return false
	}
	read, different := [4]arm64asm.Reg{n, m, first, second}, 0
	for i, register := range read {
		repeated := false
		for _, earlier := range read[:i] {
			repeated = repeated || earlier == register
		}
		if !repeated {
			different++
		}
	}
	return different <= 2
}

// wordRegister returns a W0-W30 operand; the zero register and the stack
// pointer are excluded.
func wordRegister(arg arm64asm.Arg) (arm64asm.Reg, bool) {
	var register arm64asm.Reg
	switch operand := arg.(type) {
	case arm64asm.Reg:
		register = operand
	case arm64asm.RegSP:
		register = arm64asm.Reg(operand)
	default:
		return 0, false
	}
	return register, register >= arm64asm.W0 && register <= arm64asm.W30
}

// fusesWithBranch reports whether inst and the branch after it form one
// operation (Spec.CompareBranchFusion): a flag-setting integer compare whose
// second operand is an immediate or an unshifted, unextended register,
// followed by B.cond; or an ADD/SUB of an immediate followed by CBZ/CBNZ of
// the register it wrote. The M5 Max NOP-loop probes measured X-register
// SUBS, CMP, CMN, ADDS, ANDS and TST and a W-register CMP with B.NE or B.GT,
// and SUB with CBNZ; a shifted-register CMP and a pair separated by a NOP did
// not fuse. The other register widths and immediate forms, mixed W/X views,
// ADD and CBZ extend those measured pairs without measurements of their own.
func fusesWithBranch(inst, next arm64asm.Inst) bool {
	if !branchFusionProducer(inst) {
		return false
	}
	if inst.Op == arm64asm.ADD || inst.Op == arm64asm.SUB {
		return (next.Op == arm64asm.CBZ || next.Op == arm64asm.CBNZ) && instinfo.ArgGPR(next.Args[0]) == instinfo.ArgGPR(inst.Args[0])
	}
	return conditionalBranch(next)
}

// branchFusionProducer reports whether inst has the form of the first half of
// a compare/branch pair; only such an instruction reads the one after it.
func branchFusionProducer(inst arm64asm.Inst) bool {
	operand := lastArg(inst)
	switch inst.Op {
	case arm64asm.CMP, arm64asm.CMN, arm64asm.TST, arm64asm.SUBS, arm64asm.ADDS, arm64asm.ANDS:
		if _, register := operand.(arm64asm.RegExtshiftAmount); register {
			_, plain := instinfo.UnmodifiedReg(operand)
			return plain
		}
		return true
	case arm64asm.ADD, arm64asm.SUB:
		_, immediate := operand.(arm64asm.ImmShift)
		written := instinfo.ArgGPR(inst.Args[0])
		return immediate && written >= 0 && written < 31
	}
	return false
}

// deliversBranchPair reports whether inst at pc can open a compare/branch pair
// that this cycle admits whole: it has a producer's form, two integer pipes
// and two ROB entries exist for the pair, and with fetch groups the branch
// belongs to inst's group. A pair that cannot be delivered whole runs as two
// ordinary instructions.
func (p *middleware) deliversBranchPair(inst arm64asm.Inst, pc uint64) bool {
	spec := p.comp.Spec
	return branchFusionProducer(inst) && spec.NumINT >= 2 && p.comp.State.RobCount+2 <= spec.ROBCapacity &&
		(spec.FetchGroupLimit == 0 || p.continuesFetchGroup(pc, pc+4))
}

// lastArg is an instruction's last operand.
func lastArg(inst arm64asm.Inst) arm64asm.Arg {
	var last arm64asm.Arg
	for _, arg := range inst.Args {
		if arg == nil {
			break
		}
		last = arg
	}
	return last
}

// conditionalBranch reports B.cond.
func conditionalBranch(inst arm64asm.Inst) bool {
	if inst.Op != arm64asm.B {
		return false
	}
	for _, arg := range inst.Args {
		if _, ok := arg.(arm64asm.Cond); ok {
			return true
		}
	}
	return false
}

// countsAsConditional reports the branches bounded by Spec.ConditionalBranches.
func countsAsConditional(inst arm64asm.Inst) bool {
	switch inst.Op {
	case arm64asm.CBZ, arm64asm.CBNZ, arm64asm.TBZ, arm64asm.TBNZ:
		return true
	}
	return conditionalBranch(inst)
}

// redirect restarts fetch at cycle after a misprediction or another
// unpredicted change of the PC; with fetch groups, the fetch buffer empties
// and a new group starts there.
func (p *middleware) redirect(cycle uint64) {
	p.comp.State.FrontendReady = cycle
	if p.comp.Spec.FetchGroupLimit > 0 {
		p.comp.State.FetchGroupCycle = cycle - 1
		p.comp.State.FetchGroupEnded = true
		clear(p.comp.State.FetchGroupRenamed)
	}
}

// startFetchGroup fetches the next group, starting at pc: one cycle after
// the previous group, and only once the fetch buffer has room, i.e. the cycle
// after the group FetchLead+1 places earlier began rename.
func (p *middleware) startFetchGroup(pc uint64) {
	s := &p.comp.State
	renamed := s.FetchGroupRenamed
	slot := s.FetchGroups % uint64(len(renamed))
	s.FetchGroupCycle = max(s.FetchGroupCycle+1, renamed[slot]+1)
	// Until the group's first instruction is renamed, assume it is renamed
	// when fetched; the dispatch loop records the actual cycle.
	renamed[slot] = max(s.FetchGroupCycle, s.Cycles)
	s.FetchGroups++
	s.FetchGroupStart = pc
	s.FetchGroupSize = 0
	s.FetchGroupEnded = false
}

// continuesFetchGroup reports whether the instruction at next, executed after
// the one at pc, belongs to the current fetch group: it must lie ahead of pc
// inside the group's window of FetchGroupLimit instruction slots.
func (p *middleware) continuesFetchGroup(pc, next uint64) bool {
	end := p.comp.State.FetchGroupStart + 4*uint64(p.comp.Spec.FetchGroupLimit)
	return next > pc && next < end
}

// fusedPartner returns the register sources of the select after inst when the
// two form a fusible pair. A pair needs two fetch slots and two ROB entries
// in one cycle; a configuration that can never provide them, a pair
// straddling an instruction line or a fetch group and an unreadable next
// instruction do not fuse.
func (p *middleware) fusedPartner(inst arm64asm.Inst, pc uint64) ([]int, bool) {
	spec := p.comp.Spec
	if spec.NumSelectFusion == 0 || inst.Op != arm64asm.CMP || spec.FetchWidth < 2 || spec.ROBCapacity < 2 ||
		pc/fusionLineBytes != (pc+4)/fusionLineBytes || spec.FetchGroupLimit > 0 && !p.continuesFetchGroup(pc, pc+4) {
		return nil, false
	}
	next, sources, _, err := p.comp.Resources.peekSources(pc + 4)
	if err != nil || !compareSelectFuses(inst, next) {
		return nil, false
	}
	return sources, true
}
