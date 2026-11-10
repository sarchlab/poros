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

// fusedPartner returns the register sources of the select after inst when the
// two form a fusible pair. A pair needs two fetch slots and two ROB entries
// in one cycle; a configuration that can never provide them, a pair
// straddling an instruction line and an unreadable next instruction do not
// fuse.
func (p *middleware) fusedPartner(inst arm64asm.Inst, pc uint64) ([]int, bool) {
	spec := p.comp.Spec
	if spec.NumSelectFusion == 0 || inst.Op != arm64asm.CMP || spec.FetchWidth < 2 || spec.ROBCapacity < 2 ||
		pc/fusionLineBytes != (pc+4)/fusionLineBytes {
		return nil, false
	}
	next, sources, _, err := p.comp.Resources.peekSources(pc + 4)
	if err != nil || !compareSelectFuses(inst, next) {
		return nil, false
	}
	return sources, true
}
