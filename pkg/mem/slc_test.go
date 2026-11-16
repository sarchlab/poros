package mem

import "testing"

// One L1 line, a two-line one-set L2 and a small memory-side cache, so three
// distinct lines are enough to push a line out of L1 and L2.
func memorySideConfig(slcLines int) CacheConfig {
	cfg := DefaultCacheConfig()
	cfg.L1Size, cfg.L1Ways = 64, 1
	cfg.L2Size, cfg.L2Ways = 2*128, 2
	cfg.SLCSize, cfg.SLCLatency = slcLines*128, 138
	cfg.TLBPageBytes = 0     // Access is called directly; translation is not involved
	cfg.PrefetchDistance = 0 // no stream prefetch in these single-set tests
	return cfg
}

const (
	lineA, lineB, lineC, lineD, lineE = 0x0000, 0x1000, 0x2000, 0x3000, 0x4000
	// spillCycles is how long one 128 B line occupies a 16 B/cycle transport.
	spillCycles = 8
)

// heldFrom returns the cycle from which the memory-side cache can supply the
// line at addr, and whether it holds it.
func heldFrom(c *Cache, addr uint64) (uint64, bool) {
	i, ok := c.slc.slot[c.l2Line(addr)]
	if !ok {
		return 0, false
	}
	return c.slc.ring[i].readyAt, true
}

func TestLineEvictedFromL2ReturnsFromMemorySideCache(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	for i, addr := range []uint64{lineA, lineB, lineC} {
		if level, ready := c.Access(addr, 8, false, uint64(1000*(i+1))); level != HitDRAM || ready != uint64(1000*(i+1)+335) {
			t.Fatal("cold access", addr, level, ready)
		}
	}
	// A left L2 when C arrived and is held behind it; B and C are in L2.
	if c.Probe(lineA) != HitSLC || c.Probe(lineB) != HitL2 || c.Probe(lineC) != HitL1 {
		t.Fatal("residency", c.Probe(lineA), c.Probe(lineB), c.Probe(lineC))
	}
	level, ready := c.Access(lineA, 8, false, 5000)
	if level != HitSLC || ready != 5138 || c.stats.SLCHits != 1 || c.stats.L2Misses != 4 {
		t.Fatal("memory-side hit", level, ready, c.stats)
	}
	// The line moved back to L2: it is no longer held behind it, and the L2
	// victim it displaced (B, the least recently used) now is.
	if _, held := heldFrom(c, lineA); held || c.Probe(lineB) != HitSLC || c.Probe(lineC) != HitL2 {
		t.Fatal("line did not move between the levels", c.Probe(lineB), c.Probe(lineC))
	}
	if level, ready := c.Access(lineB, 8, false, 6000); level != HitSLC || ready != 6138 {
		t.Fatal(level, ready)
	}
}

func TestDisabledMemorySideCacheKeepsDRAMTiming(t *testing.T) {
	cfg := memorySideConfig(4)
	cfg.SLCSize, cfg.SLCLatency = 0, 0
	c := NewCache(cfg)
	for i, addr := range []uint64{lineA, lineB, lineC} {
		c.Access(addr, 8, false, uint64(1000*(i+1)))
	}
	if c.Probe(lineA) != HitDRAM || c.spill != nil {
		t.Fatal(c.Probe(lineA), c.spill)
	}
	if level, ready := c.Access(lineA, 8, false, 5000); level != HitDRAM || ready != 5335 || c.stats.SLCHits != 0 {
		t.Fatal(level, ready, c.stats)
	}
}

// The memory-side cache holds the most recent victims and drops the oldest.
func TestMemorySideCacheIsFiniteAndOrderedByEviction(t *testing.T) {
	c := NewCache(memorySideConfig(2))
	for i, addr := range []uint64{lineA, lineB, lineC, lineD, lineE} {
		c.Access(addr, 8, false, uint64(1000*(i+1)))
	}
	// L2 holds D and E. A, B and C were evicted in that order; two fit.
	if c.Probe(lineA) != HitDRAM || c.Probe(lineB) != HitSLC || c.Probe(lineC) != HitSLC {
		t.Fatal("eviction order", c.Probe(lineA), c.Probe(lineB), c.Probe(lineC))
	}
	if level, _ := c.Access(lineB, 8, false, 9000); level != HitSLC {
		t.Fatal(level)
	}
	if level, ready := c.Access(lineA, 8, false, 10000); level != HitDRAM || ready != 10335 {
		t.Fatal("dropped line came back early", level, ready)
	}
}

