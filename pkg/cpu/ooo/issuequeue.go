package ooo

import (
	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"golang.org/x/arch/arm64/arm64asm"
)

// The FP issue queue (Spec.FPIssueQueue) holds FP work between dispatch and
// issue: operations on the FP pipes and the data of an FP store. Dispatch
// stops while the queue is full, so a chain of dependent FP operations can run
// only that many links ahead of its oldest unissued one, however large the
// window is.
//
// The multiply issue queue (Spec.MultiplyIssueQueue) holds integer multiplies
// in the same way and bounds a chain of dependent multiplies alike.
//
// An entry is held from the cycle its instruction is dispatched in until the
// cycle the instruction issues, and for the whole dispatch cycle at least:
// dispatch can reuse it in the cycle the instruction issues, but never in the
// cycle that dispatched it, even when the instruction issues at once.

// queuesFP reports whether an instruction waits in the FP issue queue.
func queuesFP(inst arm64asm.Inst, fpSources []int) bool {
	switch classifyUnit(inst) {
	case unitFP, unitFPDiv:
		return true
	case unitStore:
		return len(fpSources) > 0
	}
	return false
}

// queuesMultiply reports whether an instruction waits in the multiply issue
// queue.
func queuesMultiply(op arm64asm.Op) bool {
	switch op {
	case arm64asm.MUL, arm64asm.MADD, arm64asm.MSUB, arm64asm.MNEG,
		arm64asm.SMULL, arm64asm.UMULL, arm64asm.SMADDL, arm64asm.UMADDL,
		arm64asm.SMSUBL, arm64asm.UMSUBL, arm64asm.SMNEGL, arm64asm.UMNEGL,
		arm64asm.SMULH, arm64asm.UMULH:
		return true
	}
	return false
}

// holdEntry takes an entry of queue for an instruction dispatched in the
// current cycle that issues at issueCycle.
func (p *middleware) holdEntry(queue *[]uint64, issueCycle uint64) {
	*queue = append(*queue, max(issueCycle, p.comp.State.Cycles+1))
}

// queueFull forgets the entries of queue that are free again and reports
// whether none of its capacity is left for another instruction.
func (p *middleware) queueFull(queue *[]uint64, capacity int) bool {
	kept := (*queue)[:0]
	for _, free := range *queue {
		if free > p.comp.State.Cycles {
			kept = append(kept, free)
		}
	}
	*queue = kept
	return len(kept) >= capacity
}

// scalarStoreDataReady reads only the stored register. A delayed base or
// cache allocation holds the ordinary store slot, but not the store data.
func (p *middleware) scalarStoreDataReady(inst arm64asm.Inst) (uint64, bool) {
	if inst.Op != arm64asm.STR {
		return 0, false
	}
	address, ok := inst.Args[1].(arm64asm.MemImmediate)
	if !ok || address.Mode != arm64asm.AddrOffset {
		return 0, false
	}
	reg, ok := inst.Args[0].(arm64asm.Reg)
	if !ok {
		return 0, false
	}
	if i := instinfo.GprIdx(reg); i >= 0 {
		if i == 31 {
			return 0, true
		}
		return p.comp.State.GprReady[i], true
	}
	if (reg >= arm64asm.S0 && reg <= arm64asm.S31) || (reg >= arm64asm.D0 && reg <= arm64asm.D31) {
		return p.comp.State.FpReady[instinfo.FpIdx(reg)], true
	}
	return 0, false
}
