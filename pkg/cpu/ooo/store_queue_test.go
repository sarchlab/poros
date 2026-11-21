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

// Both register files use the same data slots. A store with ready data still
// needs a free slot, even when it writes the same bytes as older stores.
func TestStoreDataQueueAdmission(t *testing.T) {
	words := []uint32{strD1X2, strX1X2, strX3X2Off8, addX9}
	for _, capacity := range []int{0, 2} {
		p := specPipeline(t, words, nil, func(s *Spec) { s.StoreDataQueue = capacity })
		p.comp.Resources.Emu.Regs.WriteX(2, 0x2000)
		p.comp.State.FpReady[1] = 100
		p.comp.State.GprReady[1] = 110
		p.fetchAndProcess()
		if capacity == 0 {
			if p.comp.State.Err != "" || p.comp.State.RobCount != 4 || len(p.comp.State.StoreDataQueue) != 0 {
				t.Fatalf("disabled: err=%q dispatched=%d data slots=%v", p.comp.State.Err, p.comp.State.RobCount, p.comp.State.StoreDataQueue)
			}
			continue
		}
		if p.comp.State.Err != "" || p.comp.State.RobCount != 2 || len(p.comp.State.StoreDataQueue) != 2 {
			t.Fatalf("full: err=%q dispatched=%d data slots=%v", p.comp.State.Err, p.comp.State.RobCount, p.comp.State.StoreDataQueue)
		}
		p.comp.State.Cycles = 99
		p.fetchAndProcess()
		if p.comp.State.RobCount != 2 {
			t.Fatal("ready store entered a full data queue")
		}
		p.comp.State.Cycles = 100
		p.fetchAndProcess()
		if p.comp.State.Err != "" || p.comp.State.RobCount != 4 {
			t.Fatalf("released: err=%q dispatched=%d", p.comp.State.Err, p.comp.State.RobCount)
		}
	}
}

func TestStoreDataSlotReleasedBeforeLateAddress(t *testing.T) {
	p := specPipeline(t, []uint32{strD1X2, strX1X2}, nil, func(s *Spec) { s.StoreDataQueue = 1 })
	p.comp.Resources.Emu.Regs.WriteX(2, 0x2000)
	p.comp.State.GprReady[2] = 100
	p.fetchAndProcess()
	if p.comp.State.RobCount != 1 || p.comp.State.Rob[0] < 100 {
		t.Fatalf("late address did not delay execution: dispatched=%d done=%d", p.comp.State.RobCount, p.comp.State.Rob[0])
	}
	p.comp.State.Cycles++
	p.fetchAndProcess()
	if p.comp.State.Err != "" || p.comp.State.RobCount != 2 || p.comp.State.StoreDrainUntil[0] < 100 {
		t.Fatalf("address retained a data slot: err=%q dispatched=%d", p.comp.State.Err, p.comp.State.RobCount)
	}
}

// Paired stores have a different measured threshold. Other unmeasured forms
// also continue to use the ordinary store slots.
func TestStoreDataQueueForms(t *testing.T) {
	p := specPipeline(t, []uint32{addX9}, nil, nil)
	for _, tc := range []struct {
		word    uint32
		tracked bool
	}{
		{strX1X2, true}, {0xb9000041, true}, {strD1X2, true}, {0xbd000041, true},
		{0xf900005f, true}, {0xad000441, false}, {0xf8216841, false}, {0x3d800041, false},
	} {
		if _, tracked := p.scalarStoreDataReady(decodeWord(t, tc.word)); tracked != tc.tracked {
			t.Errorf("encoding %#x tracked %v, want %v", tc.word, tracked, tc.tracked)
		}
	}
}

func TestRegisterOffsetStoresShareDataQueue(t *testing.T) {
	const strD1Indexed = 0xfc237841 // STR D1, [X2, X3, LSL #3]
	for _, enabled := range []bool{false, true} {
		p := specPipeline(t, []uint32{strD1Indexed, strX1X2, addX9}, nil, func(s *Spec) {
			s.StoreDataQueue, s.RegisterOffsetStoreData = 1, enabled
		})
		p.comp.Resources.Emu.Regs.WriteX(2, 0x2000)
		p.comp.State.FpReady[1] = 50
		p.comp.State.GprReady[3] = 100
		p.fetchAndProcess()
		if !enabled {
			if p.comp.State.RobCount != 3 {
				t.Fatalf("disabled: admitted %d", p.comp.State.RobCount)
			}
			continue
		}
		if p.comp.State.RobCount != 1 || p.comp.State.StoreDataQueue[0] != 50 {
			t.Fatalf("indexed store did not hold shared slot until its data was ready")
		}
		p.comp.State.Cycles = 49
		p.fetchAndProcess()
		if p.comp.State.RobCount != 1 {
			t.Fatal("immediate store entered occupied data slot")
		}
		p.comp.State.Cycles++
		p.fetchAndProcess()
		if p.comp.State.RobCount != 3 || p.comp.State.StoreDrainUntil[0] < 100 {
			t.Fatal("data slot release followed address execution")
		}
	}
}

func TestRegisterOffsetStoreDataShapes(t *testing.T) {
	p := specPipeline(t, []uint32{nop}, nil, func(s *Spec) { s.StoreDataQueue, s.RegisterOffsetStoreData = 2, true })
	for _, tc := range []struct {
		word    uint32
		tracked bool
	}{
		{0xf8236841, true}, {0xfc237841, true}, {0xfc23d841, true}, {0xb8236841, false},
		{0xbc236841, false}, {0xfc008441, false}, {0x3ca36841, false},
	} {
		if _, ok := p.scalarStoreDataReady(decodeWord(t, tc.word)); ok != tc.tracked {
			t.Errorf("encoding %#x tracked %v", tc.word, ok)
		}
	}
}
