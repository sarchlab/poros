package ooo

import (
	"testing"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
)

// storeTiming gives the cache one store per instruction however the backend
// records the instruction's bytes. A run of contiguous stores with one store
// elsewhere per trip stays an interleaved run, whose stores wait for their
// lines, whether that store is recorded as one aligned access, as one access
// across a cache line (the emulator backend), as two pieces cut at a word
// boundary (the Linux process backend) or as the two registers of a pair.
// Classified access by access, each of the last four would be two stores
// elsewhere and end the run. The run's own stores are classified whole too:
// both registers of a pair wait from the pair that completes the first line.
func TestStoreTimingClassifiesAnInstructionsStoreOnce(t *testing.T) {
	const a, other = 0x10000, 0x800000 + 8192 // other: another line of the page
	write := func(addr uint64, size int) emu.MemAccess {
		return emu.MemAccess{Addr: addr, Size: size, IsWrite: true}
	}
	word := func(trip uint64) []emu.MemAccess { return []emu.MemAccess{write(a+8*trip, 8)} }
	pair := func(trip uint64) []emu.MemAccess {
		return []emu.MemAccess{write(a+16*trip, 8), write(a+16*trip+8, 8)}
	}
	for _, tc := range []struct {
		name      string
		run       func(trip uint64) []emu.MemAccess
		elsewhere []emu.MemAccess
		trips     uint64 // two lines of the run
		waits     uint64 // accesses of the run that wait
	}{
		{"aligned", word, []emu.MemAccess{write(other, 8)}, 16, 9},
		{"across a line, one access", word, []emu.MemAccess{write(other+60, 8)}, 16, 9},
		{"across a line, word pieces", word, []emu.MemAccess{write(other+60, 4), write(other+64, 4)}, 16, 9},
		{"within a line, word pieces", word, []emu.MemAccess{write(other+20, 4), write(other+24, 4)}, 16, 9},
		{"a pair elsewhere", word, []emu.MemAccess{write(other, 8), write(other+8, 8)}, 16, 9},
		{"a run of pairs", pair, []emu.MemAccess{write(other, 8)}, 8, 10},
	} {
		cfg := mem.DefaultCacheConfig()
		cfg.InterleavedStoreRunsWait = true
		cache := mem.NewCache(cfg)
		p := reservationPipeline(t, []uint32{strX1X2}, cache)
		memory := p.comp.Resources.Emu.Mem
		var accepted, drained uint64
		for trip := uint64(0); trip < tc.trips; trip++ {
			memory.Accesses = tc.run(trip)
			accepted, drained = p.storeTiming(1 + trip)
			memory.Accesses = tc.elsewhere
			if released, _ := p.storeTiming(1 + trip); released >= drained {
				t.Errorf("%s: the store elsewhere waits until %d", tc.name, released)
			}
		}
		if got := cache.Stats().StoreRunWaits; accepted != drained || got != tc.waits {
			t.Errorf("%s: last run store accepted at %d, its line arrives at %d, %d waits counted, want %d",
				tc.name, accepted, drained, got, tc.waits)
		}
	}
}
