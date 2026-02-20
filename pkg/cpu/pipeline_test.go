package cpu

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/m2sim3/pkg/emu"
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
func runPipeline(t *testing.T, em *emu.Emulator) *Pipeline {
	t.Helper()
	engine := sim.NewSerialEngine()
	p := NewPipeline("TestPipeline", engine, 3490*sim.MHz, em)
	p.TickLater()
	if err := engine.Run(); err != nil {
		t.Fatalf("engine.Run() failed: %v", err)
	}
	if p.Err != nil {
		t.Fatalf("pipeline error: %v", p.Err)
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
		p.InstDone, p.Cycles, p.CPI())
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
		p.InstDone, p.Cycles, p.CPI())

	if p.CPI() <= 1.0 {
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
		p.InstDone, p.Cycles, p.CPI())

	// The taken branch should cause a pipeline flush, adding penalty cycles.
	// 3 instructions executed: B, MOV #42, RET (MOV #99 is skipped).
	if p.InstDone != 3 {
		t.Errorf("expected 3 instructions completed, got %d", p.InstDone)
	}
}

func TestPipeline_PolyBenchAtax(t *testing.T) {
	// End-to-end PolyBench kernel_atax through the pipeline.
	m, n := 38, 42

	A := make([]float64, m*n)
	x := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = 1.0 + float64(i)/float64(n)
	}
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			A[i*n+j] = float64((i+j)%n) / float64(5*m)
		}
	}

	// Golden reference.
	expectedY := make([]float64, n)
	for i := 0; i < m; i++ {
		var tmpI float64
		for j := 0; j < n; j++ {
			tmpI += A[i*n+j] * x[j]
		}
		for j := 0; j < n; j++ {
			expectedY[j] += A[i*n+j] * tmpI
		}
	}

	code := []uint32{
		0x7100041f, 0x5400046b, 0xd2800008, 0x2a0103e9,
		0xd37d7c2a, 0x2a0003eb, 0x14000005, 0x91000508,
		0x8b0a0042, 0xeb0b011f, 0x54000340, 0xf82878bf,
		0x7100043f, 0x54ffff4b, 0x2f00e400, 0xaa0903ec,
		0xaa0303ed, 0xaa0203ee, 0xfc4085c1, 0xfc4085a2,
		0x1f420020, 0xfc2878a0, 0xf100058c, 0x54ffff61,
		0x7100043f, 0x54fffdcb, 0xd280000c, 0xfc6c6880,
		0xfc6c6841, 0xfc6878a2, 0x1f420020, 0xfc2c6880,
		0x9100218c, 0xeb0c015f, 0x54ffff21, 0x17ffffe4,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	xBase := uint64(0x200000)
	yBase := uint64(0x300000)
	tmpBase := uint64(0x400000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load A matrix.
	aBuf := make([]byte, m*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load x vector.
	xBuf := make([]byte, n*8)
	for i, v := range x {
		binary.LittleEndian.PutUint64(xBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(xBase, xBuf)

	// Load y (zeroed) and tmp (zeroed).
	em.LoadProgram(yBase, make([]byte, n*8))
	em.LoadProgram(tmpBase, make([]byte, m*8))

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(m))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(2, aBase)
	em.Regs.WriteX(3, xBase)
	em.Regs.WriteX(4, yBase)
	em.Regs.WriteX(5, tmpBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = 0x500000 + 8192
	em.SetHaltPC(haltPC)

	p := runPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify y[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(yBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedY[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("y[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("kernel_atax: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
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
		{arm64asm.FDIV, 10},
		{arm64asm.FSQRT, 12},
		{arm64asm.B, 1},
		{arm64asm.LDR, 1}, // execute latency; load latency is in Memory stage
	}
	for _, tc := range tests {
		got := ExecuteLatency(tc.op)
		if got != tc.want {
			t.Errorf("ExecuteLatency(%s) = %d, want %d", tc.op, got, tc.want)
		}
	}
}
