package ooo

import "testing"

const (
	fdivD0D1D2  = 0x1e621820 // FDIV D0, D1, D2
	strX3X2Off8 = 0xf9000443 // STR X3, [X2, #8]
)

// Stores leave an in-order store queue in program order: a store whose data
// is late holds the slot of a younger store that executed long before.
// Without it each slot is released when its own store has executed.
func TestStoreQueueInOrderReleasesSlotsInProgramOrder(t *testing.T) {
	for _, inOrder := range []bool{false, true} {
		p := specPipeline(t, []uint32{strX1X2, strX3X2Off8}, nil, func(s *Spec) { s.StoreQueueInOrder = inOrder })
		p.comp.Resources.Emu.Regs.WriteX(2, 0x2000)
		p.comp.State.GprReady[1] = 100
		p.fetchAndProcess()
		slots := p.comp.State.StoreDrainUntil
		if p.comp.State.Err != "" || p.comp.State.RobCount != 2 {
			t.Fatalf("setup: err=%q dispatched=%d", p.comp.State.Err, p.comp.State.RobCount)
		}
		if slots[0] != 103 || (inOrder && slots[1] != 103) || (!inOrder && slots[1] >= 100) {
			t.Fatalf("inOrder=%v: slots released at %v", inOrder, slots[:2])
		}
	}
}

// The order is among stores only: a store does not wait for an older
// instruction that is not a store. M1 keeps more than 300 stores in flight
// behind an unfinished divide chain (evaluation/model-profile.md, "Store
// runs"), far more than the stores this queue lets wait for the cache.
func TestStoreQueueInOrderDoesNotWaitForOlderNonStores(t *testing.T) {
	p := specPipeline(t, []uint32{fdivD0D1D2, strX1X2, strX3X2Off8}, nil, func(s *Spec) { s.StoreQueueInOrder = true })
	p.comp.Resources.Emu.Regs.WriteX(2, 0x2000)
	p.fetchAndProcess()
	rob, slots := p.comp.State.Rob, p.comp.State.StoreDrainUntil
	if p.comp.State.Err != "" || p.comp.State.RobCount != 3 || rob[0] <= rob[2] {
		t.Fatalf("setup: err=%q dispatched=%d completions=%v", p.comp.State.Err, p.comp.State.RobCount, rob[:3])
	}
	if slots[0] != rob[1] || slots[1] != rob[2] {
		t.Fatalf("stores behind the divide release at their own completions %v, got %v (divide completes at %d)", rob[1:3], slots[:2], rob[0])
	}
}
