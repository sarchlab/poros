package ooo

import (
	"testing"

	"github.com/sarchlab/poros/pkg/mem"
)

// These tests run the complete default memory path (translation timing and
// the memory-side cache both on) and compare it with the same path without
// translation, so they pin how the mechanisms combine rather than absolute
// fill arithmetic.

const storePairX0X2ToX1 = 0xa9000820 // STP X0,X2,[X1]

func loadFrom(base, dest int) uint32 { return 0xf9400000 | uint32(base)<<5 | uint32(dest) } // LDR Xdest,[Xbase]

// completions runs words once on cache and returns each instruction's
// completion cycle.
func completions(t *testing.T, words []uint32, cache *mem.Cache, adjust func(*Spec), prepare func(*middleware)) []uint64 {
	t.Helper()
	p := specPipeline(t, words, cache, adjust)
	prepare(p)
	p.fetchAndProcess()
	if p.comp.State.Err != "" {
		t.Fatal(p.comp.State.Err)
	}
	return append([]uint64(nil), p.comp.State.Rob[:len(words)]...)
}

// A load that crosses a page boundary needs both translations. The walks run
// side by side, so the load is one walk later than without translation, and
// both of its line fills still happen.
func TestDefaultsCrossPageLoadWalksBothPagesOnce(t *testing.T) {
	cfg := mem.DefaultCacheConfig()
	base := func(p *middleware) { p.comp.Resources.Emu.Regs.WriteX(1, 0x8000-4) }
	plain := completions(t, []uint32{loadX0FromX1}, fillCache(), nil, base)
	cache := mem.NewCache(cfg)
	walked := completions(t, []uint32{loadX0FromX1}, cache, nil, base)
	if walked[0] != plain[0]+uint64(cfg.TLBWalkLatency) {
		t.Fatalf("cross-page load completes at %d, want %d (one walk after %d)", walked[0], plain[0]+uint64(cfg.TLBWalkLatency), plain[0])
	}
	if stats := cache.Stats(); stats.TLBWalks != 2 || stats.L2Misses != 2 || stats.SLCHits != 0 {
		t.Fatal(stats)
	}
}

// A load that has to wait for a miss entry walks while it waits: the wait and
// the walk overlap instead of adding.
func TestDefaultsWalkOverlapsMissEntryWait(t *testing.T) {
	cfg := mem.DefaultCacheConfig()
	for _, busyUntil := range []uint64{10, 1000} {
		prepare := func(p *middleware) {
			p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
			p.comp.State.MshrBusyUntil[0] = busyUntil
		}
		plain := completions(t, []uint32{loadX0FromX1}, fillCache(), nil, prepare)
		walked := completions(t, []uint32{loadX0FromX1}, mem.NewCache(cfg), nil, prepare)
		want := max(plain[0], 1+uint64(cfg.TLBWalkLatency+cfg.DRAMLatency))
		if walked[0] != want {
			t.Fatalf("miss entry busy until %d: completion %d, want %d (without translation %d)", busyUntil, walked[0], want, plain[0])
		}
	}
}

// Six cold loads to six pages through two miss entries. The first four walks
// run at once and the last two follow; every load beyond the second waits for
// a miss entry far longer than for its walk, so the whole chain is one walk
// later than without translation, not one walk per load.
func TestDefaultsMissEntrySaturationAbsorbsLaterWalks(t *testing.T) {
	cfg := mem.DefaultCacheConfig()
	words := make([]uint32, 6)
	for i := range words {
		words[i] = loadFrom(i+1, i+10)
	}
	adjust := func(s *Spec) { s.NumLDST, s.NumMSHREntries = 2, 2 }
	prepare := func(p *middleware) {
		for i := range words {
			p.comp.Resources.Emu.Regs.WriteX(i+1, uint64(i+1)*0x10000)
		}
	}
	plain := completions(t, words, fillCache(), adjust, prepare)
	cache := mem.NewCache(cfg)
	walked := completions(t, words, cache, adjust, prepare)
	for i := range words {
		if walked[i] != plain[i]+uint64(cfg.TLBWalkLatency) {
			t.Fatalf("load %d completes at %d, want %d; all: %v without translation %v", i, walked[i], plain[i]+uint64(cfg.TLBWalkLatency), walked, plain)
		}
	}
	if plain[2] < plain[0]+uint64(cfg.DRAMLatency) {
		t.Fatalf("third load did not wait for a miss entry: %v", plain)
	}
	if stats := cache.Stats(); stats.TLBWalks != 6 || stats.L2Misses != 6 || stats.SLCHits != 0 {
		t.Fatal(stats)
	}
}

