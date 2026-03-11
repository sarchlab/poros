package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestParsecSwaptions_Cholesky(t *testing.T) {
	// End-to-end test: Swaptions Cholesky decomposition kernel.
	//
	// The kernel computes the Cholesky decomposition of an n×n symmetric
	// positive-definite matrix A, overwriting the lower triangle with L
	// such that A = L*L^T.
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops
	//
	// Native timing reference (M2, -O1 -fno-vectorize, median of 7 runs):
	//   Cholesky 8×8: ~180 ns = 1.80e-07 seconds
	//
	// Calling convention (ARM64 AAPCS):
	//   X0=A (pointer to double), W1=n (int)

	n := 8

	// Build a positive-definite matrix: A = B*B^T + n*I
	B := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			B[i*n+j] = float64((i*3+j*7+1)%(n+1)) / float64(n+1)
		}
	}
	A := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			for k := 0; k < n; k++ {
				A[i*n+j] += B[i*n+k] * B[j*n+k]
			}
		}
		A[i*n+i] += float64(n) // ensure positive definiteness
	}

	// Golden reference: Cholesky decomposition
	expectedA := make([]float64, n*n)
	copy(expectedA, A)
	for j := 0; j < n; j++ {
		sum := 0.0
		for k := 0; k < j; k++ {
			sum += expectedA[j*n+k] * expectedA[j*n+k]
		}
		expectedA[j*n+j] = math.Sqrt(expectedA[j*n+j] - sum)

		for i := j + 1; i < n; i++ {
			sum = 0.0
			for k := 0; k < j; k++ {
				sum += expectedA[i*n+k] * expectedA[j*n+k]
			}
			expectedA[i*n+j] = (expectedA[i*n+j] - sum) / expectedA[j*n+j]
		}
	}

	// Machine code: cholesky function (59 instructions)
	// Compiled from parsec/kernels/swaptions.c
	// Uses FSQRT instruction (no BL to sqrt needed)
	code := []uint32{
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

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	stackBase := uint64(0x200000)
	haltPC := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 10_000_000

	// Load program
	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	// Load A matrix
	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Allocate stack space
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers per ARM64 ABI: X0=A, W1=n
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, aBase)
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify A[] matches golden reference (lower triangle)
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

	t.Logf("swaptions_cholesky completed in %d instructions, max relative error = %e",
		em.InstCount, maxRelErr)
}

