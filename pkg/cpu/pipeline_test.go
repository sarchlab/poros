package cpu

import (
	"encoding/binary"
	"fmt"
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

func TestPipeline_ComparisonTable(t *testing.T) {
	const freq = 3.49e9 // M2 Avalanche core frequency

	type kernelResult struct {
		name   string
		insts  uint64
		cycles uint64
		cpi    float64
		native float64 // native time in seconds
	}

	results := make([]kernelResult, 0, 3)

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

		p := runPipeline(t, em)
		if !em.Halted {
			t.Fatal("atax: emulator should have halted")
		}
		results = append(results, kernelResult{"atax", p.InstDone, p.Cycles, p.CPI(), 0.000008})
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

		p := runPipeline(t, em)
		if !em.Halted {
			t.Fatal("mvt: emulator should have halted")
		}
		results = append(results, kernelResult{"mvt", p.InstDone, p.Cycles, p.CPI(), 0.000004})
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

		p := runPipeline(t, em)
		if !em.Halted {
			t.Fatal("gemm: emulator should have halted")
		}
		results = append(results, kernelResult{"gemm", p.InstDone, p.Cycles, p.CPI(), 0.000016})
	}

	// Print comparison table.
	t.Logf("\n%-8s | %12s | %10s | %5s | %14s | %12s | %8s",
		"Kernel", "Instructions", "Cycles", "CPI", "Sim Time (s)", "Native (s)", "Ratio")
	t.Logf("%-8s-+-%12s-+-%10s-+-%5s-+-%14s-+-%12s-+-%8s",
		"--------", "------------", "----------", "-----", "--------------", "------------", "--------")
	for _, r := range results {
		simTime := float64(r.cycles) / freq
		ratio := simTime / r.native
		t.Logf("%-8s | %12d | %10d | %5.2f | %14e | %12e | %8.1f",
			r.name, r.insts, r.cycles, r.cpi, simTime, r.native, ratio)
	}

	// Also print as a clean formatted string for easy copy-paste.
	fmt.Println()
	fmt.Printf("%-8s | %12s | %10s | %5s | %14s | %12s | %8s\n",
		"Kernel", "Instructions", "Cycles", "CPI", "Sim Time (s)", "Native (s)", "Ratio")
	fmt.Printf("%-8s-+-%12s-+-%10s-+-%5s-+-%14s-+-%12s-+-%8s\n",
		"--------", "------------", "----------", "-----", "--------------", "------------", "--------")
	for _, r := range results {
		simTime := float64(r.cycles) / freq
		ratio := simTime / r.native
		fmt.Printf("%-8s | %12d | %10d | %5.2f | %14e | %12e | %8.1f\n",
			r.name, r.insts, r.cycles, r.cpi, simTime, r.native, ratio)
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
