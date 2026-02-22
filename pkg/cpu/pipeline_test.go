package cpu

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/m2sim3/pkg/emu"
	"github.com/sarchlab/m2sim3/pkg/mem"
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

// helper: run pipeline with a cache model.
func runPipelineWithCache(t *testing.T, em *emu.Emulator, cache *mem.Cache) *Pipeline {
	t.Helper()
	engine := sim.NewSerialEngine()
	p := NewPipeline("TestPipeline", engine, 3490*sim.MHz, em)
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

func TestPipeline_PolyBenchMvt(t *testing.T) {
	// End-to-end PolyBench kernel_mvt through the pipeline.
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

	p := runPipeline(t, em)

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

func TestPipeline_PolyBenchGemm(t *testing.T) {
	// End-to-end PolyBench kernel_gemm through the pipeline.
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

	p := runPipeline(t, em)

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

func TestPipeline_PolyBench2mm(t *testing.T) {
	// End-to-end PolyBench kernel_2mm through the pipeline.
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

	p := runPipeline(t, em)

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

func TestPipeline_PolyBench3mm(t *testing.T) {
	// End-to-end PolyBench kernel_3mm through the pipeline.
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

	p := runPipeline(t, em)

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

func TestPipeline_PolyBenchBicg(t *testing.T) {
	// End-to-end PolyBench kernel_bicg through the pipeline.
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

	pip := runPipeline(t, em)

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

func TestPipeline_ComparisonTable(t *testing.T) {
	const freq = 3.49e9 // M2 Avalanche core frequency

	type kernelResult struct {
		name      string
		insts     uint64
		cycles    uint64
		cpi       float64
		native    float64 // native time in seconds
		l1HitRate float64
		l2HitRate float64
	}

	results := make([]kernelResult, 0, 6)

	// --- kernel_atax ---
	{
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

		cache := mem.NewCache(mem.DefaultCacheConfig())
		p := runPipelineWithCache(t, em, cache)
		if !em.Halted {
			t.Fatal("atax: emulator should have halted")
		}
		stats := cache.Stats()
		l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses)
		l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses)
		results = append(results, kernelResult{"atax", p.InstDone, p.Cycles, p.CPI(), 0.000008, l1Hit, l2Hit})
	}

	// --- kernel_mvt ---
	{
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

		cache := mem.NewCache(mem.DefaultCacheConfig())
		p := runPipelineWithCache(t, em, cache)
		if !em.Halted {
			t.Fatal("mvt: emulator should have halted")
		}
		stats := cache.Stats()
		l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses)
		l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses)
		results = append(results, kernelResult{"mvt", p.InstDone, p.Cycles, p.CPI(), 0.000004, l1Hit, l2Hit})
	}

	// --- kernel_gemm ---
	{
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

		cache := mem.NewCache(mem.DefaultCacheConfig())
		p := runPipelineWithCache(t, em, cache)
		if !em.Halted {
			t.Fatal("gemm: emulator should have halted")
		}
		stats := cache.Stats()
		l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses)
		l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses)
		results = append(results, kernelResult{"gemm", p.InstDone, p.Cycles, p.CPI(), 0.000016, l1Hit, l2Hit})
	}

	// --- kernel_2mm ---
	{
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

		cache := mem.NewCache(mem.DefaultCacheConfig())
		p := runPipelineWithCache(t, em, cache)
		if !em.Halted {
			t.Fatal("2mm: emulator should have halted")
		}
		stats := cache.Stats()
		l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses)
		l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses)
		results = append(results, kernelResult{"2mm", p.InstDone, p.Cycles, p.CPI(), 0.000020, l1Hit, l2Hit})
	}

	// --- kernel_3mm ---
	{
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

		cache := mem.NewCache(mem.DefaultCacheConfig())
		p := runPipelineWithCache(t, em, cache)
		if !em.Halted {
			t.Fatal("3mm: emulator should have halted")
		}
		stats := cache.Stats()
		l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses)
		l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses)
		results = append(results, kernelResult{"3mm", p.InstDone, p.Cycles, p.CPI(), 0.000030, l1Hit, l2Hit})
	}

	// --- kernel_bicg ---
	{
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

		cache := mem.NewCache(mem.DefaultCacheConfig())
		pip := runPipelineWithCache(t, em, cache)
		if !em.Halted {
			t.Fatal("bicg: emulator should have halted")
		}
		stats := cache.Stats()
		l1Hit := float64(stats.L1Hits) / float64(stats.L1Hits+stats.L1Misses)
		l2Hit := float64(stats.L2Hits) / float64(stats.L2Hits+stats.L2Misses)
		results = append(results, kernelResult{"bicg", pip.InstDone, pip.Cycles, pip.CPI(), 0.000006, l1Hit, l2Hit})
	}

	// Print comparison table.
	t.Logf("\n%-8s | %12s | %10s | %5s | %14s | %12s | %8s | %8s | %8s",
		"Kernel", "Instructions", "Cycles", "CPI", "Sim Time (s)", "Native (s)", "Ratio", "L1 Hit%", "L2 Hit%")
	t.Logf("%-8s-+-%12s-+-%10s-+-%5s-+-%14s-+-%12s-+-%8s-+-%8s-+-%8s",
		"--------", "------------", "----------", "-----", "--------------", "------------", "--------", "--------", "--------")
	for _, r := range results {
		simTime := float64(r.cycles) / freq
		ratio := simTime / r.native
		t.Logf("%-8s | %12d | %10d | %5.2f | %14e | %12e | %8.1f | %7.1f%% | %7.1f%%",
			r.name, r.insts, r.cycles, r.cpi, simTime, r.native, ratio, r.l1HitRate*100, r.l2HitRate*100)
	}

	// Also print as a clean formatted string for easy copy-paste.
	fmt.Println()
	fmt.Printf("%-8s | %12s | %10s | %5s | %14s | %12s | %8s | %8s | %8s\n",
		"Kernel", "Instructions", "Cycles", "CPI", "Sim Time (s)", "Native (s)", "Ratio", "L1 Hit%", "L2 Hit%")
	fmt.Printf("%-8s-+-%12s-+-%10s-+-%5s-+-%14s-+-%12s-+-%8s-+-%8s-+-%8s\n",
		"--------", "------------", "----------", "-----", "--------------", "------------", "--------", "--------", "--------")
	for _, r := range results {
		simTime := float64(r.cycles) / freq
		ratio := simTime / r.native
		fmt.Printf("%-8s | %12d | %10d | %5.2f | %14e | %12e | %8.1f | %7.1f%% | %7.1f%%\n",
			r.name, r.insts, r.cycles, r.cpi, simTime, r.native, ratio, r.l1HitRate*100, r.l2HitRate*100)
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
	for i, cnt := range p.gprPending {
		if cnt < 0 {
			t.Errorf("gprPending[%d] = %d, want >= 0", i, cnt)
		}
	}
	for i, cnt := range p.fpPending {
		if cnt < 0 {
			t.Errorf("fpPending[%d] = %d, want >= 0", i, cnt)
		}
	}

	// All counters should be exactly 0 after full drain.
	for i, cnt := range p.gprPending {
		if cnt != 0 {
			t.Errorf("gprPending[%d] = %d, want 0 after drain", i, cnt)
		}
	}
	for i, cnt := range p.fpPending {
		if cnt != 0 {
			t.Errorf("fpPending[%d] = %d, want 0 after drain", i, cnt)
		}
	}

	if em.Regs.ReadX(0) != 42 {
		t.Errorf("X0 = %d, want 42", em.Regs.ReadX(0))
	}

	t.Logf("ScoreboardFlush: %d instructions, %d cycles, CPI=%.2f",
		p.InstDone, p.Cycles, p.CPI())
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
		{arm64asm.FDIV, 22},
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