func TestParsecSwaptions_HJMStep(t *testing.T) {
	// End-to-end test: Swaptions HJM step kernel.
	//
	// The kernel computes:
	//   discount = exp(-r * dt)
	//   for i in 0..n:
	//     sum = 0
	//     for j in 0..i: sum += L[i*n+j] * Z[j]
	//     dF[i] = sum * discount
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops
	//
	// Native timing reference (M2, -O1 -fno-vectorize, median of 7 runs):
	//   HJM step 8-factor: ~95 ns = 9.5e-08 seconds
	//
	// Calling convention (ARM64 AAPCS):
	//   X0=dF, X1=L, X2=Z, D0=r, D1=dt, W3=n
	//
	// Contains BL to exp at offset 0x24 from function start.

	n := 8
	r := 0.05
	dt := 0.25

	// Initialize L (lower triangular, from a simple pattern)
	L := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			L[i*n+j] = float64(i-j+1) * 0.1
		}
	}

	// Initialize Z (random normals, using a simple deterministic pattern)
	Z := make([]float64, n)
	for i := 0; i < n; i++ {
		Z[i] = float64(i*3-n) * 0.2
	}

	// Golden reference
	discount := math.Exp(-r * dt)
	expectedDF := make([]float64, n)
	for i := 0; i < n; i++ {
		sum := 0.0
		for j := 0; j <= i; j++ {
			sum += L[i*n+j] * Z[j]
		}
		expectedDF[i] = sum * discount
	}

	// Machine code: hjm_step function (35 instructions)
	// BL to exp is at index 9 (offset 0x24 from function start).
	// The BL target is patched to point to expHookAddr.
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

	hjmBase := uint64(0x10000)
	expHookAddr := uint64(0x20000)
	dfBase := uint64(0x100000)
	lBase := uint64(0x200000)
	zBase := uint64(0x300000)
	stackBase := uint64(0x400000)
	haltPC := uint64(0xFFFF0000)

	// Patch BL at index 9 to target expHookAddr
	blPC := hjmBase + 9*4 // 0x10024
	blOffset := int64(expHookAddr) - int64(blPC)
	imm26 := uint32(blOffset/4) & 0x03FFFFFF
	hjmCode[9] = 0x94000000 | imm26

	em := NewEmulator()
	em.MaxInst = 10_000_000

	// Load program
	codeBuf := make([]byte, len(hjmCode)*4)
	for i, inst := range hjmCode {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(hjmBase, codeBuf)

	// Load dF (output, initialize to zero)
	dfBuf := make([]byte, n*8)
	em.LoadProgram(dfBase, dfBuf)

	// Load L matrix
	lBuf := make([]byte, n*n*8)
	for i, v := range L {
		binary.LittleEndian.PutUint64(lBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(lBase, lBuf)

	// Load Z vector
	zBuf := make([]byte, n*8)
	for i, v := range Z {
		binary.LittleEndian.PutUint64(zBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(zBase, zBuf)

	// Allocate stack space
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers per ARM64 ABI: X0=dF, X1=L, X2=Z, D0=r, D1=dt, W3=n
	em.Regs.PC = hjmBase
	em.Regs.WriteX(0, dfBase)
	em.Regs.WriteX(1, lBase)
	em.Regs.WriteX(2, zBase)
	em.Regs.WriteD(0, r)
	em.Regs.WriteD(1, dt)
	em.Regs.WriteX(3, uint64(n))
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	// Register BL hook for exp: D0 → D0
	em.RegisterFuncHook(expHookAddr, func(em *Emulator) {
		input := em.Regs.ReadD(0)
		em.Regs.WriteD(0, math.Exp(input))
	})

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Verify dF[] matches golden reference
	maxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(dfBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedDF[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("dF[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("swaptions_hjm_step completed in %d instructions, max relative error = %e",
		em.InstCount, maxRelErr)
}

func TestParsecSwaptions_Combined(t *testing.T) {
	// End-to-end test: Cholesky decomposition + HJM step.
	// First run Cholesky on the SPD matrix to get L, then run HJM step
	// using L as input.
	//
	// Native timing reference (M2, -O1 -fno-vectorize, median of 7 runs):
	//   Combined (Cholesky + HJM): ~275 ns = 2.75e-07 seconds

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

	// --- Run Cholesky first ---
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

	em := NewEmulator()
	em.MaxInst = 10_000_000

	codeBuf := make([]byte, len(cholCode)*4)
	for i, inst := range cholCode {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(cholBase, codeBuf)

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

	if err := em.Run(); err != nil {
		t.Fatalf("Cholesky Run() error: %v", err)
	}
	if !em.Halted {
		t.Fatal("Cholesky: emulator should have halted")
	}
	cholInsts := em.InstCount
	t.Logf("Cholesky completed in %d instructions", cholInsts)

	// --- Now run HJM step using the Cholesky result as L ---
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

	// Patch BL
	blPC := hjmBase + 9*4
	blOffset := int64(expHookAddr) - int64(blPC)
	imm26 := uint32(blOffset/4) & 0x03FFFFFF
	hjmCode[9] = 0x94000000 | imm26

	hjmBuf := make([]byte, len(hjmCode)*4)
	for i, inst := range hjmCode {
		binary.LittleEndian.PutUint32(hjmBuf[i*4:], inst)
	}
	em.LoadProgram(hjmBase, hjmBuf)

	// Load Z vector
	zBuf := make([]byte, n*8)
	for i, v := range Z {
		binary.LittleEndian.PutUint64(zBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(zBase, zBuf)

	// Load dF (output, zero)
	em.LoadProgram(dfBase, make([]byte, n*8))

	// Reset emulator state for second run
	em.Halted = false
	em.InstCount = 0

	em.Regs.PC = hjmBase
	em.Regs.WriteX(0, dfBase)
	em.Regs.WriteX(1, aBase) // A now contains L (Cholesky result)
	em.Regs.WriteX(2, zBase)
	em.Regs.WriteD(0, r)
	em.Regs.WriteD(1, dt)
	em.Regs.WriteX(3, uint64(n))
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	em.RegisterFuncHook(expHookAddr, func(em *Emulator) {
		input := em.Regs.ReadD(0)
		em.Regs.WriteD(0, math.Exp(input))
	})

	if err := em.Run(); err != nil {
		t.Fatalf("HJM Run() error: %v", err)
	}
	if !em.Halted {
		t.Fatal("HJM: emulator should have halted")
	}

	// Verify dF[] matches golden reference
	maxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(dfBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedDF[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("dF[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("swaptions_combined: cholesky=%d insts, hjm_step=%d insts, maxRelErr=%e",
		cholInsts, em.InstCount, maxRelErr)
}
