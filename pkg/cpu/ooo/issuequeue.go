package ooo

import "golang.org/x/arch/arm64/arm64asm"

// The FP issue queue (Spec.FPIssueQueue) holds FP work between dispatch and
// issue: operations on the FP pipes and the data of an FP store. Dispatch
// stops while the queue is full, so a chain of dependent FP operations can run
// only that many links ahead of its oldest unissued one, however large the
// window is. Work that issues in its dispatch cycle still takes an entry for
// that cycle.

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

// fpQueueFull forgets the entries that have issued and reports whether no
// entry is left for another instruction.
func (p *middleware) fpQueueFull() bool {
	queue := p.comp.State.FpQueue
	kept := queue[:0]
	for _, issue := range queue {
		if issue > p.comp.State.Cycles {
			kept = append(kept, issue)
		}
	}
	p.comp.State.FpQueue = kept
	return len(kept) >= p.comp.Spec.FPIssueQueue
}
