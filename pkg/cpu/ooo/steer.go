package ooo

import (
	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"golang.org/x/arch/arm64/arm64asm"
)

// When integer issue contains only known arithmetic results, select the
// earliest available eligible pipe. A load-fed integer consumer waiting for
// its data makes future issue slots uncertain at dispatch. Until that operand
// arrives, steer by outstanding queue lengths and rotate the preferred pipe
// on ties. Neither assignment changes an existing instruction's issue cycle.
func (p *middleware) steerInteger(inst arm64asm.Inst, ready, occupancy uint64) uint64 {
	units := p.comp.State.IntUnits
	switch {
	case instinfo.IsBranch(inst.Op):
		units = units[:p.comp.Spec.NumBranchPipes]
	case instinfo.SetsFlags(inst.Op):
		units = units[:p.comp.Spec.NumFlagPipes]
	}
	if p.comp.State.LoadConsumerReady <= p.comp.State.Cycles {
		best := 0
		issue := firstUnitGap(units[0], ready, occupancy)
		for i := 1; i < len(units) && issue > ready; i++ {
			if at := firstUnitGap(units[i], ready, occupancy); at < issue {
				best, issue = i, at
			}
		}
		p.book(unitINT, best, issue, occupancy)
		return issue
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

// Record the producer kind alongside architectural register renaming. Base
// writeback is an address calculation, independent of the loaded value.
func (s *State) setLoadProducer(reg int, loaded bool) {
	if reg < 0 || reg >= 31 {
		return
	}
	bit := uint32(1) << reg
	if loaded {
		s.LoadResults |= bit
	} else {
		s.LoadResults &^= bit
	}
}
