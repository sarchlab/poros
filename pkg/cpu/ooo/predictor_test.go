package ooo

import (
	"testing"
)

func TestBranchPred_LoopPrediction(t *testing.T) {
	bp := NewBranchPredictor()

	branchPC := uint64(0x1000)
	takenTarget := uint64(0x0F00)
	iterations := 100

	for i := 0; i < iterations; i++ {
		taken := i < iterations-1 // taken 99 times, not-taken on last
		predTaken, _ := bp.Predict(branchPC, false, false)
		bp.Update(branchPC, taken, takenTarget, predTaken, false, false)
	}

	accuracy := float64(bp.Hits) / float64(bp.Predictions)
	t.Logf("Loop prediction: %d/%d hits (%.1f%%)",
		bp.Hits, bp.Predictions, accuracy*100)

	if accuracy <= 0.95 {
		t.Errorf("loop prediction accuracy %.2f%% <= 95%%", accuracy*100)
	}
}

func TestBranchPred_UnconditionalBranch(t *testing.T) {
	bp := NewBranchPredictor()

	branchPC := uint64(0x2000)
	target := uint64(0x3000)
	totalIter := 50

	for i := 0; i < totalIter; i++ {
		predTaken, _ := bp.Predict(branchPC, false, false)
		bp.Update(branchPC, true, target, predTaken, false, false)
	}

	// After warmup, should be near 100%.
	accuracy := float64(bp.Hits) / float64(bp.Predictions)
	t.Logf("Unconditional branch: %d/%d hits (%.1f%%)",
		bp.Hits, bp.Predictions, accuracy*100)

	// First 1-2 might miss; with 50 iterations we should be well above 95%.
	if accuracy < 0.95 {
		t.Errorf("unconditional branch accuracy %.2f%% < 95%%", accuracy*100)
	}

	// Verify last 10 iterations were all correct by running 10 more.
	bp2 := NewBranchPredictor()
	// Warmup.
	for i := 0; i < 5; i++ {
		pred, _ := bp2.Predict(branchPC, false, false)
		bp2.Update(branchPC, true, target, pred, false, false)
	}
	bp2.Predictions = 0
	bp2.Hits = 0
	for i := 0; i < 10; i++ {
		pred, predTarget := bp2.Predict(branchPC, false, false)
		bp2.Update(branchPC, true, target, pred, false, false)
		if !pred {
			t.Errorf("iteration %d: predicted not-taken for always-taken branch", i)
		}
		if predTarget != target {
			t.Errorf("iteration %d: predicted target %#x, want %#x", i, predTarget, target)
		}
	}
	if bp2.Hits != 10 {
		t.Errorf("post-warmup: %d/10 hits, want 10/10", bp2.Hits)
	}
}

func TestBranchPred_CallReturn(t *testing.T) {
	bp := NewBranchPredictor()

	// Simple call/return.
	callPC := uint64(0x4000)
	callTarget := uint64(0x5000)
	retPC := uint64(0x5100)
	retTarget := uint64(0x4004) // callPC + 4

	// Train with a few iterations.
	for i := 0; i < 5; i++ {
		// Call: BL at callPC -> callTarget.
		predTaken, _ := bp.Predict(callPC, true, false)
		bp.Update(callPC, true, callTarget, predTaken, true, false)

		// Return: RET at retPC -> retTarget.
		predTaken, predTarget := bp.Predict(retPC, false, true)
		bp.Update(retPC, true, retTarget, predTaken, false, true)

		if i > 0 && predTarget != retTarget {
			t.Errorf("iter %d: RAS predicted return target %#x, want %#x",
				i, predTarget, retTarget)
		}
	}

	// Nested calls: BL 0x4000→0x5000, BL 0x5000→0x6000,
	//               RET 0x6100→0x5004, RET 0x5004→0x4004.
	bp2 := NewBranchPredictor()

	outerCallPC := uint64(0x4000)
	outerCallTarget := uint64(0x5000)
	innerCallPC := uint64(0x5000)
	innerCallTarget := uint64(0x6000)
	innerRetPC := uint64(0x6100)
	innerRetTarget := uint64(0x5004) // innerCallPC + 4
	outerRetPC := uint64(0x5004)
	outerRetTarget := uint64(0x4004) // outerCallPC + 4

	// Train a few rounds so BTB learns targets.
	for round := 0; round < 5; round++ {
		pred, _ := bp2.Predict(outerCallPC, true, false)
		bp2.Update(outerCallPC, true, outerCallTarget, pred, true, false)

		pred, _ = bp2.Predict(innerCallPC, true, false)
		bp2.Update(innerCallPC, true, innerCallTarget, pred, true, false)

		pred, predTarget := bp2.Predict(innerRetPC, false, true)
		bp2.Update(innerRetPC, true, innerRetTarget, pred, false, true)
		if round > 0 && predTarget != innerRetTarget {
			t.Errorf("round %d: inner RET predicted %#x, want %#x",
				round, predTarget, innerRetTarget)
		}

		pred, predTarget = bp2.Predict(outerRetPC, false, true)
		bp2.Update(outerRetPC, true, outerRetTarget, pred, false, true)
		if round > 0 && predTarget != outerRetTarget {
			t.Errorf("round %d: outer RET predicted %#x, want %#x",
				round, predTarget, outerRetTarget)
		}
	}

	t.Logf("Nested call/return test passed after 5 rounds")
}