// There are no sets: lines that would share one set of any set-associative
// cache are all held, up to the capacity.
func TestMemorySideCacheHasNoSetConflicts(t *testing.T) {
	cfg := memorySideConfig(64)
	c := NewCache(cfg)
	const stride = 1 << 24 // the same low address bits on every line
	for i := uint64(0); i < 50; i++ {
		c.Access(i*stride, 8, false, 1000*(i+1))
	}
	for i := uint64(0); i < 48; i++ {
		if c.Probe(i*stride) != HitSLC {
			t.Fatalf("line %d of 48 victims is not held (%d)", i, c.Probe(i*stride))
		}
	}
}

// A line that returns to L2 leaves an empty place; it does not let an older
// line stay longer. Every victim is dropped after exactly the store's
// capacity in further evictions.
func TestReturningLineDoesNotLengthenTheStayOfOthers(t *testing.T) {
	s := newVictimStore(3)
	keep := func(line, readyAt uint64) { s.keep(victimLine{line: line, leftAt: readyAt - 5, readyAt: readyAt}) }
	keep(1, 10)
	keep(2, 20)
	keep(3, 30)
	if victim, ok := s.take(2); !ok || victim.readyAt != 20 || victim.leftAt != 15 || s.holds(2) {
		t.Fatal(victim, ok)
	}
	keep(4, 40) // takes line 1's place: three evictions after line 1 arrived
	if s.holds(1) || !s.holds(3) || !s.holds(4) {
		t.Fatal("oldest line was not the one dropped", s.slot)
	}
	keep(5, 50) // the empty place of line 2
	keep(6, 60) // line 3's place
	if s.holds(3) || !s.holds(4) || !s.holds(5) || !s.holds(6) {
		t.Fatal(s.slot)
	}
	// A line evicted again while still held keeps its place and its later times.
	keep(4, 70)
	if victim, _ := s.take(4); victim.readyAt != 70 || victim.leftAt != 65 {
		t.Fatal(victim)
	}
}

// A line pushed out of L2 before its own fill arrived cannot be supplied
// before that data exists and has been moved: the memory-side latency counts
// from the end of its transfer.
func TestMemorySideHitWaitsForEvictedPendingFill(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	if _, ready := c.Access(lineA, 8, false, 100); ready != 435 {
		t.Fatal(ready)
	}
	c.Access(lineB, 8, false, 101)
	c.Access(lineC, 8, false, 102) // evicts A at cycle 102, 333 cycles before its data
	level, ready := c.Access(lineA, 8, false, 103)
	if want := uint64(435 + spillCycles + 138); level != HitSLC || ready != want {
		t.Fatalf("level %d ready %d, want memory-side hit at %d", level, ready, want)
	}
}

