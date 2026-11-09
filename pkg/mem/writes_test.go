package mem

import (
	"fmt"
	"reflect"
	"testing"
)

func TestPendingStoresCombineBytesButRetainArrival(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.WriteBufferEntries = 2
	c := NewCache(cfg)
	accepted, arrival := c.Store(0x1000, 8, 1)
	if accepted != 1 || arrival != 336 {
		t.Fatalf("first: accepted %d arrival %d", accepted, arrival)
	}
	for offset := uint64(8); offset < 64; offset += 8 {
		a, d := c.Store(0x1000+offset, 8, 2)
		if a != 2 || d != arrival {
			t.Fatalf("merge accepted %d arrival %d", a, d)
		}
	}
	if c.stats.WriteAllocations != 1 || c.stats.WriteMerges != 7 {
		t.Fatal(c.stats)
	}

	if _, ready := c.Access(0x1000, 8, false, 3); ready < arrival {
		t.Fatal("combining exposed unarrived line")
	}
	if c.DrainUntil() != arrival {
		t.Fatal("lost write allocation drain")
	}
	// Ordinary resident-hit stores still pay the L1 access completion.
	c.Advance(arrival)
	a, d := c.Store(0x1000, 8, arrival)
	if a != arrival+uint64(cfg.L1Latency) || d != a {
		t.Fatal("resident hit timing changed", a, d)
	}
}

func TestWriteCapacityAndFutureIssueReservations(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.WriteBufferEntries = 1
	c := NewCache(cfg)
	a, first := c.Store(0x1000, 8, 1000)
	if a != 1000 {
		t.Fatal(a)
	}
	// A younger request issuing earlier cannot discard the older future slot.
	a, second := c.Store(0x8000, 8, 1)
	if a < first || second < a+uint64(cfg.DRAMLatency) {
		t.Fatalf("future reservation lost: %d %d %d", first, a, second)
	}
	a, third := c.Store(0x1000, 8, 2)
	// The first line still exists but cannot merge into the second generation.
	// It is pending at issue2, so bounded admission retains the booked queue.
	if a < second || third < a {
		t.Fatalf("queue occupancy forgotten: %d %d %d", second, a, third)
	}
	if c.DrainUntil() < third {
		t.Fatal("terminal drain missing")
	}
}

func TestStoreCrossLineAndPartialByteOwnership(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.WriteBufferEntries = 2
	c := NewCache(cfg)
	a, d := c.Store(0x103e, 4, 1)
	if a != 1 || d != 338 {
		t.Fatal(a, d)
	}
	if c.stats.WriteAllocations != 2 {
		t.Fatal("cross-line store did not allocate both lines")
	}

	c.Store(0x103f, 1, 2)
	if c.stats.WriteMerges != 1 {
		t.Fatal("partial-byte store not merged")
	}
}

func TestDirtyEvictionWaitsForFutureFillAndStoreData(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.L1Size = 128
	cfg.L1Ways = 1
	cfg.L2Size = 256
	cfg.L2Ways = 1
	cfg.PrefetchDistance = 0
	c := NewCache(cfg)
	_, fill := c.Store(0x1000, 8, 1000)
	c.Access(0x2000, 8, false, 1) // dispatch-order eviction before the old future fill
	if c.stats.L1Writebacks == 0 || c.DrainUntil() < fill+2 {
		t.Fatalf("dirty future fill discarded: fill%d drain%d stats%+v", fill, c.DrainUntil(), c.stats)
	}
	// Force the newly dirtied L2 line out after its L1 writeback was reserved.
	c.Access(0x3000, 8, false, 2)
	if c.stats.L2Writebacks == 0 || c.DrainUntil() < fill+2+8 {
		t.Fatalf("dirty L2 traffic lost: drain%d stats%+v", c.DrainUntil(), c.stats)
	}
	for _, slot := range c.dram.busy {
		if slot.Start < 1000 && slot.End > 1000 {
			t.Fatal("future transfer began before producing data")
		}
	}
}

func TestEvictedPendingGenerationCannotAbsorbNewStore(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.L1Size = 128
	cfg.L1Ways = 1
	cfg.PrefetchDistance = 0
	cfg.WriteBufferEntries = 2
	c := NewCache(cfg)
	_, first := c.Store(0x1000, 8, 1000)
	c.Access(0x2000, 8, false, 1)
	c.Store(0x1000, 8, 2)
	if c.stats.WriteAllocations != 2 || c.stats.WriteMerges != 0 {
		t.Fatal("merged into evicted allocation generation", c.stats)
	}
	if c.DrainUntil() < first {
		t.Fatal("eviction lost original allocation")
	}
}

func TestSharedWritebackKeepsFutureTransportAndDataReadiness(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.L1Size = 128
	cfg.L1Ways = 1
	cfg.L2Size = 256
	cfg.L2Ways = 1
	cfg.PrefetchDistance = 0
	shared := NewSharedL2(cfg)
	a, b := NewPrivateL1WithSharedL2(cfg, shared), NewPrivateL1WithSharedL2(cfg, shared)
	a.Store(0x1000, 8, 1000)
	a.Access(0x2000, 8, false, 1)
	dirty := shared.level.peek(0x1000)
	if dirty == nil || !dirty.dirty || dirty.readyAt < 1337 {
		t.Fatal("shared L2 exposes dirty data before writeback")
	}
	b.Access(0x3000, 8, false, 2)
	if b.stats.L2Writebacks == 0 || b.DrainUntil() < 1345 {
		t.Fatal("shared dirty eviction not charged", b.stats, b.DrainUntil())
	}
	b.Advance(10)
	a.FinishTiming()
	// Removing one client must preserve the other client's future bookings.
	if len(shared.dram.busy) == 0 {
		t.Fatal("future shared transport erased")
	}
	if _, ready := b.Access(0x4000, 8, false, 10); ready < 345 {
		t.Fatal("shared fill arrived early")
	}
}