// A paired store whose halves lie on two pages is accepted by the cache when
// both are translated. The two walks run side by side, so its slot is held
// until one walk after its address is known; its own completion and its two
// forwarding records are unchanged.
func TestDefaultsPairedStoreAcrossPagesWaitsForBothTranslations(t *testing.T) {
	cfg := mem.DefaultCacheConfig()
	run := func(cache *mem.Cache) (complete, slot uint64, records int) {
		p := reservationPipeline(t, []uint32{storePairX0X2ToX1}, cache)
		p.comp.Resources.Emu.Regs.WriteX(1, 0x8000-8)
		p.fetchAndProcess()
		if p.comp.State.Err != "" {
			t.Fatal(p.comp.State.Err)
		}
		return p.comp.State.Rob[0], p.comp.State.StoreDrainUntil[0], len(p.comp.State.StoreBuffer)
	}
	complete, slot, records := run(fillCache())
	cache := mem.NewCache(cfg)
	walkedComplete, walkedSlot, walkedRecords := run(cache)
	if walkedComplete != complete || walkedRecords != records || records != 2 {
		t.Fatalf("completion %d->%d records %d->%d", complete, walkedComplete, records, walkedRecords)
	}
	if slot != complete {
		t.Fatalf("without translation the slot is held until %d, want the store's completion %d", slot, complete)
	}
	if want := 1 + uint64(cfg.TLBWalkLatency); walkedSlot != want {
		t.Fatalf("store slot held until %d, want %d (one walk after the address at cycle 1)", walkedSlot, want)
	}
	if stats := cache.Stats(); stats.TLBWalks != 2 {
		t.Fatal(stats)
	}
}

// smallLevels shrinks L1 and L2 to one set each so three lines push the first
// out of L2, and leaves every other default (translation included) in place.
func smallLevels() mem.CacheConfig {
	cfg := mem.DefaultCacheConfig()
	cfg.L1Size, cfg.L1Ways = 64, 1
	cfg.L2Size, cfg.L2Ways = 2*128, 2
	cfg.SLCSize = 4 * 128
	cfg.PrefetchDistance = 0
	return cfg
}

// spillCycles is how long one 128 B line occupies the default spill transport.
func spillCycles(cfg mem.CacheConfig) int { return cfg.L2LineSize / cfg.DRAMBytesPerCycle }

// A line held behind L2 on a page that is not translated: the walk comes
// first, then the memory-side latency.
func TestDefaultsWalkThenMemorySideHit(t *testing.T) {
	cfg := smallLevels()
	cache := mem.NewCache(cfg)
	for _, addr := range []uint64{0x4000, 0x5000, 0x6000} {
		cache.Access(addr, 8, false, 0)
	}
	if cache.Probe(0x4000) != mem.HitSLC {
		t.Fatal("setup did not leave the line in the memory-side cache")
	}
	p := reservationPipeline(t, []uint32{loadX0FromX1}, cache)
	p.comp.State.Cycles = 2000
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.fetchAndProcess()
	want := uint64(2000 + cfg.TLBWalkLatency + cfg.SLCLatency)
	if p.comp.State.Err != "" || p.comp.State.Rob[0] != want || p.comp.State.MshrBusyUntil[0] != want {
		t.Fatalf("completion %d miss entry %d, want %d (%s)", p.comp.State.Rob[0], p.comp.State.MshrBusyUntil[0], want, p.comp.State.Err)
	}
	if stats := cache.Stats(); stats.TLBWalks != 1 || stats.SLCHits != 1 {
		t.Fatal(stats)
	}
}

// Tags change in dispatch order. An older load whose address arrives late
// pushes a line out of L2; a younger load of that line whose address is early
// is visited afterwards and finds the line behind L2. It is supplied from
// there no earlier than the older load's request plus the line's move: later
// than hardware that would still have had the line in L2, never earlier than
// the eviction.
func TestDefaultsYoungerEarlyLoadOfLineEvictedByOlderLateLoad(t *testing.T) {
	cfg := smallLevels()
	cache := mem.NewCache(cfg)
	cache.Access(0x4000, 8, false, 0) // the line the younger load reads
	cache.Access(0x5000, 8, false, 0)
	for _, addr := range []uint64{0x4000, 0x5000, 0x6000} {
		cache.Translate(addr, 8, 0)
	}
	adjust := func(s *Spec) { s.NumLDST, s.NumMSHREntries = 2, 2 }
	p := specPipeline(t, []uint32{loadX0FromX1, loadX2FromX3}, cache, adjust)
	p.comp.State.Cycles = 2000
	p.comp.Resources.Emu.Regs.WriteX(1, 0x6000) // older: displaces 0x4000 from L2
	p.comp.Resources.Emu.Regs.WriteX(3, 0x4000) // younger
	p.comp.State.GprReady[1] = 3000             // the older load's address is late
	p.fetchAndProcess()
	if p.comp.State.Err != "" {
		t.Fatal(p.comp.State.Err)
	}
	older, younger := p.comp.State.Rob[0], p.comp.State.Rob[1]
	if older != uint64(3000+cfg.DRAMLatency) {
		t.Fatalf("older load completes at %d, want %d", older, 3000+cfg.DRAMLatency)
	}
	if want := uint64(3000 + spillCycles(cfg) + cfg.SLCLatency); younger != want {
		t.Fatalf("younger load completes at %d, want %d: a memory-side hit counted from the end of the evicted line's move", younger, want)
	}
}
