package mem

import (
	"container/heap"
	"fmt"
	"sort"
)

// StreamSpec configures an optional analytical L2 stream prefetcher. The service
// is a read-fill bound, not a complete DRAM model: store/writeback traffic is not
// included. Demand-cache lookup retains its existing program-order approximation.
type StreamSpec struct {
	Enabled           bool `json:"enabled"`
	TableEntries      int  `json:"table_entries"`
	Trigger           int  `json:"trigger"`
	Distance          int  `json:"distance"`
	Degree            int  `json:"degree"`
	MaxPending        int  `json:"max_pending"`
	MaxObservations   int  `json:"max_observations"`
	ReadBytesPerCycle int  `json:"read_bytes_per_cycle"`
}

func (v StreamSpec) Validate() error {
	if !v.Enabled {
		return nil
	}
	if v.TableEntries < 1 || v.TableEntries > 4096 || v.Trigger < 1 || v.Trigger > 64 || v.Distance < 1 || v.Distance > 4096 || v.Degree < 1 || v.Degree > 128 || v.MaxPending < 1 || v.MaxPending > 1024 || v.MaxObservations < 1 || v.MaxObservations > 16384 || v.ReadBytesPerCycle < 1 || v.ReadBytesPerCycle > 4096 {
		return fmt.Errorf("invalid timed L2 stream configuration")
	}
	return nil
}

type StreamStats struct {
	DemandReadLines     uint64 `json:"demand_read_lines"`
	PrefetchIssued      uint64 `json:"prefetch_issued"`
	PrefetchCompleted   uint64 `json:"prefetch_completed"`
	PrefetchInstalled   uint64 `json:"prefetch_installed"`
	ResidentSkipped     uint64 `json:"resident_skipped"`
	DuplicateSkipped    uint64 `json:"duplicate_skipped"`
	QueueFullDropped    uint64 `json:"queue_full_dropped"`
	ObservationsDropped uint64 `json:"observations_dropped"`
	ReservedReadBytes   uint64 `json:"reserved_read_bytes"`
	MaxInFlight         uint64 `json:"max_in_flight"`
}
type TransferWindow struct{ Start, End uint64 }
type StreamObservation struct{ Issue, Sequence, PC, Line uint64 }
type ObservationQueue []StreamObservation

func (q ObservationQueue) Len() int { return len(q) }
func (q ObservationQueue) Less(i, j int) bool {
	return q[i].Issue < q[j].Issue || (q[i].Issue == q[j].Issue && q[i].Sequence < q[j].Sequence)
}
func (q ObservationQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *ObservationQueue) Push(x any)   { *q = append(*q, x.(StreamObservation)) }
func (q *ObservationQueue) Pop() any     { a := *q; x := a[len(a)-1]; *q = a[:len(a)-1]; return x }

type StreamRequest struct{ Line, Issue, Ready, Sequence uint64 }
type RequestQueue []StreamRequest

func (q RequestQueue) Len() int { return len(q) }
func (q RequestQueue) Less(i, j int) bool {
	return q[i].Ready < q[j].Ready || (q[i].Ready == q[j].Ready && q[i].Sequence < q[j].Sequence)
}
func (q RequestQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *RequestQueue) Push(x any)   { *q = append(*q, x.(StreamRequest)) }
func (q *RequestQueue) Pop() any     { a := *q; x := a[len(a)-1]; *q = a[:len(a)-1]; return x }

type StreamHistory struct {
	Line, Used            uint64
	Direction, Confidence int
}

// StreamState is owned by the CPU. Advance is called only at actual CPU cycles.
// Recording a future observation or reserving a future transfer never advances
// the cache or prunes earlier reservations.
type StreamState struct {
	Cycle, Sequence uint64
	Observations    ObservationQueue
	Requests        RequestQueue
	Pending         map[uint64]bool
	History         map[uint64]StreamHistory
	Transfers       []TransferWindow
	Stats           StreamStats
}

func (s *StreamState) initialize() {
	if s.Pending == nil {
		s.Pending = make(map[uint64]bool)
	}
	if s.History == nil {
		s.History = make(map[uint64]StreamHistory)
	}
}
func (s *StreamState) Observe(pc, addr, issue, lineBytes uint64, v StreamSpec) {
	if !v.Enabled {
		return
	}
	if len(s.Observations) >= v.MaxObservations {
		s.Stats.ObservationsDropped++
		return
	}
	s.Sequence++
	heap.Push(&s.Observations, StreamObservation{Issue: issue, Sequence: s.Sequence, PC: pc, Line: addr / lineBytes})
}

