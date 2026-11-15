package mem

import "testing"

// strideWalk reads one word every stride bytes, n times from base, and returns
// the cache with the level each read was satisfied from.
func strideWalk(cfg CacheConfig, base uint64, stride int64, n int) (*Cache, []HitLevel) {
	c := NewCache(cfg)
	levels := make([]HitLevel, n)
	for i := range levels {
		levels[i], _ = c.Access(uint64(int64(base)+int64(i)*stride), 8, false, uint64(100+i))
	}
	return c, levels
}

// By default only neighbouring lines form a stream: a walk with a larger
// stride is never prefetched. With a stride limit, the third miss of a
// constant stride starts prefetching the lines the walk will reach.
func TestStreamPrefetcherFollowsConstantStrides(t *testing.T) {
	const base = 0x4000000
	for _, stride := range []int64{6168, -6168, 65536, -65536} { // deriche-like, and exactly the limit
		plain := DefaultCacheConfig()
		c, levels := strideWalk(plain, base, stride, 8)
		if s := c.Stats(); s.Prefetches != 0 || s.StreamsTrained != 0 || levels[7] != HitDRAM {
			t.Fatalf("stride %d without a limit: %+v, last read from level %d", stride, s, levels[7])
		}
		followed := plain
		followed.PrefetchStrideBytes = 65536
		c, levels = strideWalk(followed, base, stride, 8)
		s := c.Stats()
		// Misses 1 to 3 are demand fetches; from the third on every later line was prefetched.
		for i, level := range levels {
			if want := map[bool]HitLevel{true: HitDRAM, false: HitL2}[i < 3]; level != want {
				t.Fatalf("stride %d: read %d from level %d, want %d (%+v)", stride, i, level, want, s)
			}
		}
		if s.StreamsTrained != 1 || s.L2Misses != 3 || s.PrefetchedL2Hits != 5 {
			t.Fatalf("stride %d: %+v", stride, s)
		}
		// Each trigger issues PrefetchDegree lines, one stride apart, ahead of demand.
		if want := uint64(6 * plain.PrefetchDegree); s.Prefetches != want {
			t.Fatalf("stride %d: %d prefetches, want %d", stride, s.Prefetches, want)
		}
		next := uint64(int64(base) + 8*stride)
		if line := c.l2.peek(next); c.Probe(next) != HitL2 || line == nil || !line.prefetched {
			t.Fatalf("stride %d: the next line of the walk is not prefetched into L2", stride)
		}
	}
}

// A stride above the limit, and a walk whose stride changes, are not followed.
func TestStridesBeyondTheLimitOrIrregularAreNotFollowed(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.PrefetchStrideBytes = 4096
	if c, _ := strideWalk(cfg, 0x4000000, 4096+int64(cfg.L2LineSize), 8); c.Stats().Prefetches != 0 {
		t.Fatalf("stride above the limit was prefetched: %+v", c.Stats())
	}
	// Steps of 1000, 1500, 2000 and 2500 bytes: no step repeats, and none is a
	// small multiple of an earlier one.
	c := NewCache(cfg)
	for i, offset := range []uint64{0, 1000, 2500, 4500, 7000} {
		c.Access(0x4000000+offset, 8, false, uint64(100+i))
	}
	if s := c.Stats(); s.Prefetches != 0 || s.StreamsTrained != 0 {
		t.Fatalf("irregular walk was prefetched: %+v", s)
	}
	// A walk whose every other access hits L1 is seen two steps at a time and
	// is still one stream.
	c = NewCache(cfg)
	for i, offset := range []uint64{0, 1000, 3000, 5000, 7000} {
		c.Access(0x4000000+offset, 8, false, uint64(100+i))
	}
	if s := c.Stats(); s.StreamsTrained != 1 || s.Prefetches == 0 {
		t.Fatalf("walk seen every second step was not followed: %+v", s)
	}
}

// Two interleaved walks with the same stride in different arrays are followed
// separately, and the prefetcher runs no farther than PrefetchDistance steps
// ahead of the demand line.
func TestInterleavedStrideWalksAndDistance(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.PrefetchStrideBytes, cfg.PrefetchDistance = 65536, 6
	c := NewCache(cfg)
	const first, second, stride = 0x4000000, 0x9000000, 3344 // symm-like columns of two arrays
	for i := uint64(0); i < 12; i++ {
		for _, base := range []uint64{first, second} {
			level, _ := c.Access(base+i*stride, 8, false, 100+i)
			if i >= 3 && level != HitL2 {
				t.Fatalf("read %d of the walk at %#x from level %d, want a prefetched L2 line", i, base, level)
			}
		}
	}
	if s := c.Stats(); s.StreamsTrained != 2 || s.L2Misses != 6 {
		t.Fatalf("%+v", s)
	}
	for _, base := range []uint64{first, second} {
		within, beyond := base+(11+6)*stride, base+(11+7)*stride
		if c.l2.peek(within) == nil || c.l2.peek(beyond) != nil {
			t.Fatalf("walk at %#x: prefetched to %v, beyond the distance %v", base, c.l2.peek(within) != nil, c.l2.peek(beyond) != nil)
		}
	}
}

// A miss that continues a walk stays with that walk when another stream has
// seen a single miss within the stride limit of it, wherever in the table the
// two entries are.
func TestContinuedStrideIsPreferredToPairing(t *testing.T) {
	const base, stride = 0x4000000, 6168
	for _, strayFirst := range []bool{true, false} {
		cfg := DefaultCacheConfig()
		cfg.PrefetchStrideBytes = 65536
		c := NewCache(cfg)
		now := uint64(100)
		read := func(addr uint64) HitLevel {
			now++
			level, _ := c.Access(addr, 8, false, now)
			return level
		}
		// An unrelated single miss that the walk comes within the limit of
		// only at its tenth step, allocated before or after the walk's entry.
		stray := uint64(base + 20*stride + 100)
		if strayFirst {
			read(stray)
		}
		for i := uint64(0); i < 16; i++ {
			if i == 3 && !strayFirst {
				read(stray)
			}
			if level := read(base + i*stride); i >= 3 && level != HitL2 {
				t.Fatalf("stray first=%v: read %d of the walk from level %d, want a prefetched L2 line", strayFirst, i, level)
			}
		}
		if s := c.Stats(); s.StreamsTrained != 1 {
			t.Fatalf("stray first=%v: %+v", strayFirst, s)
		}
	}
}

// Following strides leaves a sequential stream exactly as it was.
func TestSequentialStreamUnchangedByStrideLimit(t *testing.T) {
	plain := DefaultCacheConfig()
	followed := plain
	followed.PrefetchStrideBytes = 65536
	a, levelsA := strideWalk(plain, 0x4000000, int64(plain.L2LineSize), 64)
	b, levelsB := strideWalk(followed, 0x4000000, int64(plain.L2LineSize), 64)
	if a.Stats() != b.Stats() {
		t.Fatalf("sequential walk differs:\n%+v\n%+v", a.Stats(), b.Stats())
	}
	for i := range levelsA {
		if levelsA[i] != levelsB[i] {
			t.Fatalf("read %d from level %d, with a stride limit %d", i, levelsA[i], levelsB[i])
		}
	}
}
