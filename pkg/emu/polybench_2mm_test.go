package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPolyBench2mm(t *testing.T) {
	// End-to-end test: PolyBench kernel_2mm with MINI dataset (NI=16, NJ=18, NK=22, NL=24).
	//
	// The kernel computes:
	//   for i in 0..ni:
	//     for j in 0..nj:
	//       tmp[i*nj+j] = 0
	//       for k in 0..nk: tmp[i*nj+j] += alpha * A[i*nk+k] * B[k*nj+j]
	//   for i in 0..ni:
	//     for j in 0..nl:
	//       D[i*nl+j] *= beta
	//       for k in 0..nj: D[i*nl+j] += tmp[i*nj+k] * C[k*nl+j]
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops
	//
	// Calling convention (ARM64 AAPCS):
	//   X0=ni, X1=nj, X2=nk, X3=nl, D0=alpha, D1=beta, X4=tmp, X5=A, X6=B, X7=C, stack[0]=D

	ni, nj, nk, nl := 16, 18, 22, 24
	alpha, beta := 1.5, 1.2

	// Initialize arrays matching PolyBench MINI init_array
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

	// Compute expected output (golden reference)
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

	// Machine code bytes (86 instructions)
	code := []uint32{
		0x7100041f, // cmp	w0, #1
		0x54000a8b, // b.lt	0x154
		0xa9be57f6, // stp	x22, x21, [sp, #-32]!
		0xa9014ff4, // stp	x20, x19, [sp, #16]
		0x5280000b, // mov	w11, #0
		0xd280000c, // mov	x12, #0
		0x2a0103e8, // mov	w8, w1
		0xf94013e9, // ldr	x9, [sp, #32]
		0x2a0003ea, // mov	w10, w0
		0xd37d7c2d, // ubfiz	x13, x1, #3, #32
		0x2a0203ee, // mov	w14, w2
		0x14000005, // b	0x40
		0x9100058c, // add	x12, x12, #1
		0x0b02016b, // add	w11, w11, w2
		0xeb0a019f, // cmp	x12, x10
		0x540003a0, // b.eq	0xb0
		0x7100043f, // cmp	w1, #1
		0x54ffff6b, // b.lt	0x30
		0xd280000f, // mov	x15, #0
		0x8b2b4cb0, // add	x16, x5, w11, uxtw #3
		0x9b087d91, // mul	x17, x12, x8
		0x8b110c91, // add	x17, x4, x17, lsl #3
		0xaa0603f3, // mov	x19, x6
		0x14000005, // b	0x70
		0x910005ef, // add	x15, x15, #1
		0x91002273, // add	x19, x19, #8
		0xeb0801ff, // cmp	x15, x8
		0x54fffe20, // b.eq	0x30
		0xf82f7a3f, // str	xzr, [x17, x15, lsl #3]
		0x7100045f, // cmp	w2, #1
		0x54ffff4b, // b.lt	0x60
		0x2f00e402, // movi	d2, #0
		0xaa0e03f4, // mov	x20, x14
		0xaa1303f5, // mov	x21, x19
		0xaa1003f6, // mov	x22, x16
		0xfc4086c3, // ldr	d3, [x22], #8
		0x1e600863, // fmul	d3, d3, d0
		0xfd4002a4, // ldr	d4, [x21]
		0x1f440862, // fmadd	d2, d3, d4, d2
		0xfc2f7a22, // str	d2, [x17, x15, lsl #3]
		0x8b0d02b5, // add	x21, x21, x13
		0xf1000694, // subs	x20, x20, #1
		0x54ffff21, // b.ne	0x8c
		0x17ffffed, // b	0x60
		0x7100041f, // cmp	w0, #1
		0x540004cb, // b.lt	0x14c
		0x5280000b, // mov	w11, #0
		0xd280000c, // mov	x12, #0
		0xd37d7c6d, // ubfiz	x13, x3, #3, #32
		0x2a0303ee, // mov	w14, w3
		0x14000005, // b	0xdc
		0x9100058c, // add	x12, x12, #1
		0x0b01016b, // add	w11, w11, w1
		0xeb0a019f, // cmp	x12, x10
		0x540003a0, // b.eq	0x14c
		0x7100047f, // cmp	w3, #1
		0x54ffff6b, // b.lt	0xcc
		0xd280000f, // mov	x15, #0
		0x8b2b4c90, // add	x16, x4, w11, uxtw #3
		0x9b0e7d91, // mul	x17, x12, x14
		0x8b110d31, // add	x17, x9, x17, lsl #3
		0xaa0703e0, // mov	x0, x7
		0x14000005, // b	0x10c
		0x910005ef, // add	x15, x15, #1
		0x91002000, // add	x0, x0, #8
		0xeb0e01ff, // cmp	x15, x14
		0x54fffe20, // b.eq	0xcc
		0xfc6f7a20, // ldr	d0, [x17, x15, lsl #3]
		0x1e610800, // fmul	d0, d0, d1
		0xfc2f7a20, // str	d0, [x17, x15, lsl #3]
		0xaa0803e2, // mov	x2, x8
		0xaa0003e5, // mov	x5, x0
		0xaa1003e6, // mov	x6, x16
		0x7100043f, // cmp	w1, #1
		0x54fffeab, // b.lt	0xfc
		0xfc4084c2, // ldr	d2, [x6], #8
		0xfd4000a3, // ldr	d3, [x5]
		0x1f430040, // fmadd	d0, d2, d3, d0
		0xfc2f7a20, // str	d0, [x17, x15, lsl #3]
		0x8b0d00a5, // add	x5, x5, x13
		0xf1000442, // subs	x2, x2, #1
		0x54ffff41, // b.ne	0x12c
		0x17ffffed, // b	0xfc
		0xa9414ff4, // ldp	x20, x19, [sp, #16]
		0xa8c257f6, // ldp	x22, x21, [sp], #32
		0xd65f03c0, // ret
	}

	codeBase := uint64(0x10000)
	tmpBase := uint64(0x100000)
	aBase := uint64(0x200000)
	bBase := uint64(0x300000)
	cBase := uint64(0x400000)
	dBase := uint64(0x500000)
	stackBase := uint64(0x600000)
	haltPC := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 10_000_000

	// Load program
	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	// Load tmp matrix
	tmpBuf := make([]byte, ni*nj*8)
	for i, v := range tmp {
		binary.LittleEndian.PutUint64(tmpBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(tmpBase, tmpBuf)

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

	// Load C matrix
	cBuf := make([]byte, nj*nl*8)
	for i, v := range C {
		binary.LittleEndian.PutUint64(cBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(cBase, cBuf)

	// Load D matrix
	dBuf := make([]byte, ni*nl*8)
	for i, v := range D {
		binary.LittleEndian.PutUint64(dBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(dBase, dBuf)

	// Allocate stack space and write D's address for stack-passed arg
	stackBuf := make([]byte, 8192)
	binary.LittleEndian.PutUint64(stackBuf[4096:], dBase)
	em.LoadProgram(stackBase, stackBuf)

	// Set registers per ARM64 ABI
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
	em.Regs.WriteX(30, haltPC) // LR = halt address
	em.Regs.SP = stackBase + 4096

	em.SetHaltPC(haltPC)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Read D[] from emulator memory and compare
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

	t.Logf("kernel_2mm completed in %d instructions, max relative error = %e", em.InstCount, maxRelErr)
}