// Requests are visited in dispatch order. A request issued at an earlier
// cycle than the eviction that put its line behind L2, but visited after it,
// is not made to wait for that eviction: at its own cycle the line is still
// in L2, and L2 supplies it. Nothing is fetched and nothing is counted as a
// miss.
func TestRequestIssuedBeforeTheEvictionFindsItsLineInL2(t *testing.T) {
	evicted := func() *Cache {
		c := NewCache(memorySideConfig(4))
		c.Access(lineA, 8, false, 100)
		c.Access(lineB, 8, false, 200)
		c.Access(lineC, 8, false, 5000) // evicts A at cycle 5000
		if at, held := heldFrom(c, lineA); !held || at != 5000+spillCycles {
			t.Fatal("setup did not leave A behind L2", at, held)
		}
		return c
	}
	c := evicted()
	fills, misses := len(c.dram.busy), c.stats.L2Misses
	// An L2 hit issued at 1000: 14 cycles, the last two of them the L1 transfer.
	level, ready := c.Access(lineA, 8, false, 1000)
	if level != HitL2 || ready != 1000+14 {
		t.Fatalf("level %d ready %d, want an L2 hit at %d", level, ready, 1000+14)
	}
	if c.stats.SLCHits != 0 || c.stats.L2HitsBeforeEviction != 1 || c.stats.L2Misses != misses || c.stats.L2Hits != 1 || len(c.dram.busy) != fills {
		t.Fatalf("counted or fetched as a miss: %+v, fills %d -> %d", c.stats, fills, len(c.dram.busy))
	}
	// The line is in the tags again and no longer behind L2. The tags changed as
	// they did when a line came back from the memory side: it displaced the
	// least recently used line, B, and the set still holds two.
	if _, held := heldFrom(c, lineA); held || c.l2.peek(lineA) == nil || c.l2.peek(lineC) == nil || c.l2.peek(lineB) != nil {
		t.Fatal("line did not return to the L2 tags in place of the least recently used one")
	}
	// B is what C's request evicts instead of A, so B leaves when C arrives, at
	// 5000 and not at 1000, behind A's own move, which stays booked.
	if victim := c.slc.ring[c.slc.slot[c.l2Line(lineB)]]; victim.leftAt != 5000 || victim.readyAt != 5000+2*spillCycles {
		t.Fatalf("displaced line left at %d and is held from %d, want 5000 and %d", victim.leftAt, victim.readyAt, 5000+2*spillCycles)
	}
	// A second request for the line, again before 5000, is a plain L2 hit: its
	// other L1 half is not in L1, and nothing more is counted or fetched.
	if level, ready := c.Access(lineA+64, 8, false, 1500); level != HitL2 || ready != 1500+14 ||
		c.stats.L2HitsBeforeEviction != 1 || c.stats.L2Hits != 2 || len(c.dram.busy) != fills {
		t.Fatal("second early request", level, ready, c.stats)
	}
	// B itself is still in L2 until 5000 and comes from behind it afterwards;
	// A, read before the eviction, was not the line evicted and is still here.
	if level, ready := c.Access(lineB, 8, false, 4999); level != HitL2 || ready != 4999+14 {
		t.Fatal("displaced line before the eviction", level, ready)
	}
	c = evicted()
	c.Access(lineA, 8, false, 1000)
	if level, ready := c.Access(lineB, 8, false, 6000); level != HitSLC || ready != 6000+138 {
		t.Fatal("displaced line after the eviction", level, ready)
	}
	if c.l2.peek(lineA) == nil || c.stats.SLCHits != 1 {
		t.Fatal("the line read before the eviction did not stay", c.stats)
	}
	// The same holds for a prefetch or writeback allocation of that line.
	c = evicted()
	if ready, level := c.fetchL2(lineA, 1000, true); level != HitL2 || ready != 435 || c.l2.peek(lineA).prefetched {
		t.Fatal(level, ready)
	}
	// From the cycle the line left L2 on, it comes from behind L2 as before.
	c = evicted()
	if level, ready := c.Access(lineA, 8, false, 5000); level != HitSLC || ready != 5000+spillCycles+138 || c.stats.L2HitsBeforeEviction != 0 {
		t.Fatal(level, ready, c.stats)
	}
}

// A line replaced before its own fill arrived is, for a request issued before
// it left L2, a hit on a pending L2 line: it waits for that fill and no longer.
func TestRequestBeforeTheEvictionWaitsForThePendingFill(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	c.Access(lineB, 8, false, 10)
	if ready, _ := c.fetchL2(lineA, 100, true); ready != 435 { // a prefetch, arriving at 435
		t.Fatal(ready)
	}
	c.Access(lineC, 8, false, 300) // replaces B; A is next
	c.Access(lineD, 8, false, 400) // replaces A at 400, before its data
	level, ready := c.Access(lineA, 8, false, 200)
	if level != HitL2 || ready != 435+14 || c.stats.PrefetchedL2Hits != 1 || c.stats.L2HitsInFlight != 1 {
		t.Fatalf("level %d ready %d %+v, want a hit on the pending prefetch at %d", level, ready, c.stats, 435+14)
	}
}

// A dirty victim has left L2 when its write transfer ends. A request issued
// before that end still finds it in L2.
func TestRequestBeforeADirtyVictimsWritebackEndsFindsItInL2(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	c.Store(lineA, 8, 100)
	c.Access(lineB, 8, false, 2000) // A leaves L1 dirty
	c.Access(lineC, 8, false, 3000) // B leaves L2 clean
	c.Access(lineD, 8, false, 4000) // A leaves L2 dirty; its write ends at 4008
	if level, ready := c.Access(lineA, 8, false, 4007); level != HitL2 || ready != 4007+14 || c.stats.L2Writebacks != 1 {
		t.Fatal(level, ready, c.stats)
	}
}

