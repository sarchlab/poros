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
// in L2, and L2 supplies it. Nothing is fetched, moved or counted as a miss,
// and the line still leaves L2 when it did.
func TestRequestIssuedBeforeTheEvictionFindsItsLineInL2(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	c.Access(lineA, 8, false, 100)
	c.Access(lineB, 8, false, 200)
	c.Access(lineC, 8, false, 5000) // evicts A at cycle 5000
	misses := c.stats.L2Misses
	// B and C are the two lines of the set, A is behind L2 from the end of its
	// move, and nothing has been fetched since the setup.
	asEvicted := func() bool {
		at, held := heldFrom(c, lineA)
		return held && at == 5000+spillCycles && c.l2.peek(lineA) == nil && c.l2.peek(lineB) != nil && c.l2.peek(lineC) != nil &&
			c.stats.L2Misses == misses && c.stats.SLCHits == 0
	}
	if !asEvicted() {
		t.Fatal("setup did not leave A behind L2")
	}
	// An L2 hit issued at 1000: 14 cycles, the last two of them the L1 transfer.
	level, ready := c.Access(lineA, 8, false, 1000)
	if level != HitL2 || ready != 1000+14 {
		t.Fatalf("level %d ready %d, want an L2 hit at %d", level, ready, 1000+14)
	}
	if c.stats.L2HitsBeforeEviction != 1 || c.stats.L2Hits != 1 || !asEvicted() {
		t.Fatalf("counted, fetched or moved as a miss: %+v", c.stats)
	}
	// The line was not consumed: a second request before 5000, for the half
	// that is not in L1, is answered the same way.
	if level, ready := c.Access(lineA+64, 8, false, 1500); level != HitL2 || ready != 1500+14 ||
		c.stats.L2HitsBeforeEviction != 2 || c.stats.L2Hits != 2 || !asEvicted() {
		t.Fatal("second early request", level, ready, c.stats)
	}
	// B was not displaced by either of them and is an ordinary L2 hit.
	if level, ready := c.Access(lineB, 8, false, 4999); level != HitL2 || ready != 4999+14 || c.stats.L2HitsBeforeEviction != 2 {
		t.Fatal("line beside the early hits", level, ready, c.stats)
	}
	// From the cycle A left L2 on it comes from behind L2, once, as it did
	// before: having been read earlier did not keep it in L2. Its return is on
	// time, so the early hits left nothing on the fill transport.
	if level, ready := c.Access(lineA, 8, false, 5000); level != HitSLC || ready != 5000+spillCycles+138 ||
		c.stats.SLCHits != 1 || c.stats.L2Misses != misses+1 {
		t.Fatal("request at the eviction", level, ready, c.stats)
	}
	if _, held := heldFrom(c, lineA); held || c.l2.peek(lineA) == nil {
		t.Fatal("the line did not move back to L2")
	}
	// A request visited after that return is an ordinary hit on the returned
	// line, whatever its cycle.
	if level, ready := c.Access(lineA+64, 8, false, 1600); level != HitL2 || ready != 5000+spillCycles+138+14 || c.stats.L2HitsBeforeEviction != 2 {
		t.Fatal("request visited after the return", level, ready, c.stats)
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

// A dirty victim leaves L2 when the eviction stage admits its write, not when
// the write has ended: from the admission the stage owns the line, and it can
// be read again only from behind L2, once written. Nothing is supplied from
// the stage. Every request here is issued after the one that evicts the line.
func TestDirtyVictimLeavesL2AtWritebackAdmission(t *testing.T) {
	const admission, completion = 4500, 4608
	evicted := func() *Cache {
		c := NewCache(memorySideConfig(4))
		c.Store(lineA, 8, 100)
		c.Access(lineB, 8, false, 2000) // A leaves L1 dirty
		c.Access(lineC, 8, false, 3000) // B leaves L2 clean; L2 holds dirty A and C
		for i := range c.l2Writebacks {
			c.l2Writebacks[i] = admission // every slot of the stage is taken until 4500
		}
		c.dram.deliver(admission, 100*16) // and the transport for 100 cycles from then
		// D's request at 4000 evicts A. A's write is admitted at 4500 and its
		// eight-cycle transfer ends at 4608; D is usable from the admission.
		if level, ready := c.Access(lineD, 8, false, 4000); level != HitDRAM || ready != admission {
			t.Fatalf("level %d ready %d, want the fill that evicts A to wait for the admission at %d", level, ready, admission)
		}
		if at, held := heldFrom(c, lineA); !held || at != completion || c.DrainUntil() != completion || c.stats.L2Writebacks != 1 {
			t.Fatalf("dirty victim held from %d (%v), drain %d, want its write to end at %d", at, held, c.DrainUntil(), completion)
		}
		return c
	}
	for _, tc := range []struct {
		name    string
		request uint64
		level   HitLevel
		ready   uint64
	}{
		{"the cycle before the admission", admission - 1, HitL2, admission - 1 + 14},
		{"at the admission", admission, HitSLC, completion + 138},
		{"while the write waits for the transport", 4550, HitSLC, completion + 138},
		{"in the last cycle of the write", completion - 1, HitSLC, completion + 138},
		{"when the write has ended", completion, HitSLC, completion + 138},
	} {
		c := evicted()
		level, ready := c.Access(lineA, 8, false, tc.request)
		early := map[bool]uint64{true: 1}[tc.level == HitL2]
		if level != tc.level || ready != tc.ready || c.stats.L2HitsBeforeEviction != early || c.stats.SLCHits != 1-early {
			t.Fatalf("%s: level %d ready %d %+v, want level %d at %d", tc.name, level, ready, c.stats, tc.level, tc.ready)
		}
		// The one write of A stays as it was booked.
		if c.stats.L2Writebacks != 1 || c.DrainUntil() != completion {
			t.Fatalf("%s: %+v, drain %d", tc.name, c.stats, c.DrainUntil())
		}
	}
	// In one run and in issue order: the request before the admission reads L2,
	// the next waits for the write and for the return from behind L2.
	c := evicted()
	c.Access(lineA, 8, false, admission-1)
	if level, ready := c.Access(lineA+64, 8, false, admission+1); level != HitSLC || ready != completion+138 ||
		c.stats.L2HitsBeforeEviction != 1 || c.stats.SLCHits != 1 {
		t.Fatalf("level %d ready %d %+v, want a return from behind L2 at %d", level, ready, c.stats, completion+138)
	}
}

// An early hit replaces nothing, so it can need no writeback slot. With every
// L2 writeback slot busy, a request issued before its line left L2 is served
// at once and the dirty line beside it stays where it is. The finite stage
// still binds whoever does displace that dirty line: here the same line's
// return from behind L2 after the eviction.
func TestEarlyHitNeedsNoWritebackSlotAndTheStageStillBinds(t *testing.T) {
	c := NewCache(memorySideConfig(4))
	c.Store(lineA, 8, 100)
	c.Access(lineB, 8, false, 2000) // A leaves L1 dirty and is the newer L2 line
	c.Access(lineC, 8, false, 3000) // B leaves L2 clean at 3000; L2 holds dirty A and C
	for i := range c.l2Writebacks {
		c.l2Writebacks[i] = 10000 // every slot of the stage is taken until 10000
	}
	writebacks, bytes, drain, misses := c.stats.L2Writebacks, c.stats.WritebackBytes, c.DrainUntil(), c.stats.L2Misses
	level, ready := c.Access(lineB, 8, false, 2500) // before 3000: B is still in L2
	if level != HitL2 || ready != 2500+14 || c.stats.L2HitsBeforeEviction != 1 {
		t.Fatalf("level %d ready %d %+v, want an undelayed L2 hit at %d", level, ready, c.stats, 2500+14)
	}
	// Nothing was evicted for it: dirty A and C are the two lines of the set,
	// no write was counted, booked or awaited, and B is still behind L2.
	if a := c.l2.peek(lineA); a == nil || !a.dirty || c.l2.peek(lineC) == nil || c.l2.peek(lineB) != nil {
		t.Fatal("the early hit changed the L2 tags")
	}
	if _, held := heldFrom(c, lineB); !held || c.stats.L2Writebacks != writebacks || c.stats.WritebackBytes != bytes ||
		c.DrainUntil() != drain || c.stats.L2Misses != misses || c.stats.SLCHits != 0 || c.l2Writebacks[0] != 10000 {
		t.Fatalf("the early hit evicted, fetched or booked something: %+v drain %d slots %v", c.stats, c.DrainUntil(), c.l2Writebacks)
	}
	// After 3000 B comes from behind L2 and displaces dirty A. A's write cannot
	// be admitted before 10000, so B is not usable in L2 before then.
	level, ready = c.Access(lineB+64, 8, false, 3500)
	if level != HitSLC || ready != 10000 || c.stats.L2Writebacks != writebacks+1 || c.l2.peek(lineA) != nil {
		t.Fatalf("level %d ready %d %+v, want B back from behind L2 at 10000, when dirty A's write is admitted", level, ready, c.stats)
	}
}

// A store issued before its line left L2 allocates from L2 as a load reads
// from it, and L2 is left without the line. When the dirty L1 line is written
// back, the writeback brings the line back from behind L2 as it always did,
// not before the line is available there, without delaying the access that
// displaced the dirty line.
func TestEarlyStoreIsWrittenBackToALineReturnedFromBehindL2(t *testing.T) {
	for _, tc := range []struct {
		name               string
		writtenBack, drain uint64
	}{
		{"after the eviction", 6000, 6000 + 138 + 2},
		{"before the eviction", 2000, 5000 + spillCycles + 138 + 2},
	} {
		c := NewCache(memorySideConfig(4))
		c.Access(lineA, 8, false, 100)
		c.Access(lineB, 8, false, 200)
		c.Access(lineC, 8, false, 5000) // evicts A at cycle 5000
		if accepted, drained := c.Store(lineA, 8, 1000); accepted != 1000 || drained != 1000+14 ||
			c.stats.L2HitsBeforeEviction != 1 || c.stats.SLCHits != 0 || c.l2.peek(lineA) != nil {
			t.Fatalf("%s: store accepted %d drained %d %+v, want an allocation from L2 by %d", tc.name, accepted, drained, c.stats, 1000+14)
		}
		// B displaces the dirty line from L1. The L2 line is fetched once, from
		// behind L2, and the two-cycle write of the 64 bytes follows its arrival.
		if level, ready := c.Access(lineB, 8, false, tc.writtenBack); level != HitL2 || ready != tc.writtenBack+14 {
			t.Fatalf("%s: level %d ready %d, want an undelayed L2 hit", tc.name, level, ready)
		}
		line := c.l2.peek(lineA)
		if _, held := heldFrom(c, lineA); held || line == nil || !line.dirty || c.stats.SLCHits != 1 || c.stats.L1Writebacks != 1 || c.DrainUntil() != tc.drain {
			t.Fatalf("%s: %+v, drain %d, want the line back in L2, dirty, written by %d", tc.name, c.stats, c.DrainUntil(), tc.drain)
		}
	}
}

// A stream prefetch of a line that has not left L2 yet fetches nothing and
// leaves the line where it is, and the demand request that follows is an L2
// hit: one line is never paid for twice. The sequential and the stride
// prefetcher do the same. Once the line has left L2, a prefetch brings it back.
func TestPrefetchBeforeTheEvictionFetchesNothing(t *testing.T) {
	const base = 0x4000000
	for _, tc := range []struct {
		name        string
		stride      uint64
		strideLimit int
	}{
		{"sequential", 128, 0},
		{"stride", 6168, 65536},
	} {
		for _, left := range []uint64{5000, 60} {
			cfg := DefaultCacheConfig()
			cfg.PrefetchStrideBytes = tc.strideLimit
			c := NewCache(cfg)
			// The fourth line of the walk was in L2 from cycle 50 and was replaced,
			// in dispatch order, by a request issued at left.
			fourth := base + 3*tc.stride
			c.slc.keep(victimLine{line: c.l2Line(fourth), leftAt: left, residentAt: 50, readyAt: left + spillCycles})
			// The third miss trains the stream, which runs four lines ahead.
			for i := uint64(0); i < 3; i++ {
				c.Access(base+i*tc.stride, 8, false, 100+i)
			}
			_, held := heldFrom(c, fourth)
			if left == 60 {
				// It left L2 before the prefetch and comes back with the other three.
				if line := c.l2.peek(fourth); held || line == nil || !line.prefetched || c.stats.Prefetches != 4 || c.stats.SLCHits != 1 {
					t.Fatalf("%s: a line that had left L2 was not prefetched from behind it: %+v", tc.name, c.stats)
				}
				continue
			}
			if !held || c.l2.peek(fourth) != nil || c.stats.Prefetches != 3 || c.stats.SLCHits != 0 {
				t.Fatalf("%s: the prefetch fetched or moved a line that was still in L2: %+v", tc.name, c.stats)
			}
			if level, ready := c.Access(fourth, 8, false, 103); level != HitL2 || ready != 103+14 ||
				c.stats.L2HitsBeforeEviction != 1 || c.stats.SLCHits != 0 || c.stats.L2Misses != 3 {
				t.Fatalf("%s: level %d ready %d %+v, want an L2 hit at %d", tc.name, level, ready, c.stats, 103+14)
			}
		}
	}
}

// The legacy adjacent-line fill asks for its line at the cycle of the demand
// access and finds it in L2 the same way.
func TestAdjacentFillBeforeTheEvictionFindsItsLineInL2(t *testing.T) {
	cfg := memorySideConfig(4)
	cfg.L1Size, cfg.L1Ways = 2*64, 2 // both halves of one L2 line
	cfg.DisableAdjacentL1Prefetch = false
	c := NewCache(cfg)
	c.Access(lineA, 8, false, 100)
	c.Access(lineB, 8, false, 200)
	c.Access(lineC, 8, false, 5000) // evicts A at cycle 5000
	misses := c.stats.L2Misses
	if level, ready := c.Access(lineA, 8, false, 1000); level != HitL2 || ready != 1000+14 {
		t.Fatal(level, ready)
	}
	// The demand half and the adjacent half were each read from L2; nothing
	// was fetched for either, and the line is still behind L2 from 5008.
	if at, held := heldFrom(c, lineA); !held || at != 5000+spillCycles || c.stats.L2HitsBeforeEviction != 2 || c.stats.SLCHits != 0 ||
		c.stats.L2Misses != misses || c.l2.peek(lineA) != nil || c.Probe(lineA+64) != HitL1 {
		t.Fatalf("held from %d (%v), %+v", at, held, c.stats)
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
