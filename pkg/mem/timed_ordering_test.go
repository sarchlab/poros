package mem

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestFutureRequestDoesNotRetireTransportHistory(t *testing.T) {
	c := NewCache(DefaultCacheConfig())
	c.Advance(0)
	_, first := c.Access(0x100000, 8, false, 0)
	c.Access(0x800000, 8, false, 1000)
	_, third := c.Access(0x400000, 8, false, 0)
	if first != 335 || third != 343 {
		t.Fatalf("first=%d third=%d, want 335,343", first, third)
	}
	c.Advance(500)
	for _, channel := range []*fillChannel{c.channel, c.dram} {
		for _, slot := range channel.busy {
			if slot.End <= 500 {
				t.Fatal("expired interval retained", slot)
			}
		}
	}
}

func TestProbeDoesNotTouchCacheState(t *testing.T) {
	c := NewCache(DefaultCacheConfig())
	c.Access(0x1000, 8, false, 100)
	stats, l1, l2, tick := c.stats, c.l1.counter, c.l2.counter, c.useTick
	channel := append([]fillInterval(nil), c.channel.busy...)
	dram := append([]fillInterval(nil), c.dram.busy...)
	streams := append([]stream(nil), c.streams...)
	for _, addr := range []uint64{0x1000, 0x1040, 0x5000} {
		c.Probe(addr)
	}
	if c.stats != stats || c.l1.counter != l1 || c.l2.counter != l2 || c.useTick != tick || !reflect.DeepEqual(channel, c.channel.busy) || !reflect.DeepEqual(dram, c.dram.busy) || !reflect.DeepEqual(streams, c.streams) {
		t.Fatal("probe mutated cache")
	}
}

func TestTransportCalendarMatchesDiscreteOccupancyOracle(t *testing.T) {
	ch := &fillChannel{bytesPerCycle: 16}
	occupied := map[uint64]bool{}
	rng := rand.New(rand.NewSource(71))
	var watermark uint64
	for n := 0; n < 4000; n++ {
		if n%7 == 0 {
			watermark += 3
			ch.prune(watermark)
		}
		earliest := watermark + uint64(rng.Intn(700))
		duration := uint64(1 + rng.Intn(16))
		start := earliest
		for {
			free := true
			for t := start; t < start+duration; t++ {
				if occupied[t] {
					free = false
					break
				}
			}
			if free {
				break
			}
			start++
		}
		got := ch.deliver(earliest, int(duration*16))
		if got != start+duration {
			t.Fatalf("request %d: finish %d want %d", n, got, start+duration)
		}
		for t := start; t < got; t++ {
			occupied[t] = true
		}
		for i, slot := range ch.busy {
			if slot.Start >= slot.End || slot.End <= watermark {
				t.Fatal("invalid interval", slot)
			}
			if i > 0 && ch.busy[i-1].End >= slot.Start {
				t.Fatal("overlap or uncoalesced adjacency")
			}
		}
	}
}

func TestPackedStoreTransportHasConstantIntervalCount(t *testing.T) {
	ch := &fillChannel{bytesPerCycle: 16}
	for i := uint64(0); i < 100000; i++ {
		if got := ch.deliver(327, 128); got != 335+i*8 {
			t.Fatal("wrong finish", got)
		}
		if len(ch.busy) != 1 {
			t.Fatalf("packed stream retained %d intervals", len(ch.busy))
		}
	}
	// A genuinely earlier gap stays available even behind a large packed run.
	if got := ch.deliver(0, 128); got != 8 {
		t.Fatalf("earlier gap lost: %d", got)
	}
}

func TestSharedDRAMBandwidthAndSlowClientWatermark(t *testing.T) {
	cfg := DefaultCacheConfig()
	shared := NewSharedL2(cfg)
	a, b := NewPrivateL1WithSharedL2(cfg, shared), NewPrivateL1WithSharedL2(cfg, shared)
	if a.dram != b.dram || a.channel == b.channel {
		t.Fatal("wrong shared transport ownership")
	}
	_, first := a.Access(0x100000, 8, false, 0)
	a.Advance(1000) // The other core is still at zero.
	_, second := b.Access(0x400000, 8, false, 0)
	if first != 335 || second != 343 {
		t.Fatalf("aggregate rate violated: %d,%d", first, second)
	}
	_, future := b.Access(0x800000, 8, false, 2000)
	b.FinishTiming()
	a.Advance(1000)
	if len(shared.clients) != 1 {
		t.Fatal("finished core still constrains watermark")
	}
	if len(shared.dram.busy) != 1 || shared.dram.busy[0].End != future {
		t.Fatalf("stopped core's future transfer lost: %+v", shared.dram.busy)
	}
	a.Advance(future)
	if len(shared.dram.busy) != 0 {
		t.Fatal("finished core prevented retirement")
	}
}

func TestSharedCacheAttachmentClosesAtFirstTimingOperation(t *testing.T) {
	for _, operation := range []string{"advance", "access"} {
		t.Run(operation, func(t *testing.T) {
			cfg := DefaultCacheConfig()
			shared := NewSharedL2(cfg)
			c := NewPrivateL1WithSharedL2(cfg, shared)
			if operation == "advance" {
				c.Advance(0)
			} else {
				c.Access(0x1000, 8, false, 0)
			}
			defer func() {
				if recover() == nil {
					t.Fatal("late client was accepted")
				}
			}()
			NewPrivateL1WithSharedL2(cfg, shared)
		})
	}
}

func TestLowLatencyTransfersNeverUnderflowOrStartBeforeIssue(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.DRAMLatency = 1
	cfg.L2Latency = 1
	cfg.FillBytesPerCycle = 1
	cfg.DRAMBytesPerCycle = 1
	cfg.PrefetchDistance = 0
	for _, issue := range []uint64{0, 1000} {
		c := NewCache(cfg)
		_, ready := c.Access(0x1000, 8, false, issue)
		if ready != issue+128 {
			t.Fatalf("issue=%d ready=%d", issue, ready)
		}
		for _, ch := range []*fillChannel{c.channel, c.dram} {
			for _, slot := range ch.busy {
				if slot.Start < issue {
					t.Fatalf("transfer before request: %+v", slot)
				}
			}
		}
	}
}

// Both APIs retain dispatch-order replacement; timed arrivals do not turn the
// residency model into a chronological event model. This also existed in base.
func TestDispatchOrderResidencyRemainsExplicit(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.L1Size = 128
	cfg.L1Ways = 2
	cfg.L2Size = 128
	cfg.L2Ways = 1
	cfg.PrefetchDistance = 0
	legacy, timed := NewCache(cfg), NewCache(cfg)
	for i, addr := range []uint64{0x1000, 0x1000, 0x2000, 0x1000} {
		want := legacy.Lookup(addr, 8, false)
		now := []uint64{0, 400, 1000, 401}[i]
		got, ready := timed.Access(addr, 8, false, now)
		if got != want {
			t.Fatalf("dispatch %d: level=%d legacy=%d", i, got, want)
		}
		if i == 3 && (got != HitDRAM || ready < 736) {
			t.Fatalf("replacement semantics changed: %d %d", got, ready)
		}
	}
}
