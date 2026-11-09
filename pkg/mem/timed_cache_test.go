package mem

import (
	"encoding/json"
	"testing"
)

func TestMissingAdjacentPolicyRetainsLegacyConfiguration(t *testing.T) {
	raw, err := json.Marshal(DefaultCacheConfig())
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "DisableAdjacentL1Prefetch")
	raw, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var legacy CacheConfig
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.DisableAdjacentL1Prefetch {
		t.Fatal("missing legacy field disabled adjacent fills")
	}
	c := NewCache(legacy)
	c.Access(0x1000, 8, false, 100)
	if level, ready := c.Access(0x1040, 8, false, 101); level != HitL1 || ready != 437 {
		t.Fatal("legacy timed policy lost", level, ready)
	}
}

func TestTrainedL2StreamsRemainFiniteWithoutAdjacentL1Fills(t *testing.T) {
	for _, direction := range []int64{-1, 1} {
		c := NewCache(DefaultCacheConfig())
		base := int64(0x10000)
		for i := int64(0); i < 3; i++ {
			c.Access(uint64(base+direction*i*128), 8, false, 100)
		}
		if c.stats.StreamsTrained != 1 || c.stats.Prefetches != 4 || c.stats.L2Misses != 3 {
			t.Fatal(c.stats)
		}
		addr := uint64(base + direction*3*128)
		line := c.l2.peek(addr)
		if c.Probe(addr) != HitL2 || line == nil || !line.prefetched || line.readyAt != 459 {
			t.Fatalf("stream lost finite DRAM arrival or populated L1: direction=%d line=%+v", direction, line)
		}
		level, ready := c.Access(addr, 8, false, 101)
		if level != HitL2 || ready != 473 || c.stats.PrefetchedL2Hits != 1 {
			t.Fatal("prefetched demand skipped L2 arrival/transfer", level, ready, c.stats)
		}
	}
}

// A timed demand fetches only its own L1 line under the calibrated policy.
// Even the other half of the same L2 line needs a later demand transfer; an
// odd-half miss must not send an unsolicited request to the next L2 line.
func TestDemandOnlyL1FillsPreserveTransportAndReadiness(t *testing.T) {
	for _, addr := range []uint64{0x1000, 0x1040} {
		for _, store := range []bool{false, true} {
			cfg := DefaultCacheConfig()
			c := NewCache(cfg)
			var ready uint64
			if store {
				accepted, drain := c.Store(addr, 8, 100)
				if accepted != 100 {
					t.Fatal("unexpected allocation admission", accepted)
				}
				ready = drain
			} else {
				_, ready = c.Access(addr, 8, false, 100)
			}
			if ready != 435 {
				t.Fatal("demand arrival changed", ready)
			}
			next := addr + 64
			wantLevel, wantReady := HitL2, uint64(449)
			if addr == 0x1040 {
				wantLevel, wantReady = HitDRAM, 443
			}
			if got := c.Probe(next); got != wantLevel {
				t.Fatalf("unsolicited adjacent fill: addr=%#x store=%t level=%d", addr, store, got)
			}
			if len(c.channel.busy) != 1 || c.channel.busy[0].End-c.channel.busy[0].Start != 2 || len(c.dram.busy) != 1 || c.dram.busy[0].End-c.dram.busy[0].Start != 8 {
				t.Fatalf("demand traffic differs: L1 %+v DRAM %+v", c.channel.busy, c.dram.busy)
			}
			if s := c.Stats(); s.StreamsTrained != 0 || s.Prefetches != 0 || s.L1Writebacks != 0 || s.L2Writebacks != 0 {
				t.Fatal(s)
			}
			level, ready := c.Access(next, 8, false, 101)
			if level != wantLevel || ready != wantReady {
				t.Fatalf("second demand: level %d ready %d want %d,%d", level, ready, wantLevel, wantReady)
			}
			// Once installed, another consumer merges but still waits for data.
			if level, r := c.Access(next, 8, false, 102); level != HitL1 || r != ready {
				t.Fatal("pending demand exposed early", level, r)
			}
		}
	}
}

func TestNoAdjacentFillAvoidsSpeculativeDirtyEviction(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.L1Size, cfg.L1Ways = 128, 1
	c := NewCache(cfg)
	c.Prefetch(0x1040) // Pre-run warming only.
	c.Store(0x1040, 8, 50)
	c.Access(0x2000, 8, false, 100)
	if line := c.l1.peek(0x1040); line == nil || !line.dirty || c.stats.L1Writebacks != 0 {
		t.Fatal("unused adjacent fill evicted dirty data", c.stats)
	}
	// Actually demand the conflicting second half; its dirty victim must now
	// write back through the same finite transport as demand fills.
	c.Access(0x2040, 8, false, 101)
	if c.stats.L1Writebacks != 1 || c.stats.WritebackBytes != 64 || c.DrainUntil() < 103 {
		t.Fatal("demand eviction lost dirty traffic/readiness", c.stats, c.DrainUntil())
	}
}

