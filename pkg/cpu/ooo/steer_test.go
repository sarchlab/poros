package ooo

import (
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

func TestIntegerDispatchUsesShortestEligibleQueue(t *testing.T) {
	p := &middleware{comp: &Comp{}}
	p.comp.Spec.NumFlagPipes, p.comp.Spec.NumBranchPipes = 3, 2
	p.comp.State.IntUnits = make([][]UnitReservation, 6)
	p.comp.State.IntUnits[0] = []UnitReservation{{Start: 8, End: 9}}
	p.comp.State.IntUnits[1] = []UnitReservation{{Start: 6, End: 7}}
	p.steerInteger(arm64asm.Inst{Op: arm64asm.CMP}, 0, 1)
	if len(p.comp.State.IntUnits[2]) != 1 {
		t.Fatal("flag producer did not select the empty eligible queue")
	}
	p.steerInteger(arm64asm.Inst{Op: arm64asm.B}, 0, 1)
	if len(p.comp.State.IntUnits[1]) != 2 || p.comp.State.IntUnits[1][0].Start != 0 {
		t.Fatal("branch did not use a gap on its eligible pipe")
	}
}