// The line an early request displaces can be dirty. Its write traffic is still
// charged, from the cycle of the later request that evicts it, it is readable
// behind L2 only once written, and none of that delays the early request.
func TestEarlyRequestDisplacingADirtyLineKeepsItsWriteback(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	c.Store(lineA, 8, 100)
	c.Access(lineB, 8, false, 2000) // A leaves L1 dirty and is the newer L2 line
	c.Access(lineC, 8, false, 3000) // B leaves L2 clean at 3000; L2 holds dirty A and C
	bytes := c.stats.WritebackBytes
	level, ready := c.Access(lineB, 8, false, 2500) // before 3000: B is still in L2
	if level != HitL2 || ready != 2500+14 || c.stats.L2HitsBeforeEviction != 1 {
		t.Fatalf("level %d ready %d %+v, want an undelayed L2 hit at %d", level, ready, c.stats, 2500+14)
	}
	// A is what C's request evicts instead: written from 3000 for eight cycles.
	at, held := heldFrom(c, lineA)
	if !held || at != 3008 || c.stats.L2Writebacks != 1 || c.stats.WritebackBytes != bytes+128 || c.DrainUntil() != 3008 {
		t.Fatalf("dirty line held from %d (%v), drain %d, %+v; want its write to end at 3008", at, held, c.DrainUntil(), c.stats)
	}
	if c.l2.peek(lineB) == nil || c.l2.peek(lineC) == nil || c.l2.peek(lineA) != nil {
		t.Fatal("the set does not hold exactly the two lines that were not displaced")
	}
	// Until its write has ended A is in L2; from then on it comes from behind it.
	if level, ready := c.Access(lineA, 8, false, 3008); level != HitSLC || ready != 3008+138 {
		t.Fatal(level, ready)
	}
}

// A stream prefetch of a line that has not left L2 yet fetches nothing, and a
// demand request that follows is an ordinary hit: one line is never paid for
// twice.
func TestPrefetchBeforeTheEvictionFetchesNothing(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	c.Access(lineA, 8, false, 100)
	c.Access(lineB, 8, false, 200)
	c.Access(lineC, 8, false, 5000) // evicts A at 5000
	fills, spills := len(c.dram.busy), len(c.spill.busy)
	if ready, level := c.fetchL2(lineA, 1000, true); level != HitL2 || ready != 435 {
		t.Fatal(level, ready)
	}
	if level, ready := c.Access(lineA, 8, false, 1200); level != HitL2 || ready != 1200+14 {
		t.Fatal(level, ready)
	}
	// No fill for A; one more move behind L2, of the line A displaced.
	if len(c.dram.busy) != fills || len(c.spill.busy) != spills || c.spill.busy[len(c.spill.busy)-1].End != 5000+2*spillCycles ||
		c.stats.SLCHits != 0 || c.stats.L2HitsBeforeEviction != 1 || c.stats.L2Misses != 3 {
		t.Fatalf("fills %d -> %d, spill %+v, %+v", fills, len(c.dram.busy), c.spill.busy, c.stats)
	}
}

// Moving a clean victim occupies the spill transport, not the fill transport:
// the fill that displaced it arrives on time, no write traffic is counted,
// and the victim can be supplied once its 128 bytes have been moved.
func TestCleanVictimMovesOnTheSpillTransport(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	c.Access(lineA, 8, false, 1000)
	c.Access(lineB, 8, false, 2000)
	fills := len(c.dram.busy)
	if level, ready := c.Access(lineC, 8, false, 3000); level != HitDRAM || ready != 3335 {
		t.Fatal("fill that displaced a clean victim was delayed", level, ready)
	}
	if len(c.dram.busy) != fills+1 || c.stats.L2Writebacks != 0 || c.stats.WritebackBytes != 0 || c.DrainUntil() != 0 {
		t.Fatalf("clean victim was charged as a fill or a write: fills=%d stats=%+v drain=%d", len(c.dram.busy)-fills, c.stats, c.DrainUntil())
	}
	if len(c.spill.busy) != 1 || c.spill.busy[0] != (fillInterval{Start: 3000, End: 3000 + spillCycles}) {
		t.Fatalf("spill transport %+v, want one transfer of %d cycles from 3000", c.spill.busy, spillCycles)
	}
	if at, held := heldFrom(c, lineA); !held || at != 3000+spillCycles {
		t.Fatalf("clean victim held from %d (%v), want %d", at, held, 3000+spillCycles)
	}
	// Asked for before its move has ended, it comes back after the move.
	if level, ready := c.Access(lineA, 8, false, 3001); level != HitSLC || ready != 3000+spillCycles+138 {
		t.Fatal(level, ready)
	}
}

