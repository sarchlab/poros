package ooo

import (
	"sort"

	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"golang.org/x/arch/arm64/arm64asm"
)

// UnitReservation occupies [Start, End) on one execution unit. Reservations
// belong to State so checkpoints retain both future bookings and earlier gaps.
type UnitReservation struct {
	Start uint64
	End   uint64
}

func (p *middleware) units(ut unitType) [][]UnitReservation {
	switch ut {
	case unitINT:
		return p.comp.State.IntUnits
	case unitIntDiv:
		if len(p.comp.State.IntDividers) > 0 {
			return p.comp.State.IntDividers
		}
		return p.comp.State.IntUnits
	case unitSelectFusion:
		return p.comp.State.IntUnits[p.comp.Spec.NumINT-p.comp.Spec.NumSelectFusion:]
	case unitFP:
		return p.comp.State.FpUnits
	case unitFPDiv:
		if len(p.comp.State.FpDividers) > 0 {
			return p.comp.State.FpDividers
		}
		return p.comp.State.FpUnits[:p.comp.Spec.NumFPDiv]
	case unitLoad:
		if p.comp.Spec.NumLoad > 0 {
			return p.comp.State.LdstUnits[:p.comp.Spec.NumLoad]
		}
		return p.comp.State.LdstUnits
	case unitStore:
		if p.comp.Spec.NumStore > 0 {
			return p.comp.State.LdstUnits[p.comp.Spec.NumLDST-p.comp.Spec.NumStore:]
		}
		return p.comp.State.LdstUnits
	default:
		panic("ooo: unknown execution unit type")
	}
}

// Discard only intervals that have ended at the actual pipeline cycle. A
// future operand-ready time must not erase reservations needed by younger work.
func (p *middleware) pruneUnitReservations() {
	for _, units := range [][][]UnitReservation{p.comp.State.IntUnits, p.comp.State.FpUnits, p.comp.State.LdstUnits, p.comp.State.IntDividers, p.comp.State.FpDividers} {
		for i, list := range units {
			n := 0
			for n < len(list) && list[n].End <= p.comp.State.Cycles {
				n++
			}
			if n > 0 {
				copy(list, list[n:])
				units[i] = list[:len(list)-n]
			}
		}
	}
}

func firstUnitGap(list []UnitReservation, earliest, occupancy uint64) uint64 {
	if len(list) == 0 || list[len(list)-1].End <= earliest {
		return earliest
	}
	for _, slot := range list {
		if earliest+occupancy <= slot.Start {
			break
		}
		if slot.End > earliest {
			earliest = slot.End
		}
	}
	return earliest
}

func (p *middleware) findUnit(ut unitType, earliest, occupancy uint64) (int, uint64) {
	units := p.units(ut)
	// Loads scan from the load-only end; stores scan from the store-only end.
	// Prefer dedicated pipes on equal issue times, without delaying a request
	// when a shared pipe has an earlier gap. Fully shared configurations retain
	// the legacy lowest-index tie break.
	best, step := 0, 1
	if ut == unitStore && p.comp.Spec.NumLoad > 0 && p.comp.Spec.NumLoad < p.comp.Spec.NumLDST {
		best, step = len(units)-1, -1
	}
	start := firstUnitGap(units[best], earliest, occupancy)
	if start == earliest {
		return best, start
	}
	for i := best + step; i >= 0 && i < len(units); i += step {
		if candidate := firstUnitGap(units[i], earliest, occupancy); candidate < start {
			best, start = i, candidate
			if start == earliest {
				return best, start
			}
		}
	}
	return best, start
}

// reserveIssue commits the earliest eligible unit interval.
func (p *middleware) reserveIssue(ut unitType, earliest, occupancy uint64) uint64 {
	unit, issue := p.findUnit(ut, earliest, occupancy)
	p.book(ut, unit, issue, occupancy)
	return issue
}

// reserveIssuePair commits the same interval on two units of a type, from the
// earliest start at which two are free together, preferring the lowest-index
// units as findUnit does.
func (p *middleware) reserveIssuePair(ut unitType, earliest, occupancy uint64) uint64 {
	units := p.units(ut)
	if len(units) < 2 {
		panic("ooo: an issue pair needs two execution units")
	}
	for {
		first, second, next := -1, -1, ^uint64(0)
		for i, list := range units {
			if start := firstUnitGap(list, earliest, occupancy); start > earliest {
				next = min(next, start)
			} else if first < 0 {
				first = i
			} else {
				second = i
				break
			}
		}
		if second >= 0 {
			p.book(ut, first, earliest, occupancy)
			p.book(ut, second, earliest, occupancy)
			return earliest
		}
		earliest = next
	}
}

// book reserves [issue, issue+occupancy) on one unit, keeping its list sorted.
func (p *middleware) book(ut unitType, unit int, issue, occupancy uint64) {
	units := p.units(ut)
	list := units[unit]
	reservation := UnitReservation{Start: issue, End: issue + occupancy}
	if len(list) == 0 || list[len(list)-1].End <= issue {
		list = append(list, reservation)
	} else {
		pos := sort.Search(len(list), func(i int) bool { return list[i].Start >= issue })
		list = append(list, UnitReservation{})
		copy(list[pos+1:], list[pos:])
		list[pos] = reservation
	}
	units[unit] = list
}

// Occupancy controls issue throughput; it is independent of result latency.
func unitOccupancy(op arm64asm.Op) uint64 {
	switch op {
	case arm64asm.SDIV, arm64asm.UDIV:
		return uint64(instinfo.ExecuteLatency(op))
	case arm64asm.FDIV:
		return 1
	case arm64asm.FSQRT:
		return 2
	default:
		return 1
	}
}
