package ooo

import "golang.org/x/arch/arm64/arm64asm"

// The FP issue queue (Spec.FPIssueQueue) holds FP work between dispatch and
// issue: operations on the FP pipes and the data of an FP store. Dispatch
// stops while the queue is full, so a chain of dependent FP operations can run
// only that many links ahead of its oldest unissued one, however large the
// window is.
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

// holdFPEntry takes an entry for an instruction dispatched in the current
// cycle that issues at issueCycle.
func (p *middleware) holdFPEntry(issueCycle uint64) {
	p.comp.State.FpQueue = append(p.comp.State.FpQueue, max(issueCycle, p.comp.State.Cycles+1))
}

// fpQueueFull forgets the entries that are free again and reports whether no
// entry is left for another instruction.
func (p *middleware) fpQueueFull() bool {
	queue := p.comp.State.FpQueue
	kept := queue[:0]
	for _, free := range queue {
		if free > p.comp.State.Cycles {
			kept = append(kept, free)
		}
	}
	p.comp.State.FpQueue = kept
	return len(kept) >= p.comp.Spec.FPIssueQueue
}
