package ooo

import (
	"testing"

	"github.com/sarchlab/poros/pkg/mem"
)

const (
	loadX0FromX1  = 0xf9400020 // LDR X0,[X1]
	loadX2FromX3  = 0xf9400062 // LDR X2,[X3]
	loadX2FromX1  = 0xf9400022 // LDR X2,[X1]
	storeX0ToX1   = 0xf9000020 // STR X0,[X1]
	translationL1 = 5          // default L1 latency
)

func translatingCache(walkLatency, walkers int, warm ...uint64) *mem.Cache {
	cfg := mem.DefaultCacheConfig()
	cfg.TLBPageBytes = 16384
	cfg.TLBSets, cfg.TLBWays = 16, 4
	cfg.TLBWalkLatency, cfg.TLBWalkers = walkLatency, walkers
	cache := mem.NewCache(cfg)
	for _, addr := range warm {
		cache.Lookup(addr, 8, false) // resident in L1; translation still cold
	}
	return cache
}

func twoLoadPipes(s *Spec) {
	s.NumLDST = 2
	s.NumMSHREntries = 2
}

// Under the complete default configuration a cold load first walks, then
// fetches its line from DRAM.
func TestColdLoadUnderDefaultsWalksThenFetches(t *testing.T) {
	cfg := mem.DefaultCacheConfig()
	cache := mem.NewCache(cfg)
	p := reservationPipeline(t, []uint32{loadX0FromX1}, cache)
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.fetchAndProcess()
	want := uint64(1 + cfg.TLBWalkLatency + cfg.DRAMLatency)
	if p.comp.State.Err != "" || p.comp.State.Rob[0] != want || p.comp.State.MshrBusyUntil[0] != want {
		t.Fatalf("completion %d MSHR %d, want %d (%s)", p.comp.State.Rob[0], p.comp.State.MshrBusyUntil[0], want, p.comp.State.Err)
	}
	if stats := cache.Stats(); stats.TLBWalks != 1 || stats.L2Misses != 1 || stats.SLCHits != 0 {
		t.Fatal(stats)
	}
}

// With translation timing off (page size zero) a resident load keeps its
// plain L1 completion.
func TestDisabledTranslationLeavesLoadTimingUnchanged(t *testing.T) {
	cache := fillCache()
	cache.Lookup(0x4000, 8, false)
	p := reservationPipeline(t, []uint32{loadX0FromX1}, cache)
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.fetchAndProcess()
	if p.comp.State.Err != "" || p.comp.State.Rob[0] != 1+translationL1 {
		t.Fatal(p.comp.State.Rob[0], p.comp.State.Err)
	}
	if stats := cache.Stats(); stats.TLBWalks != 0 || stats.TLBWaitCycles != 0 {
		t.Fatal(stats)
	}
}

// A load whose page is not translated starts its cache access when the walk
// ends; a second load to the same page shares that walk.
func TestLoadsWaitForOneSharedPageWalk(t *testing.T) {
	cache := translatingCache(30, 1, 0x4000, 0x4100)
	p := specPipeline(t, []uint32{loadX0FromX1, loadX2FromX3}, cache, twoLoadPipes)
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.Resources.Emu.Regs.WriteX(3, 0x4100)
	p.fetchAndProcess()
	if p.comp.State.Err != "" {
		t.Fatal(p.comp.State.Err)
	}
	want := uint64(1 + 30 + translationL1)
	if p.comp.State.Rob[0] != want || p.comp.State.Rob[1] != want {
		t.Fatalf("completions %v, want both %d", p.comp.State.Rob[:2], want)
	}
	if stats := cache.Stats(); stats.TLBWalks != 1 {
		t.Fatal(stats)
	}
}

// Walks to different pages are serialized by the walkers, not by the loads'
// own pipes.
func TestIndependentPageWalksQueueOnWalkers(t *testing.T) {
	for _, tc := range []struct {
		walkers int
		second  uint64
	}{{1, 1 + 60 + translationL1}, {2, 1 + 30 + translationL1}} {
		cache := translatingCache(30, tc.walkers, 0x4000, 0x8000)
		p := specPipeline(t, []uint32{loadX0FromX1, loadX2FromX3}, cache, twoLoadPipes)
		p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
		p.comp.Resources.Emu.Regs.WriteX(3, 0x8000)
		p.fetchAndProcess()
		if p.comp.State.Err != "" || p.comp.State.Rob[0] != 1+30+translationL1 || p.comp.State.Rob[1] != tc.second {
			t.Fatalf("%d walkers: completions %v, want %d and %d (%s)", tc.walkers, p.comp.State.Rob[:2], 1+30+translationL1, tc.second, p.comp.State.Err)
		}
	}
}

// An older load whose address arrives late books its walk in the future. A
// younger load with an early address uses the walker before that booking.
func TestYoungerEarlyWalkPrecedesOlderLateWalk(t *testing.T) {
	cache := translatingCache(30, 1, 0x4000, 0x8000)
	p := specPipeline(t, []uint32{loadX0FromX1, loadX2FromX3}, cache, twoLoadPipes)
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.Resources.Emu.Regs.WriteX(3, 0x8000)
	p.comp.State.GprReady[1] = 100 // the older load's base register is late
	p.fetchAndProcess()
	if p.comp.State.Err != "" {
		t.Fatal(p.comp.State.Err)
	}
	if older, younger := p.comp.State.Rob[0], p.comp.State.Rob[1]; older != 100+30+translationL1 || younger != 1+30+translationL1 {
		t.Fatalf("older %d younger %d, want %d and %d", older, younger, 100+30+translationL1, 1+30+translationL1)
	}
}

// A store's bytes reach the cache only after its page is translated, so its
// slot is held longer; a load forwarded from that store does not wait for
// the walk.
func TestStoreTranslationDelaysAcceptanceNotForwarding(t *testing.T) {
	completions := func(cache *mem.Cache) (store, load, slot uint64) {
		p := reservationPipeline(t, []uint32{storeX0ToX1, loadX2FromX1}, cache)
		p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
		p.fetchAndProcess()
		if p.comp.State.Err != "" {
			t.Fatal(p.comp.State.Err)
		}
		return p.comp.State.Rob[0], p.comp.State.Rob[1], p.comp.State.StoreDrainUntil[0]
	}
	plain := fillCache()
	plain.Lookup(0x4000, 8, false)
	store, load, slot := completions(plain)
	walkedStore, walkedLoad, walkedSlot := completions(translatingCache(30, 1, 0x4000))
	if walkedStore != store || walkedLoad != load {
		t.Fatalf("translation changed store completion %d->%d or forwarded load %d->%d", store, walkedStore, load, walkedLoad)
	}
	if walkedSlot != slot+30 {
		t.Fatalf("store slot held until %d, want %d (one walk later than %d)", walkedSlot, slot+30, slot)
	}
}