func TestAccessWriteContractAndLegacyDisabledStage(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.WriteBufferEntries = 0
	c := NewCache(cfg)
	accepted, ready := c.Store(0x1000, 8, 1)
	if accepted != 336 || ready != 336 || c.stats.WriteAllocations != 0 {
		t.Fatal("disabled stage changed allocation semantics")
	}
	c = NewCache(DefaultCacheConfig())
	_, ready = c.Access(0x1000, 8, true, 1)
	if ready != 336 || c.stats.WriteAllocations != 0 || !c.l1.peek(0x1000).dirty {
		t.Fatal("Access must retain data readiness and dirty tracking without admission")
	}
}

func TestDirtyWritebackAdmissionIsFiniteAndShared(t *testing.T) {
	slots := make([]uint64, 1)
	channel := &fillChannel{bytesPerCycle: 16}
	admitted, end := reserveWriteback(slots, channel, 1, 1000, 64)
	if admitted != 1 || end != 1004 {
		t.Fatal(admitted, end)
	}
	admitted, end = reserveWriteback(slots, channel, 2, 2, 64)
	if admitted != 1004 || end != 1008 {
		t.Fatal("unbounded dirty admission", admitted, end)
	}
	cfg := DefaultCacheConfig()
	cfg.WriteBufferEntries = 1
	shared := NewSharedL2(cfg)
	a, b := NewPrivateL1WithSharedL2(cfg, shared), NewPrivateL1WithSharedL2(cfg, shared)
	a.l2Writebacks[0] = 2000
	if b.l2Writebacks[0] != 2000 {
		t.Fatal("shared eviction capacity duplicated per core")
	}
	if a.l1Writebacks[0] != 0 || b.l1Writebacks[0] != 0 {
		t.Fatal("private eviction stages must remain independent")
	}
}

func TestResidentDirtyEvictionUsesStoreDataAvailability(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.L1Size = 128
	cfg.L1Ways = 1
	cfg.PrefetchDistance = 0
	c := NewCache(cfg)
	c.Lookup(0x1000, 8, false)
	c.Store(0x1000, 8, 100) // address and data are available at100
	c.Access(0x2000, 8, false, 101)
	if c.stats.L1Writebacks != 1 || c.writebackUntil < 103 {
		t.Fatal("dirty store data/transfer completed early", c.stats, c.writebackUntil)
	}
}

func TestWritebackAndInstallSelectSameVictim(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []cacheLine
		want  int
	}{
		{"first free", []cacheLine{{}, {valid: true, lastUsed: 1}, {}}, 0},
		{"later free", []cacheLine{{valid: true, lastUsed: 1}, {}, {valid: true, lastUsed: 2}}, 1},
		{"least recent", []cacheLine{{valid: true, lastUsed: 3}, {valid: true, lastUsed: 5}, {valid: true, lastUsed: 1}}, 2},
		{"equal ages", []cacheLine{{valid: true, lastUsed: 1}, {valid: true, lastUsed: 1}, {valid: true, lastUsed: 1}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			level := newCacheLevel(3*64, 3, 64)
			copy(level.sets[0], tc.lines)
			level.counter = 10
			before := append([]cacheLine(nil), level.sets[0]...)
			victim := level.victim(0x4000)
			if victim != &level.sets[0][tc.want] {
				t.Fatal("wrong free/LRU/tie selection")
			}
			installed := level.installAt(0x4000, 123, true)
			if installed != victim || installed.tag != level.tag(0x4000) || installed.readyAt != 123 || !installed.prefetched {
				t.Fatal("writeback/install selection diverged")
			}
			for i := range before {
				if i != tc.want && !reflect.DeepEqual(before[i], level.sets[0][i]) {
					t.Fatal("unselected line changed")
				}
			}
		})
	}
}

func TestNegativeWriteCapacityDiagnosticMatchesSharedCache(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.WriteBufferEntries = -1
	for _, create := range []func(){func() { NewCache(cfg) }, func() { NewSharedL2(cfg) }} {
		func() {
			defer func() {
				if got := fmt.Sprint(recover()); got != "mem: negative write-buffer capacity" {
					t.Fatalf("unexpected diagnostic %q", got)
				}
			}()
			create()
		}()
	}
}

func TestStoreAdoptsLoadCreatedPendingLine(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.WriteBufferEntries = 1
	cache := NewCache(cfg)
	_, arrival := cache.Access(0x1000, 8, false, 1)
	generation := cache.l1.peek(0x1000).generation
	accepted, drained := cache.Store(0x1000, 8, 2)
	if accepted != 2 || drained != arrival || cache.stats.WriteAllocations != 1 || cache.stats.WriteMerges != 0 {
		t.Fatalf("pending load was not adopted: accepted=%d drained=%d arrival=%d stats=%+v", accepted, drained, arrival, cache.stats)
	}
	if len(cache.writes) != 1 || cache.writes[0].Generation != generation || cache.writes[0].Line != 0x1000/uint64(cfg.L1LineSize) {
		t.Fatalf("bounded slot does not own load generation: %+v", cache.writes)
	}
	accepted, drained = cache.Store(0x1008, 8, 3)
	if accepted != 3 || drained != arrival || cache.stats.WriteAllocations != 1 || cache.stats.WriteMerges != 1 || cache.writes[0].Generation != generation {
		t.Fatalf("subsequent store did not merge: accepted=%d drained=%d stats=%+v", accepted, drained, cache.stats)
	}
}
