package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPolyBenchGemm(t *testing.T) {
	// End-to-end test: PolyBench kernel_gemm with MINI dataset (NI=20, NJ=25, NK=30).
	//
	// The kernel computes:
	//   for i in 0..ni:
	//     for j in 0..nj: C[i*nj+j] *= beta
	//     for k in 0..nk:
	//       for j in 0..nj: C[i*nj+j] += alpha * A[i*nk+k] * B[k*nj+j]
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops
	//
	// Calling convention (ARM64 AAPCS):
	//   X0=ni, X1=nj, X2=nk, D0=alpha, D1=beta, X3=C, X4=A, X5=B

	ni, nj, nk := 20, 25, 30
	alpha, beta := 1.5, 1.2

	// Initialize arrays matching PolyBench MINI init_array
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

	// Compute expected output (golden reference)
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

	// Machine code bytes (53 instructions)
	code := []uint32{
		0x7100041f, // cmp	w0, #1
		0x5400066b, // b.lt	#0xd0
		0xa9bf4ff4, // stp	x20, x19, [sp, #-16]!
		0x52800008, // mov	w8, #0
		0xd2800009, // mov	x9, #0
		0x2a0103ea, // mov	w10, w1
		0x2a0203eb, // mov	w11, w2
		0xd37d7c2c, // ubfiz	x12, x1, #3, #32
		0xaa0303ed, // mov	x13, x3
		0x2a0003ee, // mov	w14, w0
		0x14000006, // b	#0x40
		0x91000529, // add	x9, x9, #1
		0x8b0c01ad, // add	x13, x13, x12
		0x0b010108, // add	w8, w8, w1
		0xeb0e013f, // cmp	x9, x14
		0x54000480, // b.eq	#0xcc
		0xaa0a03ef, // mov	x15, x10
		0xaa0d03f0, // mov	x16, x13
		0x7100043f, // cmp	w1, #1
		0x540000cb, // b.lt	#0x64
		0xfd400202, // ldr	d2, [x16]
		0x1e610842, // fmul	d2, d2, d1
		0xfc008602, // str	d2, [x16], #8
		0xf10005ef, // subs	x15, x15, #1
		0x54ffff81, // b.ne	#0x50
		0x7100045f, // cmp	w2, #1
		0x54fffe2b, // b.lt	#0x2c
		0xd280000f, // mov	x15, #0
		0x8b284c70, // add	x16, x3, w8, uxtw #3
		0x9b0b7d31, // mul	x17, x9, x11
		0x8b110c91, // add	x17, x4, x17, lsl #3
		0xaa0503e0, // mov	x0, x5
		0x14000005, // b	#0x94
		0x910005ef, // add	x15, x15, #1
		0x8b0c0000, // add	x0, x0, x12
		0xeb0b01ff, // cmp	x15, x11
		0x54fffce0, // b.eq	#0x2c
		0xaa0a03e6, // mov	x6, x10
		0xaa0003e7, // mov	x7, x0
		0xaa1003f3, // mov	x19, x16
		0x7100043f, // cmp	w1, #1
		0x54ffff0b, // b.lt	#0x84
		0xfc6f7a22, // ldr	d2, [x17, x15, lsl #3]
		0xfc4084e3, // ldr	d3, [x7], #8
		0x1e600842, // fmul	d2, d2, d0
		0xfd400264, // ldr	d4, [x19]
		0x1f431042, // fmadd	d2, d2, d3, d4
		0xfc008662, // str	d2, [x19], #8
		0xf10004c6, // subs	x6, x6, #1
		0x54ffff21, // b.ne	#0xa8
		0x17ffffef, // b	#0x84
		0xa8c14ff4, // ldp	x20, x19, [sp], #16
		0xd65f03c0, // ret
	}

	codeBase := uint64(0x10000)
	cBase := uint64(0x100000)
	aBase := uint64(0x200000)
	bBase := uint64(0x300000)
	stackBase := uint64(0x400000)
	haltPC := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 10_000_000

	// Load program
	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	// Load C matrix
	cBuf := make([]byte, ni*nj*8)
	for i, v := range C {
		binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(cBase, cBuf)

	// Load A matrix
	aBuf := make([]byte, ni*nk*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load B matrix
	bBuf := make([]byte, nk*nj*8)
	for i, v := range B {
		binary.LittleEndian.PutUint64(bBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(bBase, bBuf)

	// Allocate stack space (for STP/LDP of callee-saved registers)
	stackBuf := make([]byte, 8192)
	em.LoadProgram(stackBase, stackBuf)

	// Set registers per ARM64 ABI
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(ni))
	em.Regs.WriteX(1, uint64(nj))
	em.Regs.WriteX(2, uint64(nk))
	em.Regs.WriteD(0, alpha)
	em.Regs.WriteD(1, beta)
	em.Regs.WriteX(3, cBase)
	em.Regs.WriteX(4, aBase)
	em.Regs.WriteX(5, bBase)
	em.Regs.WriteX(30, haltPC) // LR = halt address
	em.Regs.SP = stackBase + 8192

	em.SetHaltPC(haltPC)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Read C[] from emulator memory and compare
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

	t.Logf("kernel_gemm completed in %d instructions, max relative error = %e", em.InstCount, maxRelErr)
}
