package mem

import "testing"

func translationConfig(sets, ways, latency, walkers int) CacheConfig {
	cfg := DefaultCacheConfig()
	cfg.TLBPageBytes = 16384
	cfg.TLBSets, cfg.TLBWays = sets, ways
	cfg.TLBWalkLatency, cfg.TLBWalkers = latency, walkers
	return cfg
}

func TestTranslationDisabledIsImmediate(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.TLBPageBytes = 0
	c := NewCache(cfg)
	for _, addr := range []uint64{0, 0x4000, 0x7fff_ffff_c000} {
		if ready := c.Translate(addr, 8, 100); ready != 100 {
			t.Fatal("disabled translation delayed an access", addr, ready)
		}
	}
	if c.stats.TLBWalks != 0 || c.stats.TLBWaitCycles != 0 {
		t.Fatal(c.stats)
	}
}

func TestTranslationWalksOncePerResidentPage(t *testing.T) {
	c := NewCache(translationConfig(4, 2, 30, 1))
	if ready := c.Translate(0x4008, 8, 100); ready != 130 {
		t.Fatal("first touch did not walk", ready)
	}
	// A second access while the walk is pending waits for that walk.
	if ready := c.Translate(0x7ff8, 8, 110); ready != 130 || c.stats.TLBWalks != 1 {
		t.Fatal("pending walk was not shared", ready, c.stats)
	}
	// Afterwards the page is translated at once.
	if ready := c.Translate(0x4000, 8, 200); ready != 200 || c.stats.TLBWalks != 1 {
		t.Fatal("resident page walked again", ready, c.stats)
	}
	if c.stats.TLBWaitCycles != 30+20 {
		t.Fatal("wait accounting", c.stats.TLBWaitCycles)
	}
}

func TestTranslationReachIsSetAssociativeLRU(t *testing.T) {
	c := NewCache(translationConfig(4, 2, 30, 4))
	page := func(n uint64) uint64 { return n * 16384 }
	// Pages 0, 4 and 8 share set 0 of four; two ways hold two of them.
	c.Translate(page(0), 8, 0)
	c.Translate(page(4), 8, 0)
	c.Translate(page(0), 8, 100) // page 0 becomes the most recently used
	c.Translate(page(8), 8, 100) // replaces page 4
	if c.stats.TLBWalks != 3 {
		t.Fatal(c.stats)
	}
	if ready := c.Translate(page(0), 8, 200); ready != 200 {
		t.Fatal("most recently used page was replaced", ready)
	}
	if ready := c.Translate(page(4), 8, 200); ready != 230 || c.stats.TLBWalks != 4 {
		t.Fatal("least recently used page stayed resident", ready, c.stats)
	}
	// A page of another set is unaffected by the conflict.
	c.Translate(page(1), 8, 300)
	c.Translate(page(0), 8, 300)
	if ready := c.Translate(page(1), 8, 400); ready != 400 {
		t.Fatal("other set disturbed", ready)
	}
}

func TestPageWalksAreBoundedByWalkers(t *testing.T) {
	for _, tc := range []struct {
		walkers int
		want    [4]uint64
	}{
		{1, [4]uint64{130, 160, 190, 220}},
		{2, [4]uint64{130, 130, 160, 160}},
		{4, [4]uint64{130, 130, 130, 130}},
	} {
		c := NewCache(translationConfig(16, 4, 30, tc.walkers))
		for i, want := range tc.want {
			if ready := c.Translate(uint64(i)*16384, 8, 100); ready != want {
				t.Fatalf("%d walkers: walk %d ready at %d, want %d", tc.walkers, i, ready, want)
			}
		}
	}
}

// Requests are visited in dispatch order, which is not time order. A walk
// requested for an earlier cycle uses the walker's earlier gap instead of
// queueing behind a later booking, and never starts before its request.
func TestEarlierWalkUsesEarlierGap(t *testing.T) {
	c := NewCache(translationConfig(16, 4, 30, 1))
	if ready := c.Translate(0, 8, 500); ready != 530 {
		t.Fatal(ready)
	}
	if ready := c.Translate(16384, 8, 100); ready != 130 {
		t.Fatal("earlier request queued behind a later walk", ready)
	}
	// A gap too short for a whole walk is not used.
	if ready := c.Translate(2*16384, 8, 480); ready != 560 {
		t.Fatal("walk overlapped a booked walk", ready)
	}
}

