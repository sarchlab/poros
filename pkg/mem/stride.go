package mem

// strideStream is one tracked walk of demand L1 misses a constant number of
// bytes apart, too far apart for the sequential stream table to follow. It is
// kept in bytes because such a stride is rarely a whole number of lines.
type strideStream struct {
	valid      bool
	last       uint64 // address of the last demand miss observed
	step       int64  // bytes from one access of the walk to the next; 0 until a second miss
	frontier   int64  // next address whose line to prefetch
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
// L2 lines, one step apart, no farther than PrefetchDistance steps ahead, no
// farther than the stride limit in bytes ahead and, with a nonzero
// PrefetchPageBytes, only within that page of the miss. Like the sequential
// table, this one changes in dispatch order.
//
// The byte bound matters because a step is the distance between two misses,
// not between two accesses: a walk whose lines are mostly in L1 misses only
// every few accesses, and its step is that many strides. Counted in steps
// alone, the lookahead of such a walk was many times the walk itself, and
// short walks restarted for every column filled L2 with lines beyond them.
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
		if s.frontier < 0 || ahead/s.step > int64(c.cfg.PrefetchDistance) || ahead > limit || ahead < -limit {
			break
		}
		if page := c.cfg.PrefetchPageBytes; page > 0 && uint64(s.frontier)/uint64(page) != addr/uint64(page) {
			break // page bounded, as the sequential frontier is: the next page starts with its own miss
		}
		if c.l2.peek(uint64(s.frontier)) == nil {
			c.fetchL2(uint64(s.frontier), now, true)
			c.stats.Prefetches++
		}
		s.frontier += s.step
	}
	return true
}