// ReserveDemand accounts for DRAM read fills, including bus contention in the
// returned completion. The caller must extend its MSHR before booking more work.
func (s *StreamState) ReserveDemand(issue, latency, lines, lineBytes uint64, v StreamSpec) uint64 {
	if !v.Enabled || lines == 0 {
		return issue + latency
	}
	s.Stats.DemandReadLines += lines
	return s.reserve(issue, latency, lines*lineBytes, v)
}
func (s *StreamState) reserve(issue, latency, bytes uint64, v StreamSpec) uint64 {
	duration := (bytes + uint64(v.ReadBytesPerCycle) - 1) / uint64(v.ReadBytesPerCycle)
	start := issue
	if latency > duration {
		start += latency - duration
	}
	for _, r := range s.Transfers {
		if start+duration <= r.Start {
			break
		}
		if r.End > start {
			start = r.End
		}
	}
	end := start + duration
	at := sort.Search(len(s.Transfers), func(i int) bool { return s.Transfers[i].Start >= start })
	s.Transfers = append(s.Transfers, TransferWindow{})
	copy(s.Transfers[at+1:], s.Transfers[at:])
	s.Transfers[at] = TransferWindow{start, end}
	s.Stats.ReservedReadBytes += bytes
	return end
}
func (s *StreamState) Advance(now uint64, c *Cache, v StreamSpec) {
	if !v.Enabled {
		return
	}
	if now < s.Cycle {
		panic("stream prefetch cannot advance backward")
	}
	s.Cycle = now
	s.initialize()
	n := 0
	for n < len(s.Transfers) && s.Transfers[n].End <= now {
		n++
	}
	if n > 0 {
		copy(s.Transfers, s.Transfers[n:])
		s.Transfers = s.Transfers[:len(s.Transfers)-n]
	}
	for len(s.Requests) > 0 && s.Requests[0].Ready <= now {
		r := heap.Pop(&s.Requests).(StreamRequest)
		delete(s.Pending, r.Line)
		s.Stats.PrefetchCompleted++
		if c.installPrefetchedL2(r.Line * uint64(c.cfg.L2LineSize)) {
			s.Stats.PrefetchInstalled++
		}
	}
	for len(s.Observations) > 0 && s.Observations[0].Issue <= now {
		o := heap.Pop(&s.Observations).(StreamObservation)
		s.train(o, now, c, v)
	}
}
func (s *StreamState) train(o StreamObservation, now uint64, c *Cache, v StreamSpec) {
	h, exists := s.History[o.PC]
	if !exists && len(s.History) >= v.TableEntries {
		var victim, oldest uint64
		first := true
		for pc, e := range s.History {
			if first || e.Used < oldest || (e.Used == oldest && pc < victim) {
				victim, oldest, first = pc, e.Used, false
			}
		}
		delete(s.History, victim)
	}
	s.Sequence++
	h.Used = s.Sequence
	if !exists {
		h.Line = o.Line
		s.History[o.PC] = h
		return
	}
	if o.Line == h.Line {
		s.History[o.PC] = h
		return
	}
	direction := 0
	if o.Line > h.Line && o.Line-h.Line == 1 {
		direction = 1
	} else if h.Line > o.Line && h.Line-o.Line == 1 {
		direction = -1
	}
	if direction == 0 {
		h.Confidence = 0
	} else if direction != h.Direction {
		h.Confidence = 1
	} else if h.Confidence < v.Trigger {
		h.Confidence++
	}
	h.Line, h.Direction = o.Line, direction
	s.History[o.PC] = h
	if direction == 0 || h.Confidence < v.Trigger {
		return
	}
	for offset := 0; offset < v.Degree; offset++ {
		i := v.Distance + offset
		line := o.Line
		if direction > 0 {
			if line > ^uint64(0)-uint64(i) {
				break
			}
			line += uint64(i)
		} else {
			if line < uint64(i) {
				break
			}
			line -= uint64(i)
		}
		if line > ^uint64(0)/uint64(c.cfg.L2LineSize) {
			break
		}
		addr := line * uint64(c.cfg.L2LineSize)
		if c.l2.contains(addr) {
			s.Stats.ResidentSkipped++
			continue
		}
		if s.Pending[line] {
			s.Stats.DuplicateSkipped++
			continue
		}
		if len(s.Requests) >= v.MaxPending {
			s.Stats.QueueFullDropped++
			continue
		}
		ready := s.reserve(now, uint64(c.cfg.DRAMLatency), uint64(c.cfg.L2LineSize), v)
		s.Sequence++
		heap.Push(&s.Requests, StreamRequest{Line: line, Issue: now, Ready: ready, Sequence: s.Sequence})
		s.Pending[line] = true
		s.Stats.PrefetchIssued++
		if uint64(len(s.Requests)) > s.Stats.MaxInFlight {
			s.Stats.MaxInFlight = uint64(len(s.Requests))
		}
	}
}
