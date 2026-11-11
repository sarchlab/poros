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
	// readyAt is the cycle from which the line can be supplied: it has left
	// L2 and its data has arrived here.
	readyAt uint64
}

func newVictimStore(lines int) *victimStore {
	return &victimStore{slot: make(map[uint64]int, lines), ring: make([]victimLine, lines)}
}

func (s *victimStore) holds(line uint64) bool {
	_, ok := s.slot[line]
	return ok
}

// take removes line and returns the cycle from which it could be supplied.
// Its position stays empty until the ring comes round: taking a line does
// not lengthen the stay of the others.
func (s *victimStore) take(line uint64) (uint64, bool) {
	i, ok := s.slot[line]
	if !ok {
		return 0, false
	}
	readyAt := s.ring[i].readyAt
	s.ring[i] = victimLine{}
	delete(s.slot, line)
	return readyAt, true
}

// keep records line as the newest victim, dropping the one that has been
// here for len(ring) evictions. A line that is already held keeps its place.
func (s *victimStore) keep(line, readyAt uint64) {
	if i, ok := s.slot[line]; ok {
		s.ring[i].readyAt = max(s.ring[i].readyAt, readyAt)
		return
	}
	if old := s.ring[s.next]; old.held {
		delete(s.slot, old.line)
	}
	s.ring[s.next] = victimLine{held: true, line: line, readyAt: readyAt}
	s.slot[line] = s.next
	s.next = (s.next + 1) % len(s.ring)
}
