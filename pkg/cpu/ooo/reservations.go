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
	for _, units := range [][][]UnitReservation{p.comp.State.IntUnits, p.comp.State.FpUnits, p.comp.State.LdstUnits, p.comp.State.StoreUnits} {
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
	return findResource(p.units(ut), earliest, occupancy)
}

func findResource(units [][]UnitReservation, earliest, occupancy uint64) (int, uint64) {
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
	units[unit] = insertReservation(units[unit], issue, occupancy)
	if mshr >= 0 {
		p.comp.State.MshrBusyUntil[mshr] = issue + latency
	}
	return issue
}

func insertReservation(list []UnitReservation, issue, occupancy uint64) []UnitReservation {
	reservation := UnitReservation{Start: issue, End: issue + occupancy}
	if len(list) == 0 || list[len(list)-1].End <= issue {
		list = append(list, reservation)
	} else {
		pos := sort.Search(len(list), func(i int) bool { return list[i].Start >= issue })
		list = append(list, UnitReservation{})
		copy(list[pos+1:], list[pos:])
		list[pos] = reservation
	}
	return list
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

// storeTransfers counts architectural transfers, not bytes or assumed physical
// micro-ops: a single scalar/Q store is one, STP is two. The empirical scalar
// throughput anchor does not establish vector store-port widths.
func storeTransfers(inst arm64asm.Inst) uint64 {
	if inst.Op == arm64asm.STP {
		return 2
	}
	return 1
}

// reserveStoreIssue finds a common gap before committing either resource.
// A pair occupies a store lane for two cycles but one shared AGU issue slot;
// functional store values and the existing result latency are unchanged.
func (p *middleware) reserveStoreIssue(earliest, transfers uint64) uint64 {
	for {
		agu, at := p.findUnit(unitLDST, earliest, 1)
		store, both := findResource(p.comp.State.StoreUnits, at, transfers)
		if both != at {
			earliest = both
			continue
		}
		p.comp.State.LdstUnits[agu] = insertReservation(p.comp.State.LdstUnits[agu], at, 1)
		p.comp.State.StoreUnits[store] = insertReservation(p.comp.State.StoreUnits[store], at, transfers)
		return at
	}
}
