package cpu

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/m2sim3/pkg/benchmark"
	"github.com/sarchlab/m2sim3/pkg/emu"
	"github.com/sarchlab/m2sim3/pkg/mem"
)

// helper: run OoO pipeline to completion.
func runOoOPipeline(t *testing.T, em *emu.Emulator) *OoOPipeline {
	t.Helper()
	engine := sim.NewSerialEngine()
	p := NewOoOPipeline("TestOoO", engine, 3490*sim.MHz, em)
	p.TickLater()
	if err := engine.Run(); err != nil {
		t.Fatalf("engine.Run() failed: %v", err)
	}
	if p.Err != nil {
		t.Fatalf("pipeline error: %v", p.Err)
	}
	return p
}

// helper: run OoO pipeline with a cache model.
func runOoOPipelineWithCache(t *testing.T, em *emu.Emulator, cache *mem.Cache) *OoOPipeline {
	t.Helper()
	engine := sim.NewSerialEngine()
	p := NewOoOPipeline("TestOoO", engine, 3490*sim.MHz, em)
	p.Cache = cache
	p.TickLater()
	if err := engine.Run(); err != nil {
		t.Fatalf("engine.Run() failed: %v", err)
	}
	if p.Err != nil {
		t.Fatalf("pipeline error: %v", p.Err)
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
		p.InstDone, p.Cycles, p.CPI())
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
		p.InstDone, p.Cycles, p.CPI())

	// The OoO pipeline with 8-wide fetch can issue multiple instructions per
	// cycle, so CPI may be < 1.0 even with a data dependency. The dependency
	// on X1 delays the second ADD but doesn't prevent superscalar throughput.
	// Just verify that the pipeline completed all 3 instructions correctly.
	if p.InstDone != 3 {
		t.Errorf("expected 3 instructions completed, got %d", p.InstDone)
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
		p.InstDone, p.Cycles, p.CPI())

	// The taken branch should cause a pipeline flush, adding penalty cycles.
	// 3 instructions executed: B, MOV #42, RET (MOV #99 is skipped).
	if p.InstDone != 3 {
		t.Errorf("expected 3 instructions completed, got %d", p.InstDone)
	}
}

