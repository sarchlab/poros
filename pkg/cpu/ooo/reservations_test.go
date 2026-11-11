package ooo

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
	"golang.org/x/arch/arm64/arm64asm"
)

// fillConfig is the default cache configuration without address-translation
// timing, for tests that pin fill, MSHR and store-allocation arithmetic on
// first-touch accesses. translation_test.go covers page walks, including the
// complete default path.
func fillConfig() mem.CacheConfig {
	cfg := mem.DefaultCacheConfig()
	cfg.TLBPageBytes = 0
	return cfg
}

func fillCache() *mem.Cache { return mem.NewCache(fillConfig()) }

func reservationPipeline(t *testing.T, words []uint32, cache *mem.Cache) *middleware {
	t.Helper()
	return specPipeline(t, words, cache, nil)
}

// specPipeline starts from one pipe of each kind and a fetch group holding
// the whole program, then applies a test's own configuration.
func specPipeline(t *testing.T, words []uint32, cache *mem.Cache, adjust func(*Spec)) *middleware {
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
	spec.NumLoad, spec.NumStore = 0, 0 // one fully shared pipe
	spec.NumMSHREntries = 1
	if adjust != nil {
		adjust(&spec)
	}
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
	p := reservationPipeline(t, []uint32{0xf9400020}, fillCache())
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
	if got := p.reserveIssue(unitINT, 8, unitOccupancy(arm64asm.SDIV)); got != 20 {
		t.Fatalf("nonpipelined reservation starts %d", got)
	}
	if unitOccupancy(arm64asm.FDIV) != 1 || unitOccupancy(arm64asm.FSQRT) != 2 || unitOccupancy(arm64asm.FADD) != 1 {
		t.Fatal("occupancy changed")
	}
}

