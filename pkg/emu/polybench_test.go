package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPolyBenchAtax(t *testing.T) {
	// End-to-end test: PolyBench kernel_atax with MINI dataset (m=38, n=42).
	//
	// The kernel computes:
	//   for i in 0..m:
	//     tmp[i] = 0
	//     for j in 0..n: tmp[i] += A[i][j] * x[j]
	//     for j in 0..n: y[j] += A[i][j] * tmp[i]
	//
	// Compiled with: clang -target arm64 -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops
	// The y-init loop was removed (y is pre-zeroed in Go).
	//
	// Calling convention:
	//   X0 = m, X1 = n, X2 = A ptr, X3 = x ptr, X4 = y ptr, X5 = tmp ptr

	m, n := 38, 42

	// Initialize arrays matching PolyBench init_array
	A := make([]float64, m*n)
	x := make([]float64, n)
	y := make([]float64, n)
	tmp := make([]float64, m)

	for i := 0; i < n; i++ {
		x[i] = 1.0 + float64(i)/float64(n)
	}
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			A[i*n+j] = float64((i+j)%n) / float64(5*m)
		}
	}

	// Compute expected output (golden reference)
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

	// Machine code bytes (37 instructions)
	code := []uint32{
		0x7100041f, // CMP W0, #1
		0x5400046b, // B.LT #0x90
		0xd2800008, // MOV X8, #0
		0x2a0103e9, // MOV W9, W1
		0xd37d7c2a, // UBFIZ X10, X1, #3, #32
		0x2a0003eb, // MOV W11, W0
		0x14000005, // B #0x2c
		0x91000508, // ADD X8, X8, #1
		0x8b0a0042, // ADD X2, X2, X10
		0xeb0b011f, // CMP X8, X11
		0x54000340, // B.EQ #0x90
		0xf82878bf, // STR XZR, [X5, X8, LSL #3]
		0x7100043f, // CMP W1, #1
		0x54ffff4b, // B.LT #0x1c
		0x2f00e400, // MOVI D0, #0
		0xaa0903ec, // MOV X12, X9
		0xaa0303ed, // MOV X13, X3
		0xaa0203ee, // MOV X14, X2
		0xfc4085c1, // LDR D1, [X14], #8
		0xfc4085a2, // LDR D2, [X13], #8
		0x1f420020, // FMADD D0, D1, D2, D0
		0xfc2878a0, // STR D0, [X5, X8, LSL #3]
		0xf100058c, // SUBS X12, X12, #1
		0x54ffff61, // B.NE #0x48
		0x7100043f, // CMP W1, #1
		0x54fffdcb, // B.LT #0x1c
		0xd280000c, // MOV X12, #0
		0xfc6c6880, // LDR D0, [X4, X12]
		0xfc6c6841, // LDR D1, [X2, X12]
		0xfc6878a2, // LDR D2, [X5, X8, LSL #3]
		0x1f420020, // FMADD D0, D1, D2, D0
		0xfc2c6880, // STR D0, [X4, X12]
		0x9100218c, // ADD X12, X12, #8
		0xeb0c015f, // CMP X10, X12
		0x54ffff21, // B.NE #0x6c
		0x17ffffe4, // B #0x1c
		0xd65f03c0, // RET
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	xBase := uint64(0x200000)
	yBase := uint64(0x300000)
	tmpBase := uint64(0x400000)
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
	aBuf := make([]byte, m*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load x vector
	xBuf := make([]byte, n*8)
	for i, v := range x {
		binary.LittleEndian.PutUint64(xBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(xBase, xBuf)

	// Load y vector (zeroed)
	yBuf := make([]byte, n*8)
	em.LoadProgram(yBase, yBuf)

	// Load tmp vector (zeroed)
	tmpBuf := make([]byte, m*8)
	em.LoadProgram(tmpBase, tmpBuf)

	// Set registers per ARM64 ABI
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(m))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(2, aBase)
	em.Regs.WriteX(3, xBase)
	em.Regs.WriteX(4, yBase)
	em.Regs.WriteX(5, tmpBase)
	em.Regs.WriteX(30, haltPC) // LR = halt address
	em.Regs.SP = 0x500000 + 8192

	em.SetHaltPC(haltPC)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Read y[] from emulator memory and compare
	_ = y
	_ = tmp
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

	t.Logf("kernel_atax completed in %d instructions, max relative error = %e", em.InstCount, maxRelErr)
}
