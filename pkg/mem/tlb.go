package mem

// tlbEntry is one cached page translation. A translation installed by a walk
// that has not finished yet is present but only usable from readyAt.
type tlbEntry struct {
	valid    bool
	page     uint64
	lastUsed uint64
	readyAt  uint64
}

// tlb models the reach of data-address translation: a set-associative table
// of page translations, indexed by page number and replaced LRU, in front of
// a finite number of page walkers. Like the cache tags, the table evolves in
// caller/dispatch order; walker bookings keep their real times.
type tlb struct {
	sets      [][]tlbEntry
	pageBytes uint64
	latency   uint64
	walkers   []*fillChannel
	counter   uint64
}

func newTLB(cfg CacheConfig) *tlb {
	if cfg.TLBPageBytes <= 0 {
		return nil
	}
	if cfg.TLBSets < 1 || cfg.TLBWays < 1 || cfg.TLBWalkLatency < 1 || cfg.TLBWalkers < 1 {
		panic("mem: translation timing needs positive sets, ways, walk latency and walkers")
	}
	t := &tlb{
		sets:      make([][]tlbEntry, cfg.TLBSets),
		pageBytes: uint64(cfg.TLBPageBytes),
		latency:   uint64(cfg.TLBWalkLatency),
		walkers:   make([]*fillChannel, cfg.TLBWalkers),
	}
	for i := range t.sets {
		t.sets[i] = make([]tlbEntry, cfg.TLBWays)
	}
	for i := range t.walkers {
		// One cycle per "byte": a walk occupies its walker for latency cycles.
		t.walkers[i] = &fillChannel{bytesPerCycle: 1}
	}
	return t
}

// translate returns when the translation of page is usable by an access whose
// address is known at now, and whether a new walk was started for it.
func (t *tlb) translate(page, now uint64) (uint64, bool) {
	set := t.sets[page%uint64(len(t.sets))]
	t.counter++
	victim := &set[0]
	for i := range set {
		e := &set[i]
		if e.valid && e.page == page {
			e.lastUsed = t.counter
			return max(now, e.readyAt), false
		}
		if victim.valid && (!e.valid || e.lastUsed < victim.lastUsed) {
			victim = e
		}
	}
	// The walk starts on the walker that can begin it first, no earlier than
	// the request, and keeps earlier gaps for requests visited out of order.
	best, start := 0, t.walkers[0].start(now, t.latency)
	for i := 1; i < len(t.walkers) && start > now; i++ {
		if candidate := t.walkers[i].start(now, t.latency); candidate < start {
			best, start = i, candidate
		}
	}
	ready := t.walkers[best].deliver(start, int(t.latency))
	*victim = tlbEntry{valid: true, page: page, lastUsed: t.counter, readyAt: ready}
	return ready, true
}

func (t *tlb) prune(now uint64) {
	for _, w := range t.walkers {
		w.prune(now)
	}
}

// start returns when a transfer occupying occ cycles and requested at
// earliestStart would begin, without booking it.
func (ch *fillChannel) start(earliestStart, occ uint64) uint64 {
	start := earliestStart
	for _, slot := range ch.busy {
		if start+occ <= slot.Start {
			break
		}
		if slot.End > start {
			start = slot.End
		}
	}
	return start
}

// Translate returns the cycle at which the address translation for the bytes
// [addr, addr+size) is available to an access whose address is known at now.
// With translation timing disabled, or when every page is already translated,
// that is now. A page that is not in the table starts a page walk on a free
// walker; accesses to a page whose walk is still pending wait for that walk.
// Translation precedes the cache access: callers issue Access or Store no
// earlier than the returned cycle.
func (c *Cache) Translate(addr uint64, size int, now uint64) uint64 {
	if c.tlb == nil {
		return now
	}
	if size <= 0 {
		panic("mem: nonpositive access size")
	}
	if c.timingDone || now < c.watermark {
		panic("mem: translation before watermark or after timing completion")
	}
	ready := now
	end := addr + uint64(size) - 1
	if end < addr {
		end = ^uint64(0) // an access cannot extend past the address space
	}
	last := end / c.tlb.pageBytes
	for page := addr / c.tlb.pageBytes; page <= last; page++ {
		at, walked := c.tlb.translate(page, now)
		if walked {
			c.stats.TLBWalks++
		}
		if at > now {
			c.stats.TLBWaitCycles += at - now
		}
		ready = max(ready, at)
	}
	return ready
}