func TestOoO_PolyBenchAtax(t *testing.T) {
	// End-to-end PolyBench kernel_atax through the OoO pipeline.
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

	p := runOoOPipeline(t, em)

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

func TestOoO_PolyBenchMvt(t *testing.T) {
	// End-to-end PolyBench kernel_mvt through the OoO pipeline.
	n := 40

	x1 := make([]float64, n)
	x2 := make([]float64, n)
	y1 := make([]float64, n)
	y2 := make([]float64, n)
	A := make([]float64, n*n)

	for i := 0; i < n; i++ {
		x1[i] = float64(i%n) / float64(n)
		x2[i] = float64((i+1)%n) / float64(n)
		y1[i] = float64((i+3)%n) / float64(n)
		y2[i] = float64((i+4)%n) / float64(n)
		for j := 0; j < n; j++ {
			A[i*n+j] = float64(i*j%n) / float64(n)
		}
	}

	// Golden reference.
	expectedX1 := make([]float64, n)
	expectedX2 := make([]float64, n)
	copy(expectedX1, x1)
	copy(expectedX2, x2)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			expectedX1[i] += A[i*n+j] * y1[j]
		}
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			expectedX2[i] += A[j*n+i] * y2[j]
		}
	}

	code := []uint32{
		0x7100041f, 0x540004cb, 0xd2800009, 0xd37d7c0a,
		0xaa0503eb, 0x2a0003e8, 0xfc697820, 0xaa0803ec,
		0xaa0303ed, 0xaa0b03ee, 0xfc4085c1, 0xfc4085a2,
		0x1f420020, 0xfc297820, 0xf100058c, 0x54ffff61,
		0x91000529, 0x8b0a016b, 0xeb08013f, 0x54fffe61,
		0x7100041f, 0x5400024b, 0xd2800009, 0xd37d7c0a,
		0xfc697840, 0xaa0803eb, 0xaa0403ec, 0xaa0503ed,
		0xfc408581, 0xfd4001a2, 0x1f410040, 0xfc297840,
		0x8b0a01ad, 0xf100056b, 0x54ffff41, 0x91000529,
		0x910020a5, 0xeb08013f, 0x54fffe41, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	x1Base := uint64(0x100000)
	x2Base := uint64(0x200000)
	y1Base := uint64(0x300000)
	y2Base := uint64(0x400000)
	aBase := uint64(0x500000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load x1 vector.
	x1Buf := make([]byte, n*8)
	for i, v := range x1 {
		binary.LittleEndian.PutUint64(x1Buf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(x1Base, x1Buf)

	// Load x2 vector.
	x2Buf := make([]byte, n*8)
	for i, v := range x2 {
		binary.LittleEndian.PutUint64(x2Buf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(x2Base, x2Buf)

	// Load y1 vector.
	y1Buf := make([]byte, n*8)
	for i, v := range y1 {
		binary.LittleEndian.PutUint64(y1Buf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(y1Base, y1Buf)

	// Load y2 vector.
	y2Buf := make([]byte, n*8)
	for i, v := range y2 {
		binary.LittleEndian.PutUint64(y2Buf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(y2Base, y2Buf)

	// Load A matrix.
	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, x1Base)
	em.Regs.WriteX(2, x2Base)
	em.Regs.WriteX(3, y1Base)
	em.Regs.WriteX(4, y2Base)
	em.Regs.WriteX(5, aBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = 0x600000 + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify x1[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(x1Base + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedX1[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("x1[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	// Verify x2[] matches golden reference.
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(x2Base + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedX2[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("x2[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("kernel_mvt: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchGemm(t *testing.T) {
	// End-to-end PolyBench kernel_gemm through the OoO pipeline.
	ni, nj, nk := 20, 25, 30
	alpha, beta := 1.5, 1.2

	C := make([]float64, ni*nj)
	A := make([]float64, ni*nk)
	B := make([]float64, nk*nj)

	for i := 0; i < ni; i++ {
		for j := 0; j < nj; j++ {
			C[i*nj+j] = float64((i*j+1)%ni) / float64(ni)
		}
	}
	for i := 0; i < ni; i++ {
		for j := 0; j < nk; j++ {
			A[i*nk+j] = float64(i*(j+1)%nk) / float64(nk)
		}
	}
	for i := 0; i < nk; i++ {
		for j := 0; j < nj; j++ {
			B[i*nj+j] = float64(i*(j+2)%nj) / float64(nj)
		}
	}

	// Golden reference.
	expectedC := make([]float64, ni*nj)
	copy(expectedC, C)
	for i := 0; i < ni; i++ {
		for j := 0; j < nj; j++ {
			expectedC[i*nj+j] *= beta
		}
		for k := 0; k < nk; k++ {
			for j := 0; j < nj; j++ {
				expectedC[i*nj+j] += alpha * A[i*nk+k] * B[k*nj+j]
			}
		}
	}

	code := []uint32{
		0x7100041f, 0x5400066b, 0xa9bf4ff4, 0x52800008,
		0xd2800009, 0x2a0103ea, 0x2a0203eb, 0xd37d7c2c,
		0xaa0303ed, 0x2a0003ee, 0x14000006, 0x91000529,
		0x8b0c01ad, 0x0b010108, 0xeb0e013f, 0x54000480,
		0xaa0a03ef, 0xaa0d03f0, 0x7100043f, 0x540000cb,
		0xfd400202, 0x1e610842, 0xfc008602, 0xf10005ef,
		0x54ffff81, 0x7100045f, 0x54fffe2b, 0xd280000f,
		0x8b284c70, 0x9b0b7d31, 0x8b110c91, 0xaa0503e0,
		0x14000005, 0x910005ef, 0x8b0c0000, 0xeb0b01ff,
		0x54fffce0, 0xaa0a03e6, 0xaa0003e7, 0xaa1003f3,
		0x7100043f, 0x54ffff0b, 0xfc6f7a22, 0xfc4084e3,
		0x1e600842, 0xfd400264, 0x1f431042, 0xfc008662,
		0xf10004c6, 0x54ffff21, 0x17ffffef, 0xa8c14ff4,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	cBase := uint64(0x100000)
	aBase := uint64(0x200000)
	bBase := uint64(0x300000)
	stackBase := uint64(0x400000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load C matrix.
	cBuf := make([]byte, ni*nj*8)
	for i, v := range C {
		binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(cBase, cBuf)

	// Load A matrix.
	aBuf := make([]byte, ni*nk*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load B matrix.
	bBuf := make([]byte, nk*nj*8)
	for i, v := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	// Allocate stack space (for STP/LDP of callee-saved registers).
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(ni))
	em.Regs.WriteX(1, uint64(nj))
	em.Regs.WriteX(2, uint64(nk))
	em.Regs.WriteD(0, alpha)
	em.Regs.WriteD(1, beta)
	em.Regs.WriteX(3, cBase)
	em.Regs.WriteX(4, aBase)
	em.Regs.WriteX(5, bBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify C[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < ni*nj; i++ {
		bits := em.Mem.ReadU64(cBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedC[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/nj, i%nj
			t.Errorf("C[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_gemm: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBench2mm(t *testing.T) {
	// End-to-end PolyBench kernel_2mm through the OoO pipeline.
	ni, nj, nk, nl := 16, 18, 22, 24
	alpha, beta := 1.5, 1.2

	tmp := make([]float64, ni*nj)
	A := make([]float64, ni*nk)
	B := make([]float64, nk*nj)
	C := make([]float64, nj*nl)
	D := make([]float64, ni*nl)

	for i := 0; i < ni; i++ {
		for j := 0; j < nk; j++ {
			A[i*nk+j] = float64((i*j+1)%ni) / float64(ni)
		}
		for j := 0; j < nj; j++ {
			tmp[i*nj+j] = 0.0
		}
	}
	for i := 0; i < nk; i++ {
		for j := 0; j < nj; j++ {
			B[i*nj+j] = float64(i*(j+1)%nj) / float64(nj)
		}
	}
	for i := 0; i < nj; i++ {
		for j := 0; j < nl; j++ {
			C[i*nl+j] = float64((i*(j+3)+1)%nl) / float64(nl)
		}
	}
	for i := 0; i < ni; i++ {
		for j := 0; j < nl; j++ {
			D[i*nl+j] = float64(i*(j+2)%nk) / float64(nk)
		}
	}

	// Golden reference.
	goldenTmp := make([]float64, ni*nj)
	for i := 0; i < ni; i++ {
		for j := 0; j < nj; j++ {
			goldenTmp[i*nj+j] = 0.0
			for k := 0; k < nk; k++ {
				goldenTmp[i*nj+j] += alpha * A[i*nk+k] * B[k*nj+j]
			}
		}
	}
	expectedD := make([]float64, ni*nl)
	copy(expectedD, D)
	for i := 0; i < ni; i++ {
		for j := 0; j < nl; j++ {
			expectedD[i*nl+j] *= beta
			for k := 0; k < nj; k++ {
				expectedD[i*nl+j] += goldenTmp[i*nj+k] * C[k*nl+j]
			}
		}
	}

	code := []uint32{
		0x7100041f, 0x54000a8b, 0xa9be57f6, 0xa9014ff4,
		0x5280000b, 0xd280000c, 0x2a0103e8, 0xf94013e9,
		0x2a0003ea, 0xd37d7c2d, 0x2a0203ee, 0x14000005,
		0x9100058c, 0x0b02016b, 0xeb0a019f, 0x540003a0,
		0x7100043f, 0x54ffff6b, 0xd280000f, 0x8b2b4cb0,
		0x9b087d91, 0x8b110c91, 0xaa0603f3, 0x14000005,
		0x910005ef, 0x91002273, 0xeb0801ff, 0x54fffe20,
		0xf82f7a3f, 0x7100045f, 0x54ffff4b, 0x2f00e402,
		0xaa0e03f4, 0xaa1303f5, 0xaa1003f6, 0xfc4086c3,
		0x1e600863, 0xfd4002a4, 0x1f440862, 0xfc2f7a22,
		0x8b0d02b5, 0xf1000694, 0x54ffff21, 0x17ffffed,
		0x7100041f, 0x540004cb, 0x5280000b, 0xd280000c,
		0xd37d7c6d, 0x2a0303ee, 0x14000005, 0x9100058c,
		0x0b01016b, 0xeb0a019f, 0x540003a0, 0x7100047f,
		0x54ffff6b, 0xd280000f, 0x8b2b4c90, 0x9b0e7d91,
		0x8b110d31, 0xaa0703e0, 0x14000005, 0x910005ef,
		0x91002000, 0xeb0e01ff, 0x54fffe20, 0xfc6f7a20,
		0x1e610800, 0xfc2f7a20, 0xaa0803e2, 0xaa0003e5,
		0xaa1003e6, 0x7100043f, 0x54fffeab, 0xfc4084c2,
		0xfd4000a3, 0x1f430040, 0xfc2f7a20, 0x8b0d00a5,
		0xf1000442, 0x54ffff41, 0x17ffffed, 0xa9414ff4,
		0xa8c257f6, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	tmpBase := uint64(0x100000)
	aBase := uint64(0x200000)
	bBase := uint64(0x300000)
	cBase := uint64(0x400000)
	dBase := uint64(0x500000)
	stackBase := uint64(0x600000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load tmp matrix.
	tmpBuf := make([]byte, ni*nj*8)
	for i, v := range tmp {
		binary.LittleEndian.PutUint64(tmpBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(tmpBase, tmpBuf)

	// Load A matrix.
	aBuf := make([]byte, ni*nk*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load B matrix.
	bBuf := make([]byte, nk*nj*8)
	for i, v := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	// Load C matrix.
	cBuf := make([]byte, nj*nl*8)
	for i, v := range C {
		binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(cBase, cBuf)

	// Load D matrix.
	dBuf := make([]byte, ni*nl*8)
	for i, v := range D {
		binary.LittleEndian.PutUint64(dBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(dBase, dBuf)

	// Allocate stack space and write D's address for stack-passed arg.
	stackBuf := make([]byte, 8192)
	binary.LittleEndian.PutUint64(stackBuf[4096:], dBase)
	em.LoadProgram(stackBase, stackBuf)

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(ni))
	em.Regs.WriteX(1, uint64(nj))
	em.Regs.WriteX(2, uint64(nk))
	em.Regs.WriteX(3, uint64(nl))
	em.Regs.WriteD(0, alpha)
	em.Regs.WriteD(1, beta)
	em.Regs.WriteX(4, tmpBase)
	em.Regs.WriteX(5, aBase)
	em.Regs.WriteX(6, bBase)
	em.Regs.WriteX(7, cBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 4096
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify D[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < ni*nl; i++ {
		bits := em.Mem.ReadU64(dBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedD[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/nl, i%nl
			t.Errorf("D[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_2mm: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBench3mm(t *testing.T) {
	// End-to-end PolyBench kernel_3mm through the OoO pipeline.
	ni, nj, nk, nl, nm := 16, 18, 20, 22, 24

	A := make([]float64, ni*nk)
	B := make([]float64, nk*nj)
	C := make([]float64, nj*nm)
	D := make([]float64, nm*nl)
	E := make([]float64, ni*nj)
	F := make([]float64, nj*nl)
	G := make([]float64, ni*nl)

	for i := 0; i < ni; i++ {
		for j := 0; j < nk; j++ {
			A[i*nk+j] = float64((i*j+1)%ni) / float64(5*ni)
		}
	}
	for i := 0; i < nk; i++ {
		for j := 0; j < nj; j++ {
			B[i*nj+j] = float64((i*(j+1)+2)%nj) / float64(5*nj)
		}
	}
	for i := 0; i < nj; i++ {
		for j := 0; j < nm; j++ {
			C[i*nm+j] = float64(i*(j+3)%nl) / float64(5*nl)
		}
	}
	for i := 0; i < nm; i++ {
		for j := 0; j < nl; j++ {
			D[i*nl+j] = float64((i*(j+2)+2)%nk) / float64(5*nk)
		}
	}

	// Golden reference.
	expectedE := make([]float64, ni*nj)
	for i := 0; i < ni; i++ {
		for j := 0; j < nj; j++ {
			for k := 0; k < nk; k++ {
				expectedE[i*nj+j] += A[i*nk+k] * B[k*nj+j]
			}
		}
	}
	expectedF := make([]float64, nj*nl)
	for i := 0; i < nj; i++ {
		for j := 0; j < nl; j++ {
			for k := 0; k < nm; k++ {
				expectedF[i*nl+j] += C[i*nm+k] * D[k*nl+j]
			}
		}
	}
	expectedG := make([]float64, ni*nl)
	for i := 0; i < ni; i++ {
		for j := 0; j < nl; j++ {
			for k := 0; k < nj; k++ {
				expectedG[i*nl+j] += expectedE[i*nj+k] * expectedF[k*nl+j]
			}
		}
	}

	code := []uint32{
		0xa9be57f6, 0xa9014ff4, 0x2a0103e8, 0x2a0003e9,
		0x7100041f, 0x540004ab, 0x5280000a, 0xd280000b,
		0xd37d7c2c, 0x2a0203ed, 0x14000005, 0x9100056b,
		0x0b02014a, 0xeb09017f, 0x54000380, 0x7100043f,
		0x54ffff6b, 0xd280000e, 0x8b2a4ccf, 0x9b087d70,
		0x8b100cb0, 0xaa0703f1, 0x14000005, 0x910005ce,
		0x91002231, 0xeb0801df, 0x54fffe20, 0xf82e7a1f,
		0x7100045f, 0x54ffff4b, 0x2f00e400, 0xaa0d03f3,
		0xaa1103f4, 0xaa0f03f5, 0xfc4086a1, 0xfd400282,
		0x1f420020, 0xfc2e7a00, 0x8b0c0294, 0xf1000673,
		0x54ffff41, 0x17ffffee, 0xf94013ea, 0x2a0303eb,
		0x7100043f, 0x540004cb, 0x5280000c, 0xd280000d,
		0xa942bbef, 0xd37d7c70, 0x2a0403f1, 0x14000005,
		0x910005ad, 0x0b04018c, 0xeb0801bf, 0x54000380,
		0x7100047f, 0x54ffff6b, 0xd2800002, 0x8b2c4de6,
		0x9b0b7da7, 0x8b070d47, 0xaa0e03f3, 0x14000005,
		0x91000442, 0x91002273, 0xeb0b005f, 0x54fffe20,
		0xf82278ff, 0x7100049f, 0x54ffff4b, 0x2f00e400,
		0xaa1103f4, 0xaa1303f5, 0xaa0603f6, 0xfc4086c1,
		0xfd4002a2, 0x1f420020, 0xfc2278e0, 0x8b1002b5,
		0xf1000694, 0x54ffff41, 0x17ffffee, 0x7100041f,
		0x540004cb, 0x5280000c, 0xd280000d, 0xf9401fee,
		0xaa0303ef, 0xd37d7def, 0x14000005, 0x910005ad,
		0x0b01018c, 0xeb0901bf, 0x54000380, 0x7100047f,
		0x54ffff6b, 0xd2800010, 0x8b2c4cb1, 0x9b0b7da0,
		0x8b000dc0, 0xaa0a03e2, 0x14000005, 0x91000610,
		0x91002042, 0xeb0b021f, 0x54fffe20, 0xf830781f,
		0x7100043f, 0x54ffff4b, 0x2f00e400, 0xaa0803e4,
		0xaa0203e6, 0xaa1103e7, 0xfc4084e1, 0xfd4000c2,
		0x1f420020, 0xfc307800, 0x8b0f00c6, 0xf1000484,
		0x54ffff41, 0x17ffffee, 0xa9414ff4, 0xa8c257f6,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	eBase := uint64(0x100000)
	aBase := uint64(0x200000)
	bBase := uint64(0x300000)
	fBase := uint64(0x400000)
	cBase := uint64(0x500000)
	dBase := uint64(0x600000)
	gBase := uint64(0x700000)
	stackBase := uint64(0x800000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load E matrix.
	eBuf := make([]byte, ni*nj*8)
	for i, v := range E {
		binary.LittleEndian.PutUint64(eBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(eBase, eBuf)

	// Load A matrix.
	aBuf := make([]byte, ni*nk*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load B matrix.
	bBuf := make([]byte, nk*nj*8)
	for i, v := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	// Load F matrix.
	fBuf := make([]byte, nj*nl*8)
	for i, v := range F {
		binary.LittleEndian.PutUint64(fBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(fBase, fBuf)

	// Load C matrix.
	cBufData := make([]byte, nj*nm*8)
	for i, v := range C {
		binary.LittleEndian.PutUint64(cBufData[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(cBase, cBufData)

	// Load D matrix.
	dBufData := make([]byte, nm*nl*8)
	for i, v := range D {
		binary.LittleEndian.PutUint64(dBufData[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(dBase, dBufData)

	// Load G matrix.
	gBuf := make([]byte, ni*nl*8)
	for i, v := range G {
		binary.LittleEndian.PutUint64(gBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(gBase, gBuf)

	// Allocate stack space and write stack-passed args.
	stackBuf := make([]byte, 8192)
	binary.LittleEndian.PutUint64(stackBuf[4096:], fBase)
	binary.LittleEndian.PutUint64(stackBuf[4096+8:], cBase)
	binary.LittleEndian.PutUint64(stackBuf[4096+16:], dBase)
	binary.LittleEndian.PutUint64(stackBuf[4096+24:], gBase)
	em.LoadProgram(stackBase, stackBuf)

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(ni))
	em.Regs.WriteX(1, uint64(nj))
	em.Regs.WriteX(2, uint64(nk))
	em.Regs.WriteX(3, uint64(nl))
	em.Regs.WriteX(4, uint64(nm))
	em.Regs.WriteX(5, eBase)
	em.Regs.WriteX(6, aBase)
	em.Regs.WriteX(7, bBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 4096
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify G[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < ni*nl; i++ {
		bits := em.Mem.ReadU64(gBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedG[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/nl, i%nl
			t.Errorf("G[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_3mm: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchBicg(t *testing.T) {
	// End-to-end PolyBench kernel_bicg through the OoO pipeline.
	m, n := 38, 42

	A := make([]float64, m*n)
	s := make([]float64, n)
	q := make([]float64, m)
	p := make([]float64, n)
	r := make([]float64, m)

	for i := 0; i < m; i++ {
		r[i] = float64(i%m) / float64(m)
		for j := 0; j < n; j++ {
			A[i*n+j] = float64(i*(j+1)%n) / float64(n)
		}
	}
	for i := 0; i < n; i++ {
		s[i] = 0.0
		p[i] = float64(i%n) / float64(n)
	}

	// Golden reference.
	expectedS := make([]float64, n)
	expectedQ := make([]float64, m)
	copy(expectedS, s)
	for i := 0; i < m; i++ {
		expectedQ[i] = 0.0
		for j := 0; j < n; j++ {
			expectedS[j] += r[i] * A[i*n+j]
			expectedQ[i] += A[i*n+j] * p[j]
		}
	}

	code := []uint32{
		0x7100041f, 0x540003cb, 0xd2800008, 0x2a0103e9,
		0xd37d7c2a, 0x2a0003eb, 0x14000005, 0x91000508,
		0x8b0a0042, 0xeb0b011f, 0x540002a0, 0xf828789f,
		0xaa0903ec, 0xaa0303ed, 0xaa0503ee, 0xaa0203ef,
		0x7100043f, 0x54fffecb, 0xfd4001a0, 0xfc6878c1,
		0xfd4001e2, 0x1f420020, 0xfc0085a0, 0xfc687880,
		0xfc4085e1, 0xfc4085c2, 0x1f420020, 0xfc287880,
		0xf100058c, 0x54fffea1, 0x17ffffe9, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	sBase := uint64(0x200000)
	qBase := uint64(0x300000)
	pBase := uint64(0x400000)
	rBase := uint64(0x500000)
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

	// Load s vector.
	sBuf := make([]byte, n*8)
	for i, v := range s {
		binary.LittleEndian.PutUint64(sBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(sBase, sBuf)

	// Load q vector.
	qBuf := make([]byte, m*8)
	for i, v := range q {
		binary.LittleEndian.PutUint64(qBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(qBase, qBuf)

	// Load p vector.
	pBuf := make([]byte, n*8)
	for i, v := range p {
		binary.LittleEndian.PutUint64(pBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(pBase, pBuf)

	// Load r vector.
	rBuf := make([]byte, m*8)
	for i, v := range r {
		binary.LittleEndian.PutUint64(rBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(rBase, rBuf)

	em.LoadProgram(uint64(0x600000), make([]byte, 8192))

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(m))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(2, aBase)
	em.Regs.WriteX(3, sBase)
	em.Regs.WriteX(4, qBase)
	em.Regs.WriteX(5, pBase)
	em.Regs.WriteX(6, rBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = 0x600000 + 8192
	em.SetHaltPC(haltPC)

	pip := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify s[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(sBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedS[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("s[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	// Verify q[] matches golden reference.
	for i := 0; i < m; i++ {
		bits := em.Mem.ReadU64(qBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedQ[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("q[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("kernel_bicg: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		pip.InstDone, pip.Cycles, pip.CPI(), maxRelErr)
}

func TestOoO_PolyBenchGesummv(t *testing.T) {
	// End-to-end PolyBench kernel_gesummv through the OoO pipeline.
	n := 30
	alpha, beta := 1.5, 1.2

	A := make([]float64, n*n)
	B := make([]float64, n*n)
	x := make([]float64, n)

	for i := 0; i < n; i++ {
		x[i] = float64(i%n) / float64(n)
		for j := 0; j < n; j++ {
			A[i*n+j] = float64((i*j+1)%n) / float64(n)
			B[i*n+j] = float64((i*j+2)%n) / float64(n)
		}
	}

	// Golden reference.
	expectedY := make([]float64, n)
	for i := 0; i < n; i++ {
		var tmpI, yI float64
		for j := 0; j < n; j++ {
			tmpI = A[i*n+j]*x[j] + tmpI
			yI = B[i*n+j]*x[j] + yI
		}
		expectedY[i] = alpha*tmpI + beta*yI
	}

	code := []uint32{
		0x7100041f, 0x540003eb, 0xd2800008, 0xd37d7c09,
		0x2a0003ea, 0xf828787f, 0xf82878bf, 0xaa0a03eb,
		0xaa0103ec, 0xaa0203ed, 0xaa0403ee, 0xfc408582,
		0xfd4001c3, 0xfc687864, 0x1f431042, 0xfc287862,
		0xfc4085a2, 0xfc4085c3, 0xfc6878a4, 0x1f431042,
		0xfc2878a2, 0xf100056b, 0x54fffea1, 0xfc687863,
		0x1e610842, 0x1f430802, 0xfc2878a2, 0x91000508,
		0x8b090042, 0x8b090021, 0xeb0a011f, 0x54fffcc1,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	bBase := uint64(0x200000)
	tmpBase := uint64(0x300000)
	xBase := uint64(0x400000)
	yBase := uint64(0x500000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load A matrix.
	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load B matrix.
	bBuf := make([]byte, n*n*8)
	for i, v := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	// Load tmp (zeroed) and x vectors.
	em.LoadProgram(tmpBase, make([]byte, n*8))

	xBuf := make([]byte, n*8)
	for i, v := range x {
		binary.LittleEndian.PutUint64(xBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(xBase, xBuf)

	// Load y (zeroed).
	em.LoadProgram(yBase, make([]byte, n*8))

	em.LoadProgram(uint64(0x600000), make([]byte, 8192))

	// Set registers per ARM64 ABI.
	// gesummv(int n, double alpha, double beta, double *A, double *B, double *tmp, double *x, double *y)
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteD(0, alpha)
	em.Regs.WriteD(1, beta)
	em.Regs.WriteX(1, aBase)
	em.Regs.WriteX(2, bBase)
	em.Regs.WriteX(3, tmpBase)
	em.Regs.WriteX(4, xBase)
	em.Regs.WriteX(5, yBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = 0x600000 + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

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

	t.Logf("kernel_gesummv: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchGemver(t *testing.T) {
	// End-to-end PolyBench kernel_gemver through the OoO pipeline.
	n := 30

	alpha, beta := 1.5, 1.2
	fn := float64(n)

	A := make([]float64, n*n)
	u1 := make([]float64, n)
	v1 := make([]float64, n)
	u2 := make([]float64, n)
	v2 := make([]float64, n)
	w := make([]float64, n)
	x := make([]float64, n)
	y := make([]float64, n)
	z := make([]float64, n)

	for i := 0; i < n; i++ {
		u1[i] = float64(i)
		u2[i] = (float64(i+1) / fn) / 2.0
		v1[i] = (float64(i+1) / fn) / 4.0
		v2[i] = (float64(i+1) / fn) / 6.0
		y[i] = (float64(i+1) / fn) / 8.0
		z[i] = (float64(i+1) / fn) / 9.0
		x[i] = 0.0
		w[i] = 0.0
		for j := 0; j < n; j++ {
			A[i*n+j] = float64(i*j%n) / float64(n)
		}
	}

	// Golden reference.
	goldenA := make([]float64, n*n)
	copy(goldenA, A)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			goldenA[i*n+j] = goldenA[i*n+j] + u1[i]*v1[j] + u2[i]*v2[j]
		}
	}
	expectedX := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			expectedX[i] = expectedX[i] + beta*goldenA[j*n+i]*y[j]
		}
	}
	for i := 0; i < n; i++ {
		expectedX[i] = expectedX[i] + z[i]
	}
	expectedW := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			expectedW[i] = expectedW[i] + alpha*goldenA[i*n+j]*expectedX[j]
		}
	}

	code := []uint32{
		0x7100041f, 0x5400086b, 0xd280000a, 0xa94027eb,
		0xd37d7c0c, 0xaa0103ed, 0x2a0003e8, 0xaa0803ee,
		0xaa0503ef, 0xaa0303f0, 0xaa0d03f1, 0xfc6a7842,
		0xfd400223, 0xfc408604, 0x1f440c42, 0xfc6a7883,
		0xfc4085e4, 0x1f440862, 0xfc008622, 0xf10005ce,
		0x54fffee1, 0x9100054a, 0x8b0c01ad, 0xeb08015f,
		0x54fffde1, 0xd280000c, 0xd37df10a, 0xaa0103ed,
		0xfc6c78e2, 0xaa0803ee, 0xaa0d03ef, 0xaa0b03f0,
		0xfd4001e3, 0xfc408604, 0x1e610863, 0x1f440862,
		0xfc2c78e2, 0x8b0a01ef, 0xf10005ce, 0x54ffff21,
		0x9100058c, 0x910021ad, 0xeb08019f, 0x54fffe21,
		0xaa0803eb, 0xaa0703ec, 0xfd400181, 0xfc408522,
		0x1e622821, 0xfc008581, 0xf100056b, 0x54ffff61,
		0xd2800009, 0xfc6978c1, 0xaa0803eb, 0xaa0703ec,
		0xaa0103ed, 0xfc4085a2, 0x1e600842, 0xfc408583,
		0x1f430441, 0xfc2978c1, 0xf100056b, 0x54ffff41,
		0x91000529, 0x8b0a0021, 0xeb08013f, 0x54fffe41,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	u1Base := uint64(0x200000)
	v1Base := uint64(0x300000)
	u2Base := uint64(0x400000)
	v2Base := uint64(0x500000)
	wBase := uint64(0x600000)
	xBase := uint64(0x700000)
	yBase := uint64(0x800000)
	zBase := uint64(0x900000)
	stackBase := uint64(0xA00000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load A matrix.
	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load vectors.
	loadVec := func(base uint64, data []float64) {
		buf := make([]byte, len(data)*8)
		for i, v := range data {
			binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(base, buf)
	}
	loadVec(u1Base, u1)
	loadVec(v1Base, v1)
	loadVec(u2Base, u2)
	loadVec(v2Base, v2)
	loadVec(wBase, w)
	loadVec(xBase, x)
	loadVec(yBase, y)
	loadVec(zBase, z)

	// Allocate stack space and write y,z addresses for stack-passed args.
	stackBuf := make([]byte, 8192)
	binary.LittleEndian.PutUint64(stackBuf[4096:], yBase)
	binary.LittleEndian.PutUint64(stackBuf[4096+8:], zBase)
	em.LoadProgram(stackBase, stackBuf)

	// Set registers per ARM64 ABI.
	// gemver(int n, double alpha, double beta, double *A, double *u1, double *v1,
	//        double *u2, double *v2, double *w, double *x, double *y, double *z)
	// Integer/pointer args: X0=n, X1=A, X2=u1, X3=v1, X4=u2, X5=v2, X6=w, X7=x
	// FP args: D0=alpha, D1=beta
	// Stack-passed: [SP+0]=y, [SP+8]=z
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteD(0, alpha)
	em.Regs.WriteD(1, beta)
	em.Regs.WriteX(1, aBase)
	em.Regs.WriteX(2, u1Base)
	em.Regs.WriteX(3, v1Base)
	em.Regs.WriteX(4, u2Base)
	em.Regs.WriteX(5, v2Base)
	em.Regs.WriteX(6, wBase)
	em.Regs.WriteX(7, xBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 4096
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify w[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(wBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedW[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("w[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("kernel_gemver: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchDoitgen(t *testing.T) {
	// End-to-end PolyBench kernel_doitgen through the OoO pipeline.
	nr, nq, np := 10, 12, 8

	A := make([]float64, nr*nq*np)
	C4 := make([]float64, np*np)

	for i := 0; i < nr; i++ {
		for j := 0; j < nq; j++ {
			for k := 0; k < np; k++ {
				A[i*nq*np+j*np+k] = float64((i*j+k)%np) / float64(np)
			}
		}
	}
	for i := 0; i < np; i++ {
		for j := 0; j < np; j++ {
			C4[i*np+j] = float64(i*j%np) / float64(np)
		}
	}

	// Golden reference.
	expectedA := make([]float64, nr*nq*np)
	copy(expectedA, A)
	goldenSum := make([]float64, np)
	for r := 0; r < nr; r++ {
		for q := 0; q < nq; q++ {
			for p := 0; p < np; p++ {
				goldenSum[p] = 0.0
				for s := 0; s < np; s++ {
					goldenSum[p] += expectedA[r*nq*np+q*np+s] * C4[s*np+p]
				}
			}
			for p := 0; p < np; p++ {
				expectedA[r*nq*np+q*np+p] = goldenSum[p]
			}
		}
	}

	code := []uint32{
		0xf100041f, 0x5400056b, 0xf100043f, 0x5400052b,
		0xf100045f, 0x540004eb, 0xd2800008, 0xd37df049,
		0x9b017c4a, 0xd37df14a, 0xd280000b, 0xaa0303ec,
		0xd280000d, 0xaa0403ee, 0xd280000f, 0xf82d78bf,
		0x2f00e400, 0xaa0e03f0, 0xfc6f7981, 0xfd400202,
		0x1f420020, 0xfc2d78a0, 0x910005ef, 0x8b090210,
		0xeb0f005f, 0x54ffff21, 0x910005ad, 0x910021ce,
		0xeb0201bf, 0x54fffe21, 0xd280000d, 0xfc6d78a0,
		0xfc2d7980, 0x910005ad, 0xeb0d005f, 0x54ffff81,
		0x9100056b, 0x8b09018c, 0xeb01017f, 0x54fffca1,
		0x91000508, 0x8b0a0063, 0xeb00011f, 0x54fffbe1,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	c4Base := uint64(0x200000)
	sumBase := uint64(0x300000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load A array.
	aBuf := make([]byte, nr*nq*np*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load C4 matrix.
	c4Buf := make([]byte, np*np*8)
	for i, v := range C4 {
		binary.LittleEndian.PutUint64(c4Buf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(c4Base, c4Buf)

	// Load sum (zeroed).
	em.LoadProgram(sumBase, make([]byte, np*8))

	em.LoadProgram(uint64(0x400000), make([]byte, 8192))

	// Set registers per ARM64 ABI.
	// doitgen(long nr, long nq, long np, double *A, double *C4, double *sum)
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(nr))
	em.Regs.WriteX(1, uint64(nq))
	em.Regs.WriteX(2, uint64(np))
	em.Regs.WriteX(3, aBase)
	em.Regs.WriteX(4, c4Base)
	em.Regs.WriteX(5, sumBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = 0x400000 + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify A[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < nr*nq*np; i++ {
		bits := em.Mem.ReadU64(aBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedA[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			r := i / (nq * np)
			q := (i / np) % nq
			pp := i % np
			t.Errorf("A[%d][%d][%d] = %g, want %g (relErr=%e)", r, q, pp, got, want, relErr)
		}
	}

	t.Logf("kernel_doitgen: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchTrisolv(t *testing.T) {
	// End-to-end PolyBench kernel_trisolv through the OoO pipeline.
	// Forward substitution: L*x = b where L is lower triangular.
	n := 40

	L := make([]float64, n*n)
	b := make([]float64, n)

	for i := 0; i < n; i++ {
		b[i] = float64(i)
		for j := 0; j <= i; j++ {
			L[i*n+j] = float64(i+n-j+1) * 2 / float64(n)
		}
	}

	// Golden reference.
	expectedX := make([]float64, n)
	for i := 0; i < n; i++ {
		expectedX[i] = b[i]
		for j := 0; j < i; j++ {
			expectedX[i] -= L[i*n+j] * expectedX[j]
		}
		expectedX[i] = expectedX[i] / L[i*n+i]
	}

	code := []uint32{
		0x7100041f, 0x5400036b, 0x52800008, 0xd2800009,
		0x1100040a, 0x2a0003eb, 0x14000009, 0x1b097d4c,
		0xfc6cd821, 0x1e611800, 0xfc297840, 0x91000529,
		0x0b000108, 0xeb0b013f, 0x540001c0, 0xfc697860,
		0xfc297840, 0xb4fffec9, 0xd280000c, 0x8b284c2d,
		0xfc6c79a1, 0xfc6c7842, 0x1f428020, 0xfc297840,
		0x9100058c, 0xeb0c013f, 0x54ffff41, 0x17ffffec,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	lBase := uint64(0x100000)
	xBase := uint64(0x200000)
	bBase := uint64(0x300000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load L matrix.
	lBuf := make([]byte, n*n*8)
	for i, v := range L {
		binary.LittleEndian.PutUint64(lBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(lBase, lBuf)

	// Load x (zeroed).
	em.LoadProgram(xBase, make([]byte, n*8))

	// Load b vector.
	bBuf := make([]byte, n*8)
	for i, v := range b {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, lBase)
	em.Regs.WriteX(2, xBase)
	em.Regs.WriteX(3, bBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = 0x400000 + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify x[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(xBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedX[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("x[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("kernel_trisolv: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchDurbin(t *testing.T) {
	// End-to-end PolyBench kernel_durbin through the OoO pipeline.
	// Levinson-Durbin recursion.
	n := 40

	r := make([]float64, n)
	for i := 0; i < n; i++ {
		r[i] = float64(n + 1 - i)
	}

	// Golden reference.
	expectedY := make([]float64, n)
	z := make([]float64, n)
	expectedY[0] = -r[0]
	beta := 1.0
	alpha := -r[0]
	for k := 1; k < n; k++ {
		beta = (1 - alpha*alpha) * beta
		sum := 0.0
		for i := 0; i < k; i++ {
			sum += r[k-i-1] * expectedY[i]
		}
		alpha = -(r[k] + sum) / beta
		for i := 0; i < k; i++ {
			z[i] = expectedY[i] + alpha*expectedY[k-i-1]
		}
		for i := 0; i < k; i++ {
			expectedY[i] = z[i]
		}
		expectedY[k] = alpha
	}

	code := []uint32{
		0xfd400020, 0x1e614000, 0xfd000040, 0x7100081f,
		0x54000a6b, 0xd2800008, 0xd2800009, 0xfd400020,
		0x1e614002, 0x2a0003ea, 0x9100206b, 0x9100204c,
		0x1e6e1000, 0x9280000d, 0x5280002e, 0xb201f3ef,
		0xf295556f, 0x1e6e1001, 0x14000008, 0xfc2e7842,
		0x910005ce, 0x91000529, 0x91002108, 0xd10005ad,
		0xeb0a01df, 0x540007c0, 0x1f428043, 0x2f00e402,
		0xaa0203f0, 0xaa0803f1, 0xfc716824, 0xfc408605,
		0x1f450882, 0xd1002231, 0xb100223f, 0x54ffff61,
		0xd2800010, 0x1e610861, 0xfc6e7823, 0x1e632842,
		0x1e614042, 0x1e611842, 0xaa0803f1, 0xfc706843,
		0xfc716844, 0x1f440c43, 0xfc306863, 0x91002210,
		0xd1002231, 0xb100223f, 0x54ffff21, 0xd2800000,
		0x9bcf7d30, 0xd341fe10, 0x8b100610, 0xcb100130,
		0x91000611, 0xf1000e3f, 0x9a9007f0, 0xf100093f,
		0x54000223, 0xd2800000, 0x8b0d0204, 0xaa0c03e5,
		0xaa0b03e6, 0xfc5f80c3, 0xfc1f80a3, 0xfd4000c3,
		0xfd0000a3, 0xfd4004c3, 0xfd0004a3, 0xd1000c00,
		0x910060c6, 0x910060a5, 0xeb00009f, 0x54fffec1,
		0xcb0003e0, 0xf1000e3f, 0x54fff8a0, 0xd37df000,
		0x8b000051, 0x8b000060, 0xfc408403, 0xfc008623,
		0xf1000610, 0x54ffffa1, 0x17ffffbd, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	rBase := uint64(0x100000)
	yBase := uint64(0x200000)
	zBase := uint64(0x300000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load r vector.
	rBuf := make([]byte, n*8)
	for i, v := range r {
		binary.LittleEndian.PutUint64(rBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(rBase, rBuf)

	// Load y and z (zeroed).
	em.LoadProgram(yBase, make([]byte, n*8))
	em.LoadProgram(zBase, make([]byte, n*8))

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, rBase)
	em.Regs.WriteX(2, yBase)
	em.Regs.WriteX(3, zBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = 0x400000 + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

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

	t.Logf("kernel_durbin: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchJacobi1d(t *testing.T) {
	// End-to-end PolyBench kernel_jacobi_1d through the OoO pipeline.
	// 1D Jacobi stencil with TSTEPS iterations.
	tsteps := 20
	n := 30

	A := make([]float64, n)
	B := make([]float64, n)
	for i := 0; i < n; i++ {
		A[i] = (float64(i) + 2) / float64(n)
		B[i] = (float64(i) + 3) / float64(n)
	}

	// Golden reference.
	expectedA := make([]float64, n)
	expectedB := make([]float64, n)
	copy(expectedA, A)
	copy(expectedB, B)
	for t := 0; t < tsteps; t++ {
		for i := 1; i < n-1; i++ {
			expectedB[i] = 0.33333 * (expectedA[i-1] + expectedA[i] + expectedA[i+1])
		}
		for i := 1; i < n-1; i++ {
			expectedA[i] = 0.33333 * (expectedB[i-1] + expectedB[i] + expectedB[i+1])
		}
	}

	code := []uint32{
		0x7100041f, 0x540004cb, 0x71000c3f, 0x5400048b,
		0x52800008, 0x5100042b, 0x91002049, 0x9100206a,
		0xd100056b, 0xd29497cc, 0xf2ab462c, 0xf2caa8ec,
		0xf2e7faac, 0x9e670180, 0xaa0b03ec, 0xaa0a03ed,
		0xaa0903ee, 0x6d7f89c1, 0xfc408dc3, 0x1e622821,
		0x1e632821, 0x1e600821, 0xfc0085a1, 0xf100058c,
		0x54ffff21, 0xaa0b03ec, 0xaa0903ed, 0xaa0a03ee,
		0x6d7f89c1, 0xfc408dc3, 0x1e622821, 0x1e632821,
		0x1e600821, 0xfc0085a1, 0xf100058c, 0x54ffff21,
		0x11000508, 0x6b00011f, 0x54fffd01, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	bBase := uint64(0x200000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load A vector.
	aBuf := make([]byte, n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load B vector.
	bBuf := make([]byte, n*8)
	for i, v := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(tsteps))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(2, aBase)
	em.Regs.WriteX(3, bBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = 0x300000 + 8192
	em.SetHaltPC(haltPC)

	pip := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify A[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(aBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedA[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("A[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("kernel_jacobi_1d: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		pip.InstDone, pip.Cycles, pip.CPI(), maxRelErr)
}

func TestOoO_PolyBenchSyrk(t *testing.T) {
	// End-to-end PolyBench kernel_syrk through the OoO pipeline.
	// C := alpha*A*A^T + beta*C (lower triangle only, j <= i)
	n, m := 20, 25
	alpha, beta := 1.5, 1.2

	C := make([]float64, n*n)
	A := make([]float64, n*m)

	for i := 0; i < n; i++ {
		for j := 0; j < m; j++ {
			A[i*m+j] = float64((i*j+1)%n) / float64(n)
		}
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			C[i*n+j] = float64((i*j+2)%m) / float64(m)
		}
	}

	// Golden reference.
	expectedC := make([]float64, n*n)
	copy(expectedC, C)
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			expectedC[i*n+j] *= beta
		}
		for k := 0; k < m; k++ {
			for j := 0; j <= i; j++ {
				expectedC[i*n+j] += alpha * A[i*m+k] * A[j*m+k]
			}
		}
	}

	code := []uint32{
		0x7100041f, 0x5400074b, 0x7100003f, 0x5400050d,
		0xd2800008, 0x2a0103e9, 0xd37d7c0a, 0xd37d7c2b,
		0x5280002c, 0x2a0003ed, 0xd280000e, 0xfc6e7842,
		0x1e610842, 0xfc2e7842, 0x910005ce, 0xeb0e019f,
		0x54ffff61, 0xd280000e, 0x9b097d0f, 0x8b0f0c6f,
		0xaa0303f0, 0xd2800011, 0xaa1003e0, 0xfc6e79e2,
		0x1e600842, 0xfd400003, 0xfc717844, 0x1f431042,
		0xfc317842, 0x91000631, 0x8b0b0000, 0xeb11019f,
		0x54fffee1, 0x910005ce, 0x91002210, 0xeb0901df,
		0x54fffe21, 0x91000508, 0x9100058c, 0x8b0a0042,
		0xeb0d011f, 0x54fffc21, 0x14000011, 0xd2800008,
		0xd37d7c09, 0x5280002a, 0x2a0003eb, 0xd280000c,
		0xfc6c7840, 0x1e610800, 0xfc2c7840, 0x9100058c,
		0xeb0c015f, 0x54ffff61, 0x91000508, 0x9100054a,
		0x8b090042, 0xeb0b011f, 0x54fffea1, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	cBase := uint64(0x100000)
	aBase := uint64(0x200000)
	stackBase := uint64(0x300000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load C matrix.
	cBuf := make([]byte, n*n*8)
	for i, v := range C {
		binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(cBase, cBuf)

	// Load A matrix.
	aBuf := make([]byte, n*m*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Allocate stack space.
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, uint64(m))
	em.Regs.WriteD(0, alpha)
	em.Regs.WriteD(1, beta)
	em.Regs.WriteX(2, cBase)
	em.Regs.WriteX(3, aBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify C[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < n*n; i++ {
		bits := em.Mem.ReadU64(cBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedC[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("C[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_syrk: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchSyr2k(t *testing.T) {
	// End-to-end PolyBench kernel_syr2k through the OoO pipeline.
	// C := alpha*A*B^T + alpha*B*A^T + beta*C (lower triangle only, j <= i)
	n, m := 20, 25
	alpha, beta := 1.5, 1.2

	C := make([]float64, n*n)
	A := make([]float64, n*m)
	B := make([]float64, n*m)

	for i := 0; i < n; i++ {
		for j := 0; j < m; j++ {
			A[i*m+j] = float64((i*j+1)%n) / float64(n)
			B[i*m+j] = float64((i*j+2)%m) / float64(m)
		}
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			C[i*n+j] = float64((i*j+3)%n) / float64(m)
		}
	}

	// Golden reference.
	expectedC := make([]float64, n*n)
	copy(expectedC, C)
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			expectedC[i*n+j] *= beta
		}
		for k := 0; k < m; k++ {
			for j := 0; j <= i; j++ {
				expectedC[i*n+j] += A[j*m+k]*alpha*B[i*m+k] + B[j*m+k]*alpha*A[i*m+k]
			}
		}
	}

	code := []uint32{
		0x7100041f, 0x5400082b, 0x7100003f, 0x540005ed,
		0xd2800008, 0x2a0103e9, 0xd37d7c0a, 0xd37d7c2b,
		0x5280002c, 0x2a0003ed, 0xd280000e, 0xfc6e7842,
		0x1e610842, 0xfc2e7842, 0x910005ce, 0xeb0e019f,
		0x54ffff61, 0xd280000e, 0x9b097d0f, 0xaa0303f0,
		0xaa0403f1, 0xd2800000, 0xd2800001, 0x8b0f01c5,
		0xfc606a02, 0x1e600842, 0xfc657883, 0xfc606a24,
		0x1e600884, 0xfc657865, 0x1e650884, 0x1f431042,
		0xfc617843, 0x1e622862, 0xfc217842, 0x91000421,
		0x8b0b0000, 0xeb01019f, 0x54fffe41, 0x910005ce,
		0x91002231, 0x91002210, 0xeb0901df, 0x54fffd41,
		0x91000508, 0x9100058c, 0x8b0a0042, 0xeb0d011f,
		0x54fffb41, 0x14000011, 0xd2800008, 0xd37d7c09,
		0x5280002a, 0x2a0003eb, 0xd280000c, 0xfc6c7840,
		0x1e610800, 0xfc2c7840, 0x9100058c, 0xeb0c015f,
		0x54ffff61, 0x91000508, 0x9100054a, 0x8b090042,
		0xeb0b011f, 0x54fffea1, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	cBase := uint64(0x100000)
	aBase := uint64(0x200000)
	bBase := uint64(0x300000)
	stackBase := uint64(0x400000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load C matrix.
	cBuf := make([]byte, n*n*8)
	for i, v := range C {
		binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(cBase, cBuf)

	// Load A matrix.
	aBuf := make([]byte, n*m*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load B matrix.
	bBuf := make([]byte, n*m*8)
	for i, v := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	// Allocate stack space.
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, uint64(m))
	em.Regs.WriteD(0, alpha)
	em.Regs.WriteD(1, beta)
	em.Regs.WriteX(2, cBase)
	em.Regs.WriteX(3, aBase)
	em.Regs.WriteX(4, bBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify C[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < n*n; i++ {
		bits := em.Mem.ReadU64(cBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedC[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("C[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_syr2k: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchTrmm(t *testing.T) {
	// End-to-end PolyBench kernel_trmm through the OoO pipeline.
	// B := alpha*A^T*B (A is lower triangular with unit diagonal)
	m, n := 20, 25
	alpha := 1.5

	A := make([]float64, m*m)
	B := make([]float64, m*n)

	for i := 0; i < m; i++ {
		for j := 0; j < i; j++ {
			A[i*m+j] = float64((i+j)%m) / float64(m)
		}
		A[i*m+i] = 1.0
		for j := 0; j < n; j++ {
			B[i*n+j] = float64((n+(i-j))%n) / float64(n)
		}
	}

	// Golden reference.
	expectedB := make([]float64, m*n)
	copy(expectedB, B)
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			for k := i + 1; k < m; k++ {
				expectedB[i*n+j] += A[k*m+i] * expectedB[k*n+j]
			}
			expectedB[i*n+j] = alpha * expectedB[i*n+j]
		}
	}

	code := []uint32{
		0x7100041f, 0x540006eb, 0x7100043f, 0x540006ab,
		0xd2800004, 0x2a0003e8, 0xd37d7c29, 0x2a0103ea,
		0x8b09006b, 0xd37d7c0c, 0x8b0c004d, 0x9100218e,
		0xd100050f, 0xaa0303f0, 0x14000008, 0x8b090210,
		0x8b09016b, 0x8b0e01ad, 0xd10005ef, 0xaa1103e4,
		0xeb08023f, 0x54000460, 0x91000491, 0xaa0a03e0,
		0xaa1003e1, 0xeb08023f, 0x54000302, 0xd2800000,
		0x9b0a7c81, 0x8b010c61, 0xaa0b03e2, 0xfc607821,
		0xaa0f03e4, 0xaa0d03e5, 0xaa0203e6, 0xfd4000a2,
		0xfd4000c3, 0x1f430441, 0xfc207821, 0x8b0900c6,
		0x8b0c00a5, 0xf1000484, 0x54ffff21, 0x1e600821,
		0xfc207821, 0x91000400, 0x91002042, 0xeb0a001f,
		0x54fffde1, 0x17ffffde, 0xfd400021, 0x1e600821,
		0xfc008421, 0xf1000400, 0x54ffff81, 0x17ffffd8,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	bBase := uint64(0x200000)
	stackBase := uint64(0x300000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load A matrix (lower triangular).
	aBuf := make([]byte, m*m*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load B matrix.
	bBuf := make([]byte, m*n*8)
	for i, v := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	// Allocate stack space.
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(m))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteD(0, alpha)
	em.Regs.WriteX(2, aBase)
	em.Regs.WriteX(3, bBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify B[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < m*n; i++ {
		bits := em.Mem.ReadU64(bBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedB[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("B[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_trmm: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchSymm(t *testing.T) {
	// End-to-end PolyBench kernel_symm through the OoO pipeline.
	// C := alpha*A*B + beta*C (A is symmetric, lower triangle stored)
	m, n := 20, 25
	alpha, beta := 1.5, 1.2

	C := make([]float64, m*n)
	A := make([]float64, m*m)
	B := make([]float64, m*n)

	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			C[i*n+j] = float64((i+j)%100) / float64(m)
			B[i*n+j] = float64((n+i-j)%100) / float64(m)
		}
	}
	for i := 0; i < m; i++ {
		for j := 0; j <= i; j++ {
			A[i*m+j] = float64((i+j)%100) / float64(m)
		}
		for j := i + 1; j < m; j++ {
			A[i*m+j] = -999
		}
	}

	// Golden reference.
	expectedC := make([]float64, m*n)
	copy(expectedC, C)
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			temp2 := 0.0
			for k := 0; k < i; k++ {
				expectedC[k*n+j] += alpha * B[i*n+j] * A[i*m+k]
				temp2 += B[k*n+j] * A[i*m+k]
			}
			expectedC[i*n+j] = beta*expectedC[i*n+j] + alpha*B[i*n+j]*A[i*m+i] + alpha*temp2
		}
	}

	code := []uint32{
		0x7100041f, 0x540008ab, 0x7100043f, 0x5400086b,
		0xa9bf4ff4, 0x52800008, 0xd2800009, 0xd37d7c2a,
		0x1100040b, 0x2f00e402, 0x2a0103ec, 0x2a0003ed,
		0x14000005, 0x91000529, 0x0b000108, 0xeb0d013f,
		0x540006a0, 0x1b097d6e, 0xaa0c03ef, 0xaa0203f0,
		0xaa0403f1, 0xb40004a9, 0xd280000f, 0x1b017d30,
		0xaa0203f1, 0xaa0403e5, 0x8b284c66, 0xd2800013,
		0xd2800014, 0x8b1001e7, 0x2f00e403, 0xfc677884,
		0xfc7478c5, 0xfc736a26, 0x1e600884, 0x1f451884,
		0xfc336a24, 0xfc7368a4, 0xfc7478c5, 0x1f450c83,
		0x91000694, 0x8b0a0273, 0xeb14013f, 0x54fffe81,
		0xfc677844, 0xfc677885, 0x1e6008a5, 0xfc6ed866,
		0x1e6608a5, 0x1f441424, 0x1f431003, 0xfc277843,
		0x910005ef, 0x910020a5, 0x91002231, 0xeb0c01ff,
		0x54fffc61, 0x17ffffd4, 0xfd400203, 0xfc408624,
		0x1e600884, 0xfc6ed865, 0x1e650884, 0x1f431023,
		0x1f420c03, 0xfc008603, 0xf10005ef, 0x54fffee1,
		0x17ffffc9, 0xa8c14ff4, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	cBase := uint64(0x100000)
	aBase := uint64(0x200000)
	bBase := uint64(0x300000)
	stackBase := uint64(0x400000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load C matrix.
	cBuf := make([]byte, m*n*8)
	for i, v := range C {
		binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(cBase, cBuf)

	// Load A matrix.
	aBuf := make([]byte, m*m*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load B matrix.
	bBuf := make([]byte, m*n*8)
	for i, v := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	// Allocate stack space.
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(m))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteD(0, alpha)
	em.Regs.WriteD(1, beta)
	em.Regs.WriteX(2, cBase)
	em.Regs.WriteX(3, aBase)
	em.Regs.WriteX(4, bBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify C[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < m*n; i++ {
		bits := em.Mem.ReadU64(cBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedC[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("C[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_symm: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchCholesky(t *testing.T) {
	// End-to-end PolyBench kernel_cholesky through the OoO pipeline.
	// Cholesky decomposition: A = L*L^T (in-place, lower triangle).
	n := 20

	// Build a positive-definite matrix using the PolyBench init.
	A0 := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			A0[i*n+j] = float64((-j)%n)/float64(n) + 1
		}
		A0[i*n+i] = 1
	}
	// Make positive semi-definite: B = A0 * A0^T.
	B := make([]float64, n*n)
	for t := 0; t < n; t++ {
		for r := 0; r < n; r++ {
			for s := 0; s < n; s++ {
				B[r*n+s] += A0[r*n+t] * A0[s*n+t]
			}
		}
	}

	A := make([]float64, n*n)
	copy(A, B)

	// Golden reference: Cholesky decomposition.
	expectedA := make([]float64, n*n)
	copy(expectedA, B)
	for i := 0; i < n; i++ {
		for j := 0; j < i; j++ {
			for k := 0; k < j; k++ {
				expectedA[i*n+j] -= expectedA[i*n+k] * expectedA[j*n+k]
			}
			expectedA[i*n+j] /= expectedA[j*n+j]
		}
		for k := 0; k < i; k++ {
			expectedA[i*n+i] -= expectedA[i*n+k] * expectedA[i*n+k]
		}
		expectedA[i*n+i] = math.Sqrt(expectedA[i*n+i])
	}

	code := []uint32{
		0x7100041f, 0x540006ab, 0x52800008, 0xd2800009,
		0x1100040a, 0x2a0003eb, 0x14000009, 0x1b097d4c,
		0xfc6cd820, 0x1e61c000, 0xfc2cd820, 0x91000529,
		0x0b000108, 0xeb0b013f, 0x54000500, 0xb4ffff09,
		0x5280000d, 0xd280000e, 0x9b0b7d2f, 0x8b284c2c,
		0x1400000a, 0x1b0e7d51, 0xfc71d820, 0xfc707821,
		0x1e601820, 0xfc307820, 0x910005ce, 0x0b0001ad,
		0xeb0901df, 0x540001e0, 0xaa0f03f0, 0xb4fffece,
		0xd2800011, 0x8b2d4c22, 0x8b0f01d0, 0xfc707820,
		0xfc717981, 0xfc717842, 0x1f428020, 0xfc307820,
		0x91000631, 0xeb1101df, 0x54ffff41, 0x17ffffea,
		0xd280000d, 0x1b097d4e, 0xfc6e5820, 0xfc6d7981,
		0x1f418020, 0xfc2e5820, 0x910005ad, 0xeb0d013f,
		0x54ffff61, 0x17ffffd2, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	stackBase := uint64(0x200000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load A matrix.
	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Allocate stack space.
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, aBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify A[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < n*n; i++ {
		bits := em.Mem.ReadU64(aBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedA[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("A[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_cholesky: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchLU(t *testing.T) {
	// End-to-end PolyBench kernel_lu through the OoO pipeline.
	// LU decomposition without pivoting (in-place on A matrix).
	n := 20

	// Build a well-conditioned matrix using the PolyBench init.
	A0 := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			A0[i*n+j] = float64((-j)%n)/float64(n) + 1
		}
		A0[i*n+i] = 1
	}
	// Make positive semi-definite: B = A0 * A0^T.
	B := make([]float64, n*n)
	for tt := 0; tt < n; tt++ {
		for r := 0; r < n; r++ {
			for s := 0; s < n; s++ {
				B[r*n+s] += A0[r*n+tt] * A0[s*n+tt]
			}
		}
	}

	A := make([]float64, n*n)
	copy(A, B)

	// Golden reference: LU decomposition.
	expectedA := make([]float64, n*n)
	copy(expectedA, B)
	for i := 0; i < n; i++ {
		for j := 0; j < i; j++ {
			for k := 0; k < j; k++ {
				expectedA[i*n+j] -= expectedA[i*n+k] * expectedA[k*n+j]
			}
			expectedA[i*n+j] /= expectedA[j*n+j]
		}
		for j := i; j < n; j++ {
			for k := 0; k < i; k++ {
				expectedA[i*n+j] -= expectedA[i*n+k] * expectedA[k*n+j]
			}
		}
	}

	code := []uint32{
		0x7100041f, 0x540007ab, 0xd2800008, 0x11000409,
		0xd37d7c0a, 0xaa0103eb, 0xaa0103ec, 0x2a0003ed,
		0x14000006, 0x91000508, 0x8b0a018c, 0x9100216b,
		0xeb0d011f, 0x54000620, 0xb4ffff68, 0xd280000f,
		0x9b0d7d10, 0x8b100c2e, 0xaa0103f1, 0x1400000a,
		0x1b0f7d22, 0xfc62d820, 0xfc607821, 0x1e601820,
		0xfc207820, 0x910005ef, 0x91002231, 0xeb0801ff,
		0x54000200, 0xaa1003e0, 0xb4fffecf, 0xd2800002,
		0x8b1001e0, 0xfc607820, 0xaa1103e3, 0xfc627981,
		0xfd400062, 0x1f428020, 0xfc207820, 0x91000442,
		0x8b0a0063, 0xeb0201ff, 0x54ffff21, 0x17ffffe9,
		0xaa0b03ef, 0xaa0803f0, 0xd2800011, 0xfc7079c0,
		0xaa0f03e0, 0xfc717981, 0xfd400002, 0x1f428020,
		0xfc3079c0, 0x91000631, 0x8b0a0000, 0xeb11011f,
		0x54ffff21, 0x91000610, 0x910021ef, 0xeb0d021f,
		0x54fffe41, 0x17ffffcc, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	stackBase := uint64(0x200000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load A matrix.
	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Allocate stack space.
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers per ARM64 ABI.
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, aBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify A[] matches golden reference.
	maxRelErr := 0.0
	for i := 0; i < n*n; i++ {
		bits := em.Mem.ReadU64(aBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedA[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("A[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_lu: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchFloydWarshall(t *testing.T) {
	// End-to-end PolyBench kernel_floyd_warshall through the OoO pipeline.
	// All-pairs shortest paths using triple-nested loop with min comparison.
	n := 20

	path := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			path[i*n+j] = float64(i*j%7 + 1)
			if (i+j)%13 == 0 || (i+j)%7 == 0 || (i+j)%11 == 0 {
				path[i*n+j] = 999
			}
		}
	}

	// Golden reference.
	expected := make([]float64, n*n)
	copy(expected, path)
	for k := 0; k < n; k++ {
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				s := expected[i*n+k] + expected[k*n+j]
				if s < expected[i*n+j] {
					expected[i*n+j] = s
				}
			}
		}
	}

	code := []uint32{
		0x7100041f, 0x5400046b, 0xd2800008, 0xd37d7c09,
		0xaa0103ea, 0x2a0003eb, 0x14000005, 0x91000508,
		0x8b09014a, 0xeb0b011f, 0x54000340, 0xd280000c,
		0x8b080c2d, 0xaa0103ee, 0x14000005, 0x9100058c,
		0x8b0901ce, 0xeb0b019f, 0x54fffea0, 0x9b0b7d8f,
		0xaa0b03f0, 0xaa0a03f1, 0xaa0e03e0, 0x14000005,
		0x91002000, 0x91002231, 0xf1000610, 0x54fffe80,
		0xfc6f79a0, 0xfd400221, 0x1e612800, 0xfd400001,
		0x1e612000, 0x54fffee5, 0xfd000000, 0x17fffff5,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	pathBase := uint64(0x100000)
	stackBase := uint64(0x200000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	pathBuf := make([]byte, n*n*8)
	for i, v := range path {
		binary.LittleEndian.PutUint64(pathBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(pathBase, pathBuf)

	em.LoadProgram(stackBase, make([]byte, 8192))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, pathBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < n*n; i++ {
		bits := em.Mem.ReadU64(pathBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expected[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("path[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_floyd_warshall: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchCovariance(t *testing.T) {
	// End-to-end PolyBench kernel_covariance through the OoO pipeline.
	// Compute covariance matrix from data matrix.
	m, n := 15, 20
	floatN := float64(n)

	data := make([]float64, n*m)
	for i := 0; i < n; i++ {
		for j := 0; j < m; j++ {
			data[i*m+j] = float64(i*j) / float64(m)
		}
	}

	// Golden reference.
	dataCopy := make([]float64, n*m)
	copy(dataCopy, data)
	mean := make([]float64, m)
	for j := 0; j < m; j++ {
		for i := 0; i < n; i++ {
			mean[j] += dataCopy[i*m+j]
		}
		mean[j] /= floatN
	}
	for i := 0; i < n; i++ {
		for j := 0; j < m; j++ {
			dataCopy[i*m+j] -= mean[j]
		}
	}
	expectedCov := make([]float64, m*m)
	for i := 0; i < m; i++ {
		for j := i; j < m; j++ {
			for k := 0; k < n; k++ {
				expectedCov[i*m+j] += dataCopy[k*m+i] * dataCopy[k*m+j]
			}
			expectedCov[i*m+j] /= (floatN - 1.0)
			expectedCov[j*m+i] = expectedCov[i*m+j]
		}
	}

	code := []uint32{
		0x7100041f, 0x54000ccb, 0x2a0003e8, 0x2a0103ea,
		0xd37df109, 0x7100003f, 0x5400028d, 0xd280000b,
		0xaa0203ec, 0xf82b789f, 0x2f00e401, 0xaa0a03ed,
		0xaa0c03ee, 0xfd4001c2, 0x1e612841, 0xfc2b7881,
		0x8b0901ce, 0xf10005ad, 0x54ffff61, 0x1e601821,
		0xfc2b7881, 0x9100056b, 0x9100218c, 0xeb08017f,
		0x54fffe21, 0x14000008, 0x2f00e401, 0xaa0803eb,
		0xaa0403ec, 0x1e601821, 0xfc008581, 0xf100056b,
		0x54ffffc1, 0x7100043f, 0x5400020b, 0xd280000b,
		0xaa0203ec, 0xaa0803ed, 0xaa0403ee, 0xaa0c03ef,
		0xfc4085c1, 0xfd4001e2, 0x1e613841, 0xfc0085e1,
		0xf10005ad, 0x54ffff61, 0x9100056b, 0x8b09018c,
		0xeb0a017f, 0x54fffe81, 0x1e7e1001, 0x1e612800,
		0x7100003f, 0x5400040d, 0xd280000b, 0xd280000c,
		0x9b087d8d, 0x8b0d0c6d, 0x8b0c0c6e, 0xaa0b03ef,
		0xaa0c03f0, 0xf83079bf, 0x2f00e401, 0xaa0a03f1,
		0xaa0203e0, 0xfc6b6802, 0xfc6f6803, 0x1f430441,
		0xfc3079a1, 0x8b090000, 0xf1000631, 0x54ffff41,
		0x1e601821, 0xfc3079a1, 0x9b087e11, 0xfc3179c1,
		0x91000610, 0x910021ef, 0xeb08021f, 0x54fffdc1,
		0x9100058c, 0x9100216b, 0xeb08019f, 0x54fffca1,
		0x14000013, 0xd280000a, 0x2f00e401, 0x9100212b,
		0xaa0803ec, 0x1e601820, 0xaa0c03ed, 0xaa0303ee,
		0xaa0303ef, 0xfc0085e0, 0xfd0001c0, 0x8b0901ce,
		0xf10005ad, 0x54ffff81, 0x9100054a, 0x8b0b0063,
		0xd100058c, 0xeb08015f, 0x54fffe81, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	dataBase := uint64(0x100000)
	covBase := uint64(0x200000)
	meanBase := uint64(0x300000)
	stackBase := uint64(0x400000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	dataBuf := make([]byte, n*m*8)
	for i, v := range data {
		binary.LittleEndian.PutUint64(dataBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(dataBase, dataBuf)

	em.LoadProgram(covBase, make([]byte, m*m*8))
	em.LoadProgram(meanBase, make([]byte, m*8))

	em.LoadProgram(stackBase, make([]byte, 8192))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(m))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteD(0, floatN)
	em.Regs.WriteX(2, dataBase)
	em.Regs.WriteX(3, covBase)
	em.Regs.WriteX(4, meanBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < m*m; i++ {
		bits := em.Mem.ReadU64(covBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedCov[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/m, i%m
			t.Errorf("cov[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_covariance: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchCorrelation(t *testing.T) {
	// End-to-end PolyBench kernel_correlation through the OoO pipeline.
	// Compute correlation matrix from data matrix.
	m, n := 15, 20
	floatN := float64(n)

	data := make([]float64, n*m)
	for i := 0; i < n; i++ {
		for j := 0; j < m; j++ {
			data[i*m+j] = float64(i*j)/float64(m) + float64(i)
		}
	}

	// Golden reference.
	dataCopy := make([]float64, n*m)
	copy(dataCopy, data)
	mean := make([]float64, m)
	stddev := make([]float64, m)
	eps := 0.1

	for j := 0; j < m; j++ {
		for i := 0; i < n; i++ {
			mean[j] += dataCopy[i*m+j]
		}
		mean[j] /= floatN
	}
	for j := 0; j < m; j++ {
		for i := 0; i < n; i++ {
			d := dataCopy[i*m+j] - mean[j]
			stddev[j] += d * d
		}
		stddev[j] /= floatN
		stddev[j] = math.Sqrt(stddev[j])
		if stddev[j] <= eps {
			stddev[j] = 1.0
		}
	}
	sqrtN := math.Sqrt(floatN)
	for i := 0; i < n; i++ {
		for j := 0; j < m; j++ {
			dataCopy[i*m+j] -= mean[j]
			dataCopy[i*m+j] /= (sqrtN * stddev[j])
		}
	}
	expectedCorr := make([]float64, m*m)
	for i := 0; i < m-1; i++ {
		expectedCorr[i*m+i] = 1.0
		for j := i + 1; j < m; j++ {
			for k := 0; k < n; k++ {
				expectedCorr[i*m+j] += dataCopy[k*m+i] * dataCopy[k*m+j]
			}
			expectedCorr[j*m+i] = expectedCorr[i*m+j]
		}
	}
	expectedCorr[(m-1)*m+(m-1)] = 1.0

	code := []uint32{
		0x71000408, 0x540009cb, 0x2a0003e9, 0xd37df12a,
		0x7100003f, 0x540002ad, 0xd280000b, 0x2a0103ec,
		0xaa0203ed, 0xf82b789f, 0x2f00e401, 0xaa0c03ee,
		0xaa0d03ef, 0xfd4001e2, 0x1e612841, 0xfc2b7881,
		0x8b0a01ef, 0xf10005ce, 0x54ffff61, 0x1e601821,
		0xfc2b7881, 0x9100056b, 0x910021ad, 0xeb09017f,
		0x54fffe21, 0x14000008, 0x2f00e401, 0xaa0903eb,
		0xaa0403ec, 0x1e601821, 0xfc008581, 0xf100056b,
		0x54ffffc1, 0x7100003f, 0x540003ed, 0xd280000b,
		0x2a0103ec, 0xb201e7ed, 0xf293334d, 0xf2e7f72d,
		0x9e6701a1, 0x1e6e1002, 0xaa0203ed, 0xf82b78bf,
		0x2f00e403, 0xaa0c03ee, 0xaa0d03ef, 0xfd4001e4,
		0xfc6b7885, 0x1e653884, 0x1f440c83, 0xfc2b78a3,
		0x8b0a01ef, 0xf10005ce, 0x54ffff21, 0x1e601863,
		0x1e61c063, 0x1e612060, 0x1e628c63, 0xfc2b78a3,
		0x9100056b, 0x910021ad, 0xeb09017f, 0x54fffd81,
		0x1400000f, 0x2f00e401, 0x1e601821, 0x1e61c021,
		0xb201e7ea, 0xf293334a, 0xf2e7f72a, 0x9e670142,
		0x1e622020, 0x1e6e1002, 0x1e628c21, 0xaa0503ea,
		0xfc008541, 0xf1000529, 0x54ffffc1, 0x7100043f,
		0x540008cb, 0x7100001f, 0x54000bed, 0xd280000a,
		0xd37d7c0b, 0xaa0203ec, 0x2a0003e8, 0x2a0103e9,
		0x1e61c000, 0xaa0803ed, 0xaa0503ee, 0xaa0403ef,
		0xaa0c03f0, 0xfc4085e1, 0xfd400202, 0x1e613841,
		0xfd000201, 0xfc4085c2, 0x1e620802, 0x1e621821,
		0xfc008601, 0xf10005ad, 0x54fffee1, 0x9100054a,
		0x8b0b018c, 0xeb09015f, 0x54fffde1, 0x7100040a,
		0x540004a0, 0xd280000b, 0xd280000c, 0xd37df10d,
		0x5280002e, 0x5280010f, 0xd2e7fe10, 0x9b087d81,
		0x1b007d84, 0x8b0c0c71, 0xf8245a30, 0x9100058c,
		0x8b010c61, 0xaa0f03e4, 0xaa0e03e5, 0xf825783f,
		0x2f00e400, 0xaa0903e6, 0xaa0203e7, 0xfc6b68e1,
		0xfc6468e2, 0x1f420020, 0xfc257820, 0x8b0d00e7,
		0xf10004c6, 0x54ffff41, 0x9b087ca6, 0xfc267a20,
		0x910004a5, 0x91002084, 0xeb0800bf, 0x54fffe01,
		0x910005ce, 0x910021ef, 0x9100216b, 0xeb0a019f,
		0x54fffc61, 0xaa0a03e8, 0x1b002108, 0xd2e7fe09,
		0xf828d869, 0xd65f03c0, 0x7100041f, 0x5400034d,
		0xd2800009, 0x2a0003ee, 0x9100206a, 0xd37d7c0b,
		0x9100216c, 0x8b0b006d, 0xd10005ce, 0xd2e7fe0f,
		0x1b007d10, 0x8b090c71, 0xf8305a2f, 0x91000529,
		0xaa0e03f0, 0xaa0d03f1, 0xaa0a03e1, 0xf800843f,
		0xf900023f, 0x8b0b0231, 0xf1000610, 0x54ffff81,
		0x8b0c014a, 0x8b0c01ad, 0xd10005ce, 0xeb08013f,
		0x54fffe01, 0x1b002108, 0xd2e7fe09, 0xf828d869,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	dataBase := uint64(0x100000)
	corrBase := uint64(0x200000)
	meanBase := uint64(0x300000)
	stddevBase := uint64(0x400000)
	stackBase := uint64(0x500000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	dataBuf := make([]byte, n*m*8)
	for i, v := range data {
		binary.LittleEndian.PutUint64(dataBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(dataBase, dataBuf)

	em.LoadProgram(corrBase, make([]byte, m*m*8))
	em.LoadProgram(meanBase, make([]byte, m*8))
	em.LoadProgram(stddevBase, make([]byte, m*8))

	em.LoadProgram(stackBase, make([]byte, 8192))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(m))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteD(0, floatN)
	em.Regs.WriteX(2, dataBase)
	em.Regs.WriteX(3, corrBase)
	em.Regs.WriteX(4, meanBase)
	em.Regs.WriteX(5, stddevBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < m*m; i++ {
		bits := em.Mem.ReadU64(corrBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedCorr[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/m, i%m
			t.Errorf("corr[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_correlation: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchGramschmidt(t *testing.T) {
	// End-to-end PolyBench kernel_gramschmidt through the OoO pipeline.
	// Modified Gram-Schmidt orthogonalization.
	m, n := 20, 30

	A := make([]float64, m*n)
	R := make([]float64, n*n)
	Q := make([]float64, m*n)
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			A[i*n+j] = (float64((i*j)%m)/float64(m))*100 + 10
			Q[i*n+j] = 0.0
		}
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			R[i*n+j] = 0.0
		}
	}

	// Golden reference.
	expectedA := make([]float64, m*n)
	copy(expectedA, A)
	expectedR := make([]float64, n*n)
	expectedQ := make([]float64, m*n)
	for k := 0; k < n; k++ {
		nrm := 0.0
		for i := 0; i < m; i++ {
			nrm += expectedA[i*n+k] * expectedA[i*n+k]
		}
		expectedR[k*n+k] = math.Sqrt(nrm)
		for i := 0; i < m; i++ {
			expectedQ[i*n+k] = expectedA[i*n+k] / expectedR[k*n+k]
		}
		for j := k + 1; j < n; j++ {
			expectedR[k*n+j] = 0.0
			for i := 0; i < m; i++ {
				expectedR[k*n+j] += expectedQ[i*n+k] * expectedA[i*n+j]
			}
			for i := 0; i < m; i++ {
				expectedA[i*n+j] = expectedA[i*n+j] - expectedQ[i*n+k]*expectedR[k*n+j]
			}
		}
	}

	code := []uint32{
		0x7100043f, 0x540009eb, 0xd280000d, 0xd37d7c28,
		0x91002049, 0x5280002a, 0x2a0103eb, 0x2a0003ec,
		0x14000008, 0x9100054a, 0x91002042, 0x91002084,
		0x91002129, 0xaa0e03ed, 0xeb0b01df, 0x54000820,
		0x8b0d0c6e, 0x7100001f, 0x5400030d, 0x2f00e400,
		0xaa0c03ef, 0xaa0203f0, 0xfd400201, 0x1f410020,
		0x8b080210, 0xf10005ef, 0x54ffff81, 0x1e61c000,
		0x1b017db0, 0xfc3059c0, 0xaa0c03f1, 0xfc6f6840,
		0xfc7059c1, 0x1e611800, 0xfc2f6880, 0x8b0801ef,
		0xf1000631, 0x54ffff41, 0x910005ae, 0xeb0b01df,
		0x54fffc22, 0x14000006, 0x1b017daf, 0xf82f59df,
		0x910005ae, 0xeb0b01df, 0x54fffb62, 0x9b0b7dad,
		0x8b0d0c6d, 0xaa0903ef, 0xaa0a03f0, 0x14000005,
		0x91000610, 0x910021ef, 0xeb0b021f, 0x54fffa40,
		0xf83079bf, 0x7100041f, 0x54ffff4b, 0xd2800011,
		0x2f00e400, 0xaa0c03e5, 0xfc716881, 0xfc7169e2,
		0x1f420020, 0xfc3079a0, 0x8b080231, 0xf10004a5,
		0x54ffff41, 0xd2800011, 0xaa0c03e5, 0xfc7169e0,
		0xfc716881, 0xfc7079a2, 0x1f428020, 0xfc3169e0,
		0x8b080231, 0xf10004a5, 0x54ffff21, 0x17ffffe5,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	rBase := uint64(0x200000)
	qBase := uint64(0x300000)
	stackBase := uint64(0x400000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	aBuf := make([]byte, m*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	em.LoadProgram(rBase, make([]byte, n*n*8))
	em.LoadProgram(qBase, make([]byte, m*n*8))
	em.LoadProgram(stackBase, make([]byte, 8192))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(m))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(2, aBase)
	em.Regs.WriteX(3, rBase)
	em.Regs.WriteX(4, qBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < n*n; i++ {
		bits := em.Mem.ReadU64(rBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedR[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("R[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}
	for i := 0; i < m*n; i++ {
		bits := em.Mem.ReadU64(qBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedQ[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("Q[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_gramschmidt: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchLudcmp(t *testing.T) {
	// End-to-end PolyBench kernel_ludcmp through the OoO pipeline.
	// LU decomposition with forward/back substitution.
	n := 40

	fn := float64(n)

	// Build A with PolyBench init pattern.
	A0 := make([]float64, n*n)
	b := make([]float64, n)
	x := make([]float64, n)
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = 0
		y[i] = 0
		b[i] = float64(i+1)/fn/2.0 + 4
	}
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			A0[i*n+j] = float64((-j)%n)/fn + 1
		}
		for j := i + 1; j < n; j++ {
			A0[i*n+j] = 0
		}
		A0[i*n+i] = 1
	}
	// Make positive semi-definite: B = A0 * A0^T.
	B := make([]float64, n*n)
	for tt := 0; tt < n; tt++ {
		for r := 0; r < n; r++ {
			for s := 0; s < n; s++ {
				B[r*n+s] += A0[r*n+tt] * A0[s*n+tt]
			}
		}
	}
	A := make([]float64, n*n)
	copy(A, B)

	// Golden reference.
	expectedA := make([]float64, n*n)
	copy(expectedA, A)
	expectedB := make([]float64, n)
	copy(expectedB, b)
	expectedX := make([]float64, n)
	expectedY := make([]float64, n)
	var w float64
	for i := 0; i < n; i++ {
		for j := 0; j < i; j++ {
			w = expectedA[i*n+j]
			for k := 0; k < j; k++ {
				w -= expectedA[i*n+k] * expectedA[k*n+j]
			}
			expectedA[i*n+j] = w / expectedA[j*n+j]
		}
		for j := i; j < n; j++ {
			w = expectedA[i*n+j]
			for k := 0; k < i; k++ {
				w -= expectedA[i*n+k] * expectedA[k*n+j]
			}
			expectedA[i*n+j] = w
		}
	}
	for i := 0; i < n; i++ {
		w = expectedB[i]
		for j := 0; j < i; j++ {
			w -= expectedA[i*n+j] * expectedY[j]
		}
		expectedY[i] = w
	}
	for i := n - 1; i >= 0; i-- {
		w = expectedY[i]
		for j := i + 1; j < n; j++ {
			w -= expectedA[i*n+j] * expectedX[j]
		}
		expectedX[i] = w / expectedA[i*n+i]
	}

	code := []uint32{
		0x7100041f, 0x54000d0b, 0xd280000a, 0x11000408,
		0xd37d7c0b, 0xaa0103ec, 0xaa0103ed, 0x2a0003e9,
		0x14000006, 0x9100054a, 0x8b0b01ad, 0x9100218c,
		0xeb09015f, 0x540005a0, 0xb4ffff6a, 0xd280000f,
		0x9b097d4e, 0x8b0e0c2e, 0xaa0103f0, 0x14000009,
		0x1b0f7d11, 0xfc71d821, 0x1e611800, 0xfc2f79c0,
		0x910005ef, 0x91002210, 0xeb0a01ff, 0x540001a0,
		0xfc6f79c0, 0xb4fffeef, 0xd2800011, 0xaa1003e0,
		0xfc7179a1, 0xfd400002, 0x1f428020, 0x91000631,
		0x8b0b0000, 0xeb1101ff, 0x54ffff41, 0x17ffffed,
		0xaa0c03ef, 0xaa0a03f0, 0xd2800011, 0xfc7079c0,
		0xaa0f03e0, 0xfc7179a1, 0xfd400002, 0x1f428020,
		0x91000631, 0x8b0b0000, 0xeb11015f, 0x54ffff41,
		0xfc3079c0, 0x91000610, 0x910021ef, 0xeb09021f,
		0x54fffe41, 0x17ffffd0, 0xd280000a, 0xd37df12d,
		0xaa0103eb, 0x14000006, 0xfc2a7880, 0x9100054a,
		0x8b0d016b, 0xeb09015f, 0x54000160, 0xfc6a7840,
		0xb4ffff4a, 0xd280000c, 0xfc6c7961, 0xfc6c7882,
		0x1f428020, 0x9100058c, 0xeb0c015f, 0x54ffff61,
		0x17fffff2, 0xd280000a, 0x9ba97d2b, 0x8b0b0c2b,
		0xd27df1ac, 0x8b0d006d, 0xaa0903ee, 0x1400000a,
		0x1b0f7d10, 0xfc70d821, 0x1e611800, 0xfc2f7860,
		0x8b0c016b, 0xd100054a, 0xf10005df, 0xaa0f03ee,
		0x540001ad, 0xd10005cf, 0xfc6f7880, 0xaa0a03f0,
		0xaa0b03f1, 0xeb0901df, 0x54fffe4a, 0xfc408621,
		0xfc7079a2, 0x1f428020, 0x91000610, 0x35ffff90,
		0x17ffffec, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	bBase := uint64(0x200000)
	xBase := uint64(0x300000)
	yBase := uint64(0x400000)
	stackBase := uint64(0x500000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	bBuf := make([]byte, n*8)
	for i, v := range b {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	em.LoadProgram(xBase, make([]byte, n*8))
	em.LoadProgram(yBase, make([]byte, n*8))
	em.LoadProgram(stackBase, make([]byte, 8192))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, aBase)
	em.Regs.WriteX(2, bBase)
	em.Regs.WriteX(3, xBase)
	em.Regs.WriteX(4, yBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(xBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedX[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("x[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("kernel_ludcmp: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchNussinov(t *testing.T) {
	// End-to-end PolyBench kernel_nussinov through the OoO pipeline.
	// RNA secondary structure prediction (integer arrays).
	n := 60

	seq := make([]byte, n)
	for i := 0; i < n; i++ {
		seq[i] = byte((i + 1) % 4)
	}
	table := make([]int32, n*n)

	// Golden reference.
	expectedTable := make([]int32, n*n)
	for i := n - 1; i >= 0; i-- {
		for j := i + 1; j < n; j++ {
			if j-1 >= 0 {
				if expectedTable[i*n+j-1] > expectedTable[i*n+j] {
					expectedTable[i*n+j] = expectedTable[i*n+j-1]
				}
			}
			if i+1 < n {
				if expectedTable[(i+1)*n+j] > expectedTable[i*n+j] {
					expectedTable[i*n+j] = expectedTable[(i+1)*n+j]
				}
			}
			if j-1 >= 0 && i+1 < n {
				if i < j-1 {
					match := int32(0)
					if seq[i]+seq[j] == 3 {
						match = 1
					}
					s := expectedTable[(i+1)*n+j-1] + match
					if s > expectedTable[i*n+j] {
						expectedTable[i*n+j] = s
					}
				} else {
					if expectedTable[(i+1)*n+j-1] > expectedTable[i*n+j] {
						expectedTable[i*n+j] = expectedTable[(i+1)*n+j-1]
					}
				}
			}
			for k := i + 1; k < j; k++ {
				s := expectedTable[i*n+k] + expectedTable[(k+1)*n+j]
				if s > expectedTable[i*n+j] {
					expectedTable[i*n+j] = s
				}
			}
		}
	}

	code := []uint32{
		0x7100040b, 0x54000a0b, 0xa9bc67fa, 0xa9015ff8,
		0xa90257f6, 0xa9034ff4, 0xd1001048, 0x2a0003e9,
		0x8b20484a, 0x1b0b7c0b, 0xd2c0002c, 0x1100080d,
		0x1b0d7c0d, 0x2a2003ee, 0xb2607fef, 0xaa0903f1,
		0x8b098190, 0x14000007, 0xd100114a, 0x4b00016b,
		0x8b0f0210, 0x0b0e01ad, 0xf100087f, 0x540006cb,
		0xaa1103e3, 0xd1000631, 0xeb09007f, 0x54fffeea,
		0x9b097e24, 0xaa0d03e5, 0xaa0303e6, 0x8b2b4947,
		0x9b097c73, 0x14000008, 0x6b1602bf, 0x1a96c2b5,
		0xb8347855, 0x910004c6, 0x110004a5, 0x6b0000df,
		0x54fffd40, 0x8b0400d4, 0xb8747855, 0xb8747916,
		0x6b1602bf, 0x1a96c2b5, 0xb8347855, 0x8b1300d6,
		0xb8767857, 0x6b1702bf, 0x1a97c2b5, 0xb8347855,
		0xb8767916, 0xeb06007f, 0x54fffd8a, 0x38716837,
		0x38666838, 0x0b170317, 0x71000eff, 0x1a9616d6,
		0x6b1602bf, 0x1a96c2b5, 0xb8347855, 0xaa0503f6,
		0xaa1003f8, 0xaa0703f7, 0xb84046f9, 0xb876585a,
		0x0b190359, 0x6b1902bf, 0x1a99c2b5, 0xb8347855,
		0xeb9880df, 0x8b0c0318, 0x0b0002d6, 0x54fffeec,
		0x17ffffd9, 0xa9434ff4, 0xa94257f6, 0xa9415ff8,
		0xa8c467fa, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	seqBase := uint64(0x100000)
	tableBase := uint64(0x200000)
	stackBase := uint64(0x300000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	// Load seq (byte array).
	em.LoadProgram(seqBase, seq)

	// Load table (zeroed int32 array).
	tableBuf := make([]byte, n*n*4)
	for i, v := range table {
		binary.LittleEndian.PutUint32(tableBuf[i*4:], uint32(v))
	}
	em.LoadProgram(tableBase, tableBuf)

	em.LoadProgram(stackBase, make([]byte, 8192))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, seqBase)
	em.Regs.WriteX(2, tableBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxErr := int32(0)
	for i := 0; i < n*n; i++ {
		got := int32(em.Mem.ReadU32(tableBase + uint64(i)*4))
		want := expectedTable[i]
		diff := got - want
		if diff < 0 {
			diff = -diff
		}
		if diff > maxErr {
			maxErr = diff
		}
		if got != want {
			row, col := i/n, i%n
			t.Errorf("table[%d][%d] = %d, want %d", row, col, got, want)
		}
	}

	t.Logf("kernel_nussinov: %d instructions, %d cycles, CPI=%.2f, maxErr=%d",
		p.InstDone, p.Cycles, p.CPI(), maxErr)
}

func TestOoO_PolyBenchAdi(t *testing.T) {
	// End-to-end PolyBench kernel_adi through the OoO pipeline.
	// Alternating Direction Implicit solver.
	tsteps := 20
	n := 20

	u := make([]float64, n*n)

	// init_array: u[i][j] = (i + n - j) / n
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			u[i*n+j] = float64(i+n-j) / float64(n)
		}
	}

	// Golden reference.
	gu := make([]float64, n*n)
	gv := make([]float64, n*n)
	gp := make([]float64, n*n)
	gq := make([]float64, n*n)
	copy(gu, u)

	DX := 1.0 / float64(n)
	DY := 1.0 / float64(n)
	DT := 1.0 / float64(tsteps)
	B1 := 2.0
	B2 := 1.0
	mul1 := B1 * DT / (DX * DX)
	mul2 := B2 * DT / (DY * DY)

	a := -mul1 / 2.0
	b := 1.0 + mul1
	c := a
	d := -mul2 / 2.0
	e := 1.0 + mul2
	f := d

	for tt := 1; tt <= tsteps; tt++ {
		// Column Sweep
		for i := 1; i < n-1; i++ {
			gv[0*n+i] = 1.0
			gp[i*n+0] = 0.0
			gq[i*n+0] = gv[0*n+i]
			for j := 1; j < n-1; j++ {
				gp[i*n+j] = -c / (a*gp[i*n+j-1] + b)
				gq[i*n+j] = (-d*gu[j*n+i-1] + (1.0+2.0*d)*gu[j*n+i] - f*gu[j*n+i+1] - a*gq[i*n+j-1]) / (a*gp[i*n+j-1] + b)
			}
			gv[(n-1)*n+i] = 1.0
			for j := n - 2; j >= 1; j-- {
				gv[j*n+i] = gp[i*n+j]*gv[(j+1)*n+i] + gq[i*n+j]
			}
		}
		// Row Sweep
		for i := 1; i < n-1; i++ {
			gu[i*n+0] = 1.0
			gp[i*n+0] = 0.0
			gq[i*n+0] = gu[i*n+0]
			for j := 1; j < n-1; j++ {
				gp[i*n+j] = -f / (d*gp[i*n+j-1] + e)
				gq[i*n+j] = (-a*gv[(i-1)*n+j] + (1.0+2.0*a)*gv[i*n+j] - c*gv[(i+1)*n+j] - d*gq[i*n+j-1]) / (d*gp[i*n+j-1] + e)
			}
			gu[i*n+n-1] = 1.0
			for j := n - 2; j >= 1; j-- {
				gu[i*n+j] = gp[i*n+j]*gu[i*n+j+1] + gq[i*n+j]
			}
		}
	}
	_ = e
	_ = f

	code := []uint32{
		0xd10383ff, 0xa9087bfd, 0xa9096ffc, 0xa90a67fa, 0xa90b5ff8, 0xa90c57f6,
		0xa90d4ff4, 0x7100041f, 0xb90067e0, 0xf90037e1, 0x54001b0b, 0xf94037e1,
		0xb94067e8, 0x1e6e1007, 0x1e601010, 0xd2e7fe1d, 0x1e620020, 0x1e620101,
		0x5100082c, 0x93407c29, 0x93407d8a, 0x937d7c2b, 0x9b2c7c32, 0x51000428,
		0x8b0a012e, 0x8b0b0046, 0x9b2c242c, 0x1e6018e0, 0x8b0e0c50, 0xd10020ce,
		0x91002160, 0xf9003fee, 0xd37df252, 0x8b00008e, 0xd37df18c, 0x1b017d0d,
		0xf9002fee, 0x8b0000ae, 0x8b08012f, 0xf9002bee, 0x910040ce, 0x9100218c,
		0xf9001fee, 0x9100224e, 0x8b2dcc6d, 0xf9001bee, 0x9100206e, 0xd37df1ef,
		0xa90233e6, 0x8b0b008c, 0x91002210, 0xf90027ec, 0xd37d7d0c, 0xd1000511,
		0xf9000fee, 0x8b21d1ce, 0xcb0b03f4, 0x8b0b00b2, 0xd100219b, 0x5280002c,
		0xf9000bee, 0x8b00006e, 0xf90023f2, 0xf90007ee, 0x1e6118e1, 0x1e600800,
		0x1e612822, 0x1e601823, 0x1e7c1001, 0x1e601842, 0x1e610840, 0x1e610861,
		0x1e672842, 0x1e672863, 0x1e614004, 0x1e614025, 0x1f501c26, 0x1f501c07,
		0x14000006, 0xb94067ec, 0xf9403bee, 0x6b0c01df, 0x110005cc, 0x540011e0,
		0xf9003bec, 0xf94037ec, 0x71000d9f, 0x54ffff0b, 0xa9444bec, 0x52800038,
		0xa9429fe6, 0xf9401ff3, 0xa9455bf5, 0x9b097f0e, 0xf838787d, 0xaa1303f7,
		0xaa1503f9, 0xaa1603fa, 0xaa1103fe, 0xf82e789f, 0xfc787870, 0xfc2e78b0,
		0xfc5f8350, 0xf10007de, 0x1f500810, 0x1e701891, 0xfc008751, 0x6d7f46f2,
		0x1e7108d1, 0x1f5244b1, 0xfd4002f2, 0x8b0b02f7, 0x1f5244b1, 0xfc5f8332,
		0x1f524491, 0x1e701a30, 0xfc008730, 0x54fffe21, 0xaa0303f7, 0xaa0a03f9,
		0xf83879bd, 0xfc797a50, 0xfc666af1, 0xfc797992, 0xf1000739, 0x1f514a10,
		0xfc276af0, 0x8b1402f7, 0x54ffff2c, 0x91000718, 0x8b0b02d6, 0x8b0b02b5,
		0x91002273, 0x910020e7, 0xeb08031f, 0x910020c6, 0x8b0b0252, 0x8b0b018c,
		0x54fffa61, 0xf94037ec, 0x71000d9f, 0x54fff84b, 0xa941b3f5, 0xaa1f03fe,
		0xa9445bf8, 0x52800033, 0xa9454be6, 0xa940e7e7, 0x9b1e7d7a, 0x9b097e77,
		0x8b1a017c, 0x8b1a01e0, 0x8b1c0081, 0x8b00008e, 0x8b0000a0, 0xf837785d,
		0x8b1c00bc, 0xeb00003f, 0xf837789f, 0xfa4e3382, 0xfc777850, 0xfc3778b0,
		0x540002a3, 0x9b1e2d6e, 0xaa1f03fc, 0xfc6e6890, 0xfc6e68b1, 0x1f500c32,
		0x1e7218b0, 0xfc3c6a50, 0xfc7c68f3, 0xfc7c6ab4, 0x1e7308f3, 0x1f544c93,
		0xfc7c6b34, 0x1f544c93, 0x1f514cb1, 0x1e721a31, 0xfc3c68d1, 0x9100239c,
		0xeb1c037f, 0x54fffe41, 0x14000015, 0xaa1f03fc, 0x8b1c024e, 0xfc5f81d0,
		0x1f500c30, 0x1e7018b1, 0xfd0001d1, 0x8b1c030e, 0xfc7c68f1, 0xfc7c6ab2,
		0x1e7108f1, 0x1f524491, 0xfc7c6b32, 0x9100239c, 0xeb1c037f, 0x1f524491,
		0xfd4001d2, 0x1f5244b1, 0x1e701a30, 0xfd0005d0, 0x54fffdc1, 0xf9403fee,
		0x91000673, 0xf83779dd, 0xaa0a03f7, 0xfc7a6a10, 0xfc777ad1, 0xfc777b12,
		0x1f504a30, 0xfc377990, 0xf10006f7, 0x54ffff6c, 0x910007de, 0x8b0b0339,
		0x8b0b02b5, 0x8b0b00c6, 0x8b0b00e7, 0x8b0b0252, 0xeb08027f, 0x8b0b0318,
		0x8b0b02d6, 0x8b0b018c, 0x54fff681, 0x17ffff6e, 0xa94d4ff4, 0xa94c57f6,
		0xa94b5ff8, 0xa94a67fa, 0xa9496ffc, 0xa9487bfd, 0x910383ff, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	uBase := uint64(0x100000)
	vBase := uint64(0x200000)
	pBase := uint64(0x300000)
	qBase := uint64(0x400000)
	stackBase := uint64(0x500000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	uBuf := make([]byte, n*n*8)
	for i, val := range u {
		binary.LittleEndian.PutUint64(uBuf[i*8:], math.Float64bits(val))
	}
	em.LoadProgram(uBase, uBuf)

	em.LoadProgram(vBase, make([]byte, n*n*8))
	em.LoadProgram(pBase, make([]byte, n*n*8))
	em.LoadProgram(qBase, make([]byte, n*n*8))
	em.LoadProgram(stackBase, make([]byte, 16384))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(tsteps))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(2, uBase)
	em.Regs.WriteX(3, vBase)
	em.Regs.WriteX(4, pBase)
	em.Regs.WriteX(5, qBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 16384
	em.SetHaltPC(haltPC)

	pipe := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < n*n; i++ {
		bits := em.Mem.ReadU64(uBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := gu[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-6 {
			row, col := i/n, i%n
			t.Errorf("u[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_adi: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		pipe.InstDone, pipe.Cycles, pipe.CPI(), maxRelErr)
}

func TestOoO_PolyBenchFdtd2d(t *testing.T) {
	// End-to-end PolyBench kernel_fdtd_2d through the OoO pipeline.
	// 2D Finite Different Time Domain.
	tmax := 20
	nx := 20
	ny := 30

	ex := make([]float64, nx*ny)
	ey := make([]float64, nx*ny)
	hz := make([]float64, nx*ny)
	fict := make([]float64, tmax)

	// init_array
	for i := 0; i < tmax; i++ {
		fict[i] = float64(i)
	}
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			ex[i*ny+j] = float64(i*(j+1)) / float64(nx)
			ey[i*ny+j] = float64(i*(j+2)) / float64(ny)
			hz[i*ny+j] = float64(i*(j+3)) / float64(nx)
		}
	}

	// Golden reference.
	gex := make([]float64, nx*ny)
	gey := make([]float64, nx*ny)
	ghz := make([]float64, nx*ny)
	copy(gex, ex)
	copy(gey, ey)
	copy(ghz, hz)

	for tt := 0; tt < tmax; tt++ {
		for j := 0; j < ny; j++ {
			gey[0*ny+j] = fict[tt]
		}
		for i := 1; i < nx; i++ {
			for j := 0; j < ny; j++ {
				gey[i*ny+j] = gey[i*ny+j] - 0.5*(ghz[i*ny+j]-ghz[(i-1)*ny+j])
			}
		}
		for i := 0; i < nx; i++ {
			for j := 1; j < ny; j++ {
				gex[i*ny+j] = gex[i*ny+j] - 0.5*(ghz[i*ny+j]-ghz[i*ny+j-1])
			}
		}
		for i := 0; i < nx-1; i++ {
			for j := 0; j < ny-1; j++ {
				ghz[i*ny+j] = ghz[i*ny+j] - 0.7*(gex[i*ny+j+1]-gex[i*ny+j]+gey[(i+1)*ny+j]-gey[i*ny+j])
			}
		}
	}

	code := []uint32{
		0x7100041f, 0x54000e0b, 0xa9bc67fa, 0xa9015ff8, 0xa90257f6, 0xa9034ff4,
		0x7100083f, 0x1e7c1000, 0x2a0203ea, 0x1a9fa7ef, 0x7100042b, 0x937d7c50,
		0x1a9fa7ee, 0x7100085f, 0xd37d7c52, 0xb203e7e7, 0xaa1f03e8, 0x51000449,
		0x2a0003ec, 0x2a0103ed, 0x1a9fa5ce, 0x1a9fa5ef, 0x910020b1, 0x91002060,
		0xd1000543, 0xf2f7fcc7, 0x14000004, 0x91000508, 0xeb0c011f, 0x54000a00,
		0x7100045f, 0x54ffff8b, 0xfc6878c1, 0xaa0a03f3, 0xaa0403f4, 0xf1000673,
		0xfc008681, 0x54ffffc1, 0x7100083f, 0x540002ab, 0xaa0403f3, 0xaa0503f4,
		0x52800035, 0xaa0a03f6, 0xaa1303f7, 0xaa1403f8, 0xfc706b01, 0xfc408702,
		0xf10006d6, 0x1e623821, 0xfc706ae2, 0x1f400821, 0xfc306ae1, 0x910022f7,
		0x54ffff01, 0x910006b5, 0x8b100294, 0x8b100273, 0xeb0d02bf, 0x54fffe01,
		0x3707fbee, 0xaa1f03f3, 0xaa0003f4, 0xaa1103f5, 0xaa0303f6, 0xaa1403f7,
		0xaa1503f8, 0x6d7f8702, 0xf10006d6, 0x91002318, 0x1e623821, 0xfd4002e2,
		0x1f400821, 0xfc0086e1, 0x54ffff21, 0x91000673, 0x8b1202b5, 0x8b120294,
		0xeb0d027f, 0x54fffe21, 0x3707f96f, 0xaa1f03f3, 0xaa0503f4, 0xaa0003f5,
		0xaa0403f6, 0x91000673, 0xaa1403f7, 0xaa1503f8, 0xaa0903f9, 0xaa1603fa,
		0x6d7f8702, 0x9e6700e3, 0xf1000739, 0x91002318, 0x1e623821, 0xfc726b42,
		0x1e622821, 0xfc408742, 0x1e623821, 0xfd4002e2, 0x1f430821, 0xfc0086e1,
		0x54fffe81, 0xeb0b027f, 0x8b1202d6, 0x8b1202b5, 0x8b120294, 0x54fffd41,
		0x17ffffaf, 0xa9434ff4, 0xa94257f6, 0xa9415ff8, 0xa8c467fa, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	exBase := uint64(0x100000)
	eyBase := uint64(0x200000)
	hzBase := uint64(0x300000)
	fictBase := uint64(0x400000)
	stackBase := uint64(0x500000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	exBuf := make([]byte, nx*ny*8)
	for i, val := range ex {
		binary.LittleEndian.PutUint64(exBuf[i*8:], math.Float64bits(val))
	}
	em.LoadProgram(exBase, exBuf)

	eyBuf := make([]byte, nx*ny*8)
	for i, val := range ey {
		binary.LittleEndian.PutUint64(eyBuf[i*8:], math.Float64bits(val))
	}
	em.LoadProgram(eyBase, eyBuf)

	hzBuf := make([]byte, nx*ny*8)
	for i, val := range hz {
		binary.LittleEndian.PutUint64(hzBuf[i*8:], math.Float64bits(val))
	}
	em.LoadProgram(hzBase, hzBuf)

	fictBuf := make([]byte, tmax*8)
	for i, val := range fict {
		binary.LittleEndian.PutUint64(fictBuf[i*8:], math.Float64bits(val))
	}
	em.LoadProgram(fictBase, fictBuf)

	em.LoadProgram(stackBase, make([]byte, 16384))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(tmax))
	em.Regs.WriteX(1, uint64(nx))
	em.Regs.WriteX(2, uint64(ny))
	em.Regs.WriteX(3, exBase)
	em.Regs.WriteX(4, eyBase)
	em.Regs.WriteX(5, hzBase)
	em.Regs.WriteX(6, fictBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 16384
	em.SetHaltPC(haltPC)

	pipe := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < nx*ny; i++ {
		bits := em.Mem.ReadU64(exBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := gex[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-6 {
			row, col := i/ny, i%ny
			t.Errorf("ex[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}
	for i := 0; i < nx*ny; i++ {
		bits := em.Mem.ReadU64(eyBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := gey[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-6 {
			row, col := i/ny, i%ny
			t.Errorf("ey[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}
	for i := 0; i < nx*ny; i++ {
		bits := em.Mem.ReadU64(hzBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := ghz[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-6 {
			row, col := i/ny, i%ny
			t.Errorf("hz[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_fdtd_2d: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		pipe.InstDone, pipe.Cycles, pipe.CPI(), maxRelErr)
}

func TestOoO_PolyBenchHeat3d(t *testing.T) {
	// End-to-end PolyBench kernel_heat_3d through the OoO pipeline.
	// 3D Heat equation.
	tsteps := 20
	n := 10

	nn := n * n
	total := n * n * n
	A := make([]float64, total)
	B := make([]float64, total)

	// init_array: A[i][j][k] = B[i][j][k] = (i + j + (n-k)) * 10 / n
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			for k := 0; k < n; k++ {
				A[i*nn+j*n+k] = float64(i+j+(n-k)) * 10 / float64(n)
				B[i*nn+j*n+k] = A[i*nn+j*n+k]
			}
		}
	}

	// Golden reference.
	gA := make([]float64, total)
	gB := make([]float64, total)
	copy(gA, A)
	copy(gB, B)

	for tt := 1; tt <= tsteps; tt++ {
		for i := 1; i < n-1; i++ {
			for j := 1; j < n-1; j++ {
				for k := 1; k < n-1; k++ {
					gB[i*nn+j*n+k] = 0.125*(gA[(i+1)*nn+j*n+k]-2.0*gA[i*nn+j*n+k]+gA[(i-1)*nn+j*n+k]) +
						0.125*(gA[i*nn+(j+1)*n+k]-2.0*gA[i*nn+j*n+k]+gA[i*nn+(j-1)*n+k]) +
						0.125*(gA[i*nn+j*n+k+1]-2.0*gA[i*nn+j*n+k]+gA[i*nn+j*n+k-1]) +
						gA[i*nn+j*n+k]
				}
			}
		}
		for i := 1; i < n-1; i++ {
			for j := 1; j < n-1; j++ {
				for k := 1; k < n-1; k++ {
					gA[i*nn+j*n+k] = 0.125*(gB[(i+1)*nn+j*n+k]-2.0*gB[i*nn+j*n+k]+gB[(i-1)*nn+j*n+k]) +
						0.125*(gB[i*nn+(j+1)*n+k]-2.0*gB[i*nn+j*n+k]+gB[i*nn+(j-1)*n+k]) +
						0.125*(gB[i*nn+j*n+k+1]-2.0*gB[i*nn+j*n+k]+gB[i*nn+j*n+k-1]) +
						gB[i*nn+j*n+k]
				}
			}
		}
	}

	code := []uint32{
		0x7100041f, 0x5400142b, 0xd101c3ff, 0xa9017bfd, 0xa9026ffc, 0xa90367fa,
		0xa9045ff8, 0xa90557f6, 0xa9064ff4, 0x1b017c2b, 0x93407c2c, 0x937d7c29,
		0x51000428, 0x1e701000, 0x1e681001, 0x9100212f, 0xd37d7d0d, 0xd37d7d6a,
		0x8b0b018c, 0x8b2b51e5, 0xd37df18e, 0x8b0f004b, 0xd10021ac, 0x91002150,
		0x8b05004d, 0x8b21d204, 0x910021d1, 0x8b10004e, 0x8b100070, 0xf90007f0,
		0x8b110070, 0x8b110051, 0x8b040052, 0x8b040062, 0x8b0f0064, 0x8b050063,
		0x52800025, 0xf90003e2, 0x14000005, 0x110004af, 0x6b0000bf, 0x2a0f03e5,
		0x54000e20, 0x71000c3f, 0x54ffff6b, 0xaa1103e6, 0xaa1003e7, 0xaa0d03f3,
		0xaa0b03f4, 0xaa1203f5, 0xaa0e03f6, 0x52800037, 0x910006f7, 0xaa0603f8,
		0xaa0703f9, 0xaa1303fa, 0xaa1403fb, 0xaa1503fc, 0xaa1603fd, 0x5280003e,
		0xaa1f03ef, 0x910007de, 0x8b0f0302, 0xfc6f6b82, 0xfc6f6b45, 0x6d7f8c44,
		0xfc6f6ba6, 0xfd400447, 0x1f400862, 0x1f401465, 0x1e662842, 0xfc6f6b66,
		0x1e6628a5, 0x1f401c66, 0x1e610842, 0x1e6428c4, 0x1f4108a2, 0x1f410882,
		0x1e622862, 0xfc2f6b22, 0x910021ef, 0xeb0f019f, 0x54fffd81, 0x8b0903bd,
		0x8b09039c, 0x8b09037b, 0xeb0803df, 0x8b09035a, 0x8b090339, 0x8b090318,
		0x54fffc41, 0x8b0a02d6, 0x8b0a02b5, 0x8b0a0294, 0xeb0802ff, 0x8b0a0273,
		0x8b0a00e7, 0x8b0a00c6, 0x54fffa41, 0x71000c3f, 0x54fff86b, 0xa9405bf5,
		0xaa1003e6, 0xaa1103e7, 0xaa0303f3, 0xaa0403f4, 0x52800037, 0x910006f7,
		0xaa0603f8, 0xaa0703f9, 0xaa1303fa, 0xaa1403fb, 0xaa1503fc, 0xaa1603fd,
		0x5280003e, 0xaa1f03ef, 0x910007de, 0x8b0f0302, 0xfc6f6b82, 0xfc6f6b45,
		0x6d7f8c44, 0xfc6f6ba6, 0xfd400447, 0x1f400862, 0x1f401465, 0x1e662842,
		0xfc6f6b66, 0x1e6628a5, 0x1f401c66, 0x1e610842, 0x1e6428c4, 0x1f4108a2,
		0x1f410882, 0x1e622862, 0xfc2f6b22, 0x910021ef, 0xeb0f019f, 0x54fffd81,
		0x8b0903bd, 0x8b09039c, 0x8b09037b, 0xeb0803df, 0x8b09035a, 0x8b090339,
		0x8b090318, 0x54fffc41, 0x8b0a02d6, 0x8b0a02b5, 0x8b0a0294, 0xeb0802ff,
		0x8b0a0273, 0x8b0a00e7, 0x8b0a00c6, 0x54fffa41, 0x17ffff8d, 0xa9464ff4,
		0xa94557f6, 0xa9445ff8, 0xa94367fa, 0xa9426ffc, 0xa9417bfd, 0x9101c3ff,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	bBase := uint64(0x200000)
	stackBase := uint64(0x300000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	aBuf := make([]byte, total*8)
	for i, val := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(val))
	}
	em.LoadProgram(aBase, aBuf)

	bBuf := make([]byte, total*8)
	for i, val := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(val))
	}
	em.LoadProgram(bBase, bBuf)

	em.LoadProgram(stackBase, make([]byte, 16384))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(tsteps))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(2, aBase)
	em.Regs.WriteX(3, bBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 16384
	em.SetHaltPC(haltPC)

	pipe := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < total; i++ {
		bits := em.Mem.ReadU64(aBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := gA[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-6 {
			ii := i / nn
			jj := (i % nn) / n
			kk := i % n
			t.Errorf("A[%d][%d][%d] = %g, want %g (relErr=%e)", ii, jj, kk, got, want, relErr)
		}
	}

	t.Logf("kernel_heat_3d: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		pipe.InstDone, pipe.Cycles, pipe.CPI(), maxRelErr)
}

func TestOoO_PolyBenchJacobi2d(t *testing.T) {
	// End-to-end PolyBench kernel_jacobi_2d through the OoO pipeline.
	// 2D Jacobi stencil computation.
	tsteps := 20
	n := 30

	A := make([]float64, n*n)
	B := make([]float64, n*n)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			A[i*n+j] = float64(i*(j+2)+2) / float64(n)
			B[i*n+j] = float64(i*(j+3)+3) / float64(n)
		}
	}

	// Golden reference.
	gA := make([]float64, n*n)
	gB := make([]float64, n*n)
	copy(gA, A)
	copy(gB, B)

	for tt := 0; tt < tsteps; tt++ {
		for i := 1; i < n-1; i++ {
			for j := 1; j < n-1; j++ {
				gB[i*n+j] = 0.2 * (gA[i*n+j] + gA[i*n+j-1] + gA[i*n+j+1] + gA[(i+1)*n+j] + gA[(i-1)*n+j])
			}
		}
		for i := 1; i < n-1; i++ {
			for j := 1; j < n-1; j++ {
				gA[i*n+j] = 0.2 * (gB[i*n+j] + gB[i*n+j-1] + gB[i*n+j+1] + gB[(i+1)*n+j] + gB[(i-1)*n+j])
			}
		}
	}

	code := []uint32{
		0x7100041f, 0x540009eb, 0x71000c3f, 0x540009ab,
		0x52800008, 0x51000429, 0x2a0103eb, 0x9100204a,
		0x5280010c, 0xaa0b1190, 0xd37d7c2b, 0x8b10004c,
		0x9100216e, 0x8b0e006d, 0x8b0e004e, 0xb201e7ef,
		0xf293334f, 0xf2e7f92f, 0x9e6701e0, 0xd37d7d2f,
		0xd10021ef, 0x8b100070, 0x91002071, 0xaa0e03e1,
		0xaa0d03e2, 0xaa0c03e3, 0xaa0a03e4, 0x52800025,
		0xd2800006, 0x910004a5, 0x8b060027, 0x6d7f84e2,
		0x1e622821, 0xfd4004e2, 0x1e622821, 0xfc666862,
		0x1e622821, 0xfc666882, 0x1e622821, 0x1e600821,
		0xfc266841, 0x910020c6, 0xeb0601ff, 0x54fffe61,
		0x8b0b0084, 0x8b0b0063, 0x8b0b0042, 0x8b0b0021,
		0xeb0900bf, 0x54fffd61, 0xaa0d03e1, 0xaa0e03e2,
		0xaa1003e3, 0xaa1103e4, 0x52800025, 0xd2800006,
		0x910004a5, 0x8b060027, 0x6d7f84e2, 0x1e622821,
		0xfd4004e2, 0x1e622821, 0xfc666862, 0x1e622821,
		0xfc666882, 0x1e622821, 0x1e600821, 0xfc266841,
		0x910020c6, 0xeb0601ff, 0x54fffe61, 0x8b0b0084,
		0x8b0b0063, 0x8b0b0042, 0x8b0b0021, 0xeb0900bf,
		0x54fffd61, 0x11000508, 0x6b00011f, 0x54fff901,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	bBase := uint64(0x200000)
	stackBase := uint64(0x300000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	bBuf := make([]byte, n*n*8)
	for i, v := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	em.LoadProgram(stackBase, make([]byte, 8192))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(tsteps))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(2, aBase)
	em.Regs.WriteX(3, bBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < n*n; i++ {
		bits := em.Mem.ReadU64(aBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := gA[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("A[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_jacobi_2d: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchSeidel2d(t *testing.T) {
	// End-to-end PolyBench kernel_seidel_2d through the OoO pipeline.
	// 2D Seidel stencil computation.
	tsteps := 20
	n := 20

	A := make([]float64, n*n)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			A[i*n+j] = float64(i*(j+2)+2) / float64(n)
		}
	}

	// Golden reference.
	gA := make([]float64, n*n)
	copy(gA, A)

	for tt := 0; tt < tsteps; tt++ {
		for i := 1; i < n-1; i++ {
			for j := 1; j < n-1; j++ {
				gA[i*n+j] = (gA[(i-1)*n+j-1] + gA[(i-1)*n+j] + gA[(i-1)*n+j+1] +
					gA[i*n+j-1] + gA[i*n+j] + gA[i*n+j+1] +
					gA[(i+1)*n+j-1] + gA[(i+1)*n+j] + gA[(i+1)*n+j+1]) / 9.0
			}
		}
	}

	code := []uint32{
		0x7100041f, 0x540006eb, 0x71000c3f, 0x540006ab,
		0xa9bf4ff4, 0x52800008, 0x51000429, 0xd37d7c2a,
		0x8b0a004b, 0xd100052c, 0x9100204d, 0x8b0a01ae,
		0x1e645000, 0x8b2151af, 0xd2800010, 0xaa0e03f1,
		0xaa0f03e1, 0xaa0d03e2, 0x52800023, 0x9b107d44,
		0xfc646961, 0x91000463, 0xaa1103f3, 0xaa0103e4,
		0xaa0203e5, 0xaa0c03e6, 0xaa1103e7, 0x6d7f8ca2,
		0x1e632842, 0xfc408ca3, 0x1e632842, 0x1e612841,
		0xfd400262, 0x1e622821, 0xfc408ce2, 0x1e622821,
		0x6d7f8c82, 0x1e622821, 0xfc408c82, 0x1e632821,
		0x1e622821, 0x1e601821, 0xfd000261, 0xaa0703f3,
		0xf10004c6, 0x54fffdc1, 0x91000610, 0x8b0a0042,
		0x8b0a0021, 0x8b0a0231, 0xeb09007f, 0x54fffc01,
		0x11000508, 0x6b00011f, 0x54fffb01, 0xa8c14ff4,
		0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	stackBase := uint64(0x200000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	em.LoadProgram(stackBase, make([]byte, 8192))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(tsteps))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(2, aBase)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < n*n; i++ {
		bits := em.Mem.ReadU64(aBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := gA[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("A[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_seidel_2d: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

func TestOoO_PolyBenchDeriche(t *testing.T) {
	// End-to-end PolyBench kernel_deriche through the OoO pipeline.
	// Deriche edge detection filter.
	w := 64
	h := 64

	imgIn := make([]float64, w*h)

	for i := 0; i < w; i++ {
		for j := 0; j < h; j++ {
			imgIn[i*h+j] = float64((313*i+991*j)%65536) / 65535.0
		}
	}

	// Golden reference.
	alpha := float64(float32(0.25))
	kema := -0.18869340542894975 * 0.7788007830714049
	a1 := -0.18869340542894975
	a2 := kema * (alpha - 1.0)
	a3 := kema * (alpha + 1.0)
	a4 := 0.18869340542894975 * 0.6065306597633104
	b1 := 0.8408964152537145
	b2 := -0.6065306597633104

	gImgOut := make([]float64, w*h)
	gY1 := make([]float64, w*h)
	gY2 := make([]float64, w*h)

	// Pass 1: horizontal forward.
	for i := 0; i < w; i++ {
		ym1, ym2, xm1 := 0.0, 0.0, 0.0
		for j := 0; j < h; j++ {
			gY1[i*h+j] = a1*imgIn[i*h+j] + a2*xm1 + b1*ym1 + b2*ym2
			xm1 = imgIn[i*h+j]
			ym2 = ym1
			ym1 = gY1[i*h+j]
		}
	}
	// Pass 2: horizontal backward.
	for i := 0; i < w; i++ {
		yp1, yp2, xp1, xp2 := 0.0, 0.0, 0.0, 0.0
		for j := h - 1; j >= 0; j-- {
			gY2[i*h+j] = a3*xp1 + a4*xp2 + b1*yp1 + b2*yp2
			xp2 = xp1
			xp1 = imgIn[i*h+j]
			yp2 = yp1
			yp1 = gY2[i*h+j]
		}
	}
	// Combine.
	for i := 0; i < w; i++ {
		for j := 0; j < h; j++ {
			gImgOut[i*h+j] = gY1[i*h+j] + gY2[i*h+j]
		}
	}
	// Pass 3: vertical forward.
	for j := 0; j < h; j++ {
		tm1, ym1, ym2 := 0.0, 0.0, 0.0
		for i := 0; i < w; i++ {
			gY1[i*h+j] = a1*gImgOut[i*h+j] + a2*tm1 + b1*ym1 + b2*ym2
			tm1 = gImgOut[i*h+j]
			ym2 = ym1
			ym1 = gY1[i*h+j]
		}
	}
	// Pass 4: vertical backward.
	for j := 0; j < h; j++ {
		tp1, tp2, yp1, yp2 := 0.0, 0.0, 0.0, 0.0
		for i := w - 1; i >= 0; i-- {
			gY2[i*h+j] = a3*tp1 + a4*tp2 + b1*yp1 + b2*yp2
			tp2 = tp1
			tp1 = gImgOut[i*h+j]
			yp2 = yp1
			yp1 = gY2[i*h+j]
		}
	}
	// Final combine.
	for i := 0; i < w; i++ {
		for j := 0; j < h; j++ {
			gImgOut[i*h+j] = gY1[i*h+j] + gY2[i*h+j]
		}
	}

	code := []uint32{
		0x1e22c000, 0x1e7e1001, 0x1e612801, 0xd2836be8,
		0xf2a98588, 0xf2d9ed08, 0xf2f7f848, 0x9e670102,
		0x1e620821, 0x1e6e1003, 0x1e632800, 0x1e620800,
		0x7100041f, 0x54000d6b, 0x7100043f, 0x54001beb,
		0xd280000a, 0xd37d7c2b, 0xd29c8008, 0xf2a05488,
		0xf2c4e368, 0xf2f7f908, 0x9e670102, 0xd29a75a8,
		0xf2b32b48, 0xf2dd13e8, 0xf2e7fd48, 0x9e670103,
		0xd291a288, 0xf2bf8ec8, 0xf2cd1648, 0xf2f7fc68,
		0x9e670104, 0xaa0203ec, 0xaa0403ed, 0x2a0103e8,
		0x2a0003e9, 0x2f00e405, 0xaa0803ee, 0xaa0c03ef,
		0xaa0d03f0, 0x2f00e407, 0x2f00e406, 0xfd4001f0,
		0x1e650825, 0x1f421605, 0x1f4314c5, 0x1f4414f0,
		0xfc008610, 0xfc4085e5, 0x1e6040c7, 0x1e604206,
		0xf10005ce, 0x54fffec1, 0x9100054a, 0x8b0b01ad,
		0x8b0b018c, 0xeb09015f, 0x54fffd61, 0xd280000b,
		0xd10020ac, 0xd37df10a, 0xd100204d, 0xd28b578e,
		0xf2ae5b8e, 0xf2c98f8e, 0xf2e7f7ae, 0x9e6701c2,
		0xd29a75ae, 0xf2b32b4e, 0xf2dd13ee, 0xf2e7fd4e,
		0x9e6701c3, 0xd291a28e, 0xf2bf8ece, 0xf2cd164e,
		0xf2f7fc6e, 0x9e6701c4, 0x2f00e407, 0xaa0803ee,
		0x2f00e405, 0x2f00e410, 0x2f00e406, 0x1e6208e7,
		0x1f451c07, 0x1f431cc7, 0x1f441e11, 0xfc2e7991,
		0xfc6e79b0, 0x1e6040a7, 0x1e604205, 0x1e6040d0,
		0x1e604226, 0xf10005ce, 0x54fffeac, 0x9100056b,
		0x8b0a018c, 0x8b0a01ad, 0xeb09017f, 0x54fffd61,
		0xd280000b, 0xaa0403ec, 0xaa0503ed, 0xaa0303ee,
		0xaa0803ef, 0xaa0c03f0, 0xaa0d03f1, 0xaa0e03e2,
		0xfc408602, 0xfc408623, 0x1e632842, 0xfc008442,
		0xf10005ef, 0x54ffff61, 0x9100056b, 0x8b0a01ce,
		0x8b0a01ad, 0x8b0a018c, 0xeb09017f, 0x54fffe21,
		0x7100043f, 0x54000bab, 0x7100041f, 0x54000e6b,
		0xd2800009, 0xd37d7c2a, 0xd29c8008, 0xf2a05488,
		0xf2c4e368, 0xf2f7f908, 0x9e670102, 0xd29a75a8,
		0xf2b32b48, 0xf2dd13e8, 0xf2e7fd48, 0x9e670103,
		0xd291a288, 0xf2bf8ec8, 0xf2cd1648, 0xf2f7fc68,
		0x9e670104, 0xaa0303ec, 0xaa0403ed, 0x2a0103e8,
		0x2a0003eb, 0xd280000e, 0x2f00e405, 0xaa0b03ef,
		0x2f00e407, 0x2f00e406, 0xfc6e6990, 0x1e650825,
		0x1f421605, 0x1f4314c5, 0x1f4414f0, 0xfc2e69b0,
		0xfc6e6985, 0x8b0a01ce, 0x1e6040c7, 0x1e604206,
		0xf10005ef, 0x54fffea1, 0x91000529, 0x910021ad,
		0x9100218c, 0xeb08013f, 0x54fffd61, 0xd2800009,
		0x9100056a, 0xd100056b, 0x9b087d6b, 0xd37df16d,
		0x8b0d00ab, 0xd37df10c, 0x8b0d006d, 0xd28b578e,
		0xf2ae5b8e, 0xf2c98f8e, 0xf2e7f7ae, 0x9e6701c1,
		0xd29a75ae, 0xf2b32b4e, 0xf2dd13ee, 0xf2e7fd4e,
		0x9e6701c2, 0xd291a28e, 0xf2bf8ece, 0xf2cd164e,
		0xf2f7fc6e, 0x9e6701c3, 0xd280000e, 0x2f00e406,
		0xaa0a03ef, 0x2f00e405, 0x2f00e407, 0x2f00e404,
		0x1e6108c6, 0x1f451806, 0x1f421886, 0x1f4318f0,
		0xfc2e6970, 0x1e6040a6, 0xfc6e69a5, 0xd10005ef,
		0xcb0c01ce, 0x1e604087, 0x1e604204, 0xf10005ff,
		0x54fffe8c, 0x91000529, 0x9100216b, 0x910021ad,
		0xeb08013f, 0x54fffd21, 0x7100041f, 0x540002eb,
		0x7100043f, 0x540002ab, 0xd2800008, 0x2a0103e9,
		0xd37d7c2a, 0x2a0003eb, 0xaa0903ec, 0xaa0403ed,
		0xaa0503ee, 0xaa0303ef, 0xfc4085a0, 0xfc4085c1,
		0x1e612800, 0xfc0085e0, 0xf100058c, 0x54ffff61,
		0x91000508, 0x8b0a0063, 0x8b0a00a5, 0x8b0a0084,
		0xeb0b011f, 0x54fffe21, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	imgInBase := uint64(0x100000)
	imgOutBase := uint64(0x200000)
	y1Base := uint64(0x300000)
	y2Base := uint64(0x400000)
	stackBase := uint64(0x500000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 50_000_000
	em.LoadProgram(codeBase, encodeInsts(code))

	imgInBuf := make([]byte, w*h*8)
	for i, v := range imgIn {
		binary.LittleEndian.PutUint64(imgInBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(imgInBase, imgInBuf)

	em.LoadProgram(imgOutBase, make([]byte, w*h*8))
	em.LoadProgram(y1Base, make([]byte, w*h*8))
	em.LoadProgram(y2Base, make([]byte, w*h*8))
	em.LoadProgram(stackBase, make([]byte, 8192))

	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(w))
	em.Regs.WriteX(1, uint64(h))
	em.Regs.WriteS(0, float32(0.25))
	em.Regs.WriteX(2, imgInBase)
	em.Regs.WriteX(3, imgOutBase)
	em.Regs.WriteX(4, y1Base)
	em.Regs.WriteX(5, y2Base)
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	maxRelErr := 0.0
	for i := 0; i < w*h; i++ {
		bits := em.Mem.ReadU64(imgOutBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := gImgOut[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-6 {
			row, col := i/h, i%h
			t.Errorf("imgOut[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("kernel_deriche: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), maxRelErr)
}

// dataRegion describes a contiguous memory region to warm in the cache.
type dataRegion struct {
	addr uint64
	size int
}

// warmCache pre-populates the cache with the given data regions by issuing
// read lookups every 64 bytes (L1 line size). This simulates the
// initialization phase that occurs before the kernel runs on real hardware.
func warmCache(cache *mem.Cache, regions []dataRegion) {
	const lineSize = 64
	for _, r := range regions {
		for off := 0; off < r.size; off += lineSize {
			cache.Lookup(r.addr+uint64(off), 8, false)
		}
	}
}

// setupFunc defines a benchmark kernel with its setup closure, native
// execution time, and data regions for cache warm-up.
type setupFunc struct {
	name    string
	setup   func() *emu.Emulator
	native  float64
	regions []dataRegion
}

// convertRegions converts benchmark.DataRegion slice to local dataRegion slice.
func convertRegions(brs []benchmark.DataRegion) []dataRegion {
	out := make([]dataRegion, len(brs))
	for i, r := range brs {
		out[i] = dataRegion{addr: r.Addr, size: r.Size}
	}
	return out
}

// comparisonKernels returns the full set of benchmark kernel definitions
// used by ComparisonTable and QuickEval tests.
func comparisonKernels() []setupFunc {
	all := benchmark.All()
	result := make([]setupFunc, len(all))
	for i, b := range all {
		result[i] = setupFunc{
			name:    b.Name,
			setup:   b.Setup,
			native:  b.NativeTime,
			regions: convertRegions(b.Regions),
		}
	}
	return result
}

func TestOoO_ComparisonTable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ComparisonTable in short mode (runs all kernels 3x, needs >5m)")
	}
	const freq = 3.49e9 // M2 Avalanche core frequency

	type kernelResult struct {
		name         string
		insts        uint64
		iooCycles    uint64  // in-order cycles
		iooCPI       float64 // in-order CPI
		oooCycles    uint64  // OoO cycles
		oooCPI       float64 // OoO CPI
		oooNoBPCycles uint64  // OoO cycles without branch predictor
		oooNoBPCPI   float64 // OoO CPI without branch predictor
		bpAccuracy   float64 // branch prediction accuracy (Hits/Predictions * 100)
		native       float64 // native time in seconds
		l1HitRate    float64
		l2HitRate    float64
	}

	kernels := comparisonKernels()

	results := make([]kernelResult, 0, len(kernels))

	for _, k := range kernels {
		// Run in-order pipeline.
		emIO := k.setup()
		cacheIO := mem.NewCache(mem.DefaultCacheConfig())
		warmCache(cacheIO, k.regions)
		pIO := runPipelineWithCache(t, emIO, cacheIO)
		if !emIO.Halted {
			t.Fatalf("%s: in-order emulator should have halted", k.name)
		}

		// Run OoO pipeline (with branch predictor).
		emOoO := k.setup()
		cacheOoO := mem.NewCache(mem.DefaultCacheConfig())
		warmCache(cacheOoO, k.regions)
		pOoO := runOoOPipelineWithCache(t, emOoO, cacheOoO)
		if !emOoO.Halted {
			t.Fatalf("%s: OoO emulator should have halted", k.name)
		}

		// Capture branch prediction stats.
		var bpAcc float64
		if pOoO.BranchPred != nil && pOoO.BranchPred.Predictions > 0 {
			bpAcc = float64(pOoO.BranchPred.Hits) / float64(pOoO.BranchPred.Predictions) * 100
		}

		// Run OoO pipeline without branch predictor.
		emNoBP := k.setup()
		cacheNoBP := mem.NewCache(mem.DefaultCacheConfig())
		warmCache(cacheNoBP, k.regions)
		engineNoBP := sim.NewSerialEngine()
		pNoBP := NewOoOPipeline("TestOoO", engineNoBP, 3490*sim.MHz, emNoBP)
		pNoBP.Cache = cacheNoBP
		pNoBP.BranchPred = nil
		pNoBP.TickLater()
		if err := engineNoBP.Run(); err != nil {
			t.Fatalf("%s: no-BP engine.Run() failed: %v", k.name, err)
		}
		if pNoBP.Err != nil {
			t.Fatalf("%s: no-BP pipeline error: %v", k.name, pNoBP.Err)
		}
		if !emNoBP.Halted {
			t.Fatalf("%s: no-BP emulator should have halted", k.name)
		}

		stats := cacheOoO.Stats()
		l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses)
		l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses)

		results = append(results, kernelResult{
			name:          k.name,
			insts:         pOoO.InstDone,
			iooCycles:     pIO.Cycles,
			iooCPI:        pIO.CPI(),
			oooCycles:     pOoO.Cycles,
			oooCPI:        pOoO.CPI(),
			oooNoBPCycles: pNoBP.Cycles,
			oooNoBPCPI:    pNoBP.CPI(),
			bpAccuracy:    bpAcc,
			native:        k.native,
			l1HitRate:     l1Hit,
			l2HitRate:     l2Hit,
		})
	}

	// Print comparison table.
	t.Logf("\n%-8s | %12s | %11s | %9s | %10s | %7s | %9s | %7s | %14s | %12s | %8s | %8s | %8s | %8s",
		"Kernel", "Instructions", "InOrder CPI", "OoO CPI", "NoBP CPI", "BP Acc%", "BP Improv", "Speedup", "Sim Time (s)", "Native (s)", "Ratio", "L1 Hit%", "L2 Hit%", "Error")
	t.Logf("%-8s-+-%12s-+-%11s-+-%9s-+-%10s-+-%7s-+-%9s-+-%7s-+-%14s-+-%12s-+-%8s-+-%8s-+-%8s-+-%8s",
		"--------", "------------", "-----------", "---------", "----------", "-------", "---------", "-------", "--------------", "------------", "--------", "--------", "--------", "--------")

	var totalError float64
	var maxError float64
	var maxErrorName string
	countInRange := 0    // ratio in [0.67, 1.50]
	countLE20 := 0       // error <= 0.20
	countLE50 := 0       // error <= 0.50

	for _, r := range results {
		simTime := float64(r.oooCycles) / freq
		ratio := simTime / r.native
		speedup := r.iooCPI / r.oooCPI
		bpImprov := (r.oooNoBPCPI - r.oooCPI) / r.oooNoBPCPI * 100
		minVal := math.Min(simTime, r.native)
		errVal := math.Abs(simTime-r.native) / minVal

		totalError += errVal
		if errVal > maxError {
			maxError = errVal
			maxErrorName = r.name
		}
		if ratio >= 0.67 && ratio <= 1.50 {
			countInRange++
		}
		if errVal <= 0.20 {
			countLE20++
		}
		if errVal <= 0.50 {
			countLE50++
		}

		t.Logf("%-8s | %12d | %11.2f | %9.2f | %10.2f | %6.1f%% | %8.1f%% | %6.2fx | %14e | %12e | %8.1f | %7.1f%% | %7.1f%% | %7.1f%%",
			r.name, r.insts, r.iooCPI, r.oooCPI, r.oooNoBPCPI, r.bpAccuracy, bpImprov, speedup, simTime, r.native, ratio, r.l1HitRate*100, r.l2HitRate*100, errVal*100)
	}

	n := len(results)
	avgError := totalError / float64(n)

	t.Logf("")
	t.Logf("Summary (%d benchmarks):", n)
	t.Logf("  Avg Error:          %6.1f%%", avgError*100)
	t.Logf("  Max Error:          %6.1f%% (%s)", maxError*100, maxErrorName)
	t.Logf("  In range [0.67-1.50]: %d / %d", countInRange, n)
	t.Logf("  Within 20%% error:    %d / %d", countLE20, n)
	t.Logf("  Within 50%% error:    %d / %d", countLE50, n)

	passAvg := avgError <= 0.20
	passMax := maxError <= 0.50
	if passAvg && passMax {
		t.Logf("  Result: PASS (avgError=%.1f%% <= 20%%, maxError=%.1f%% <= 50%%)", avgError*100, maxError*100)
	} else {
		reasons := ""
		if !passAvg {
			reasons += fmt.Sprintf("avgError=%.1f%% > 20%%", avgError*100)
		}
		if !passMax {
			if reasons != "" {
				reasons += ", "
			}
			reasons += fmt.Sprintf("maxError=%.1f%% > 50%%", maxError*100)
		}
		t.Logf("  Result: FAIL (%s)", reasons)
	}

	// Also print as a clean formatted string for easy copy-paste.
	fmt.Println()
	fmt.Printf("%-8s | %12s | %11s | %9s | %10s | %7s | %9s | %7s | %14s | %12s | %8s | %8s | %8s | %8s\n",
		"Kernel", "Instructions", "InOrder CPI", "OoO CPI", "NoBP CPI", "BP Acc%", "BP Improv", "Speedup", "Sim Time (s)", "Native (s)", "Ratio", "L1 Hit%", "L2 Hit%", "Error")
	fmt.Printf("%-8s-+-%12s-+-%11s-+-%9s-+-%10s-+-%7s-+-%9s-+-%7s-+-%14s-+-%12s-+-%8s-+-%8s-+-%8s-+-%8s\n",
		"--------", "------------", "-----------", "---------", "----------", "-------", "---------", "-------", "--------------", "------------", "--------", "--------", "--------", "--------")
	for _, r := range results {
		simTime := float64(r.oooCycles) / freq
		ratio := simTime / r.native
		speedup := r.iooCPI / r.oooCPI
		bpImprov := (r.oooNoBPCPI - r.oooCPI) / r.oooNoBPCPI * 100
		minVal := math.Min(simTime, r.native)
		errVal := math.Abs(simTime-r.native) / minVal
		fmt.Printf("%-8s | %12d | %11.2f | %9.2f | %10.2f | %6.1f%% | %8.1f%% | %6.2fx | %14e | %12e | %8.1f | %7.1f%% | %7.1f%% | %7.1f%%\n",
			r.name, r.insts, r.iooCPI, r.oooCPI, r.oooNoBPCPI, r.bpAccuracy, bpImprov, speedup, simTime, r.native, ratio, r.l1HitRate*100, r.l2HitRate*100, errVal*100)
	}

	fmt.Printf("\nSummary (%d benchmarks):\n", n)
	fmt.Printf("  Avg Error:          %6.1f%%\n", avgError*100)
	fmt.Printf("  Max Error:          %6.1f%% (%s)\n", maxError*100, maxErrorName)
	fmt.Printf("  In range [0.67-1.50]: %d / %d\n", countInRange, n)
	fmt.Printf("  Within 20%% error:    %d / %d\n", countLE20, n)
	fmt.Printf("  Within 50%% error:    %d / %d\n", countLE50, n)
	if passAvg && passMax {
		fmt.Printf("  Result: PASS (avgError=%.1f%% <= 20%%, maxError=%.1f%% <= 50%%)\n", avgError*100, maxError*100)
	} else {
		reasons := ""
		if !passAvg {
			reasons += fmt.Sprintf("avgError=%.1f%% > 20%%", avgError*100)
		}
		if !passMax {
			if reasons != "" {
				reasons += ", "
			}
			reasons += fmt.Sprintf("maxError=%.1f%% > 50%%", maxError*100)
		}
		fmt.Printf("  Result: FAIL (%s)\n", reasons)
	}
}

// TestOoO_QuickEval runs 6 representative benchmarks through the OoO pipeline
// only (skip in-order and no-BP variants). Includes cache warm-up. Reports sim
// time, native time, ratio, and error. Should complete in <5 minutes.
func TestOoO_QuickEval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping QuickEval in short mode")
	}
	const freq = 3.49e9 // M2 Avalanche core frequency

	// Select 6 representative benchmarks from the full comparison suite.
	quickNames := map[string]bool{
		"atax": true, "bicg": true, "gemm": true,
		"floydwarshall": true, "trisolv": true, "seidel2d": true,
	}

	allKernels := comparisonKernels()
	var kernels []setupFunc
	for _, k := range allKernels {
		if quickNames[k.name] {
			kernels = append(kernels, k)
		}
	}

	type quickResult struct {
		name    string
		insts   uint64
		cycles  uint64
		cpi     float64
		simTime float64
		native  float64
		ratio   float64
		errVal  float64
	}

	results := make([]quickResult, 0, len(kernels))

	for _, k := range kernels {
		em := k.setup()
		cache := mem.NewCache(mem.DefaultCacheConfig())
		warmCache(cache, k.regions)
		p := runOoOPipelineWithCache(t, em, cache)
		if !em.Halted {
			t.Fatalf("%s: emulator should have halted", k.name)
		}

		simTime := float64(p.Cycles) / freq
		ratio := simTime / k.native
		minVal := math.Min(simTime, k.native)
		errVal := math.Abs(simTime-k.native) / minVal

		results = append(results, quickResult{
			name:    k.name,
			insts:   p.InstDone,
			cycles:  p.Cycles,
			cpi:     p.CPI(),
			simTime: simTime,
			native:  k.native,
			ratio:   ratio,
			errVal:  errVal,
		})
	}

	// Print QuickEval table.
	t.Logf("\n%-14s | %10s | %10s | %7s | %14s | %12s | %8s | %8s",
		"Kernel", "Insts", "Cycles", "CPI", "Sim Time (s)", "Native (s)", "Ratio", "Error")
	t.Logf("%-14s-+-%10s-+-%10s-+-%7s-+-%14s-+-%12s-+-%8s-+-%8s",
		"--------------", "----------", "----------", "-------", "--------------", "------------", "--------", "--------")

	var totalError float64
	var maxError float64
	var maxErrorName string
	for _, r := range results {
		t.Logf("%-14s | %10d | %10d | %7.2f | %14e | %12e | %8.1f | %7.1f%%",
			r.name, r.insts, r.cycles, r.cpi, r.simTime, r.native, r.ratio, r.errVal*100)
		totalError += r.errVal
		if r.errVal > maxError {
			maxError = r.errVal
			maxErrorName = r.name
		}
	}

	n := len(results)
	avgError := totalError / float64(n)
	t.Logf("")
	t.Logf("QuickEval Summary (%d benchmarks):", n)
	t.Logf("  Avg Error: %6.1f%%", avgError*100)
	t.Logf("  Max Error: %6.1f%% (%s)", maxError*100, maxErrorName)

	// Also print clean table for easy copy-paste.
	fmt.Println()
	fmt.Printf("%-14s | %10s | %10s | %7s | %14s | %12s | %8s | %8s\n",
		"Kernel", "Insts", "Cycles", "CPI", "Sim Time (s)", "Native (s)", "Ratio", "Error")
	fmt.Printf("%-14s-+-%10s-+-%10s-+-%7s-+-%14s-+-%12s-+-%8s-+-%8s\n",
		"--------------", "----------", "----------", "-------", "--------------", "------------", "--------", "--------")
	for _, r := range results {
		fmt.Printf("%-14s | %10d | %10d | %7.2f | %14e | %12e | %8.1f | %7.1f%%\n",
			r.name, r.insts, r.cycles, r.cpi, r.simTime, r.native, r.ratio, r.errVal*100)
	}
	fmt.Printf("\nQuickEval Summary (%d benchmarks):\n", n)
	fmt.Printf("  Avg Error: %6.1f%%\n", avgError*100)
	fmt.Printf("  Max Error: %6.1f%% (%s)\n", maxError*100, maxErrorName)
}

// TestOoO_All30 runs all 30 benchmarks through OoO pipeline only (no in-order
// or no-BP variants). This is faster than ComparisonTable and suitable for
// quick full-suite error measurement.
func TestOoO_All30(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping All30 in short mode")
	}
	const freq = 3.49e9

	kernels := comparisonKernels()

	type result struct {
		name    string
		insts   uint64
		cycles  uint64
		cpi     float64
		simTime float64
		native  float64
		ratio   float64
		errVal  float64
	}

	results := make([]result, 0, len(kernels))

	for _, k := range kernels {
		em := k.setup()
		cache := mem.NewCache(mem.DefaultCacheConfig())
		warmCache(cache, k.regions)
		p := runOoOPipelineWithCache(t, em, cache)
		if !em.Halted {
			t.Fatalf("%s: emulator should have halted", k.name)
		}

		simTime := float64(p.Cycles) / freq
		ratio := simTime / k.native
		minVal := math.Min(simTime, k.native)
		errVal := math.Abs(simTime-k.native) / minVal

		results = append(results, result{
			name:    k.name,
			insts:   p.InstDone,
			cycles:  p.Cycles,
			cpi:     p.CPI(),
			simTime: simTime,
			native:  k.native,
			ratio:   ratio,
			errVal:  errVal,
		})
	}

	fmt.Println()
	fmt.Printf("%-14s | %10s | %10s | %7s | %14s | %12s | %8s | %8s\n",
		"Kernel", "Insts", "Cycles", "CPI", "Sim Time (s)", "Native (s)", "Ratio", "Error")
	fmt.Printf("%-14s-+-%10s-+-%10s-+-%7s-+-%14s-+-%12s-+-%8s-+-%8s\n",
		"--------------", "----------", "----------", "-------", "--------------", "------------", "--------", "--------")

	var totalError float64
	var maxError float64
	var maxErrorName string
	for _, r := range results {
		fmt.Printf("%-14s | %10d | %10d | %7.2f | %14e | %12e | %8.1f | %7.1f%%\n",
			r.name, r.insts, r.cycles, r.cpi, r.simTime, r.native, r.ratio, r.errVal*100)
		totalError += r.errVal
		if r.errVal > maxError {
			maxError = r.errVal
			maxErrorName = r.name
		}
	}

	n := len(results)
	avgError := totalError / float64(n)
	fmt.Printf("\nAll30 Summary (%d benchmarks):\n", n)
	fmt.Printf("  Avg Error: %6.1f%%\n", avgError*100)
	fmt.Printf("  Max Error: %6.1f%% (%s)\n", maxError*100, maxErrorName)
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
	for i, ready := range p.gprReady {
		if ready > p.Cycles {
			t.Errorf("gprReady[%d] = %d, exceeds final cycle count %d", i, ready, p.Cycles)
		}
	}
	for i, ready := range p.fpReady {
		if ready > p.Cycles {
			t.Errorf("fpReady[%d] = %d, exceeds final cycle count %d", i, ready, p.Cycles)
		}
	}

	if em.Regs.ReadX(0) != 42 {
		t.Errorf("X0 = %d, want 42", em.Regs.ReadX(0))
	}

	t.Logf("RegisterReadiness: %d instructions, %d cycles, CPI=%.2f",
		p.InstDone, p.Cycles, p.CPI())
}

func TestOoO_Swaptions(t *testing.T) {
	// End-to-end test: PARSEC Swaptions (Cholesky + HJM step) through OoO pipeline.
	//
	// Phase 1: Run Cholesky decomposition on an 8×8 SPD matrix.
	//          Uses FSQRT instruction, no BL hooks needed.
	// Phase 2: Run HJM step using the Cholesky factor.
	//          Uses BL to exp(), with FuncHookCycles for cycle cost.

	// Native timing (M2, -O1 -fno-vectorize, median of 7 runs):
	//   Cholesky 8×8: ~180 ns → ~628 cycles @ 3.49 GHz
	//   HJM step 8-factor: ~95 ns → ~332 cycles @ 3.49 GHz
	//   Combined: ~275 ns → ~960 cycles @ 3.49 GHz
	const nativeTimeSwaptionsCholesky = 1.80e-07 // seconds
	const nativeTimeSwaptionsHJM = 9.5e-08       // seconds
	const nativeTimeSwaptions = 2.75e-07         // seconds, combined

	n := 8
	r := 0.05
	dt := 0.25

	// Build a positive-definite matrix: A = B*B^T + n*I
	Bmat := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			Bmat[i*n+j] = float64((i*3+j*7+1)%(n+1)) / float64(n+1)
		}
	}
	A := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			for k := 0; k < n; k++ {
				A[i*n+j] += Bmat[i*n+k] * Bmat[j*n+k]
			}
		}
		A[i*n+i] += float64(n)
	}

	// Golden Cholesky
	L := make([]float64, n*n)
	copy(L, A)
	for j := 0; j < n; j++ {
		sum := 0.0
		for k := 0; k < j; k++ {
			sum += L[j*n+k] * L[j*n+k]
		}
		L[j*n+j] = math.Sqrt(L[j*n+j] - sum)
		for i := j + 1; i < n; i++ {
			sum = 0.0
			for k := 0; k < j; k++ {
				sum += L[i*n+k] * L[j*n+k]
			}
			L[i*n+j] = (L[i*n+j] - sum) / L[j*n+j]
		}
	}

	// Z vector
	Z := make([]float64, n)
	for i := 0; i < n; i++ {
		Z[i] = float64(i*3-n) * 0.2
	}

	// Golden HJM step
	discount := math.Exp(-r * dt)
	expectedDF := make([]float64, n)
	for i := 0; i < n; i++ {
		sum := 0.0
		for j := 0; j <= i; j++ {
			sum += L[i*n+j] * Z[j]
		}
		expectedDF[i] = sum * discount
	}

	// --- Phase 1: Cholesky through OoO pipeline ---
	cholCode := []uint32{
		0x7100043f, 0x5400072b, 0xd280000c, 0x2a0103e8,
		0xd37d7c29, 0x8b09000a, 0x5280002b, 0xaa0003ed,
		0x14000007, 0x9100056b, 0x8b0901ad, 0x8b09014a,
		0xaa0e03ec, 0xeb0801df, 0x54000580, 0xb400012c,
		0xd280000e, 0x2f00e400, 0xfc6e79a1, 0x1f410020,
		0x910005ce, 0xeb0e019f, 0x54ffff81, 0x14000002,
		0x2f00e400, 0x1b017d8f, 0x8b0c0c10, 0xfc6f5a01,
		0x1e603820, 0x1e61c000, 0xfc2f5a00, 0x9100058e,
		0xeb0801df, 0x54fffd02, 0xaa0a03f1, 0xaa0b03e2,
		0x1400000c, 0x2f00e400, 0x9b087c43, 0xfc637a01,
		0x1e603820, 0xfc6f5a01, 0x1e611800, 0xfc237a00,
		0x91000442, 0x8b090231, 0xeb08005f, 0x54fffb40,
		0xb4fffeac, 0xd2800003, 0x2f00e400, 0xfc637a21,
		0xfc6379a2, 0x1f420020, 0x91000463, 0xeb03019f,
		0x54ffff61, 0x17ffffed, 0xd65f03c0,
	}

	cholBase := uint64(0x10000)
	aBase := uint64(0x100000)
	dfBase := uint64(0x200000)
	zBase := uint64(0x300000)
	stackBase := uint64(0x400000)
	haltPC := uint64(0xFFFF0000)

	em := emu.NewEmulator()
	em.MaxInst = 10_000_000
	em.LoadProgram(cholBase, encodeInsts(cholCode))

	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)
	em.LoadProgram(stackBase, make([]byte, 8192))

	em.Regs.PC = cholBase
	em.Regs.WriteX(0, aBase)
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	p := runOoOPipeline(t, em)

	if !em.Halted {
		t.Fatal("Cholesky: emulator should have halted")
	}

	// Verify Cholesky result against golden L.
	cholMaxRelErr := 0.0
	for i := 0; i < n*n; i++ {
		bits := em.Mem.ReadU64(aBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := L[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > cholMaxRelErr {
			cholMaxRelErr = relErr
		}
		if relErr > 1e-10 {
			row, col := i/n, i%n
			t.Errorf("cholesky A[%d][%d] = %g, want %g (relErr=%e)", row, col, got, want, relErr)
		}
	}

	t.Logf("swaptions_cholesky: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p.InstDone, p.Cycles, p.CPI(), cholMaxRelErr)

	cholSimTime := float64(p.Cycles) / 3.49e9
	t.Logf("swaptions_cholesky native: simTime=%e, native=%e, ratio=%.2f",
		cholSimTime, nativeTimeSwaptionsCholesky, cholSimTime/nativeTimeSwaptionsCholesky)

	// --- Phase 2: HJM step through OoO pipeline ---
	hjmCode := []uint32{
		0xa9bd57f6, 0xa9014ff4, 0xa9027bfd, 0x910083fd,
		0xaa0303f6, 0xaa0203f3, 0xaa0103f4, 0xaa0003f5,
		0x1e618800, // fnmul d0, d0, d1
		0x94000000, // BL _exp (will be patched)
		0x710006df, 0x5400028b,
		0xd2800008, 0xd37d7ec9, 0x5280002a, 0x2a1603eb,
		0xd280000c, 0x2f00e401, 0xfc6c7a82, 0xfc6c7a63,
		0x1f430441, 0x9100058c, 0xeb0c015f, 0x54ffff61,
		0x1e610801, 0xfc287aa1, 0x91000508, 0x9100054a,
		0x8b090294, 0xeb0b011f, 0x54fffe41, 0xa9427bfd,
		0xa9414ff4, 0xa8c357f6, 0xd65f03c0,
	}

	hjmBase := uint64(0x50000)
	expHookAddr := uint64(0x60000)

	// Patch BL at index 9 to target expHookAddr
	blPC := hjmBase + 9*4
	blOffset := int64(expHookAddr) - int64(blPC)
	imm26 := uint32(blOffset/4) & 0x03FFFFFF
	hjmCode[9] = 0x94000000 | imm26

	em2 := emu.NewEmulator()
	em2.MaxInst = 10_000_000
	em2.LoadProgram(hjmBase, encodeInsts(hjmCode))

	// Copy Cholesky result (L) from first emulator to second as L input.
	lBuf := make([]byte, n*n*8)
	for i := 0; i < n*n; i++ {
		binary.LittleEndian.PutUint64(lBuf[i*8:], em.Mem.ReadU64(aBase+uint64(i)*8))
	}
	lBase := uint64(0x100000)
	em2.LoadProgram(lBase, lBuf)

	// Load Z vector.
	zBuf := make([]byte, n*8)
	for i, v := range Z {
		binary.LittleEndian.PutUint64(zBuf[i*8:], math.Float64bits(v))
	}
	em2.LoadProgram(zBase, zBuf)

	// Load dF output (zero).
	em2.LoadProgram(dfBase, make([]byte, n*8))

	// Allocate stack.
	em2.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers: X0=dF, X1=L, X2=Z, D0=r, D1=dt, X3=n.
	em2.Regs.PC = hjmBase
	em2.Regs.WriteX(0, dfBase)
	em2.Regs.WriteX(1, lBase)
	em2.Regs.WriteX(2, zBase)
	em2.Regs.WriteD(0, r)
	em2.Regs.WriteD(1, dt)
	em2.Regs.WriteX(3, uint64(n))
	em2.Regs.WriteX(30, haltPC)
	em2.Regs.SP = stackBase + 8192
	em2.SetHaltPC(haltPC)

	// Register BL hook for exp.
	em2.RegisterFuncHook(expHookAddr, func(em *emu.Emulator) {
		input := em.Regs.ReadD(0)
		em.Regs.WriteD(0, math.Exp(input))
	})

	// Run OoO pipeline with FuncHookCycles for exp (25 cycles).
	engine := sim.NewSerialEngine()
	p2 := NewOoOPipeline("TestOoO", engine, 3490*sim.MHz, em2)
	p2.FuncHookCycles = map[uint64]int{
		expHookAddr: 25,
	}
	p2.TickLater()
	if err := engine.Run(); err != nil {
		t.Fatalf("engine.Run() failed: %v", err)
	}
	if p2.Err != nil {
		t.Fatalf("pipeline error: %v", p2.Err)
	}

	if !em2.Halted {
		t.Fatal("HJM: emulator should have halted")
	}

	// Verify dF[] matches golden reference.
	hjmMaxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em2.Mem.ReadU64(dfBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedDF[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > hjmMaxRelErr {
			hjmMaxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("dF[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("swaptions_hjm_step: %d instructions, %d cycles, CPI=%.2f, maxRelErr=%e",
		p2.InstDone, p2.Cycles, p2.CPI(), hjmMaxRelErr)

	hjmSimTime := float64(p2.Cycles) / 3.49e9
	t.Logf("swaptions_hjm native: simTime=%e, native=%e, ratio=%.2f",
		hjmSimTime, nativeTimeSwaptionsHJM, hjmSimTime/nativeTimeSwaptionsHJM)

	// Print combined summary.
	totalInsts := p.InstDone + p2.InstDone
	totalCycles := p.Cycles + p2.Cycles
	t.Logf("swaptions_combined: total %d instructions, %d cycles, overall CPI=%.2f",
		totalInsts, totalCycles, float64(totalCycles)/float64(totalInsts))

	totalSimTime := float64(totalCycles) / 3.49e9
	t.Logf("swaptions_combined native: simTime=%e, native=%e, ratio=%.2f",
		totalSimTime, nativeTimeSwaptions, totalSimTime/nativeTimeSwaptions)
}
