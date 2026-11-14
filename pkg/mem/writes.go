package mem

// writeAllocation owns one pending line allocation. Forwarding byte ranges
// remain in the CPU store records. Replacement can evict a
// line before it arrives, so the generation (not just its address) identifies
// the allocation. Slots retain their latest completion even for future issues.
type writeAllocation struct {
	Line, Generation uint64
	Start, Ready     uint64
}

// Store receives the bytes of one store, whose address and data are available
// at now. It accepts them into the finite write-allocation stage. accepted is
// when the instruction's store-buffer slot may be released (subject to its own
// data latency); drained is when allocation/write traffic completes. This is
// distinct from Access, whose return value always means data readiness.
//
// Resident L1 stores retain the ordinary Access completion as acceptance. A
// miss/merge can be accepted earlier, when its pending-line slot is available;
// the CPU still retains its instruction slot through execution completion.
// This deliberate asymmetry preserves existing hit timing. Pending stores to
// the same resident allocation share a bounded line entry, without making the
// fill instantaneous. Queue capacity is reserved conservatively in dispatch
// order: an earlier-issuing younger request may wait for a future reservation,
// but can never erase it or exceed the finite number of slots.
//
// Store is ClassifyStore followed by StorePart, for a store the caller holds
// in one piece. With InterleavedStoreRunsWait, calls must therefore come in
// program order, and a miss or merge by a store of an interleaved run is
// accepted only when its line has arrived.
func (c *Cache) Store(addr uint64, size int, now uint64) (accepted, drained uint64) {
	return c.StorePart(c.ClassifyStore(addr, size), addr, size, now)
}

// StoreClass is the store path's decision about one store, taken once from
// the store's whole range and carried by every piece of it.
type StoreClass struct {
	addr, end uint64 // the classified bytes [addr, end)
	waits     bool   // a store of an established, interleaved run (storeRun)
}

// ClassifyStore decides how the store path treats one store: all the bytes
// [addr, addr+size) that one instruction writes, however many accesses its
// backend records them in and however many cache lines they touch. With
// InterleavedStoreRunsWait it advances the followed run (storeRun) by that
// store, so calls must come in program order, exactly once per store. The
// store's pieces then go to StorePart with the result.
func (c *Cache) ClassifyStore(addr uint64, size int) StoreClass {
	if size <= 0 {
		panic("mem: nonpositive store size")
	}
	class := StoreClass{addr: addr, end: addr + uint64(size)}
	if c.cfg.InterleavedStoreRunsWait {
		class.waits = c.run.follow(addr, uint64(size), uint64(c.cfg.L1LineSize), uint64(c.cfg.TLBPageBytes))
	}
	return class
}

// StorePart is Store for the piece [addr, addr+size) of a store classified by
// ClassifyStore. A caller delivers a store in pieces when each has its own
// address translation and so its own now; the pieces share the store's class
// and keep their own line allocations.
func (c *Cache) StorePart(class StoreClass, addr uint64, size int, now uint64) (accepted, drained uint64) {
	if size <= 0 {
		panic("mem: nonpositive store size")
	}
	if addr < class.addr || addr+uint64(size) > class.end {
		panic("mem: store piece outside its classified store")
	}
	accepted = now
	for size > 0 {
		n := c.cfg.L1LineSize - int(addr%uint64(c.cfg.L1LineSize))
		if n > size {
			n = size
		}
		a, d := c.storeLine(addr, n, now, class.waits)
		if a > accepted {
			accepted = a
		}
		if d > drained {
			drained = d
		}
		addr += uint64(n)
		size -= n
	}
	return
}

// storeRun is the one run of contiguous stores the store path follows, in
// program order. It is this model's reading of Apple M1 probe timings and raw
// event counts (evaluation/model-profile.md, "Store runs"), which constrain
// what the core does with a store miss but do not identify its detector or
// its queues. The reading has three cases:
//
//   - a store outside any run allocates its line in L1, and such misses are
//     prefetched (three or four interleaved streams, a stream that leaves
//     gaps);
//   - a run that nothing interrupts is streamed without entering L1 (one
//     ascending or descending stream, alone or among loads);
//   - a run that continues across single stores elsewhere is streamed too,
//     but its lines cannot complete in one piece: each waits for memory, with
//     no lookahead beyond the stores in flight (two interleaved streams, one
//     stream plus another store per trip).
//
// The first two cost what a store miss already costs here; the third is what
// follow reports.
type storeRun struct {
	active bool
	lo, hi uint64 // the bytes [lo, hi) the run has written
	last   uint64 // address of its most recent store
	bytes  uint64
	// elsewhere counts the stores to other places since the run's last store.
	elsewhere int
	// interleaved: a store elsewhere fell between two of the run's stores, and
	// the run has not written a whole line since (clean bytes).
	interleaved bool
	clean       uint64
}

// follow advances the run by one store, the size bytes at addr that one
// instruction writes, and reports whether that store belongs to an
// established, interleaved run: one that has written at least a line and
// continues across stores elsewhere. A store is one step whatever its
// alignment: a caller that splits it at line or word boundaries would make
// one store elsewhere count as two and end the run.
//
// A store continues the run when it starts where the run ends or ends where
// the run starts. Any other store is elsewhere. The run survives one store
// elsewhere between two of its own; it ends at a second one, and at once when
// the store starts in the line of the page (pageBytes) in which the run's
// last store starts: another stream at the same page offset. The store that
// ends a run starts the next.
func (r *storeRun) follow(addr, size, line, pageBytes uint64) bool {
	end := addr + size
	if r.active && (addr == r.hi || end == r.lo) {
		if r.elsewhere > 0 {
			r.interleaved, r.clean = true, 0
		}
		if r.clean += size; r.clean >= line {
			r.interleaved = false
		}
		if addr == r.hi {
			r.hi = end
		} else {
			r.lo = addr
		}
		r.last, r.bytes, r.elsewhere = addr, r.bytes+size, 0
		return r.interleaved && r.bytes >= line
	}
	sameLineOfPage := pageBytes > 0 && addr%pageBytes/line == r.last%pageBytes/line
	if r.active && r.elsewhere == 0 && !sameLineOfPage {
		r.elsewhere = 1
		return false
	}
	*r = storeRun{active: true, lo: addr, hi: end, last: addr, bytes: size, clean: size}
	return false
}