func TestUnitReservationFastPathsKeepFirstUnitOnTies(t *testing.T) {
	p := &middleware{comp: &Comp{State: State{IntUnits: [][]UnitReservation{{{10, 20}}, nil}}}}
	// Both units can issue before the first future reservation.
	if unit, issue := p.findUnit(unitINT, 5, 1); unit != 0 || issue != 5 {
		t.Fatalf("gap tie: unit=%d issue=%d", unit, issue)
	}
	// Both can also issue beyond the tail. Preserve the same lowest-index tie.
	if unit, issue := p.findUnit(unitINT, 30, 4); unit != 0 || issue != 30 {
		t.Fatalf("tail tie: unit=%d issue=%d", unit, issue)
	}
	if got := p.reserveIssue(unitINT, 30, 4); got != 30 {
		t.Fatal(got)
	}
	if want := []UnitReservation{{10, 20}, {30, 34}}; !reflect.DeepEqual(p.comp.State.IntUnits[0], want) {
		t.Fatalf("append changed reservations: %+v", p.comp.State.IntUnits[0])
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
		got := p.reserveIssue(unitINT, ready, duration)
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
	if got := p.reserveIssue(unitINT, 50, 1); got != 50 {
		t.Fatal(got)
	}
	if got := p.reserveIssue(unitINT, 5, 1); got != 5 {
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
		p.reserveIssue(unitINT, ready, 1)
	}
}

func TestSharedDividerCapacityAndOrdinaryFPPipeCompetition(t *testing.T) {
	spec := DefaultSpec()
	p := &middleware{comp: &Comp{Spec: spec, State: State{FpUnits: make([][]UnitReservation, spec.NumFP)}}}
	// All divisions share the eligible subset, not four independent one-cycle dividers.
	for i := uint64(0); i < 4; i++ {
		got := p.reserveIssue(unitFPDiv, 10, unitOccupancy(arm64asm.FDIV))
		if got != 10+i {
			t.Fatalf("divide %d issued at %d", i, got)
		}
	}
	if got := p.reserveIssue(unitFP, 10, 1); got != 10 {
		t.Fatal("unrelated FP pipe needlessly blocked")
	}
	if got := p.reserveIssue(unitFPDiv, 8, 2); got != 8 {
		t.Fatal("future divide reservation erased earlier sqrt gap")
	}
	if got := p.reserveIssue(unitFPDiv, 9, 2); got != 14 {
		t.Fatalf("sqrt interval overlap: %d", got)
	}
	if !reflect.DeepEqual(p.comp.State.FpUnits[0], []UnitReservation{{8, 10}, {10, 11}, {11, 12}, {12, 13}, {13, 14}, {14, 16}}) {
		t.Fatal(p.comp.State.FpUnits)
	}
	raw, _ := json.Marshal(p.comp.State)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	p.comp.State = restored
	if got := p.reserveIssue(unitFPDiv, 15, 1); got != 16 {
		t.Fatal("checkpoint lost shared divider")
	}
}

func TestMemoryPipeEligibilityAndPhysicalReservations(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		total, loads, stores int
		// Physical pipe indices in dedicated-first preference order.
		loadPipes, storePipes []int
	}{
		{"M2", 4, 3, 2, []int{0, 1, 2}, []int{3, 2}},
		{"legacy one", 1, 0, 0, []int{0}, []int{0}},
		{"legacy two", 2, 0, 0, []int{0, 1}, []int{0, 1}},
		{"explicit shared", 2, 2, 2, []int{0, 1}, []int{0, 1}},
		{"disjoint", 3, 2, 1, []int{0, 1}, []int{2}},
		{"multiple shared and dedicated", 6, 4, 4, []int{0, 1, 2, 3}, []int{5, 4, 3, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := DefaultSpec()
			spec.NumLDST, spec.NumLoad, spec.NumStore = tc.total, tc.loads, tc.stores
			// Check Spec serialization retains explicit zero/zero legacy mode.
			raw, err := json.Marshal(spec)
			if err != nil {
				t.Fatal(err)
			}
			var restored Spec
			if err := json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(spec, restored) {
				t.Fatal("memory eligibility did not roundtrip", err)
			}
			simulation := sim.MakeBuilder().Build()
			t.Cleanup(func() {
				if err := simulation.Terminate(); err != nil {
					t.Error(err)
				}
			})
			em := emu.NewEmulator()
			em.Mem.TrackAccesses = true
			core := modeling.MakeBuilder[*Comp](restored).WithSimulation(simulation).WithResources(Resources{Emu: em}).Build("CPU")
			if err := simulation.Initialize(); err != nil {
				t.Fatal(err)
			}
			p := core.Middlewares.Pipeline
			occupied := make([]map[uint64]bool, tc.total)
			for i := range occupied {
				occupied[i] = make(map[uint64]bool)
			}
			rng := rand.New(rand.NewSource(18))
			for n := 0; n < 300; n++ {
				ut, eligible := unitLoad, tc.loadPipes
				if rng.Intn(2) == 0 {
					ut, eligible = unitStore, tc.storePipes
				}
				ready, duration := uint64(rng.Intn(100)), uint64(1+rng.Intn(3))
				want, physical := ready, -1
				for physical < 0 {
					for _, i := range eligible {
						free := true
						for c := want; c < want+duration; c++ {
							if occupied[i][c] {
								free = false
								break
							}
						}
						if free {
							physical = i
							break
						}
					}
					if physical < 0 {
						want++
					}
				}
				got := p.reserveIssue(ut, ready, duration)
				if got != want {
					t.Fatalf("request %d: start %d want %d", n, got, want)
				}
				for c := want; c < want+duration; c++ {
					occupied[physical][c] = true
				}
				for i, list := range core.State.LdstUnits {
					actual := make(map[uint64]bool)
					for _, slot := range list {
						for c := slot.Start; c < slot.End; c++ {
							if actual[c] {
								t.Fatalf("overbooked physical pipe %d at %d", i, c)
							}
							actual[c] = true
						}
					}
					if !reflect.DeepEqual(actual, occupied[i]) {
						t.Fatalf("request %d: wrong physical pipe %d", n, i)
					}
				}
				if n == 150 {
					// Restore in the middle: both classes must still share the same
					// physical calendars, including future bookings and earlier gaps.
					raw, err := json.Marshal(core.State)
					if err != nil {
						t.Fatal(err)
					}
					core.State = State{}
					if err := json.Unmarshal(raw, &core.State); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestM2LoadStoreIssueLimitsAndDedicatedPreference(t *testing.T) {
	for _, tc := range []struct {
		ops    []unitType
		starts []uint64
	}{
		{[]unitType{unitLoad, unitLoad, unitLoad, unitLoad}, []uint64{10, 10, 10, 11}},
		{[]unitType{unitStore, unitStore, unitStore}, []uint64{10, 10, 11}},
		{[]unitType{unitLoad, unitLoad, unitStore, unitStore, unitLoad}, []uint64{10, 10, 10, 10, 11}},
		{[]unitType{unitStore, unitLoad, unitLoad, unitLoad, unitStore}, []uint64{10, 10, 10, 10, 11}},
	} {
		t.Run(fmt.Sprint(tc.ops), func(t *testing.T) {
			p := &middleware{comp: &Comp{Spec: DefaultSpec(), State: State{LdstUnits: make([][]UnitReservation, 4)}}}
			for i, ut := range tc.ops {
				if got := p.reserveIssue(ut, 10, 1); got != tc.starts[i] {
					t.Fatalf("request %d issued at %d want %d", i, got, tc.starts[i])
				}
			}
		})
	}
}

func TestFusedAndDivideDependencyResultLatency(t *testing.T) {
	for _, tc := range []struct {
		word    uint32
		latency uint64
		unit    unitType
	}{
		{0x1f420c20, 4, unitFP}, {0x1f428c20, 4, unitFP}, {0x1f620c20, 4, unitFP}, {0x1f628c20, 4, unitFP},
		{0x1e621820, 10, unitFPDiv}, {0x1e61c020, 13, unitFPDiv},
	} {
		p := reservationPipeline(t, []uint32{tc.word}, nil)
		p.comp.State.FpReady[1] = 100
		p.fetchAndProcess()
		if p.comp.State.Err != "" || p.comp.State.Rob[0] != 100+tc.latency {
			t.Fatalf("%08x result %d error %s", tc.word, p.comp.State.Rob[0], p.comp.State.Err)
		}
	}
}

func TestFPWindowCountsPairedDestinationsAndAllowsNonFP(t *testing.T) {
	p := reservationPipeline(t, []uint32{0x91000422, 0x6d400440}, nil) // ADD X2,X1,#1; LDP D0,D1,[X2]
	p.comp.Spec.FpPhysLimit = 360
	p.comp.State.FpInFlight = 359
	p.fetchAndProcess()
	if p.comp.Resources.Emu.InstCount != 1 {
		t.Fatal("paired FP load overallocated final single slot")
	}
	p.comp.State.FpInFlight = 358
	p.fetchAndProcess()
	if p.comp.Resources.Emu.InstCount != 2 || p.comp.State.FpInFlight != 360 {
		t.Fatal("paired FP destination count")
	}
	if DefaultSpec().ROBCapacity >= DefaultSpec().FpPhysLimit {
		t.Fatal("default ROB masking must remain explicit")
	}
}