// The coordinator's early-fill counterexample: a demand miss at cycle 100
// installs a 128 B L2 line arriving at 435; the adjacent 64 B L1 line must not
// be usable before that fill, whether it is reached through the timed path or
// through the legacy untimed Lookup path.
func TestAccessAdjacentLineWaitsForPendingFill(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.DisableAdjacentL1Prefetch = false // Explicit legacy timed policy.
	c := NewCache(cfg)
	level, ready := c.Access(0x1000, 8, false, 100)
	if level != HitDRAM || ready != 435 {
		t.Fatalf("demand miss: level=%d ready=%d", level, ready)
	}
	// The other half arrives with the same DRAM fill and then needs its own
	// 2-cycle L2-to-L1 transfer behind the demanded half: 435 + 2.
	level, ready = c.Access(0x1040, 8, false, 115)
	if level != HitL1 || ready != 437 {
		t.Fatalf("adjacent half of a pending fill: level=%d ready=%d, want L1 hit at 437", level, ready)
	}
	if s := c.Stats(); s.L1HitsInFlight != 1 || s.FillWaitCycles == 0 {
		t.Fatalf("in-flight hit not accounted: %+v", s)
	}

	// Untimed store to a pending line must not make it ready for a later load.
	c = NewCache(cfg)
	c.Access(0x2000, 8, false, 100)
	if got := c.Lookup(0x2040, 8, true); got != HitL1 {
		t.Fatalf("store to adjacent pending line: %d", got)
	}
	if _, ready = c.Access(0x2040, 8, false, 115); ready != 437 {
		t.Fatalf("store exposed pending data early: ready=%d", ready)
	}

	// A store whose line is pending in L2 only (not yet in L1) copies the
	// pending arrival into L1 instead of an instantly ready line.
	c = NewCache(cfg)
	c.Access(0x3000, 8, false, 100) // L2 line 0x60 arrives at 435
	// Evict nothing; touch a different L1 line of the same L2 line via store.
	c.l1.sets[c.l1.setIndex(0x3040)] = make([]cacheLine, c.l1.ways) // drop the adjacent copy
	if got := c.Lookup(0x3040, 8, true); got != HitL2 {
		t.Fatalf("store to L2-pending line: %d", got)
	}
	if _, ready = c.Access(0x3040, 8, false, 120); ready != 435 {
		t.Fatalf("untimed L2 copy lost the pending arrival: ready=%d", ready)
	}
}

// Independent misses share one finite transport: with 16 B/cycle each 128 B
// fill occupies 8 cycles, so a burst of misses issued in the same cycle
// completes 8 cycles apart instead of all at once.
func TestFillTransportSerializesConcurrentMisses(t *testing.T) {
	c := NewCache(DefaultCacheConfig())
	for i := uint64(0); i < 8; i++ {
		// Far apart and alternating direction so no stream is trained.
		addr := uint64(0x10000000) + (i%2)*0x8000000 + i*0x100000
		level, ready := c.Access(addr, 8, false, 0)
		if level != HitDRAM {
			t.Fatalf("miss %d: level %d", i, level)
		}
		if want := uint64(335) + 8*i; ready != want {
			t.Fatalf("miss %d: ready %d want %d", i, ready, want)
		}
	}
	if s := c.Stats(); s.Prefetches != 0 {
		t.Fatalf("unexpected stream prefetches: %+v", s)
	}

	// Transports are not charged when disabled.
	cfg := DefaultCacheConfig()
	cfg.FillBytesPerCycle = 0
	cfg.DRAMBytesPerCycle = 0
	c = NewCache(cfg)
	for i := uint64(0); i < 4; i++ {
		if _, ready := c.Access(0x20000000+i*0x100000, 8, false, 0); ready != 335 {
			t.Fatalf("unbounded transport delayed miss %d to %d", i, ready)
		}
	}
}