func TestBranchPred_BimodalSaturation(t *testing.T) {
	bp := NewBranchPredictor()

	pc := uint64(0x6000)
	target := uint64(0x6100)

	// Initial state: bimodal counter starts at 2 (weakly taken).
	// Verify initial prediction is taken.
	pred, _ := bp.Predict(pc, false, false)
	if !pred {
		t.Errorf("initial prediction should be taken (counter=2)")
	}
	bp.Update(pc, true, target, pred, false, false)

	// After one taken update: counter=3 (strongly taken).
	// One not-taken should move to 2 (weakly taken), still predicts taken.
	pred, _ = bp.Predict(pc, false, false)
	bp.Update(pc, false, target, pred, false, false) // counter: 3 -> 2

	pred, _ = bp.Predict(pc, false, false)
	if !pred {
		t.Errorf("after strong→weak taken (counter=2), should still predict taken")
	}

	// Another not-taken: counter=2->1 (weakly not-taken).
	bp.Update(pc, false, target, pred, false, false) // counter: 2 -> 1

	pred, _ = bp.Predict(pc, false, false)
	if pred {
		t.Errorf("after counter=1, should predict not-taken")
	}

	// Another not-taken: counter=1->0 (strongly not-taken).
	bp.Update(pc, false, target, pred, false, false) // counter: 1 -> 0

	// From strongly not-taken (0), one taken should move to 1 (weakly not-taken).
	pred, _ = bp.Predict(pc, false, false)
	if pred {
		t.Errorf("counter=0 (strong not-taken), should predict not-taken")
	}
	bp.Update(pc, true, target, pred, false, false) // counter: 0 -> 1

	pred, _ = bp.Predict(pc, false, false)
	if pred {
		t.Errorf("counter=1 (weak not-taken), should still predict not-taken")
	}

	// One more taken: counter=1->2 (weakly taken).
	bp.Update(pc, true, target, pred, false, false) // counter: 1 -> 2

	pred, _ = bp.Predict(pc, false, false)
	if !pred {
		t.Errorf("counter=2 (weak taken), should predict taken")
	}

	t.Logf("Bimodal saturation test passed: 2-bit counter transitions verified")
}

