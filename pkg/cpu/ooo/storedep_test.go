package ooo

import "testing"

const (
	strX1X2   = 0xf9000041 // STR X1, [X2]
	ldrX3X4   = 0xf9400083 // LDR X3, [X4]
	addX2X2X5 = 0x8b050042 // ADD X2, X2, X5
	branchM12 = 0x17fffffd // B .-12
)

// storeLoadLoop runs two trips of STR X1,[X2]; LDR X3,[X4]; ADD X2,X2,X5; B.
// The first trip's load reads the bytes its store wrote. Before the second
// trip the store's data is made late (ready at cycle 100) and X2 advances by
// stride, so the second store writes other bytes when stride is nonzero. It
// returns the completion cycle of the second trip's load.
func storeLoadLoop(t *testing.T, predict bool, stride uint64) uint64 {
	t.Helper()
	p := specPipeline(t, []uint32{strX1X2, ldrX3X4, addX2X2X5, branchM12}, nil, func(s *Spec) {
		s.NumLDST, s.NumINT = 2, 2
		s.StoreDependencePrediction = predict
	})
	regs := p.comp.Resources.Emu.Regs
	regs.WriteX(2, 0x2000)
	regs.WriteX(4, 0x2000)
	regs.WriteX(5, stride)
	p.fetchAndProcess() // first trip; its branch is not yet predicted
	if p.comp.State.Err != "" || p.comp.State.RobCount != 4 {
		t.Fatalf("first trip: err=%q dispatched=%d", p.comp.State.Err, p.comp.State.RobCount)
	}
	p.comp.State.Cycles = p.comp.State.FrontendReady
	p.comp.State.GprReady[1] = 100
	p.fetchAndProcess()
	if p.comp.State.Err != "" || p.comp.State.RobCount != 8 {
		t.Fatalf("second trip: err=%q dispatched=%d", p.comp.State.Err, p.comp.State.RobCount)
	}
	return p.comp.State.Rob[5]
}

// A load predicted to depend on a static store waits for that store's
// youngest instance even when the instance writes other bytes.
func TestPredictedStoreDependenceDelaysLoadOfOtherBytes(t *testing.T) {
	without, with := storeLoadLoop(t, false, 8), storeLoadLoop(t, true, 8)
	if without >= 100 {
		t.Fatalf("without prediction the load must not wait for the unrelated store: ready at %d", without)
	}
	if want := uint64(100 + 3 + 5); with != want {
		t.Fatalf("predicted load ready at %d, want %d (store issue 100, completion 3, load latency 5)", with, want)
	}
}

// When the predicted instance does supply the load, forwarding alone times
// it: the prediction adds nothing to a true dependence.
func TestPredictedStoreDependenceKeepsTrueForwardingTiming(t *testing.T) {
	without, with := storeLoadLoop(t, false, 0), storeLoadLoop(t, true, 0)
	if with != without || without < 100 {
		t.Fatalf("true dependence: predicted %d, unpredicted %d", with, without)
	}
}

// A load that overlaps no in-flight store forgets its prediction, and a store
// is recorded as its static instruction's youngest instance.
func TestStoreDependenceTraining(t *testing.T) {
	p := specPipeline(t, []uint32{strX1X2, ldrX3X4, ldrX3X4}, nil, func(s *Spec) { s.StoreDependencePrediction = true })
	regs := p.comp.Resources.Emu.Regs
	regs.WriteX(2, 0x2000)
	regs.WriteX(4, 0x2000)
	p.fetchAndProcess()
	s := p.comp.State.StoreDependence
	if s.LoadStore[0x1004] != 0x1000 || s.LoadStore[0x1008] != 0x1000 {
		t.Fatalf("loads of the stored bytes must predict the store: %v", s.LoadStore)
	}
	if got := s.Youngest[0x1000]; got.Addr != 0x2000 || got.End != 0x2008 || got.CompleteCycle != p.comp.State.Rob[0] {
		t.Fatalf("youngest store instance %+v, store completes at %d", got, p.comp.State.Rob[0])
	}
	p.comp.State.StoreBuffer = nil
	p.trainStoreDependence(0x1004, nil)
	if _, ok := p.comp.State.StoreDependence.LoadStore[0x1004]; ok {
		t.Fatal("a load overlapping no in-flight store must forget its prediction")
	}
}

// A fetch group ends at a correctly predicted taken branch unless it may
// still cross one: a four-instruction loop then admits eight per cycle.
func TestFetchGroupContinuesAcrossTakenBranches(t *testing.T) {
	for _, tc := range []struct {
		taken, perCycle int
	}{{0, 4}, {1, 8}, {2, 8}} {
		p := specPipeline(t, []uint32{nop, nop, nop, 0x17fffffd}, nil, func(s *Spec) { // NOP x3; B .-12
			s.FetchWidth = 8
			s.FetchTakenBranches = tc.taken
		})
		p.fetchAndProcess() // train the branch target
		for p.comp.State.RobCount > 0 {
			p.comp.State.Cycles++
			p.retire()
		}
		p.comp.State.Cycles = max(p.comp.State.Cycles, p.comp.State.FrontendReady)
		before := p.comp.State.InstDone + uint64(p.comp.State.RobCount)
		p.fetchAndProcess()
		if got := int(p.comp.State.InstDone + uint64(p.comp.State.RobCount) - before); p.comp.State.Err != "" || got != tc.perCycle {
			t.Fatalf("taken=%d: admitted %d in one cycle, want %d (err %q)", tc.taken, got, tc.perCycle, p.comp.State.Err)
		}
	}
}

// Spec validation rejects a negative taken-branch count.
func TestNegativeFetchTakenBranchesRejected(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("negative fetch_taken_branches accepted")
		}
	}()
	specPipeline(t, []uint32{nop}, nil, func(s *Spec) { s.FetchTakenBranches = -1 })
}
