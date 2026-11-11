package ooo

import (
	"testing"

	"github.com/sarchlab/poros/pkg/mem"
)

// A load whose line was pushed out of L2 completes after the memory-side
// latency, not the DRAM latency, and holds its MSHR until then.
func TestLoadFromMemorySideCacheUsesItsLatency(t *testing.T) {
	cfg := fillConfig()
	cfg.L1Size, cfg.L1Ways = 64, 1
	cfg.L2Size, cfg.L2Ways = 2*128, 2
	cfg.SLCSize, cfg.SLCLatency = 4*128, 138
	cfg.PrefetchDistance = 0
	for _, tc := range []struct {
		name    string
		evicted bool
		want    uint64
	}{{"held behind L2", true, 1 + 138}, {"never seen", false, 1 + 335}} {
		cache := mem.NewCache(cfg)
		if tc.evicted {
			// Three lines through a two-line L2 leave the first one behind it.
			for _, addr := range []uint64{0x4000, 0x5000, 0x6000} {
				cache.Access(addr, 8, false, 0)
			}
			if cache.Probe(0x4000) != mem.HitSLC {
				t.Fatal("setup did not leave the line in the memory-side cache")
			}
		}
		p := reservationPipeline(t, []uint32{loadX0FromX1}, cache)
		p.comp.State.Cycles = 2000 // after the setup fills have arrived
		p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
		p.fetchAndProcess()
		want := 2000 - 1 + tc.want
		if p.comp.State.Err != "" || p.comp.State.Rob[0] != want || p.comp.State.MshrBusyUntil[0] != want {
			t.Fatalf("%s: completion %d MSHR %d, want %d (%s)", tc.name, p.comp.State.Rob[0], p.comp.State.MshrBusyUntil[0], want, p.comp.State.Err)
		}
	}
}
