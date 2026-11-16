package mem

// strideStream is one tracked walk of demand L1 misses a constant number of
// bytes apart, too far apart for the sequential stream table to follow. It is
// kept in bytes because such a stride is rarely a whole number of lines.
type strideStream struct {
	valid bool
	last  uint64 // address of the last demand miss observed
	step  int64  // bytes from one miss of the walk to the next; 0 until a second miss
	// stride is the bytes from one access of the walk to the next, as far as
	// the follower has seen: the step, or the smallest whole fraction of it
	// at which a load found its line in L1 (strideHit).
	stride     int64
	frontier   int64 // next address whose line to prefetch
	confidence int
	lastUse    uint64
}

// strideSkip is how many steps a continuation may advance at once: the walk's
// accesses that hit L1 are not seen here.
const strideSkip = 4

// followStride lets the stream prefetcher follow constant strides of up to
// PrefetchStrideBytes (CacheConfig). It is called for a demand load miss at
// addr that continues no sequential stream, and reports whether a stride
// stream took the miss; the sequential table then allocates no entry for it.
//
// A second miss within the stride limit of a stream's first sets its step. A
// miss a few whole steps past the last one continues the stream, and from
// PrefetchTrainMisses consistent misses on it prefetches up to PrefetchDegree
// L2 lines, one step apart, no farther than PrefetchDistance accesses of the
// walk ahead, no farther than those lines have room in L1 (roomInL1) and,
// with a nonzero PrefetchPageBytes, only within that page of the miss. Like
// the sequential table, this one changes in dispatch order.
//
// The distance is counted in accesses because a step is the distance between
// two misses: a walk whose lines are mostly in L1 misses only every few
// accesses, and its step is that many strides. Counted in steps, the
// lookahead of such a walk was many times the walk itself, and short walks
// restarted for every column filled L2 with lines beyond them. Bounded in
// bytes instead, it was one or two steps for a walk whose misses are tens of
// kilobytes apart, and every line arrived after its access.
//
// Limitation: the frontier is a signed 64-bit address, so a walk at or above
// 1<<63 is followed and never prefetched. User programs are loaded below
// 1<<48; other users of the cache get no stride prefetch up there.
func (c *Cache) followStride(addr uint64, now uint64) bool {
	if len(c.strides) == 0 {
		return false
	}
	limit := int64(c.cfg.PrefetchStrideBytes)
	continued, paired, victim := -1, -1, 0
	for i := range c.strides {
		s := &c.strides[i]
		if !s.valid {
			victim = i
			continue
		}
		delta := int64(addr) - int64(s.last)
		switch {
		case s.step != 0 && delta%s.step == 0 && delta/s.step >= 1 && delta/s.step <= strideSkip:
			continued = i
		case s.step == 0 && delta != 0 && delta <= limit && delta >= -limit &&
			(paired < 0 || s.lastUse > c.strides[paired].lastUse):
			paired = i
		}
		if c.strides[victim].valid && s.lastUse < c.strides[victim].lastUse {
			victim = i
		}
	}
	if continued < 0 && paired < 0 {
		c.strides[victim] = strideStream{valid: true, last: addr, confidence: 1, lastUse: c.useTick}
		return false
	}
	// A miss that continues one stream is not also the second miss of another.
	index := continued
	if index < 0 {
		index = paired
	}
	s := &c.strides[index]
	if continued < 0 {
		s.step = int64(addr) - int64(s.last)
		s.stride = s.step
		s.frontier = int64(addr) + s.step
	}
	s.last, s.lastUse = addr, c.useTick
	s.confidence++
	// The frontier never falls behind the demand access.
	if (s.frontier-int64(addr))/s.step < 1 {
		s.frontier = int64(addr) + s.step
	}
	threshold := max(c.cfg.PrefetchTrainMisses, 2)
	if s.confidence < threshold {
		return true
	}
	if s.confidence == threshold {
		c.stats.StreamsTrained++
	}
	for issued := 0; issued < max(c.cfg.PrefetchDegree, 1); issued++ {
		ahead := s.frontier - int64(addr)
		if s.frontier < 0 || ahead/s.stride > int64(c.cfg.PrefetchDistance) || !c.roomInL1(addr, s) {
			break
		}
		if page := c.cfg.PrefetchPageBytes; page > 0 && uint64(s.frontier)/uint64(page) != addr/uint64(page) {
			break // page bounded, as the sequential frontier is: the next page starts with its own miss
		}
		if c.l2.peek(uint64(s.frontier)) == nil && c.notYetEvicted(uint64(s.frontier), now) == nil {
			c.fetchL2(uint64(s.frontier), now, true)
			c.stats.Prefetches++
		}
		s.frontier += s.step
	}
	return true
}

// strideHit tells the follower of a demand load at addr that found its line
// in L1. followStride sees misses only, so the accesses of a walk between two
// of its misses are known from here: a hit that lies a whole fraction of a
// stream's step beyond its last miss, and at least a line, is an access of
// that walk, and the smallest such fraction is the walk's stride.
//
// The follower has no instruction address to tell walks apart. A load of
// something else at such an address would shorten a stream's stride, and with
// it the stream's reach; it cannot lengthen either.
func (c *Cache) strideHit(addr uint64) {
	for i := range c.strides {
		s := &c.strides[i]
		if !s.valid || s.step == 0 {
			continue
		}
		offset, step, stride := int64(addr)-int64(s.last), s.step, s.stride
		if step < 0 {
			offset, step, stride = -offset, -step, -stride
		}
		if offset >= int64(c.cfg.L1LineSize) && offset < stride && step%offset == 0 {
			s.stride = int64(addr) - int64(s.last)
		}
	}
}

// roomInL1 reports whether the line at the frontier of s has room in L1 with
// the lines the follower is already ahead with, when the walk has missed at
// addr: fewer than L1Ways of the steps between the miss and the frontier fall
// into the frontier's L1 set.
//
// The follower fills L2 only. The bound stands for a prefetcher that fills L1
// as well, as the Neoverse V2's does: a set of L1 holds L1Ways lines, so a
// walk whose lines fall into few sets has few of them in L1 at once, and
// lines fetched farther ahead displace the ones about to be read. That such
// a walk is prefetched no farther is an assumption; what the core loses when
// its prefetches conflict was not measured.
func (c *Cache) roomInL1(addr uint64, s *strideStream) bool {
	set, taken := c.l1.setIndex(uint64(s.frontier)), 0
	for at := int64(addr) + s.step; at != s.frontier; at += s.step {
		if at >= 0 && c.l1.setIndex(uint64(at)) == set {
			if taken++; taken == c.cfg.L1Ways {
				return false
			}
		}
	}
	return true
}
