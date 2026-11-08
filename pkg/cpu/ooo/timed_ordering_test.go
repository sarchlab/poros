package ooo

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sarchlab/poros/pkg/mem"
)

func TestMSHRWaitPrecedesSupplyingFill(t *testing.T) {
	p := reservationPipeline(t, []uint32{0xf9400020, 0xf9400062}, mem.NewCache(mem.DefaultCacheConfig()))
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.Resources.Emu.Regs.WriteX(3, 0x4040)
	p.comp.State.MshrBusyUntil[0] = 1000
	p.fetchAndProcess()
	first, second := p.comp.State.Rob[0], p.comp.State.Rob[1]
	if first != 1335 || second != 1337 || p.comp.State.MshrBusyUntil[0] != 1335 {
		t.Fatalf("supplier=%d adjacent=%d MSHR=%d", first, second, p.comp.State.MshrBusyUntil[0])
	}
	t.Logf("supplier=%d adjacent=%d MSHR=%d", first, second, p.comp.State.MshrBusyUntil[0])
}

func TestStoreFillUsesFinalUnitIssueAndIndependentDrain(t *testing.T) {
	p := reservationPipeline(t, []uint32{0xf9000020}, mem.NewCache(mem.DefaultCacheConfig())) // STR X0,[X1]
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.State.LdstUnits[0] = []UnitReservation{{1, 1000}}
	p.fetchAndProcess()
	if p.comp.State.Rob[0] != 1003 || p.comp.State.StoreDrainUntil[0] != 1335 {
		t.Fatalf("data=%d allocation=%d", p.comp.State.Rob[0], p.comp.State.StoreDrainUntil[0])
	}
	_, ready := p.comp.Resources.Cache.Access(0x4040, 8, false, 1)
	if ready != 1337 {
		t.Fatalf("store supplied adjacent line before issue: %d", ready)
	}
	if p.comp.State.StoreBuffer[0].CompleteCycle != 1003 {
		t.Fatal("allocation readiness changed forwarding data")
	}
}

func TestStoreAllocationPoolBoundsStreamAndDrainsTerminalStores(t *testing.T) {
	const stores = 512
	words := make([]uint32, stores)
	for i := range words {
		words[i] = 0xf9000020
	} // STR X0,[X1]
	p := reservationPipeline(t, words, mem.NewCache(mem.DefaultCacheConfig()))
	p.comp.Spec.FetchWidth = 1
	p.comp.Spec.StoreBufferSize = 2
	p.comp.State.StoreDrainUntil = make([]uint64, 2)
	p.comp.Resources.Emu.SetHaltPC(0x1000 + stores*4)
	var finalDrain uint64
	sawBackpressure := false
	for !p.comp.State.Done && p.comp.State.Cycles < 200000 {
		before := p.comp.Resources.Emu.InstCount
		p.comp.Resources.Emu.Regs.WriteX(1, 0x100000+before*4096)
		p.Handle(nil)
		active := 0
		for _, until := range p.comp.State.StoreDrainUntil {
			if until > p.comp.State.Cycles {
				active++
			}
			if until > finalDrain {
				finalDrain = until
			}
		}
		if active > 2 || len(p.comp.State.StoreBuffer) > 2 {
			t.Fatalf("unbounded allocation records: active=%d records=%d", active, len(p.comp.State.StoreBuffer))
		}
		if before < stores && before == p.comp.Resources.Emu.InstCount && active == 2 {
			sawBackpressure = true
		}
		if p.comp.State.Done && p.comp.State.Cycles < finalDrain {
			t.Fatal("terminated before final allocation drained")
		}
	}
	if !p.comp.State.Done || p.comp.State.InstDone != stores || !sawBackpressure {
		t.Fatalf("done=%v retired=%d backpressure=%v err=%s", p.comp.State.Done, p.comp.State.InstDone, sawBackpressure, p.comp.State.Err)
	}
	t.Logf("%d stores, capacity=2, last drain=%d completion=%d", stores, finalDrain, p.comp.State.Cycles)
}

func TestPairedStoreHoldsAllocationUntilBothFills(t *testing.T) {
	p := reservationPipeline(t, []uint32{0xa9000820}, mem.NewCache(mem.DefaultCacheConfig())) // STP X0,X2,[X1]
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4038)                                               // two different L1 halves
	p.fetchAndProcess()
	if len(p.comp.State.StoreBuffer) != 2 {
		t.Fatalf("pair accesses=%d", len(p.comp.State.StoreBuffer))
	}
	if p.comp.State.StoreDrainUntil[0] != 338 {
		t.Fatalf("pair drain=%d", p.comp.State.StoreDrainUntil[0])
	}
	for _, entry := range p.comp.State.StoreBuffer {
		if entry.DrainCycle != 338 || entry.CompleteCycle != 4 {
			t.Fatalf("pair record %+v", entry)
		}
	}
}

