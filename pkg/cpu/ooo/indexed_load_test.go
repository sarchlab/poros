package ooo

import (
	"slices"
	"testing"

	"github.com/sarchlab/poros/pkg/mem"
)

const postLoadX0FromX1 = 0xf8408420 // LDR X0,[X1],#8

// Two independent base updates in a single fetch group share one physical
// pipe. Only the cold translations need a later issue for the cache access.
func TestIndexedLoadsShareAddressPipe(t *testing.T) {
	for _, translation := range []string{"hot", "cold"} {
		t.Run(translation, func(t *testing.T) {
			cfg := mem.DefaultCacheConfig()
			cache := mem.NewCache(cfg)
			const start uint64 = 1000
			for _, addr := range []uint64{0x4000, 0x8000} {
				cache.Access(addr, 8, false, 0) // Both L1 fills arrive before start.
				if translation == "hot" {
					cache.Translate(addr, 8, 0)
				}
			}
			words := []uint32{postLoadX0FromX1, 0xf8408462, // LDR X2,[X3],#8
				0x91000424, 0x91000465} // ADD X4,X1,#1; ADD X5,X3,#1
			p := specPipeline(t, words, cache, func(s *Spec) { s.NumINT = 2 })
			p.comp.State.Cycles = start
			p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
			p.comp.Resources.Emu.Regs.WriteX(3, 0x8000)
			p.fetchAndProcess()
			s := &p.comp.State
			if s.Err != "" || s.RobCount != len(words) {
				t.Fatalf("dispatched %d instructions: %s", s.RobCount, s.Err)
			}
			if s.GprReady[1] != start+1 || s.GprReady[3] != start+2 ||
				s.GprReady[4] != start+2 || s.GprReady[5] != start+3 {
				t.Fatalf("bases ready at %d,%d; consumers at %d,%d", s.GprReady[1], s.GprReady[3], s.GprReady[4], s.GprReady[5])
			}
			access := start
			want := []UnitReservation{{start, start + 1}, {start + 1, start + 2}}
			if translation == "cold" {
				access += uint64(cfg.TLBWalkLatency)
				want = append(want, UnitReservation{access, access + 1}, UnitReservation{access + 1, access + 2})
			}
			if !slices.Equal(s.LdstUnits[0], want) || s.Rob[0] != access+uint64(cfg.L1Latency) || s.Rob[1] != s.Rob[0]+1 {
				t.Fatalf("reservations %v, want %v; loads complete at %v", s.LdstUnits[0], want, s.Rob[:2])
			}
		})
	}
}

// The indexed load waits for an MSHR and then for a previously booked pipe
// slot. A younger store and plain load fill the gap after its address stage.
func TestIndexedLoadBaseWhileMissEntryBusy(t *testing.T) {
	for _, translation := range []string{"hot", "cold"} {
		t.Run(translation, func(t *testing.T) {
			cfg := mem.DefaultCacheConfig()
			cache := mem.NewCache(cfg)
			if translation == "hot" {
				cache.Translate(0x4000, 8, 0)
			}
			for _, addr := range []uint64{0xc000, 0x10000} {
				cache.Translate(addr, 8, 0)
				cache.Access(addr, 8, false, 0)
			}
			words := []uint32{postLoadX0FromX1, 0x91000424, // ADD X4,X1,#1
				0xf90000e6, loadFrom(9, 8)} // STR X6,[X7]; LDR X8,[X9]
			p := reservationPipeline(t, words, cache)
			s := &p.comp.State
			s.Cycles, s.MshrBusyUntil[0] = 1000, 5000
			s.LdstUnits[0] = []UnitReservation{{5000, 5002}}
			p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
			p.comp.Resources.Emu.Regs.WriteX(7, 0xc000)
			p.comp.Resources.Emu.Regs.WriteX(9, 0x10000)
			p.fetchAndProcess()
			if s.Err != "" || s.RobCount != len(words) {
				t.Fatalf("dispatched %d instructions: %s", s.RobCount, s.Err)
			}
			want := []UnitReservation{{1000, 1001}, {1001, 1002}, {1002, 1003}, {5000, 5002}, {5002, 5003}}
			if !slices.Equal(s.LdstUnits[0], want) {
				t.Fatalf("shared pipe %v, want %v", s.LdstUnits[0], want)
			}
			if s.GprReady[1] != 1001 || s.GprReady[4] != 1002 || s.Rob[0] != 5002+uint64(cfg.DRAMLatency) || s.MshrBusyUntil[0] != s.Rob[0] {
				t.Fatalf("base %d, consumer %d, load %d, MSHR %d", s.GprReady[1], s.GprReady[4], s.Rob[0], s.MshrBusyUntil[0])
			}
			if s.Rob[2] != 1001+3 || s.Rob[3] != 1002+uint64(cfg.L1Latency) {
				t.Fatalf("younger store/load completed at %v", s.Rob[2:4])
			}
		})
	}
}

// Conventional store ordering delays the pre-indexed load until cycle 10,
// when the pipe is occupied. Its address becomes available at the next slot.
func TestIndexedLoadRechecksPortAfterOverlappingStore(t *testing.T) {
	for _, size := range []int{4, 8} {
		cfg := mem.DefaultCacheConfig()
		cache := mem.NewCache(cfg)
		cache.Lookup(0x4008, 8, false)                           // resident data, cold translation
		p := reservationPipeline(t, []uint32{0xf8408c20}, cache) // LDR X0,[X1,#8]!
		p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
		s := &p.comp.State
		s.LdstUnits[0] = []UnitReservation{{10, 13}}
		s.StoreBuffer = []storeBufferEntry{{Addr: 0x4008, Size: size, CompleteCycle: 10, DrainCycle: 50}}
		p.fetchAndProcess()
		if s.Err != "" || s.RobCount != 1 || s.GprReady[1] != 14 {
			t.Fatalf("%d-byte overlap: base ready %d, dispatched %d: %s", size, s.GprReady[1], s.RobCount, s.Err)
		}
		want := []UnitReservation{{10, 13}, {13, 14}}
		complete := uint64(13 + conventionalForwardLatency)
		var walks uint64
		if size == 4 { // Partial coverage needs the cache; full coverage forwards.
			access := 13 + uint64(cfg.TLBWalkLatency)
			want = append(want, UnitReservation{access, access + 1})
			complete, walks = access+uint64(cfg.L1Latency), 1
		}
		if !slices.Equal(s.LdstUnits[0], want) || s.Rob[0] != complete || cache.Stats().TLBWalks != walks || s.MshrBusyUntil[0] != 0 {
			t.Fatalf("%d-byte overlap: reservations %v, completion %d, walks %d, MSHR %d", size, s.LdstUnits[0], s.Rob[0], cache.Stats().TLBWalks, s.MshrBusyUntil[0])
		}
	}
}
