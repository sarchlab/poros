package mem

// victimStore is the memory-side cache: the lines most recently evicted from
// L2, in the order they left. It has no sets or ways, because nothing
// measured says how the hardware indexes it; a line simply stays for the next
// len(ring) evictions from L2 unless it returns to L2 first. Like the cache
// tags, it changes in caller/dispatch order.
type victimStore struct {
	slot map[uint64]int // L2 line number -> position in ring
	ring []victimLine
	next int // position the next victim takes; whatever is there is dropped
}

type victimLine struct {
	held bool
	line uint64
	// leftAt is the cycle the line left L2: the issue cycle of the request
	// that replaced a clean line, the admission of its write for a dirty one.
	// Until then it was there, with its data arrived at residentAt and
	// prefetched as the L2 line had it.
	leftAt, residentAt uint64
	prefetched         bool
	// readyAt is the cycle from which the line can be supplied from here: it
	// has left L2 and its data has arrived here, for a dirty line at the end
	// of its write.
	readyAt uint64
}

func newVictimStore(lines int) *victimStore {
	return &victimStore{slot: make(map[uint64]int, lines), ring: make([]victimLine, lines)}
}

func (s *victimStore) holds(line uint64) bool {
	_, ok := s.slot[line]
	return ok
}

// find returns the held line without removing it, or nil.
func (s *victimStore) find(line uint64) *victimLine {
	if i, ok := s.slot[line]; ok {
		return &s.ring[i]
	}
	return nil
}

// take removes line and returns it as it was held. Its position stays empty
// until the ring comes round: taking a line does not lengthen the stay of the
// others.
func (s *victimStore) take(line uint64) (victimLine, bool) {
	i, ok := s.slot[line]
	if !ok {
		return victimLine{}, false
	}
	victim := s.ring[i]
	s.ring[i] = victimLine{}
	delete(s.slot, line)
	return victim, true
}

// keep records victim as the newest one, dropping the one that has been here
// for len(ring) evictions. A line that is already held keeps its place and
// the later of its times.
func (s *victimStore) keep(victim victimLine) {
	victim.held = true
	if i, ok := s.slot[victim.line]; ok {
		old := &s.ring[i]
		old.leftAt, old.residentAt = max(old.leftAt, victim.leftAt), max(old.residentAt, victim.residentAt)
		old.readyAt = max(old.readyAt, victim.readyAt)
		return
	}
	if old := s.ring[s.next]; old.held {
		delete(s.slot, old.line)
	}
	s.ring[s.next] = victim
	s.slot[victim.line] = s.next
	s.next = (s.next + 1) % len(s.ring)
}
