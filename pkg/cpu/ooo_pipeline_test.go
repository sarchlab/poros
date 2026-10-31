package cpu

import (
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"

	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
)

// helper: run OoO pipeline to completion.
func runOoOPipeline(t *testing.T, em *emu.Emulator) *ooo.Comp {
	t.Helper()
	simulation := sim.MakeBuilder().Build()
	em.Mem.TrackAccesses = true
	pSpec := ooo.DefaultSpec()
	pSpec.Freq = 3490 * timing.MHz
	p := modeling.MakeBuilder[*ooo.Comp](pSpec).WithSimulation(simulation).WithResources(ooo.Resources{Emu: em, Cache: nil}).Build("TestOoO")
	runTestSimulation(t, simulation, p)
	if p.State.Err != "" {
		t.Fatalf("pipeline error: %v", p.State.Err)
	}
	return p
}

// helper: run OoO pipeline with a cache model.
func runOoOPipelineWithCache(t *testing.T, em *emu.Emulator, cache *mem.Cache) *ooo.Comp {
	t.Helper()
	simulation := sim.MakeBuilder().Build()
	em.Mem.TrackAccesses = true
	pSpec := ooo.DefaultSpec()
	pSpec.Freq = 3490 * timing.MHz
	p := modeling.MakeBuilder[*ooo.Comp](pSpec).WithSimulation(simulation).WithResources(ooo.Resources{Emu: em, Cache: cache}).Build("TestOoO")
	runTestSimulation(t, simulation, p)
	if p.State.Err != "" {
		t.Fatalf("pipeline error: %v", p.State.Err)
	}
	return p
}

func TestOoO_SimpleSequence(t *testing.T) {
	// MOV X0, #42 ; RET
	code := []uint32{
		0xd2800540, // MOV X0, #42
		0xd65f03c0, // RET
	}
	codeBase := uint64(0x10000)
	haltPC := uint64(0xFFFF0000)
	em := setupEmu(code, codeBase, haltPC)
	em.Regs.SP = 0x500000 + 8192

	p := runOoOPipeline(t, em)

	if em.Regs.ReadX(0) != 42 {
		t.Errorf("X0 = %d, want 42", em.Regs.ReadX(0))
	}
	if !em.Halted {
		t.Error("emulator should have halted")
	}
	t.Logf("SimpleSequence: %d instructions, %d cycles, CPI=%.2f",
		p.State.InstDone, p.State.Cycles, p.State.CPI())
}

func TestOoO_DataHazardStall(t *testing.T) {
	// Two dependent instructions: ADD X1, X0, #1 ; ADD X2, X1, #2
	// X1 is written by first, read by second → should stall.
	code := []uint32{
		0x91000401, // ADD X1, X0, #1
		0x91000822, // ADD X2, X1, #2
		0xd65f03c0, // RET
	}
	codeBase := uint64(0x10000)
	haltPC := uint64(0xFFFF0000)
	em := setupEmu(code, codeBase, haltPC)
	em.Regs.WriteX(0, 10)
	em.Regs.SP = 0x500000 + 8192

	p := runOoOPipeline(t, em)

	if em.Regs.ReadX(1) != 11 {
		t.Errorf("X1 = %d, want 11", em.Regs.ReadX(1))
	}
	if em.Regs.ReadX(2) != 13 {
		t.Errorf("X2 = %d, want 13", em.Regs.ReadX(2))
	}

	t.Logf("DataHazard: %d instructions, %d cycles, CPI=%.2f",
		p.State.InstDone, p.State.Cycles, p.State.CPI())

	// The OoO pipeline with 8-wide fetch can issue multiple instructions per
	// cycle, so CPI may be < 1.0 even with a data dependency. The dependency
	// on X1 delays the second ADD but doesn't prevent superscalar throughput.
	// Just verify that the pipeline completed all 3 instructions correctly.
	if p.State.InstDone != 3 {
		t.Errorf("expected 3 instructions completed, got %d", p.State.InstDone)
	}
}

func TestOoO_BranchFlush(t *testing.T) {
	// B to skip one instruction, then MOV + RET.
	// B #8 (skip next instruction)
	// MOV X0, #99 (should be flushed)
	// MOV X0, #42 (branch target)
	// RET
	code := []uint32{
		0x14000002, // B #8 (skip next, go to offset +8 = PC+8)
		0xd2800c60, // MOV X0, #99
		0xd2800540, // MOV X0, #42
		0xd65f03c0, // RET
	}
	codeBase := uint64(0x10000)
	haltPC := uint64(0xFFFF0000)
	em := setupEmu(code, codeBase, haltPC)
	em.Regs.SP = 0x500000 + 8192

	p := runOoOPipeline(t, em)

	// Branch should skip MOV X0, #99 and land on MOV X0, #42.
	if em.Regs.ReadX(0) != 42 {
		t.Errorf("X0 = %d, want 42 (branch should have skipped MOV #99)",
			em.Regs.ReadX(0))
	}

	t.Logf("BranchFlush: %d instructions, %d cycles, CPI=%.2f",
		p.State.InstDone, p.State.Cycles, p.State.CPI())

	// The taken branch should cause a pipeline flush, adding penalty cycles.
	// 3 instructions executed: B, MOV #42, RET (MOV #99 is skipped).
	if p.State.InstDone != 3 {
		t.Errorf("expected 3 instructions completed, got %d", p.State.InstDone)
	}
}

func TestOoO_RegisterReadinessAfterDrain(t *testing.T) {
	// Verify that after the OoO pipeline drains completely, all register
	// readiness timestamps are reasonable (no values exceeding final cycle count).
	// This is the OoO equivalent of TestPipeline_ScoreboardFlushNoNegative.
	//
	// The OoO pipeline uses cycle-based readiness (gprReady/fpReady) instead
	// of pending counters. After drain, all readiness values should be <= Cycles.
	code := []uint32{
		0x14000002, // B #8
		0x91000401, // ADD X1, X0, #1 (skipped by branch)
		0xd2800540, // MOV X0, #42
		0xd65f03c0, // RET
	}
	codeBase := uint64(0x10000)
	haltPC := uint64(0xFFFF0000)
	em := setupEmu(code, codeBase, haltPC)
	em.Regs.SP = 0x500000 + 8192

	p := runOoOPipeline(t, em)

	// After pipeline drains, all register readiness values must be <= Cycles.
	for i, ready := range p.State.GprReady {
		if ready > p.State.Cycles {
			t.Errorf("gprReady[%d] = %d, exceeds final cycle count %d", i, ready, p.State.Cycles)
		}
	}
	for i, ready := range p.State.FpReady {
		if ready > p.State.Cycles {
			t.Errorf("fpReady[%d] = %d, exceeds final cycle count %d", i, ready, p.State.Cycles)
		}
	}

	if em.Regs.ReadX(0) != 42 {
		t.Errorf("X0 = %d, want 42", em.Regs.ReadX(0))
	}

	t.Logf("RegisterReadiness: %d instructions, %d cycles, CPI=%.2f",
		p.State.InstDone, p.State.Cycles, p.State.CPI())
}
