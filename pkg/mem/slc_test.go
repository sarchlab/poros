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
	s.keep(1, 10)
	s.keep(2, 20)
	s.keep(3, 30)
	if at, ok := s.take(2); !ok || at != 20 || s.holds(2) {
		t.Fatal(at, ok)
	}
	s.keep(4, 40) // takes line 1's place: three evictions after line 1 arrived
	if s.holds(1) || !s.holds(3) || !s.holds(4) {
		t.Fatal("oldest line was not the one dropped", s.slot)
	}
	s.keep(5, 50) // the empty place of line 2
	s.keep(6, 60) // line 3's place
	if s.holds(3) || !s.holds(4) || !s.holds(5) || !s.holds(6) {
		t.Fatal(s.slot)
	}
	// A line evicted again while still held keeps its place and its later time.
	s.keep(4, 70)
	if at, _ := s.take(4); at != 70 {
		t.Fatal(at)
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
// finds the line there; it must not be supplied before the line left L2 and
// was moved.
func TestMemorySideHitCannotPrecedeTheEviction(t *testing.T) {
	want := uint64(5000 + spillCycles + 138)
	c := NewCache(memorySideConfig(4))
	c.Access(lineA, 8, false, 100)
	c.Access(lineB, 8, false, 200)
	c.Access(lineC, 8, false, 5000) // evicts A at cycle 5000
	level, ready := c.Access(lineA, 8, false, 1000)
	if level != HitSLC || ready != want {
		t.Fatalf("level %d ready %d, want a memory-side hit at %d", level, ready, want)
	}
	// The same holds for a prefetch or writeback allocation of that line.
	c = NewCache(memorySideConfig(4))
	c.Access(lineA, 8, false, 100)
	c.Access(lineB, 8, false, 200)
	c.Access(lineC, 8, false, 5000)
	if ready, level := c.fetchL2(lineA, 1000, true); level != HitSLC || ready != want {
		t.Fatal(level, ready)
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
	c.slc.keep(lineD/128, 60)
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
	other := cfg
	other.SLCLatency = 100
	defer func() {
		if recover() == nil {
			t.Fatal("inconsistent memory-side configuration attached")
		}
	}()
	NewPrivateL1WithSharedL2(other, shared)
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
