package ooo

import (
	"testing"

	"github.com/sarchlab/poros/pkg/mem"
)

// A load to the other half of a 128 B line whose DRAM fill is pending must
// complete when that fill arrives, not after a nominal L1 hit latency.
func TestLoadToPendingLineWaitsForFillArrival(t *testing.T) {
	// LDR X0,[X1]; LDR X2,[X3]
	cfg := mem.DefaultCacheConfig()
	cfg.DisableAdjacentL1Prefetch = false // Legacy adjacent-fill merging.
	p := reservationPipeline(t, []uint32{0xf9400020, 0xf9400062}, mem.NewCache(cfg))
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.Resources.Emu.Regs.WriteX(3, 0x4040)
	p.comp.Resources.Emu.Mem.WriteU64(0x4000, 1)
	p.comp.Resources.Emu.Mem.WriteU64(0x4040, 2)
	p.fetchAndProcess()
	if p.comp.State.Err != "" {
		t.Fatal(p.comp.State.Err)
	}
	first, second := p.comp.State.Rob[0], p.comp.State.Rob[1]
	if first != 336 {
		t.Fatalf("demand miss completes at %d, want 336", first)
	}
	// The adjacent half shares the DRAM fill and takes the next 2-cycle
	// L2-to-L1 transfer slot: 336 + 2, never the nominal L1 hit at 7.
	if second != 338 {
		t.Fatalf("adjacent load completed at %d, want 338 (after the fill arrived)", second)
	}
	if p.comp.State.MshrBusyUntil[0] != 336 {
		t.Fatalf("MSHR held until %d", p.comp.State.MshrBusyUntil[0])
	}
}

func TestSecondHalfDemandWaitsForMSHRAndL2Fill(t *testing.T) {
	p := reservationPipeline(t, []uint32{0xf9400020, 0xf9400062}, mem.NewCache(mem.DefaultCacheConfig()))
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.Resources.Emu.Regs.WriteX(3, 0x4040)
	p.fetchAndProcess()
	// No speculative adjacent L1 fill: the second load is an L1 miss, waits
	// for the one MSHR at 336, then pays L2 latency for its demanded half.
	if p.comp.State.Err != "" || p.comp.State.Rob[0] != 336 || p.comp.State.Rob[1] != 350 || p.comp.State.MshrBusyUntil[0] != 350 {
		t.Fatalf("second-half demand bypassed resources: completions=%v MSHR=%v error=%s", p.comp.State.Rob[:2], p.comp.State.MshrBusyUntil, p.comp.State.Err)
	}
}

// Two independent misses in the same cycle are serialized by the finite fill
// transport: the second completes one line transfer later.
func TestIndependentMissesShareFillTransport(t *testing.T) {
	p := reservationPipeline(t, []uint32{0xf9400020, 0xf9400062}, mem.NewCache(mem.DefaultCacheConfig()))
	p.comp.Spec.NumLDST = 2
	p.comp.State.LdstUnits = make([][]UnitReservation, 2)
	p.comp.State.MshrBusyUntil = make([]uint64, 2)
	p.comp.Resources.Emu.Regs.WriteX(1, 0x4000)
	p.comp.Resources.Emu.Regs.WriteX(3, 0x9000)
	p.fetchAndProcess()
	if p.comp.State.Err != "" {
		t.Fatal(p.comp.State.Err)
	}
	if p.comp.State.Rob[0] != 336 || p.comp.State.Rob[1] != 344 {
		t.Fatalf("completions %d %d, want 336 and 344 (one 8-cycle line transfer later)", p.comp.State.Rob[0], p.comp.State.Rob[1])
	}
}
