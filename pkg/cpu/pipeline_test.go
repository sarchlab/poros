package cpu

import (
	"encoding/binary"

	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
	"golang.org/x/arch/arm64/arm64asm"
)

// helper: encode ARM64 instructions into bytes.
func encodeInsts(insts []uint32) []byte {
	buf := make([]byte, len(insts)*4)
	for i, inst := range insts {
		binary.LittleEndian.PutUint32(buf[i*4:], inst)
	}
	return buf
}

// helper: set up emulator with code and return it.
func setupEmu(code []uint32, codeBase, haltPC uint64) *emu.Emulator {
	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))
	em.Regs.PC = codeBase
	em.Regs.WriteX(30, haltPC)
	em.SetHaltPC(haltPC)
	return em
}

// helper: run pipeline to completion.
func runPipeline(t *testing.T, em *emu.Emulator) *inorder.Comp {
	t.Helper()
	simulation := sim.MakeBuilder().Build()
	em.Mem.TrackAccesses = true
	pSpec := inorder.DefaultSpec()
	pSpec.Freq = 3490 * timing.MHz
	p := modeling.MakeBuilder[*inorder.Comp](pSpec).WithSimulation(simulation).WithResources(inorder.Resources{Emu: em, Cache: nil}).Build("TestPipeline")
	runTestSimulation(t, simulation, p)
	if p.State.Err != "" {
		t.Fatalf("pipeline error: %v", p.State.Err)
	}
	return p
}

// helper: run pipeline with a cache model.
func runPipelineWithCache(t *testing.T, em *emu.Emulator, cache *mem.Cache) *inorder.Comp {
	t.Helper()
	simulation := sim.MakeBuilder().Build()
	em.Mem.TrackAccesses = true
	pSpec := inorder.DefaultSpec()
	pSpec.Freq = 3490 * timing.MHz
	p := modeling.MakeBuilder[*inorder.Comp](pSpec).WithSimulation(simulation).WithResources(inorder.Resources{Emu: em, Cache: cache}).Build("TestPipeline")
	runTestSimulation(t, simulation, p)
	if p.State.Err != "" {
		t.Fatalf("pipeline error: %v", p.State.Err)
	}
	return p
}

func TestPipeline_SimpleSequence(t *testing.T) {
	// MOV X0, #42 ; RET
	code := []uint32{
		0xd2800540, // MOV X0, #42
		0xd65f03c0, // RET
	}
	codeBase := uint64(0x10000)
	haltPC := uint64(0xFFFF0000)
	em := setupEmu(code, codeBase, haltPC)
	em.Regs.SP = 0x500000 + 8192

	p := runPipeline(t, em)

	if em.Regs.ReadX(0) != 42 {
		t.Errorf("X0 = %d, want 42", em.Regs.ReadX(0))
	}
	if !em.Halted {
		t.Error("emulator should have halted")
	}
	t.Logf("SimpleSequence: %d instructions, %d cycles, CPI=%.2f",
		p.State.InstDone, p.State.Cycles, p.State.CPI())
}

func TestPipeline_DataHazardStall(t *testing.T) {
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

	p := runPipeline(t, em)

	if em.Regs.ReadX(1) != 11 {
		t.Errorf("X1 = %d, want 11", em.Regs.ReadX(1))
	}
	if em.Regs.ReadX(2) != 13 {
		t.Errorf("X2 = %d, want 13", em.Regs.ReadX(2))
	}

	// Without stall, 3 instructions in a 5-stage pipeline take 7 cycles (5 + 2).
	// With data hazard stall on X1, should take more.
	t.Logf("DataHazard: %d instructions, %d cycles, CPI=%.2f",
		p.State.InstDone, p.State.Cycles, p.State.CPI())

	if p.State.CPI() <= 1.0 {
		t.Error("expected CPI > 1.0 due to data hazard stall")
	}
}

func TestPipeline_BranchFlush(t *testing.T) {
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

	p := runPipeline(t, em)

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

func TestPipeline_ScoreboardFlushNoNegative(t *testing.T) {
	// Verify that flush() after a taken branch doesn't produce negative
	// scoreboard counts. The decode-stage instruction was never issued to
	// execute, so its scoreboard entries were never incremented.
	//
	// Sequence:
	//   B #8       (taken branch — flushes the next-fetched instruction)
	//   ADD X1, X0, #1  (fetched speculatively, should be flushed)
	//   MOV X0, #42     (branch target)
	//   RET
	code := []uint32{
		0x14000002, // B #8
		0x91000401, // ADD X1, X0, #1
		0xd2800540, // MOV X0, #42
		0xd65f03c0, // RET
	}
	codeBase := uint64(0x10000)
	haltPC := uint64(0xFFFF0000)
	em := setupEmu(code, codeBase, haltPC)
	em.Regs.SP = 0x500000 + 8192

	p := runPipeline(t, em)

	// After pipeline drains, all scoreboard counters must be >= 0.
	for i, cnt := range p.State.GprPending {
		if cnt < 0 {
			t.Errorf("gprPending[%d] = %d, want >= 0", i, cnt)
		}
	}
	for i, cnt := range p.State.FpPending {
		if cnt < 0 {
			t.Errorf("fpPending[%d] = %d, want >= 0", i, cnt)
		}
	}

	// All counters should be exactly 0 after full drain.
	for i, cnt := range p.State.GprPending {
		if cnt != 0 {
			t.Errorf("gprPending[%d] = %d, want 0 after drain", i, cnt)
		}
	}
	for i, cnt := range p.State.FpPending {
		if cnt != 0 {
			t.Errorf("fpPending[%d] = %d, want 0 after drain", i, cnt)
		}
	}

	if em.Regs.ReadX(0) != 42 {
		t.Errorf("X0 = %d, want 42", em.Regs.ReadX(0))
	}

	t.Logf("ScoreboardFlush: %d instructions, %d cycles, CPI=%.2f",
		p.State.InstDone, p.State.Cycles, p.State.CPI())
}

func TestLatency_KnownOps(t *testing.T) {
	tests := []struct {
		op   arm64asm.Op
		want int
	}{
		{arm64asm.ADD, 1},
		{arm64asm.MUL, 3},
		{arm64asm.SDIV, 10},
		{arm64asm.FADD, 3},
		{arm64asm.FMUL, 4},
		{arm64asm.FMADD, 4},
		{arm64asm.FMSUB, 4},
		{arm64asm.FNMADD, 4},
		{arm64asm.FNMSUB, 4},
		{arm64asm.FDIV, 10},
		{arm64asm.FSQRT, 13},
		{arm64asm.B, 1},
		{arm64asm.LDR, 1}, // execute latency; load latency is in Memory stage
	}
	for _, tc := range tests {
		got := instinfo.ExecuteLatency(tc.op)
		if got != tc.want {
			t.Errorf("instinfo.ExecuteLatency(%s) = %d, want %d", tc.op, got, tc.want)
		}
	}
}

// Keep one simulation alive per run, including the comparison-table loops.
func runTestSimulation(t *testing.T, s *sim.Simulation, c interface{ TickLater() }) {
	t.Helper()
	defer func() {
		if err := s.Terminate(); err != nil {
			t.Error(err)
		}
	}()
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	c.TickLater()
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
}