func TestPendingOldStoreSurvivesYoungerDrainingStores(t *testing.T) {
	// One far-future STR, many independent STPs, then an exact overlapping LDR.
	words := []uint32{0xf9000020}
	for i := 0; i < 80; i++ {
		words = append(words, 0xa9001063)
	} // STP X3,X4,[X3]
	words = append(words, 0xf9400025) // LDR X5,[X1]
	p := reservationPipeline(t, words, nil)
	p.comp.Spec.FetchWidth = 1
	p.comp.Spec.StoreBufferSize = 3
	p.comp.State.StoreDrainUntil = make([]uint64, 3)
	p.comp.State.GprReady[0] = 10000
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.Resources.Emu.Regs.WriteX(3, 0x8000)
	p.comp.Resources.Emu.SetHaltPC(0x1000 + uint64(len(words))*4)
	for p.comp.Resources.Emu.InstCount < uint64(len(words)) && p.comp.State.Cycles < 1000 {
		p.Handle(nil)
		if len(p.comp.State.StoreBuffer) > 6 {
			t.Fatal("unbounded pair records")
		}
	}
	if p.comp.Resources.Emu.InstCount != uint64(len(words)) {
		t.Fatalf("younger stores failed to make progress: %d", p.comp.Resources.Emu.InstCount)
	}
	if p.comp.State.GprReady[5] != 10001 {
		t.Fatalf("old pending store lost: load ready=%d want10001", p.comp.State.GprReady[5])
	}
	if p.comp.State.StoreBuffer[0].Addr != 0x4000 {
		t.Fatal("old store record evicted")
	}
}

func TestAllocationDoesNotDelayExactGPRForwarding(t *testing.T) {
	p := reservationPipeline(t, []uint32{0xf9000020, 0xf9400022}, mem.NewCache(mem.DefaultCacheConfig()))
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.fetchAndProcess()
	if p.comp.State.Rob[0] != 4 || p.comp.State.Rob[1] != 3 || p.comp.State.StoreDrainUntil[0] != 336 {
		t.Fatalf("store=%d load=%d drain=%d", p.comp.State.Rob[0], p.comp.State.Rob[1], p.comp.State.StoreDrainUntil[0])
	}
}

func TestDoneAndFaultPathsReleaseSharedTimingClient(t *testing.T) {
	for _, alreadyDone := range []bool{false, true} {
		cfg := mem.DefaultCacheConfig()
		shared := mem.NewSharedL2(cfg)
		cache := mem.NewPrivateL1WithSharedL2(cfg, shared)
		sibling := mem.NewPrivateL1WithSharedL2(cfg, shared)
		p := reservationPipeline(t, []uint32{0xd4200000}, cache)
		p.comp.State.Done = alreadyDone
		p.Handle(nil)
		if !p.comp.State.Done {
			t.Fatal("fault did not stop pipeline")
		}
		// FinishTiming's public contract disallows further accesses from the
		// stopped cache; checking it catches missing cleanup on either path.
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("stopped cache remained active")
				}
			}()
			cache.Access(0x1000, 8, false, 2)
		}()
		sibling.Advance(1000)
		sibling.Access(0x8000, 8, false, 1000)
	}
}

func TestRejectsLegacyOrIncompatibleStoreAllocationState(t *testing.T) {
	for _, slots := range []int{0, 1} {
		t.Run(fmt.Sprint(slots), func(t *testing.T) {
			p := reservationPipeline(t, []uint32{0xf9000020}, nil)
			// Deserialization of legacy State leaves this new field nil. A wrong
			// nonzero length is equally incompatible with the configured capacity.
			p.comp.State.StoreDrainUntil = make([]uint64, slots)
			defer func() {
				r := recover()
				if r == nil || !strings.Contains(fmt.Sprint(r), "store-allocation state") {
					t.Fatalf("expected explicit state error, got %v", r)
				}
			}()
			p.Handle(nil)
		})
	}
}

func TestCheckedFetchFaultDrainsPriorStoreAllocations(t *testing.T) {
	p := reservationPipeline(t, []uint32{0xf9000020, 0x00000000}, mem.NewCache(mem.DefaultCacheConfig()))
	em := p.comp.Resources.Emu
	em.Regs.WriteX(1, 0x4000)
	// Emulator satisfies Execution; this enables the checked-process fault
	// policy while preserving the minimal reproducible instruction stream.
	p.comp.Resources.Execution = em
	p.comp.Resources.Emu = nil
	p.comp.Spec.FetchWidth = 1
	p.Handle(nil)
	drain := p.comp.State.StoreDrainUntil[0]
	if drain <= p.comp.State.Cycles {
		t.Fatal("store did not leave an allocation")
	}
	for !p.comp.State.Done && p.comp.State.Cycles <= drain {
		p.Handle(nil)
		if p.comp.State.Done && p.comp.State.Cycles < drain {
			t.Fatalf("fetch fault terminated at%d before drain%d", p.comp.State.Cycles, drain)
		}
	}
	if !p.comp.State.Done || !strings.Contains(p.comp.State.Err, "fetch:") || p.comp.State.Cycles != drain || len(p.comp.State.StoreBuffer) != 0 {
		t.Fatalf("done=%v cycles=%d drain=%d records=%d err=%s", p.comp.State.Done, p.comp.State.Cycles, drain, len(p.comp.State.StoreBuffer), p.comp.State.Err)
	}
}
