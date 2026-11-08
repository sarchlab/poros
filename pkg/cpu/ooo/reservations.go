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
	case unitFP:
		return p.comp.State.FpUnits
	default:
		return p.comp.State.LdstUnits
	}
}

// Discard only intervals that have ended at the actual pipeline cycle. A
// future operand-ready time must not erase reservations needed by younger work.
func (p *middleware) pruneUnitReservations() {
	for _, units := range [][][]UnitReservation{p.comp.State.IntUnits, p.comp.State.FpUnits, p.comp.State.LdstUnits} {
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
	best, start := 0, firstUnitGap(units[0], earliest, occupancy)
	if start == earliest {
		return best, start
	}
	for i := 1; i < len(units); i++ {
		if candidate := firstUnitGap(units[i], earliest, occupancy); candidate < start {
			best, start = i, candidate
			if start == earliest {
				return best, start
			}
		}
	}
	return best, start
}

// reserveIssue commits one unit interval and, for a miss, one MSHR. MSHRs
// retain the existing busy-until policy: once the earliest slot is available,
// it stays available at any later unit-selected time. No tentative reservation
// or iterative release/rebook operation is needed.
func (p *middleware) reserveIssue(ut unitType, earliest, occupancy uint64, miss bool, latency uint64) uint64 {
	mshr := -1
	if miss {
		mshr, earliest = p.acquireMSHR(earliest)
	}
	unit, issue := p.findUnit(ut, earliest, occupancy)
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
	if mshr >= 0 {
		p.comp.State.MshrBusyUntil[mshr] = issue + latency
	}
	return issue
}

// Occupancy controls issue throughput; it is independent of result latency.
func unitOccupancy(op arm64asm.Op) uint64 {
	switch op {
	case arm64asm.SDIV, arm64asm.UDIV:
		return uint64(instinfo.ExecuteLatency(op))
	case arm64asm.FDIV:
		return 10
	case arm64asm.FSQRT:
		return 6
	default:
		return 1
	}
}
