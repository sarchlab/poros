package mem

import "testing"

func TestCache_DRAMMiss(t *testing.T) {
	c := NewCache(DefaultCacheConfig())
	level := c.Lookup(0x1000, 4, false)
	if level != HitDRAM {
		t.Fatalf("cold access: got %d, want HitDRAM (%d)", level, HitDRAM)
	}
	s := c.Stats()
	if s.L1Misses != 1 || s.L2Misses != 1 {
		t.Fatalf("stats: L1Misses=%d L2Misses=%d, want 1,1", s.L1Misses, s.L2Misses)
	}
}

func TestCache_L1Hit(t *testing.T) {
	c := NewCache(DefaultCacheConfig())
	c.Lookup(0x2000, 4, false) // cold miss
	level := c.Lookup(0x2000, 4, false) // should hit L1
	if level != HitL1 {
		t.Fatalf("second access: got %d, want HitL1 (%d)", level, HitL1)
	}
	s := c.Stats()
	if s.L1Hits != 1 {
		t.Fatalf("L1Hits=%d, want 1", s.L1Hits)
	}
}

func TestCache_L1MissL2Hit(t *testing.T) {
	cfg := DefaultCacheConfig()
	c := NewCache(cfg)

	// L1: 256 sets, 8 ways, 64-byte lines.
	// All addresses in this test map to L1 set 0: addr/64 % 256 == 0.
	// stride = 256 * 64 = 16384 bytes between addresses in the same set.
	stride := uint64(cfg.L1LineSize) * uint64(cfg.L1Size/(cfg.L1Ways*cfg.L1LineSize))

	// Fill all 8 ways of L1 set 0.
	for i := 0; i < cfg.L1Ways; i++ {
		c.Lookup(stride*uint64(i), 4, false)
	}

	// This 9th address evicts the LRU (address 0) from L1.
	c.Lookup(stride*uint64(cfg.L1Ways), 4, false)

	// Access address 0 again — should miss L1 but hit L2.
	level := c.Lookup(0, 4, false)
	if level != HitL2 {
		t.Fatalf("evicted address: got %d, want HitL2 (%d)", level, HitL2)
	}
}

func TestCache_LRUEviction(t *testing.T) {
	cfg := DefaultCacheConfig()
	c := NewCache(cfg)

	stride := uint64(cfg.L1LineSize) * uint64(cfg.L1Size/(cfg.L1Ways*cfg.L1LineSize))

	// Fill 8 ways of L1 set 0 with addresses 0..7*stride.
	for i := 0; i < cfg.L1Ways; i++ {
		c.Lookup(stride*uint64(i), 4, false)
	}

	// Re-access ways 1..7 to make way 0 the LRU.
	for i := 1; i < cfg.L1Ways; i++ {
		c.Lookup(stride*uint64(i), 4, false)
	}

	// Install a new line — should evict way 0 (address 0).
	c.Lookup(stride*uint64(cfg.L1Ways), 4, false)

	// Address 0 should now miss L1 but hit L2.
	level := c.Lookup(0, 4, false)
	if level != HitL2 {
		t.Fatalf("LRU victim: got %d, want HitL2 (%d)", level, HitL2)
	}

	// Address 7*stride should still be in L1 (most recently used).
	level = c.Lookup(stride*7, 4, false)
	if level != HitL1 {
		t.Fatalf("non-victim: got %d, want HitL1 (%d)", level, HitL1)
	}
}