// The spill transport is finite: victims that leave L2 in the same cycle
// arrive behind L2 one line after another, and so do their returns.
func TestVictimsLeavingTogetherQueueOnTheSpillTransport(t *testing.T) {
	cfg := memorySideConfig(8)
	cfg.L2Size, cfg.L2Ways = 4*128, 2 // two sets of two lines
	c := NewCache(cfg)
	const set1 = 128 // the second L2 set
	for _, addr := range []uint64{lineA, lineB, lineA + set1, lineB + set1} {
		c.Access(addr, 8, false, 1000)
	}
	// One line into each set at the same cycle evicts A and A+set1 together.
	c.Access(lineC, 8, false, 5000)
	c.Access(lineC+set1, 8, false, 5000)
	first, _ := heldFrom(c, lineA)
	second, _ := heldFrom(c, lineA+set1)
	if first != 5000+spillCycles || second != 5000+2*spillCycles {
		t.Fatalf("victims held from %d and %d, want %d and %d", first, second, 5000+spillCycles, 5000+2*spillCycles)
	}
	// Retired at the simulation clock like the other transports.
	c.Advance(6000)
	if len(c.spill.busy) != 0 {
		t.Fatal("finished spill transfers were not retired", c.spill.busy)
	}
}

// Memory-side fills share the finite fill transport with DRAM fills.
func TestMemorySideFillsUseTheMemoryTransport(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	for i, addr := range []uint64{lineA, lineB, lineC, lineD} {
		c.Access(addr, 8, false, uint64(1000*(i+1)))
	}
	// A and B are held behind L2. Requested in the same cycle, the second
	// transfer of a 128 B line follows the first by its 8-cycle occupancy.
	if _, ready := c.Access(lineA, 8, false, 9000); ready != 9138 {
		t.Fatal(ready)
	}
	if level, ready := c.Access(lineB, 8, false, 9000); level != HitSLC || ready != 9146 {
		t.Fatal("second memory-side fill did not queue on the transport", level, ready)
	}
}

// A held line's return cannot use the fill transport before the line is
// available behind L2, even when the return latency is shorter than the
// transfer of the line itself.
func TestMemorySideReturnTransferStartsWhenTheLineIsAvailable(t *testing.T) {
	cfg := memorySideConfig(4)
	cfg.SLCLatency = 1 // below the eight cycles a 128 B line occupies the transport
	for _, tc := range []struct {
		name            string
		request, starts uint64
	}{
		{"requested before the line is available", 50, 100},
		{"requested after the line is available", 200, 200},
	} {
		c := NewCache(cfg)
		c.slc.keep(victimLine{line: lineD / 128, readyAt: 100})
		ready, level := c.fetchL2(lineD, tc.request, false)
		want := fillInterval{Start: tc.starts, End: tc.starts + spillCycles}
		if level != HitSLC || ready != want.End {
			t.Fatalf("%s: level %d ready %d, want a memory-side hit at %d", tc.name, level, ready, want.End)
		}
		if len(c.dram.busy) != 1 || c.dram.busy[0] != want {
			t.Fatalf("%s: fill transport %+v, want %+v", tc.name, c.dram.busy, want)
		}
	}
	// The same through the public path: the load's data is ready when the
	// transfer that started at availability has ended.
	c := NewCache(cfg)
	c.slc.keep(victimLine{line: lineD / 128, readyAt: 100})
	if level, ready := c.Access(lineD, 8, false, 50); level != HitSLC || ready != 100+spillCycles {
		t.Fatal("access", level, ready)
	}
}

