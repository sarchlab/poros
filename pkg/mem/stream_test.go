package mem

import (
	"encoding/json"
	"reflect"
	"testing"
)

func testStreamSpec() StreamSpec {
	return StreamSpec{Enabled: true, TableEntries: 4, Trigger: 2, Distance: 1, Degree: 1, MaxPending: 4, MaxObservations: 16, ReadBytesPerCycle: 16}
}
func testStreamCache() *Cache { v := DefaultCacheConfig(); v.DRAMLatency = 20; return NewCache(v) }
func TestStreamObservationsAndFillsUseActualTime(t *testing.T) {
	c := testStreamCache()
	v := testStreamSpec()
	s := StreamState{}
	// Dispatch order is intentionally not issue order.
	s.Observe(7, 256, 102, 128, v)
	s.Observe(7, 0, 100, 128, v)
	s.Observe(7, 128, 101, 128, v)
	s.Advance(99, c, v)
	if len(s.History) != 0 || len(s.Requests) != 0 || c.l2.contains(384) {
		t.Fatal("future observation trained or filled early")
	}
	s.Advance(102, c, v)
	if len(s.Requests) != 1 || s.Requests[0].Line != 3 || s.Requests[0].Issue != 102 || s.Requests[0].Ready != 122 {
		t.Fatalf("wrong chronological training/request: %+v", s.Requests)
	}
	s.Advance(121, c, v)
	if c.l2.contains(384) {
		t.Fatal("prefetch available before completion")
	}
	s.Advance(122, c, v)
	if !c.l2.contains(384) || c.l1.contains(384) || s.Stats.PrefetchCompleted != 1 || s.Stats.PrefetchInstalled != 1 {
		t.Fatal("completed prefetch must fill only L2")
	}
}
func TestReadFillReservationsPreserveEarlierGaps(t *testing.T) {
	v := testStreamSpec()
	s := StreamState{}
	if ready := s.ReserveDemand(100, 20, 1, 128, v); ready != 120 {
		t.Fatal(ready)
	}
	if ready := s.ReserveDemand(10, 20, 1, 128, v); ready != 30 {
		t.Fatalf("future booking delayed earlier transfer: %d", ready)
	}
	if ready := s.ReserveDemand(11, 20, 1, 128, v); ready != 38 {
		t.Fatalf("overlapping transfer not serialized: %d", ready)
	}
	want := []TransferWindow{{22, 30}, {30, 38}, {112, 120}}
	if !reflect.DeepEqual(s.Transfers, want) {
		t.Fatalf("transfers%+v", s.Transfers)
	}
	c := testStreamCache()
	s.Advance(30, c, v)
	if !reflect.DeepEqual(s.Transfers, want[1:]) {
		t.Fatal("pruned a future transfer")
	}
}
func TestPrefetchSharesDemandTransferServiceAndIsBounded(t *testing.T) {
	c := testStreamCache()
	v := testStreamSpec()
	v.Degree = 4
	v.MaxPending = 2
	v.MaxObservations = 3
	s := StreamState{}
	s.ReserveDemand(10, 20, 1, 128, v)
	for i := 0; i < 4; i++ {
		s.Observe(7, uint64(i*128), uint64(i), 128, v)
	}
	s.Advance(10, c, v)
	if len(s.Requests) != 2 || s.Requests[0].Ready != 38 || s.Stats.QueueFullDropped != 2 || s.Stats.ObservationsDropped != 1 || s.Stats.MaxInFlight != 2 {
		t.Fatalf("capacity/service accounting: requests%+v stats%+v", s.Requests, s.Stats)
	}
	if s.Stats.ReservedReadBytes != 384 {
		t.Fatal("demand and prefetch bytes not both charged")
	}
	// A second trained PC requests the same pending lines; neither is reissued.
	for i := 0; i < 3; i++ {
		s.Observe(8, uint64(i*128), 10, 128, v)
	}
	s.Advance(10, c, v)
	if len(s.Requests) != 2 || s.Stats.PrefetchIssued != 2 || s.Stats.DuplicateSkipped != 2 {
		t.Fatalf("duplicate request allocated capacity: %+v", s.Stats)
	}
}
func TestPrefetchResidentProbeAndCompletionDoNotDuplicateTags(t *testing.T) {
	c := testStreamCache()
	v := testStreamSpec()
	s := StreamState{}
	for i := 0; i < 3; i++ {
		s.Observe(7, uint64(i*128), 0, 128, v)
	}
	s.Advance(0, c, v)
	c.LookupWithoutPrefetch(384, 8, false) // demand installs before queued fill completes
	before := c.Stats()
	s.Advance(20, c, v)
	if s.Stats.PrefetchCompleted != 1 || s.Stats.PrefetchInstalled != 0 || c.Stats() != before {
		t.Fatal("completion duplicated demand fill or changed demand stats")
	}
	matches := 0
	for _, line := range c.l2.sets[c.l2.setIndex(384)] {
		if line.valid && line.tag == c.l2.tag(384) {
			matches++
		}
	}
	if matches != 1 {
		t.Fatal("duplicated resident tag", matches)
	}
}
func TestStreamStateRoundTripAndDisabledMode(t *testing.T) {
	v := testStreamSpec()
	c := testStreamCache()
	s := StreamState{}
	for i := 0; i < 3; i++ {
		s.Observe(7, uint64(i*128), uint64(i), 128, v)
	}
	s.Advance(2, c, v)
	s.Observe(7, 384, 100, 128, v)
	s.ReserveDemand(200, 20, 1, 128, v)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored StreamState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, restored) {
		t.Fatal("pending timing state lost in roundtrip")
	}
	if a, b := s.ReserveDemand(50, 20, 1, 128, v), restored.ReserveDemand(50, 20, 1, 128, v); a != b {
		t.Fatal("restored reservation changed")
	}
	disabled := StreamSpec{}
	empty := StreamState{}
	empty.Observe(1, 0, 0, 128, disabled)
	empty.Advance(100, c, disabled)
	if !reflect.DeepEqual(empty, StreamState{}) {
		t.Fatal("disabled model changed state")
	}
}