// A sequential consumer trains a stream after three misses in one direction;
// later lines are found in L2 (prefetched). With an explicit page bound, demand
// DRAM misses are confined to page starts (the bound is off by default).
func TestStreamPrefetchHidesSequentialLatencyAndStopsAtPages(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.PrefetchPageBytes = 16384
	c := NewCache(cfg)
	base := uint64(0x4000000)
	const pages = 4
	bytes := uint64(cfg.PrefetchPageBytes * pages)
	now := uint64(0)
	exposed := 0
	var pageStartLevels []HitLevel
	for off := uint64(0); off < bytes; off += 8 {
		level, ready := c.Access(base+off, 8, false, now)
		if off%uint64(cfg.PrefetchPageBytes) == 0 {
			pageStartLevels = append(pageStartLevels, level)
		}
		if ready > now+uint64(cfg.L2Latency)+8 && off >= 2048 {
			exposed++
		}
		if ready > now {
			now = ready // dependent consumer: proceed when data arrives
		}
		now++ // one 8-byte element per cycle otherwise
	}
	s := c.Stats()
	lines := bytes / uint64(cfg.L2LineSize)
	if s.StreamsTrained != 1 {
		t.Fatalf("streams trained %d, want 1 (%+v)", s.StreamsTrained, s)
	}
	if s.Prefetches < lines-uint64(pages)*3 || s.Prefetches > lines {
		t.Fatalf("prefetched %d of %d lines", s.Prefetches, lines)
	}
	// Each page start must be a demand DRAM miss: the frontier stops at the page end.
	for i, level := range pageStartLevels {
		if level != HitDRAM {
			t.Fatalf("page %d start was %d, want DRAM miss (prefetch crossed the page)", i, level)
		}
	}
	// Demand DRAM misses are limited to training and page re-starts.
	if s.L2Misses > uint64(pages)*3 {
		t.Fatalf("too many demand DRAM misses with a trained stream: %+v", s)
	}
	// After the ramp, accesses see L2 latency, not DRAM latency.
	accesses := int(bytes/8) - 256
	if exposed > accesses/20 {
		t.Fatalf("exposed accesses %d of %d after ramp", exposed, accesses)
	}
	if s.PrefetchedL2Hits == 0 {
		t.Fatalf("no demand hit on a prefetched line: %+v", s)
	}
}

// Random 128 B nodes (the chase microbenchmarks) must not train a stream.
func TestRandomChaseDoesNotTrainStreams(t *testing.T) {
	c := NewCache(DefaultCacheConfig())
	x := uint64(0x9E3779B97F4A7C15)
	for i := 0; i < 4096; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		addr := (x % (64 << 20)) &^ 127
		c.Access(addr, 8, false, uint64(i)*400)
	}
	if s := c.Stats(); s.StreamsTrained != 0 || s.Prefetches != 0 {
		t.Fatalf("random access trained a stream: %+v", s)
	}
}

// With the timed mechanisms disabled, Access reports the same levels as the
// legacy Lookup for the same address sequence and the nominal latencies.
func TestAccessWithoutTimedMechanismsMatchesLookupLevels(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.FillBytesPerCycle = 0
	cfg.DRAMBytesPerCycle = 0
	cfg.PrefetchDistance = 0
	cfg.WriteBufferEntries = 0
	cfg.DisableAdjacentL1Prefetch = false
	legacy := NewCache(cfg)
	timed := NewCache(cfg)
	x := uint64(12345)
	for i := 0; i < 20000; i++ {
		x = x*6364136223846793005 + 1442695040888963407
		var addr uint64
		if i%3 == 0 {
			addr = (x >> 20) % (1 << 22) // random within 4 MiB
		} else {
			addr = uint64(i) * 8 // sequential
		}
		// This comparison covers one-line accesses. Timed cross-line requests
		// now visit both lines; legacy Lookup deliberately keeps its old scope.
		addr &^= 7
		want := legacy.Lookup(addr, 8, false)
		now := uint64(i) * 1000 // far apart: every fill has arrived
		got, ready := timed.Access(addr, 8, false, now)
		if got != want {
			t.Fatalf("access %d addr %#x: level %d want %d", i, addr, got, want)
		}
		nominal := map[HitLevel]uint64{HitL1: uint64(cfg.L1Latency), HitL2: uint64(cfg.L2Latency), HitDRAM: uint64(cfg.DRAMLatency)}[got]
		if ready != now+nominal {
			t.Fatalf("access %d: ready %d want %d", i, ready, now+nominal)
		}
	}
	ls, ts := legacy.Stats(), timed.Stats()
	if ls.L1Hits != ts.L1Hits || ls.L1Misses != ts.L1Misses || ls.L2Hits != ts.L2Hits || ls.L2Misses != ts.L2Misses {
		t.Fatalf("stats diverged: legacy %+v timed %+v", ls, ts)
	}
}