// A dirty L2 victim still pays its write traffic on the fill transport when
// it leaves L2, is not charged a second time on the spill transport, and its
// bytes cannot be read back from the memory side before they were written.
func TestDirtyVictimKeepsWritebackAndDataOrder(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	if accepted, _ := c.Store(lineA, 8, 100); accepted != 100 {
		t.Fatal(accepted)
	}
	c.Access(lineB, 8, false, 2000) // A leaves L1 dirty; the writeback touches its L2 line
	c.Access(lineC, 8, false, 3000) // B, now the older L2 line, leaves L2 clean
	spilled := len(c.spill.busy)
	c.Access(lineD, 8, false, 4000) // A leaves L2 dirty
	if c.stats.L1Writebacks != 1 || c.stats.L2Writebacks != 1 {
		t.Fatal("dirty eviction traffic was dropped", c.stats)
	}
	// The write transfer of the 128 B line occupies the fill transport for
	// eight cycles from the eviction at 4000; the line is readable behind L2
	// only once that transfer has ended.
	at, held := heldFrom(c, lineA)
	if !held || at != 4008 || c.DrainUntil() != 4008 || len(c.spill.busy) != spilled {
		t.Fatalf("dirty victim held from %d (%v), drain %d, spill transfers %d -> %d; want 4008, 4008 and no new spill", at, held, c.DrainUntil(), spilled, len(c.spill.busy))
	}
	if level, ready := c.Access(lineA, 8, false, 5000); level != HitSLC || ready != 5138 {
		t.Fatal(level, ready)
	}
}

// Stream prefetches and writeback allocations fetch through the same path.
func TestPrefetchOfHeldLineComesFromMemorySideCache(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	c.slc.keep(victimLine{line: lineD / 128, readyAt: 60})
	ready, level := c.fetchL2(lineD, 100, true)
	line := c.l2.peek(lineD)
	if _, held := heldFrom(c, lineD); level != HitSLC || ready != 238 || line == nil || !line.prefetched || c.stats.SLCHits != 1 || held {
		t.Fatal(level, ready, line, c.stats)
	}
}

// The untimed legacy path neither consults nor consumes the memory-side cache.
func TestUntimedLookupIgnoresMemorySideCache(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	for i, addr := range []uint64{lineA, lineB, lineC} {
		c.Access(addr, 8, false, uint64(1000*(i+1)))
	}
	if got := c.Lookup(lineA, 8, false); got != HitDRAM || c.stats.SLCHits != 0 {
		t.Fatal(got, c.stats)
	}
}

func TestMemorySideCacheIsSharedWithTheSharedL2(t *testing.T) {
	cfg := memorySideConfig(4)
	shared := NewSharedL2(cfg)
	first, second := NewPrivateL1WithSharedL2(cfg, shared), NewPrivateL1WithSharedL2(cfg, shared)
	for i, addr := range []uint64{lineA, lineB, lineC} {
		first.Access(addr, 8, false, uint64(1000*(i+1)))
	}
	if first.spill != second.spill || first.spill == nil {
		t.Fatal("spill transport is not shared with the shared L2")
	}
	if level, ready := second.Access(lineA, 8, false, 5000); level != HitSLC || ready != 5138 {
		t.Fatal("other core did not see the shared memory-side cache", level, ready)
	}
	// The shared spill transport is retired at the slowest client's clock.
	first.Advance(9000)
	if len(shared.spill.busy) == 0 {
		t.Fatal("spill history retired before every client passed it")
	}
	second.Advance(9000)
	if len(shared.spill.busy) != 0 {
		t.Fatal("finished spill transfers were not retired", shared.spill.busy)
	}
}

// Configuration is compared when a client attaches. A shared L2 that has not
// started timing takes a client with an identical configuration and refuses
// one whose memory-side cache differs, for that reason and no other.
func TestSharedL2RefusesADifferentMemorySideConfiguration(t *testing.T) {
	cfg := memorySideConfig(4)
	for name, mutate := range map[string]func(*CacheConfig){
		"latency": func(c *CacheConfig) { c.SLCLatency = 100 },
		"size":    func(c *CacheConfig) { c.SLCSize = 8 * 128 },
	} {
		shared := NewSharedL2(cfg)
		if NewPrivateL1WithSharedL2(cfg, shared).slc != shared.slc {
			t.Fatal("identical configuration did not attach to the shared memory-side cache")
		}
		other := cfg
		mutate(&other)
		func() {
			defer func() {
				if got := recover(); got != "mem: inconsistent shared L2 timing configuration" {
					t.Fatalf("different %s: recovered %v, want the configuration panic", name, got)
				}
			}()
			NewPrivateL1WithSharedL2(other, shared)
		}()
	}
}

func TestMemorySideConfigurationIsValidated(t *testing.T) {
	for _, mutate := range []func(*CacheConfig){
		func(c *CacheConfig) { c.SLCLatency = 0 },
		func(c *CacheConfig) { c.SLCSize = 64 },
	} {
		cfg := memorySideConfig(4)
		mutate(&cfg)
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("invalid memory-side configuration accepted: %+v", cfg)
				}
			}()
			NewCache(cfg)
		}()
	}
}