func TestAccessSpanningTwoPagesNeedsBothTranslations(t *testing.T) {
	c := NewCache(translationConfig(16, 4, 30, 1))
	if ready := c.Translate(16384-4, 8, 100); ready != 160 || c.stats.TLBWalks != 2 {
		t.Fatal("cross-page access", ready, c.stats)
	}
}

// An entry replaced while its walk is still in flight is simply gone: the
// access that started the walk keeps the arrival it was given, the walker
// stays booked, and a later access to that page walks again.
func TestEntryEvictedDuringItsWalk(t *testing.T) {
	c := NewCache(translationConfig(4, 2, 30, 1))
	page := func(n uint64) uint64 { return n * 16384 }
	if ready := c.Translate(page(0), 8, 100); ready != 130 {
		t.Fatal(ready)
	}
	// Two more pages of the same set, requested before the first walk ends,
	// queue on the single walker and push page 0 out of its two-way set.
	if ready := c.Translate(page(4), 8, 101); ready != 160 {
		t.Fatal("second walk did not queue behind the first", ready)
	}
	if ready := c.Translate(page(8), 8, 102); ready != 190 {
		t.Fatal("third walk did not queue", ready)
	}
	if ready := c.Translate(page(0), 8, 120); ready != 220 || c.stats.TLBWalks != 4 {
		t.Fatal("evicted in-flight entry was reused or its walker freed", ready, c.stats)
	}
}

func TestTranslationAtTopOfAddressSpace(t *testing.T) {
	c := NewCache(translationConfig(16, 4, 30, 2))
	if ready := c.Translate(^uint64(0)-3, 8, 100); ready != 130 || c.stats.TLBWalks != 1 {
		t.Fatal("wrapping access", ready, c.stats)
	}
	// With one-byte pages the last page number is the largest integer; the
	// page loop must end there instead of wrapping to page zero.
	cfg := translationConfig(16, 4, 30, 2)
	cfg.TLBPageBytes = 1
	c = NewCache(cfg)
	if ready := c.Translate(^uint64(0), 1, 100); ready != 130 || c.stats.TLBWalks != 1 {
		t.Fatal("last addressable page", ready, c.stats)
	}
	// The four bytes up to the top of the address space are four pages: two
	// walkers take them two at a time, and page zero is never touched.
	c = NewCache(cfg)
	if ready := c.Translate(^uint64(0)-3, 8, 100); ready != 160 || c.stats.TLBWalks != 4 {
		t.Fatal("access clamped at the top of the address space", ready, c.stats)
	}
	if ready := c.Translate(0, 1, 500); ready != 530 || c.stats.TLBWalks != 5 {
		t.Fatal("page zero was translated by the wrapping access", ready, c.stats)
	}
}

// Advancing the simulation clock retires finished walks only; a walk booked
// for the future still occupies its walker.
func TestAdvanceKeepsFutureWalks(t *testing.T) {
	c := NewCache(translationConfig(16, 4, 30, 1))
	c.Translate(0, 8, 500)
	c.Advance(100)
	if ready := c.Translate(16384, 8, 490); ready != 560 {
		t.Fatal("future walk was retired early", ready)
	}
	c.Advance(600)
	if ready := c.Translate(2*16384, 8, 600); ready != 630 {
		t.Fatal(ready)
	}
}

func TestTranslationConfigurationIsValidated(t *testing.T) {
	for _, mutate := range []func(*CacheConfig){
		func(c *CacheConfig) { c.TLBSets = 0 },
		func(c *CacheConfig) { c.TLBWays = 0 },
		func(c *CacheConfig) { c.TLBWalkLatency = 0 },
		func(c *CacheConfig) { c.TLBWalkers = 0 },
	} {
		cfg := translationConfig(16, 4, 30, 1)
		mutate(&cfg)
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("invalid translation configuration accepted: %+v", cfg)
				}
			}()
			NewCache(cfg)
		}()
	}
}