func TestBranchPred_BTBTargetUpdate(t *testing.T) {
	bp := NewBranchPredictor()

	branchPC := uint64(0x7000)
	target1 := uint64(0x8000)
	target2 := uint64(0x9000)

	// Train with target1.
	for i := 0; i < 5; i++ {
		pred, _ := bp.Predict(branchPC, false, false)
		bp.Update(branchPC, true, target1, pred, false, false)
	}

	// Verify BTB returns target1.
	_, predTarget := bp.Predict(branchPC, false, false)
	if predTarget != target1 {
		t.Errorf("before update: predicted target %#x, want %#x", predTarget, target1)
	}
	bp.Update(branchPC, true, target1, true, false, false)

	// Now switch to target2.
	for i := 0; i < 5; i++ {
		pred, _ := bp.Predict(branchPC, false, false)
		bp.Update(branchPC, true, target2, pred, false, false)
	}

	// Verify BTB now returns target2.
	_, predTarget = bp.Predict(branchPC, false, false)
	if predTarget != target2 {
		t.Errorf("after update: predicted target %#x, want %#x", predTarget, target2)
	}

	t.Logf("BTB target update: correctly switched from %#x to %#x", target1, target2)
}

func TestBranchPred_ConfigurableSizes(t *testing.T) {
	bp := &predictor{PredictorState: &PredictorState{}, spec: PredictorSpec{
		BimodalSize:     256,
		TaggedTableSize: 128,
		BTBSize:         64,
		BTBWays:         2,
		RASSize:         8,
		TaggedHistLen:   32,
	}}

	bp.init()

	// Verify tables were allocated with correct sizes.
	if len(bp.Bimodal) != 256 {
		t.Errorf("bimodal size %d, want 256", len(bp.Bimodal))
	}
	if len(bp.Tagged) != 128 {
		t.Errorf("tagged size %d, want 128", len(bp.Tagged))
	}
	if len(bp.Btb) != 64 {
		t.Errorf("btb size %d, want 64", len(bp.Btb))
	}
	if len(bp.Ras) != 8 {
		t.Errorf("ras size %d, want 8", len(bp.Ras))
	}

	// Run predictions to verify no panics.
	pc := uint64(0xBEEF)
	target := uint64(0xDEAD)

	for i := 0; i < 20; i++ {
		pred, _ := bp.Predict(pc, false, false)
		bp.Update(pc, true, target, pred, false, false)
	}

	// Call/return with small RAS.
	for i := 0; i < 10; i++ {
		callPC := uint64(0x1000 + uint64(i)*0x100)
		bp.Predict(callPC, true, false)
		bp.Update(callPC, true, uint64(0x2000+uint64(i)*0x100), true, true, false)
	}
	for i := 9; i >= 0; i-- {
		retPC := uint64(0x2000 + uint64(i)*0x100 + 0x50)
		pred, _ := bp.Predict(retPC, false, true)
		bp.Update(retPC, true, uint64(0x1000+uint64(i)*0x100+4), pred, false, true)
	}

	t.Logf("Configurable predictor: %d predictions, %d hits (%.1f%%)",
		bp.Predictions, bp.Hits,
		float64(bp.Hits)/float64(bp.Predictions)*100)
}

func TestBranchPred_Accuracy90PercentLoop(t *testing.T) {
	bp := NewBranchPredictor()

	outerPC := uint64(0xA000)
	outerTarget := uint64(0xA004)
	innerPC := uint64(0xA100)
	innerTarget := uint64(0xA004)

	outerIterations := 10
	innerIterations := 100

	for i := 0; i < outerIterations; i++ {
		for j := 0; j < innerIterations; j++ {
			innerTaken := j < innerIterations-1
			pred, _ := bp.Predict(innerPC, false, false)
			bp.Update(innerPC, innerTaken, innerTarget, pred, false, false)
		}

		outerTaken := i < outerIterations-1
		pred, _ := bp.Predict(outerPC, false, false)
		bp.Update(outerPC, outerTaken, outerTarget, pred, false, false)
	}

	accuracy := float64(bp.Hits) / float64(bp.Predictions)
	t.Logf("Double loop: %d predictions, %d hits (%.1f%%)",
		bp.Predictions, bp.Hits, accuracy*100)
	t.Logf("  Outer loop: %d iterations, Inner loop: %d iterations",
		outerIterations, innerIterations)

	if accuracy <= 0.90 {
		t.Errorf("double loop accuracy %.2f%% <= 90%%", accuracy*100)
	}
}

func NewBranchPredictor() *predictor {
	p := &predictor{PredictorState: &PredictorState{}, spec: DefaultSpec().Predictor}
	p.init()
	return p
}