func TestCache_Stats(t *testing.T) {
	c := NewCache(DefaultCacheConfig())

	c.Lookup(0x1000, 4, false) // DRAM miss
	c.Lookup(0x1000, 4, false) // L1 hit
	c.Lookup(0x2000, 4, true)  // DRAM miss (write)
	c.Lookup(0x2000, 4, true)  // L1 hit

	s := c.Stats()
	if s.L1Hits != 2 {
		t.Fatalf("L1Hits=%d, want 2", s.L1Hits)
	}
	if s.L1Misses != 2 {
		t.Fatalf("L1Misses=%d, want 2", s.L1Misses)
	}
	if s.L2Misses != 2 {
		t.Fatalf("L2Misses=%d, want 2", s.L2Misses)
	}

	c.ResetStats()
	s = c.Stats()
	if s.L1Hits != 0 || s.L1Misses != 0 || s.L2Hits != 0 || s.L2Misses != 0 {
		t.Fatalf("ResetStats failed: %+v", s)
	}
}

func TestMultiCoreCache(t *testing.T) {
	cfg := DefaultCacheConfig()
	shared := NewSharedL2(cfg)
	c1 := NewPrivateL1WithSharedL2(cfg, shared)
	c2 := NewPrivateL1WithSharedL2(cfg, shared)

	// Cold access from core 1 — misses both L1 and L2, goes to DRAM.
	level := c1.Lookup(0x1000, 8, false)
	if level != HitDRAM {
		t.Fatalf("core1 cold access: got %d, want HitDRAM (%d)", level, HitDRAM)
	}

	// Same address from core 2 — should miss private L1 but hit shared L2.
	level = c2.Lookup(0x1000, 8, false)
	if level != HitL2 {
		t.Fatalf("core2 shared L2 hit: got %d, want HitL2 (%d)", level, HitL2)
	}

	// After core 2's lookup, address should now be in core 2's L1 as well.
	level = c2.Lookup(0x1000, 8, false)
	if level != HitL1 {
		t.Fatalf("core2 L1 hit after install: got %d, want HitL1 (%d)", level, HitL1)
	}

	// Core 1's L1 should still have the address.
	level = c1.Lookup(0x1000, 8, false)
	if level != HitL1 {
		t.Fatalf("core1 L1 hit: got %d, want HitL1 (%d)", level, HitL1)
	}

	// Stats are per-cache (per-core), not shared.
	s1 := c1.Stats()
	s2 := c2.Stats()
	if s1.L2Misses != 1 {
		t.Fatalf("core1 L2Misses=%d, want 1", s1.L2Misses)
	}
	if s2.L2Hits != 1 {
		t.Fatalf("core2 L2Hits=%d, want 1", s2.L2Hits)
	}
}

func TestMultiCoreCache_Independence(t *testing.T) {
	cfg := DefaultCacheConfig()
	shared := NewSharedL2(cfg)
	c1 := NewPrivateL1WithSharedL2(cfg, shared)
	c2 := NewPrivateL1WithSharedL2(cfg, shared)

	// Access different addresses from different cores.
	c1.Lookup(0x10000, 4, false) // DRAM miss
	c2.Lookup(0x20000, 4, false) // DRAM miss

	// Each address should only be in the originating core's L1.
	level := c1.Lookup(0x20000, 4, false) // Not in c1's L1, but in shared L2
	if level != HitL2 {
		t.Fatalf("cross-core L2 hit: got %d, want HitL2 (%d)", level, HitL2)
	}

	level = c2.Lookup(0x10000, 4, false) // Not in c2's L1, but in shared L2
	if level != HitL2 {
		t.Fatalf("cross-core L2 hit: got %d, want HitL2 (%d)", level, HitL2)
	}
}

func TestCache_WriteAllocate(t *testing.T) {
	c := NewCache(DefaultCacheConfig())

	// Write to uncached address — should be a DRAM miss (write-allocate).
	level := c.Lookup(0x3000, 4, true)
	if level != HitDRAM {
		t.Fatalf("write-allocate cold: got %d, want HitDRAM (%d)", level, HitDRAM)
	}

	// Subsequent read should hit L1 (line was allocated).
	level = c.Lookup(0x3000, 4, false)
	if level != HitL1 {
		t.Fatalf("write-allocate read-back: got %d, want HitL1 (%d)", level, HitL1)
	}
}
