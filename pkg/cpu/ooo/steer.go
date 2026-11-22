package ooo

import (
	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"golang.org/x/arch/arm64/arm64asm"
)

// Dispatch assigns an integer operation before its inputs are necessarily
// ready. Each pipe has its own waiting work. Among equally short queues,
// start with a rotating preferred pipe. The selected pipe still issues ready
// younger work into gaps left by an older operation waiting for an operand.
func (p *middleware) steerInteger(inst arm64asm.Inst, ready, occupancy uint64) uint64 {
	units := p.comp.State.IntUnits
	switch {
	case instinfo.IsBranch(inst.Op):
		units = units[:p.comp.Spec.NumBranchPipes]
	case instinfo.SetsFlags(inst.Op):
		units = units[:p.comp.Spec.NumFlagPipes]
	}
	start := int(p.comp.State.IntegerDispatch % uint64(len(units)))
	p.comp.State.IntegerDispatch++
	best := start
	for i := 1; i < len(units); i++ {
		unit := (start + i) % len(units)
		if len(units[unit]) < len(units[best]) {
			best = unit
		}
	}
	issue := firstUnitGap(units[best], ready, occupancy)
	p.book(unitINT, best, issue, occupancy)
	return issue
}
