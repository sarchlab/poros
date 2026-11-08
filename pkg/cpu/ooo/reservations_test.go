package ooo

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
	"golang.org/x/arch/arm64/arm64asm"
)

func reservationPipeline(t *testing.T, words []uint32, cache *mem.Cache) *middleware {
	t.Helper()
	em := emu.NewEmulator()
	em.Mem.TrackAccesses = true
	em.Regs.PC = 0x1000
	for i, word := range words {
		em.Mem.WriteU32(0x1000+uint64(4*i), word)
	}
	spec := DefaultSpec()
	spec.FetchWidth = len(words)
	spec.NumINT = 1
	spec.NumFP = 1
	spec.NumLDST = 1
	spec.NumMSHREntries = 1
	simulation := sim.MakeBuilder().Build()
	t.Cleanup(func() {
		if err := simulation.Terminate(); err != nil {
			t.Error(err)
		}
	})
	core := modeling.MakeBuilder[*Comp](spec).WithSimulation(simulation).WithResources(Resources{Emu: em, Cache: cache}).Build("CPU")
	if err := simulation.Initialize(); err != nil {
		t.Fatal(err)
	}
	core.State.Cycles = 1
	return core.Middlewares.Pipeline
}

func TestIndependentInstructionsFillFutureReservationGaps(t *testing.T) {
	for _, tc := range []struct {
		name    string
		words   []uint32
		fp      bool
		latency uint64
	}{
		{"integer", []uint32{0x91000422, 0x91000463}, false, 1},       // ADD X2,X1,#1; ADD X3,X3,#1
		{"floating point", []uint32{0x1e622820, 0x1e652883}, true, 3}, // FADD D0,D1,D2; FADD D3,D4,D5
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := reservationPipeline(t, tc.words, nil)
			if tc.fp {
				p.comp.State.FpReady[1] = 100
			} else {
				p.comp.State.GprReady[1] = 100
			}
			p.fetchAndProcess()
			if p.comp.State.Err != "" {
				t.Fatal(p.comp.State.Err)
			}
			first, second := p.comp.State.Rob[0], p.comp.State.Rob[1]
			if first != 100+tc.latency || second != 1+tc.latency {
				t.Fatalf("dependent=%d independent=%d", first, second)
			}
			ut := unitINT
			if tc.fp {
				ut = unitFP
			}
			want := []UnitReservation{{1, 2}, {100, 101}}
			if !reflect.DeepEqual(p.units(ut)[0], want) {
				t.Fatalf("reservations %+v", p.units(ut)[0])
			}
		})
	}
}

func TestForwardingDelayRechecksExecutionUnit(t *testing.T) {
	p := reservationPipeline(t, []uint32{0xf9400020}, nil) // LDR X0,[X1]
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.Resources.Emu.Mem.WriteU64(0x4000, 42)
	p.comp.State.StoreBuffer = []storeBufferEntry{{Addr: 0x4000, Size: 8, CompleteCycle: 10}}
	p.comp.State.LdstUnits[0] = []UnitReservation{{10, 12}}
	p.fetchAndProcess()
	if p.comp.State.Err != "" {
		t.Fatal(p.comp.State.Err)
	}
	if p.comp.State.GprReady[0] != 17 || p.comp.Resources.Emu.Regs.ReadX(0) != 42 {
		t.Fatalf("ready=%d", p.comp.State.GprReady[0])
	}
	if want := []UnitReservation{{10, 12}, {12, 13}}; !reflect.DeepEqual(p.comp.State.LdstUnits[0], want) {
		t.Fatalf("overlap or ghost reservation: %+v", p.comp.State.LdstUnits[0])
	}
	if p.comp.State.MshrBusyUntil[0] != 0 {
		t.Fatal("forwarded load allocated MSHR")
	}
}

func TestExactForwardingUsesGapButWaitsForStoreData(t *testing.T) {
	p := reservationPipeline(t, []uint32{0xf9400020}, nil)
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.Resources.Emu.Mem.WriteU64(0x4000, 42)
	p.comp.State.StoreBuffer = []storeBufferEntry{{Addr: 0x4000, Size: 8, ReadyCycle: 10, CompleteCycle: 13, FastForwardEligible: true}}
	p.comp.State.LdstUnits[0] = []UnitReservation{{20, 21}}
	p.fetchAndProcess()
	if p.comp.State.Err != "" {
		t.Fatal(p.comp.State.Err)
	}
	if p.comp.State.GprReady[0] != 11 {
		t.Fatalf("load bypassed data readiness or lost forwarding: %d", p.comp.State.GprReady[0])
	}
	if want := []UnitReservation{{1, 2}, {20, 21}}; !reflect.DeepEqual(p.comp.State.LdstUnits[0], want) {
		t.Fatalf("reservations %+v", p.comp.State.LdstUnits[0])
	}
	if p.comp.State.MshrBusyUntil[0] != 0 {
		t.Fatal("exact forwarded load allocated MSHR")
	}
}