func (c *Cache) storeLine(addr uint64, size int, now uint64, waits bool) (uint64, uint64) {
	line := c.l1.peek(addr)
	if len(c.writes) == 0 || (line != nil && line.readyAt <= now) {
		_, ready := c.accessLine(addr, size, true, now)
		return ready, ready
	}
	lineAddr := addr / uint64(c.cfg.L1LineSize)
	slot := -1
	if line != nil {
		for i := range c.writes {
			w := &c.writes[i]
			if w.Ready > c.watermark && w.Line == lineAddr && w.Generation == line.generation {
				slot = i
				if w.Start > now {
					now = w.Start
				}
				break
			}
		}
	}
	merged := slot >= 0
	if !merged {
		slot = 0
		for i := 1; i < len(c.writes); i++ {
			if c.writes[i].Ready < c.writes[slot].Ready {
				slot = i
			}
		}
		if c.writes[slot].Ready > now {
			now = c.writes[slot].Ready
		}
	}
	_, ready := c.accessLine(addr, size, true, now)
	line = c.l1.peek(addr)
	w := &c.writes[slot]
	if !merged {
		generation := uint64(0)
		if line != nil {
			generation = line.generation
		}
		*w = writeAllocation{Line: lineAddr, Generation: generation, Start: now}
		c.stats.WriteAllocations++
	} else {
		c.stats.WriteMerges++
	}
	if ready > w.Ready {
		w.Ready = ready
	}
	if waits {
		// The store keeps its place in the store queue until its line is here.
		c.stats.StoreRunWaits++
		return max(now, ready), ready
	}
	return now, ready
}

// DrainUntil includes pending line allocations and dirty evictions, even after
// the producing instruction retired or the cache tag was replaced. Clean
// speculative prefetches are not part of store drain.
func (c *Cache) DrainUntil() uint64 {
	until := c.writebackUntil
	for _, w := range c.writes {
		if w.Ready > until {
			until = w.Ready
		}
	}
	return until
}

func (c *Cache) installL1(addr, ready uint64, prefetched bool, now uint64) *cacheLine {
	if victim := c.l1.victim(addr); victim.valid && victim.dirty {
		ready = max(ready, c.writebackL1(*victim, now))
	}
	return c.l1.installAt(addr, ready, prefetched)
}

// installL2 replaces the L2 victim for addr on behalf of a request issued at
// now. With a memory-side cache the victim stays reachable there once it has
// been moved. A dirty victim's write traffic is charged on the fill transport
// as before and it has left L2 when that transfer ends. A clean victim leaves
// at the request's issue cycle and travels on the spill transport
// (keepInSLC); it does not delay the fill that displaced it.
func (c *Cache) installL2(addr, ready uint64, prefetched bool, now uint64) *cacheLine {
	if victim := c.l2.victim(addr); victim.valid {
		left := now
		if victim.dirty {
			admitted, end := reserveWriteback(c.l2Writebacks, c.dram, now, max(victim.readyAt, victim.dirtyAt), c.cfg.L2LineSize)
			ready = max(ready, admitted)
			c.writebackUntil = max(c.writebackUntil, end)
			c.stats.L2Writebacks++
			c.stats.WritebackBytes += uint64(c.cfg.L2LineSize)
			left = end
		}
		c.keepInSLC(*victim, left)
	}
	return c.l2.installAt(addr, ready, prefetched)
}

func (c *Cache) writebackL1(victim cacheLine, now uint64) uint64 {
	addr := victim.tag * uint64(c.cfg.L1LineSize)
	start := max(now, victim.readyAt, victim.dirtyAt)
	line := c.l2.find(addr)
	if line == nil {
		// Preserve the other L1-sized portion(s) of this L2 line. A partial
		// dirty line never silently owns bytes that were not written.
		c.fetchL2(addr, start, false)
		line = c.l2.peek(addr)
	}
	start = max(start, line.readyAt)
	admitted, end := reserveWriteback(c.l1Writebacks, c.channel, now, start, c.cfg.L1LineSize)
	line.dirty = true
	line.dirtyAt = max(line.dirtyAt, end)
	line.readyAt = max(line.readyAt, end)
	c.writebackUntil = max(c.writebackUntil, end)
	c.stats.L1Writebacks++
	c.stats.WritebackBytes += uint64(c.cfg.L1LineSize)
	return admitted
}

// The finite eviction stage owns the dirty line while awaiting data/fill and
// bandwidth. Queue admission is separate from transport service. Dispatch-order
// allocation deliberately does not reclaim a future booking for earlier work.
func reserveWriteback(slots []uint64, channel *fillChannel, now, dataReady uint64, bytes int) (admitted, end uint64) {
	if len(slots) == 0 {
		panic("mem: dirty eviction without a writeback stage")
	}
	slot := 0
	for i := 1; i < len(slots); i++ {
		if slots[i] < slots[slot] {
			slot = i
		}
	}
	admitted = max(now, slots[slot])
	end = channel.deliver(max(admitted, dataReady), bytes)
	slots[slot] = end
	return
}
