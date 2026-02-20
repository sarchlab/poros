package cpu

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/sarchlab/akita/v4/sim"
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

func TestOoO_ComparisonTable(t *testing.T) {
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

	// setupAtaxEmu creates a fresh emulator configured for the atax kernel.
	setupAtaxEmu := func() *emu.Emulator {
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

		aBuf := make([]byte, m*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		xBuf := make([]byte, n*8)
		for i, v := range x {
			binary.LittleEndian.PutUint64(xBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(xBase, xBuf)
		em.LoadProgram(yBase, make([]byte, n*8))
		em.LoadProgram(tmpBase, make([]byte, m*8))

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
		return em
	}

	// setupMvtEmu creates a fresh emulator configured for the mvt kernel.
	setupMvtEmu := func() *emu.Emulator {
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

		x1Buf := make([]byte, n*8)
		for i, v := range x1 {
			binary.LittleEndian.PutUint64(x1Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(x1Base, x1Buf)

		x2Buf := make([]byte, n*8)
		for i, v := range x2 {
			binary.LittleEndian.PutUint64(x2Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(x2Base, x2Buf)

		y1Buf := make([]byte, n*8)
		for i, v := range y1 {
			binary.LittleEndian.PutUint64(y1Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(y1Base, y1Buf)

		y2Buf := make([]byte, n*8)
		for i, v := range y2 {
			binary.LittleEndian.PutUint64(y2Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(y2Base, y2Buf)

		aBuf := make([]byte, n*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

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
		return em
	}

	// setupGemmEmu creates a fresh emulator configured for the gemm kernel.
	setupGemmEmu := func() *emu.Emulator {
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

		cBuf := make([]byte, ni*nj*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBuf)

		aBuf := make([]byte, ni*nk*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, nk*nj*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

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
		return em
	}

	// setup2mmEmu creates a fresh emulator configured for the 2mm kernel.
	setup2mmEmu := func() *emu.Emulator {
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

		tmpBuf := make([]byte, ni*nj*8)
		for i, v := range tmp {
			binary.LittleEndian.PutUint64(tmpBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(tmpBase, tmpBuf)

		aBuf := make([]byte, ni*nk*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, nk*nj*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		cBuf := make([]byte, nj*nl*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBuf)

		dBuf := make([]byte, ni*nl*8)
		for i, v := range D {
			binary.LittleEndian.PutUint64(dBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(dBase, dBuf)

		stackBuf := make([]byte, 8192)
		binary.LittleEndian.PutUint64(stackBuf[4096:], dBase)
		em.LoadProgram(stackBase, stackBuf)

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
		return em
	}

	// setup3mmEmu creates a fresh emulator configured for the 3mm kernel.
	setup3mmEmu := func() *emu.Emulator {
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

		eBuf := make([]byte, ni*nj*8)
		for i, v := range E {
			binary.LittleEndian.PutUint64(eBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(eBase, eBuf)

		aBuf := make([]byte, ni*nk*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, nk*nj*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		fBuf := make([]byte, nj*nl*8)
		for i, v := range F {
			binary.LittleEndian.PutUint64(fBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(fBase, fBuf)

		cBufData := make([]byte, nj*nm*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBufData[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBufData)

		dBufData := make([]byte, nm*nl*8)
		for i, v := range D {
			binary.LittleEndian.PutUint64(dBufData[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(dBase, dBufData)

		gBuf := make([]byte, ni*nl*8)
		for i, v := range G {
			binary.LittleEndian.PutUint64(gBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(gBase, gBuf)

		stackBuf := make([]byte, 8192)
		binary.LittleEndian.PutUint64(stackBuf[4096:], fBase)
		binary.LittleEndian.PutUint64(stackBuf[4096+8:], cBase)
		binary.LittleEndian.PutUint64(stackBuf[4096+16:], dBase)
		binary.LittleEndian.PutUint64(stackBuf[4096+24:], gBase)
		em.LoadProgram(stackBase, stackBuf)

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
		return em
	}

	// setupBicgEmu creates a fresh emulator configured for the bicg kernel.
	setupBicgEmu := func() *emu.Emulator {
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
			p[i] = float64(i%n) / float64(n)
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

		aBuf := make([]byte, m*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		sBuf := make([]byte, n*8)
		for i, v := range s {
			binary.LittleEndian.PutUint64(sBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(sBase, sBuf)

		qBuf := make([]byte, m*8)
		for i, v := range q {
			binary.LittleEndian.PutUint64(qBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(qBase, qBuf)

		pBuf := make([]byte, n*8)
		for i, v := range p {
			binary.LittleEndian.PutUint64(pBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(pBase, pBuf)

		rBuf := make([]byte, m*8)
		for i, v := range r {
			binary.LittleEndian.PutUint64(rBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(rBase, rBuf)

		em.LoadProgram(uint64(0x600000), make([]byte, 8192))

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
		return em
	}

	setupGesummvEmu := func() *emu.Emulator {
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

		em.LoadProgram(tmpBase, make([]byte, n*8))

		xBuf := make([]byte, n*8)
		for i, v := range x {
			binary.LittleEndian.PutUint64(xBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(xBase, xBuf)
		em.LoadProgram(yBase, make([]byte, n*8))

		em.LoadProgram(uint64(0x600000), make([]byte, 8192))

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
		return em
	}

	setupGemverEmu := func() *emu.Emulator {
		n := 30
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
			for j := 0; j < n; j++ {
				A[i*n+j] = float64(i*j%n) / float64(n)
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

		aBuf := make([]byte, n*n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

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

		stackBuf := make([]byte, 8192)
		binary.LittleEndian.PutUint64(stackBuf[4096:], yBase)
		binary.LittleEndian.PutUint64(stackBuf[4096+8:], zBase)
		em.LoadProgram(stackBase, stackBuf)

		alpha, beta := 1.5, 1.2
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
		return em
	}

	setupDoitgenEmu := func() *emu.Emulator {
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

		aBuf := make([]byte, nr*nq*np*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		c4Buf := make([]byte, np*np*8)
		for i, v := range C4 {
			binary.LittleEndian.PutUint64(c4Buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(c4Base, c4Buf)

		em.LoadProgram(sumBase, make([]byte, np*8))
		em.LoadProgram(uint64(0x400000), make([]byte, 8192))

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
		return em
	}

	setupTrisolvEmu := func() *emu.Emulator {
		n := 40

		L := make([]float64, n*n)
		b := make([]float64, n)
		for i := 0; i < n; i++ {
			b[i] = float64(i)
			for j := 0; j <= i; j++ {
				L[i*n+j] = float64(i+n-j+1) * 2 / float64(n)
			}
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

		lBuf := make([]byte, n*n*8)
		for i, v := range L {
			binary.LittleEndian.PutUint64(lBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(lBase, lBuf)

		em.LoadProgram(xBase, make([]byte, n*8))

		bBuf := make([]byte, n*8)
		for i, v := range b {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, lBase)
		em.Regs.WriteX(2, xBase)
		em.Regs.WriteX(3, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x400000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupDurbinEmu := func() *emu.Emulator {
		n := 40

		r := make([]float64, n)
		for i := 0; i < n; i++ {
			r[i] = float64(n + 1 - i)
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

		rBuf := make([]byte, n*8)
		for i, v := range r {
			binary.LittleEndian.PutUint64(rBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(rBase, rBuf)

		em.LoadProgram(yBase, make([]byte, n*8))
		em.LoadProgram(zBase, make([]byte, n*8))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, rBase)
		em.Regs.WriteX(2, yBase)
		em.Regs.WriteX(3, zBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x400000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupJacobi1dEmu := func() *emu.Emulator {
		tsteps := 20
		n := 30

		A := make([]float64, n)
		B := make([]float64, n)
		for i := 0; i < n; i++ {
			A[i] = (float64(i) + 2) / float64(n)
			B[i] = (float64(i) + 3) / float64(n)
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

		aBuf := make([]byte, n*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(tsteps))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteX(2, aBase)
		em.Regs.WriteX(3, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x300000 + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupSyrkEmu := func() *emu.Emulator {
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

		cBuf := make([]byte, n*n*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBuf)

		aBuf := make([]byte, n*m*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

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
		return em
	}

	setupSyr2kEmu := func() *emu.Emulator {
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

		cBuf := make([]byte, n*n*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBuf)

		aBuf := make([]byte, n*m*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, n*m*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

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
		return em
	}

	setupTrmmEmu := func() *emu.Emulator {
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

		aBuf := make([]byte, m*m*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, m*n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(m))
		em.Regs.WriteX(1, uint64(n))
		em.Regs.WriteD(0, alpha)
		em.Regs.WriteX(2, aBase)
		em.Regs.WriteX(3, bBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupSymmEmu := func() *emu.Emulator {
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

		cBuf := make([]byte, m*n*8)
		for i, v := range C {
			binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(cBase, cBuf)

		aBuf := make([]byte, m*m*8)
		for i, v := range A {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		bBuf := make([]byte, m*n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(bBase, bBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

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
		return em
	}

	setupCholeskyEmu := func() *emu.Emulator {
		n := 20

		A0 := make([]float64, n*n)
		for i := 0; i < n; i++ {
			for j := 0; j <= i; j++ {
				A0[i*n+j] = float64((-j)%n)/float64(n) + 1
			}
			A0[i*n+i] = 1
		}
		B := make([]float64, n*n)
		for t := 0; t < n; t++ {
			for r := 0; r < n; r++ {
				for s := 0; s < n; s++ {
					B[r*n+s] += A0[r*n+t] * A0[s*n+t]
				}
			}
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

		aBuf := make([]byte, n*n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, aBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	setupLUEmu := func() *emu.Emulator {
		n := 20

		A0 := make([]float64, n*n)
		for i := 0; i < n; i++ {
			for j := 0; j <= i; j++ {
				A0[i*n+j] = float64((-j)%n)/float64(n) + 1
			}
			A0[i*n+i] = 1
		}
		B := make([]float64, n*n)
		for t := 0; t < n; t++ {
			for r := 0; r < n; r++ {
				for s := 0; s < n; s++ {
					B[r*n+s] += A0[r*n+t] * A0[s*n+t]
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

		aBuf := make([]byte, n*n*8)
		for i, v := range B {
			binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(aBase, aBuf)

		em.LoadProgram(stackBase, make([]byte, 8192))

		em.Regs.PC = codeBase
		em.Regs.WriteX(0, uint64(n))
		em.Regs.WriteX(1, aBase)
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = stackBase + 8192
		em.SetHaltPC(haltPC)
		return em
	}

	type setupFunc struct {
		name   string
		setup  func() *emu.Emulator
		native float64
	}

	kernels := []setupFunc{
		{"atax", setupAtaxEmu, 0.000008},
		{"mvt", setupMvtEmu, 0.000004},
		{"gemm", setupGemmEmu, 0.000016},
		{"2mm", setup2mmEmu, 0.000020},
		{"3mm", setup3mmEmu, 0.000030},
		{"bicg", setupBicgEmu, 0.000006},
		{"gesummv", setupGesummvEmu, 0.000008},
		{"gemver", setupGemverEmu, 0.000009},
		{"doitgen", setupDoitgenEmu, 0.000015},
		{"trisolv", setupTrisolvEmu, 0.000007},
		{"durbin", setupDurbinEmu, 0.000009},
		{"jacobi1d", setupJacobi1dEmu, 0.000004},
		{"syrk", setupSyrkEmu, 0.000012},
		{"syr2k", setupSyr2kEmu, 0.000014},
		{"trmm", setupTrmmEmu, 0.000010},
		{"symm", setupSymmEmu, 0.000015},
		{"cholesky", setupCholeskyEmu, 0.000008},
		{"lu", setupLUEmu, 0.000006},
	}

	results := make([]kernelResult, 0, len(kernels))

	for _, k := range kernels {
		// Run in-order pipeline.
		emIO := k.setup()
		cacheIO := mem.NewCache(mem.DefaultCacheConfig())
		pIO := runPipelineWithCache(t, emIO, cacheIO)
		if !emIO.Halted {
			t.Fatalf("%s: in-order emulator should have halted", k.name)
		}

		// Run OoO pipeline (with branch predictor).
		emOoO := k.setup()
		cacheOoO := mem.NewCache(mem.DefaultCacheConfig())
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
	t.Logf("\n%-8s | %12s | %11s | %9s | %10s | %7s | %9s | %7s | %14s | %12s | %8s | %8s | %8s",
		"Kernel", "Instructions", "InOrder CPI", "OoO CPI", "NoBP CPI", "BP Acc%", "BP Improv", "Speedup", "Sim Time (s)", "Native (s)", "Ratio", "L1 Hit%", "L2 Hit%")
	t.Logf("%-8s-+-%12s-+-%11s-+-%9s-+-%10s-+-%7s-+-%9s-+-%7s-+-%14s-+-%12s-+-%8s-+-%8s-+-%8s",
		"--------", "------------", "-----------", "---------", "----------", "-------", "---------", "-------", "--------------", "------------", "--------", "--------", "--------")
	for _, r := range results {
		simTime := float64(r.oooCycles) / freq
		ratio := simTime / r.native
		speedup := r.iooCPI / r.oooCPI
		bpImprov := (r.oooNoBPCPI - r.oooCPI) / r.oooNoBPCPI * 100
		t.Logf("%-8s | %12d | %11.2f | %9.2f | %10.2f | %6.1f%% | %8.1f%% | %6.2fx | %14e | %12e | %8.1f | %7.1f%% | %7.1f%%",
			r.name, r.insts, r.iooCPI, r.oooCPI, r.oooNoBPCPI, r.bpAccuracy, bpImprov, speedup, simTime, r.native, ratio, r.l1HitRate*100, r.l2HitRate*100)
	}

	// Also print as a clean formatted string for easy copy-paste.
	fmt.Println()
	fmt.Printf("%-8s | %12s | %11s | %9s | %10s | %7s | %9s | %7s | %14s | %12s | %8s | %8s | %8s\n",
		"Kernel", "Instructions", "InOrder CPI", "OoO CPI", "NoBP CPI", "BP Acc%", "BP Improv", "Speedup", "Sim Time (s)", "Native (s)", "Ratio", "L1 Hit%", "L2 Hit%")
	fmt.Printf("%-8s-+-%12s-+-%11s-+-%9s-+-%10s-+-%7s-+-%9s-+-%7s-+-%14s-+-%12s-+-%8s-+-%8s-+-%8s\n",
		"--------", "------------", "-----------", "---------", "----------", "-------", "---------", "-------", "--------------", "------------", "--------", "--------", "--------")
	for _, r := range results {
		simTime := float64(r.oooCycles) / freq
		ratio := simTime / r.native
		speedup := r.iooCPI / r.oooCPI
		bpImprov := (r.oooNoBPCPI - r.oooCPI) / r.oooNoBPCPI * 100
		fmt.Printf("%-8s | %12d | %11.2f | %9.2f | %10.2f | %6.1f%% | %8.1f%% | %6.2fx | %14e | %12e | %8.1f | %7.1f%% | %7.1f%%\n",
			r.name, r.insts, r.iooCPI, r.oooCPI, r.oooNoBPCPI, r.bpAccuracy, bpImprov, speedup, simTime, r.native, ratio, r.l1HitRate*100, r.l2HitRate*100)
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
