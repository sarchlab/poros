package mem

// writeAllocation owns one pending line allocation. Forwarding byte ranges
// remain in the CPU store records. Replacement can evict a
// line before it arrives, so the generation (not just its address) identifies
// the allocation. Slots retain their latest completion even for future issues.
type writeAllocation struct {
	Line, Generation uint64
	Start, Ready     uint64
}

// Store receives bytes whose address and data are available at now. It accepts
// them into the finite write-allocation stage. accepted is when
// the instruction's store-buffer slot may be released (subject to its own data
// latency); drained is when allocation/write traffic completes. This is distinct
// from Access, whose return value always means data readiness.
//
// Resident L1 stores retain the ordinary Access completion. Pending stores to
// the same resident allocation share a bounded line entry, without making the
// fill instantaneous. Queue capacity is reserved conservatively in dispatch
// order: an earlier-issuing younger request may wait for a future reservation,
// but can never erase it or exceed the finite number of slots.
func (c *Cache) Store(addr uint64, size int, now uint64) (accepted, drained uint64) {
	if size <= 0 {
		panic("mem: nonpositive store size")
	}
	accepted = now
	for size > 0 {
		n := c.cfg.L1LineSize - int(addr%uint64(c.cfg.L1LineSize))
		if n > size {
			n = size
		}
		a, d := c.storeLine(addr, n, now)
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

func (c *Cache) storeLine(addr uint64, size int, now uint64) (uint64, uint64) {
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

func later(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func (cl *cacheLevel) victim(addr uint64) *cacheLine {
	set := cl.sets[cl.setIndex(addr)]
	var victim *cacheLine
	for i := range set {
		if !set[i].valid {
			return nil
		}
		if victim == nil || set[i].lastUsed < victim.lastUsed {
			victim = &set[i]
		}
	}
	return victim
}

func (c *Cache) installL1(addr, ready uint64, prefetched bool, now uint64) *cacheLine {
	if victim := c.l1.victim(addr); victim != nil && victim.dirty {
		ready = later(ready, c.writebackL1(*victim, now))
	}
	return c.l1.installAt(addr, ready, prefetched)
}

func (c *Cache) installL2(addr, ready uint64, prefetched bool, now uint64) *cacheLine {
	if victim := c.l2.victim(addr); victim != nil && victim.dirty {
		admitted, end := reserveWriteback(c.l2Writebacks, c.dram, now, later(victim.readyAt, victim.dirtyAt), c.cfg.L2LineSize)
		ready = later(ready, admitted)
		c.writebackUntil = later(c.writebackUntil, end)
		c.stats.L2Writebacks++
		c.stats.WritebackBytes += uint64(c.cfg.L2LineSize)
	}
	return c.l2.installAt(addr, ready, prefetched)
}

func (c *Cache) writebackL1(victim cacheLine, now uint64) uint64 {
	addr := victim.tag * uint64(c.cfg.L1LineSize)
	start := later(now, later(victim.readyAt, victim.dirtyAt))
	line := c.l2.find(addr)
	if line == nil {
		// Preserve the other L1-sized portion(s) of this L2 line. A partial
		// dirty line never silently owns bytes that were not written.
		c.fetchL2(addr, start, false)
		line = c.l2.peek(addr)
	}
	start = later(start, line.readyAt)
	admitted, end := reserveWriteback(c.l1Writebacks, c.channel, now, start, c.cfg.L1LineSize)
	line.dirty = true
	line.dirtyAt = later(line.dirtyAt, end)
	line.readyAt = later(line.readyAt, end)
	c.writebackUntil = later(c.writebackUntil, end)
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
	admitted = later(now, slots[slot])
	end = channel.deliver(later(admitted, dataReady), bytes)
	slots[slot] = end
	return
}
