package mem

import "testing"

// runTrips replays trips trips of a loop whose stores of size 8 go to the
// given base addresses plus stride*trip, in order, and returns how many of
// the last trip's stores belong to an established interleaved run.
func runTrips(trips int, stride int64, bases ...uint64) int {
	var r storeRun
	waits := 0
	for trip := 0; trip < trips; trip++ {
		waits = 0
		for _, base := range bases {
			if r.follow(uint64(int64(base)+stride*int64(trip)), 8, 64, 16384) {
				waits++
			}
		}
	}
	return waits
}

// The patterns of the M1 store probes (evaluation/model-profile.md, "Store
// runs"): which of them leave a run that continues across stores elsewhere.
func TestStoreRunClassifiesTheMeasuredPatterns(t *testing.T) {
	const (
		p1 = 0x81ea908
		p2 = 0x7d8908 // 8 KiB from p1 modulo the 16 KiB page
		l1 = 0x408008 // an L1-resident word
	)
	for _, tc := range []struct {
		name   string
		stride int64
		bases  []uint64
		waits  int // stores per trip that wait
	}{
		{"one ascending stream", 16, []uint64{p1, p1 + 8}, 0},
		{"one descending stream", -16, []uint64{p1 + 1<<20 + 8, p1 + 1<<20}, 0},
		{"one stream, 8 bytes per trip", 8, []uint64{p1}, 0},
		{"one stream that leaves gaps", 16, []uint64{p1}, 0},
		{"two streams", 8, []uint64{p1, p2}, 1},
		{"two streams 4 KiB apart in the page", 8, []uint64{p1, p2 + 4096}, 1},
		{"two streams entering lines in different trips", 8, []uint64{p1, p2 + 32}, 1},
		{"two streams at one page offset", 8, []uint64{p1, p1 + 2500*16384}, 0},
		{"three streams", 8, []uint64{p1, p2, p1 + 2500*16384}, 0},
		{"four streams", 8, []uint64{p1, p1 + 1250*16384, p2, p2 + 1250*16384}, 0},
		{"four streams at four page offsets", 8, []uint64{p1, p1 + 1250*16384 + 4096, p2, p2 + 1250*16384 + 4096}, 0},
		{"one stream and a fixed store", 16, []uint64{p1, p1 + 8, l1}, 2},
		{"two streams and a fixed store", 8, []uint64{p1, p2, l1}, 0},
		{"one stream and two fixed stores", 16, []uint64{p1, p1 + 8, l1, l1 + 128}, 0},
	} {
		if got := runTrips(64, tc.stride, tc.bases...); got != tc.waits {
			t.Errorf("%s: %d stores of a trip wait, want %d", tc.name, got, tc.waits)
		}
	}
	// A moving store elsewhere interleaves the stream like a fixed one.
	var r storeRun
	waits := 0
	for trip := uint64(0); trip < 64; trip++ {
		waits = 0
		for _, addr := range []uint64{p1 + 16*trip, p1 + 16*trip + 8, l1 + 8*(trip%512)} {
			if r.follow(addr, 8, 64, 16384) {
				waits++
			}
		}
	}
	if waits != 2 {
		t.Errorf("one stream and a moving store: %d stores of a trip wait, want 2", waits)
	}
}

// A run is established only after a whole line, and is interleaved only until
// it has again written a whole line without a store elsewhere.
func TestStoreRunNeedsALineAndForgetsAnInterruption(t *testing.T) {
	var r storeRun
	const a, other = 0x10000, 0x92000 // other: another line of the page
	for i := uint64(0); i < 7; i++ { // 56 bytes: not yet a line
		if r.follow(a+8*i, 8, 64, 16384) || r.follow(other, 8, 64, 16384) {
			t.Fatal("no store waits before the run has written a line, and a store elsewhere never does")
		}
	}
	if !r.follow(a+56, 8, 64, 16384) {
		t.Fatal("the run has written a line across stores elsewhere and must be interleaved")
	}
	for i := uint64(8); i < 14; i++ { // 56 bytes since the store elsewhere: still interleaved
		if !r.follow(a+8*i, 8, 64, 16384) {
			t.Fatalf("store %d: interleaved until a clean line has been written", i)
		}
	}
	if r.follow(a+112, 8, 64, 16384) || r.follow(a+120, 8, 64, 16384) {
		t.Fatal("a whole line without a store elsewhere makes the run clean again")
	}
	// Without a page size there is no same-offset rule: the partner at the
	// same page offset is then an ordinary store elsewhere.
	r = storeRun{}
	waits := false
	for i := uint64(0); i < 32; i++ {
		waits = r.follow(a+8*i, 8, 64, 0)
		r.follow(a+5*16384+8*i, 8, 64, 0)
	}
	if !waits {
		t.Fatal("with translation timing off two streams interleave whatever their page offsets")
	}
}

// A store of an interleaved run is accepted when its line arrives; every other
// store miss, and every store without the rule, is accepted at once.
func TestInterleavedRunStoreWaitsForItsLine(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		cfg := DefaultCacheConfig()
		cfg.InterleavedStoreRunsWait = enabled
		c := NewCache(cfg)
		const a, b = 0x10000, 0x800000 + 8192
		var acceptedA, arrivalA, acceptedB uint64
		for i := uint64(0); i < 16; i++ { // two lines of each stream
			acceptedA, arrivalA = c.Store(a+8*i, 8, 1+i)
			acceptedB, _ = c.Store(b+8*i, 8, 1+i)
		}
		if acceptedB != 16 {
			t.Fatalf("enabled=%v: the stream outside the run is accepted at issue, got %d", enabled, acceptedB)
		}
		if want := map[bool]uint64{false: 16, true: arrivalA}[enabled]; acceptedA != want || arrivalA < 300 {
			t.Fatalf("enabled=%v: run store accepted at %d, want %d (line arrives at %d)", enabled, acceptedA, want, arrivalA)
		}
		// The run is established by the store that completes its first line:
		// that store and the eight of the second line wait.
		if want := map[bool]uint64{false: 0, true: 9}[enabled]; c.stats.StoreRunWaits != want {
			t.Fatalf("enabled=%v: %d waits counted, want %d", enabled, c.stats.StoreRunWaits, want)
		}
		// A resident line is written at once either way.
		c.Advance(arrivalA + 1000)
		if accepted, _ := c.Store(a+64, 8, arrivalA+1000); accepted != arrivalA+1000+uint64(cfg.L1Latency) {
			t.Fatalf("enabled=%v: resident store accepted at %d", enabled, accepted)
		}
	}
}