func TestMSHRDelayRechecksExecutionUnitBeforeBooking(t *testing.T) {
	p := reservationPipeline(t, []uint32{0xf9400020}, mem.NewCache(mem.DefaultCacheConfig()))
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.Resources.Emu.Mem.WriteU64(0x4000, 42)
	p.comp.State.MshrBusyUntil[0] = 10
	p.comp.State.LdstUnits[0] = []UnitReservation{{10, 12}, {20, 21}}
	p.fetchAndProcess()
	if p.comp.State.Err != "" {
		t.Fatal(p.comp.State.Err)
	}
	if p.comp.State.GprReady[0] != 347 || p.comp.State.MshrBusyUntil[0] != 347 {
		t.Fatalf("load=%d MSHR=%d", p.comp.State.GprReady[0], p.comp.State.MshrBusyUntil[0])
	}
	want := []UnitReservation{{10, 12}, {12, 13}, {20, 21}}
	if !reflect.DeepEqual(p.comp.State.LdstUnits[0], want) {
		t.Fatalf("reservations %+v", p.comp.State.LdstUnits[0])
	}
}

func TestUnitReservationsUseWholeOccupancyAndEarliestAvailableUnit(t *testing.T) {
	p := &middleware{comp: &Comp{State: State{IntUnits: [][]UnitReservation{{{10, 20}}, {{15, 25}}}}}}
	if unit, start := p.findUnit(unitINT, 8, 4); unit != 1 || start != 8 {
		t.Fatalf("unit=%d start=%d", unit, start)
	}
	if got := p.reserveIssue(unitINT, 8, unitOccupancy(arm64asm.SDIV), false, 12); got != 20 {
		t.Fatalf("nonpipelined reservation starts %d", got)
	}
	if unitOccupancy(arm64asm.FDIV) != 10 || unitOccupancy(arm64asm.FSQRT) != 6 || unitOccupancy(arm64asm.FADD) != 1 {
		t.Fatal("occupancy changed")
	}
}

func TestUnitReservationSearchMatchesDiscreteResourceOracle(t *testing.T) {
	p := &middleware{comp: &Comp{State: State{IntUnits: make([][]UnitReservation, 3)}}}
	rng := rand.New(rand.NewSource(7))
	occupied := [3]map[uint64]bool{{}, {}, {}}
	for n := 0; n < 300; n++ {
		ready, duration := uint64(rng.Intn(100)), uint64(1+rng.Intn(12))
		expected := ready
		for {
			found := false
			for _, unit := range occupied {
				free := true
				for c := expected; c < expected+duration; c++ {
					if unit[c] {
						free = false
						break
					}
				}
				if free {
					found = true
					break
				}
			}
			if found {
				break
			}
			expected++
		}
		unit, _ := p.findUnit(unitINT, ready, duration)
		got := p.reserveIssue(unitINT, ready, duration, false, 1)
		if got != expected {
			t.Fatalf("request %d: got %d want %d", n, got, expected)
		}
		for c := got; c < got+duration; c++ {
			if occupied[unit][c] {
				t.Fatal("overbooked unit")
			}
			occupied[unit][c] = true
		}
	}
}

func TestReservationCheckpointPreservesGapsAndPrunesOnlyPast(t *testing.T) {
	original := State{Cycles: 5, IntUnits: [][]UnitReservation{{{1, 3}, {10, 12}, {100, 101}}}, FpUnits: make([][]UnitReservation, 1), LdstUnits: make([][]UnitReservation, 1)}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, restored) {
		t.Fatal("reservation state did not roundtrip")
	}
	p := &middleware{comp: &Comp{State: restored}}
	p.pruneUnitReservations()
	if got := p.reserveIssue(unitINT, 50, 1, false, 1); got != 50 {
		t.Fatal(got)
	}
	if got := p.reserveIssue(unitINT, 5, 1, false, 1); got != 5 {
		t.Fatal(got)
	}
	want := []UnitReservation{{5, 6}, {10, 12}, {50, 51}, {100, 101}}
	if !reflect.DeepEqual(p.comp.State.IntUnits[0], want) {
		t.Fatalf("lost future booking: %+v", p.comp.State.IntUnits[0])
	}
	if len(original.IntUnits[0]) != 3 {
		t.Fatal("restored state shares original array")
	}
	if err := json.Unmarshal([]byte(`{"IntUnits":[101]}`), &State{}); err == nil {
		t.Fatal("legacy busy-until JSON silently accepted as interval state")
	}
}

// Exercise a steady stream of future-ready work, including multi-cycle units.
// Report allocations so reservation maintenance cannot quietly grow per tick.
func BenchmarkUnitReservations(b *testing.B) {
	p := &middleware{comp: &Comp{State: State{IntUnits: make([][]UnitReservation, 6)}}}
	for i := range p.comp.State.IntUnits {
		p.comp.State.IntUnits[i] = make([]UnitReservation, 0, 64)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.comp.State.Cycles = uint64(i / 4)
		if i%4 == 0 {
			p.pruneUnitReservations()
		}
		ready := p.comp.State.Cycles + uint64(i%31)
		p.reserveIssue(unitINT, ready, 1, false, 1)
	}
}