// The legacy untimed path keeps its exact historical semantics.
func TestLegacyLookupSemanticsUnchanged(t *testing.T) {
	c := NewCache(DefaultCacheConfig())
	steps := []struct {
		addr uint64
		want HitLevel
	}{
		{0x1000, HitDRAM}, // miss; adjacent 0x1040 prefetched (same L2 line)
		{0x1040, HitL1},
		{0x1080, HitDRAM}, // next L2 line
		{0x10c0, HitL1},
		{0x5040, HitDRAM}, // odd half first: adjacent 0x5080 is the next L2 line, fetched instantly
		{0x5080, HitL1},
		{0x50c0, HitL2}, // its L2 line was installed by the instant prefetch
	}
	for i, s := range steps {
		if got := c.Lookup(s.addr, 8, false); got != s.want {
			t.Fatalf("step %d addr %#x: %d want %d", i, s.addr, got, s.want)
		}
	}
	if st := c.Stats(); st.L1Hits != 3 || st.L1Misses != 4 || st.L2Hits != 1 || st.L2Misses != 3 || st.Prefetches != 0 {
		t.Fatalf("legacy stats changed: %+v", st)
	}
}

// Check that the two effective transport rates remain distinct: a saturated
// L2-resident reader follows the 32 B/cycle fill channel, while cold streaming
// follows the 16 B/cycle DRAM channel. This is a model behavior test, not a
// hardware-cycle measurement; see evaluation/model-profile.md.
func TestSaturatingStreamRatesFollowTheTwoTransports(t *testing.T) {
	cfg := DefaultCacheConfig()
	rate := func(prefill bool) float64 {
		c := NewCache(cfg)
		base := uint64(0x10000000)
		bytes := uint64(2 << 20) // 2 MiB, L2-resident after the first pass
		now := uint64(0)
		if prefill {
			for off := uint64(0); off < bytes; off += 64 {
				c.Lookup(base+off, 8, false) // untimed warm pass: everything in L2
			}
		}
		// Greedy consumer with eight independent loads per 64 B: proceed as
		// soon as the latest line of a 16-line window has arrived (window-
		// bounded MLP, like the OoO pipeline on the read microbenchmark).
		var pending []uint64
		start := uint64(0)
		for off := uint64(0); off < bytes; off += 64 {
			_, ready := c.Access(base+off, 8, false, now)
			pending = append(pending, ready)
			if len(pending) > 16 {
				if pending[0] > now {
					now = pending[0]
				}
				pending = pending[1:]
			}
			now++
			if off == 256<<10 { // skip training and ramp
				start = now
			}
		}
		for _, r := range pending {
			if r > now {
				now = r
			}
		}
		return float64(now-start) / float64((bytes-(256<<10))/64)
	}
	l2, dram := rate(true), rate(false)
	if l2 < 2 || l2 > 2.6 {
		t.Fatalf("L2-resident stream %.2f cycles per 64 B line, want the 32 B/cycle transport bound (2-2.6)", l2)
	}
	if dram < 4 || dram > 4.6 {
		t.Fatalf("DRAM-resident stream %.2f cycles per 64 B line, want the 16 B/cycle DRAM bound (4-4.6) with no exposed page misses", dram)
	}
}

// A store stream allocates lines through the DRAM transport: a load miss issued
// in the same cycle as sixteen store-missed lines completes one line transfer
// later per store line, but the stores never train a stream.
func TestStoreMissesOccupyTheDRAMTransportWithoutTraining(t *testing.T) {
	c := NewCache(DefaultCacheConfig())
	for i := uint64(0); i < 16; i++ {
		c.Access(0x40000000+i*128, 8, true, 0) // 16 distinct L2 lines, sequential
	}
	level, ready := c.Access(0x50000000, 8, false, 0)
	if level != HitDRAM || ready != 335+16*8 {
		t.Fatalf("load behind a store burst: level=%d ready=%d, want DRAM at %d", level, ready, 335+16*8)
	}
	if s := c.Stats(); s.StreamsTrained != 0 || s.Prefetches != 0 {
		t.Fatalf("stores trained a stream: %+v", s)
	}
	// The store line itself is usable by a later load only when its fill arrived.
	if _, r := c.Access(0x40000000, 8, false, 1); r < 335 {
		t.Fatalf("store-allocated line readable before its fill: %d", r)
	}
}

// A descending stream trains and prefetches toward lower addresses.
func TestStreamPrefetchDescending(t *testing.T) {
	cfg := DefaultCacheConfig()
	c := NewCache(cfg)
	top := uint64(0x8000000)
	now := uint64(0)
	for off := uint64(8); off <= 16384; off += 8 {
		_, ready := c.Access(top-off, 8, false, now)
		if ready > now {
			now = ready
		}
		now++
	}
	if s := c.Stats(); s.StreamsTrained != 1 || s.Prefetches < 100 || s.L2Misses > 4 {
		t.Fatalf("descending stream not prefetched: %+v", s)
	}
}
