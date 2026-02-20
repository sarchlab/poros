package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPolyBench3mm(t *testing.T) {
	// End-to-end test: PolyBench kernel_3mm with MINI dataset (NI=16, NJ=18, NK=20, NL=22, NM=24).
	//
	// The kernel computes:
	//   E = A * B   (ni x nj) = (ni x nk) * (nk x nj)
	//   F = C * D   (nj x nl) = (nj x nm) * (nm x nl)
	//   G = E * F   (ni x nl) = (ni x nj) * (nj x nl)
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops
	//
	// Calling convention (ARM64 AAPCS):
	//   X0=ni, X1=nj, X2=nk, X3=nl, X4=nm, X5=E, X6=A, X7=B
	//   stack[0]=F, stack[8]=C, stack[16]=D, stack[24]=G

	ni, nj, nk, nl, nm := 16, 18, 20, 22, 24

	// Initialize arrays matching PolyBench MINI init_array
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

	// Compute expected output (golden reference)
	expectedE := make([]float64, ni*nj)
	for i := 0; i < ni; i++ {
		for j := 0; j < nj; j++ {
			expectedE[i*nj+j] = 0.0
			for k := 0; k < nk; k++ {
				expectedE[i*nj+j] += A[i*nk+k] * B[k*nj+j]
			}
		}
	}
	expectedF := make([]float64, nj*nl)
	for i := 0; i < nj; i++ {
		for j := 0; j < nl; j++ {
			expectedF[i*nl+j] = 0.0
			for k := 0; k < nm; k++ {
				expectedF[i*nl+j] += C[i*nm+k] * D[k*nl+j]
			}
		}
	}
	expectedG := make([]float64, ni*nl)
	for i := 0; i < ni; i++ {
		for j := 0; j < nl; j++ {
			expectedG[i*nl+j] = 0.0
			for k := 0; k < nj; k++ {
				expectedG[i*nl+j] += expectedE[i*nj+k] * expectedF[k*nl+j]
			}
		}
	}

	// Machine code bytes (125 instructions)
	code := []uint32{
		0xa9be57f6, // stp	x22, x21, [sp, #-32]!
		0xa9014ff4, // stp	x20, x19, [sp, #16]
		0x2a0103e8, // mov	w8, w1
		0x2a0003e9, // mov	w9, w0
		0x7100041f, // cmp	w0, #1
		0x540004ab, // b.lt	0xa8
		0x5280000a, // mov	w10, #0
		0xd280000b, // mov	x11, #0
		0xd37d7c2c, // ubfiz	x12, x1, #3, #32
		0x2a0203ed, // mov	w13, w2
		0x14000005, // b	0x3c
		0x9100056b, // add	x11, x11, #1
		0x0b02014a, // add	w10, w10, w2
		0xeb09017f, // cmp	x11, x9
		0x54000380, // b.eq	0xa8
		0x7100043f, // cmp	w1, #1
		0x54ffff6b, // b.lt	0x2c
		0xd280000e, // mov	x14, #0
		0x8b2a4ccf, // add	x15, x6, w10, uxtw #3
		0x9b087d70, // mul	x16, x11, x8
		0x8b100cb0, // add	x16, x5, x16, lsl #3
		0xaa0703f1, // mov	x17, x7
		0x14000005, // b	0x6c
		0x910005ce, // add	x14, x14, #1
		0x91002231, // add	x17, x17, #8
		0xeb0801df, // cmp	x14, x8
		0x54fffe20, // b.eq	0x2c
		0xf82e7a1f, // str	xzr, [x16, x14, lsl #3]
		0x7100045f, // cmp	w2, #1
		0x54ffff4b, // b.lt	0x5c
		0x2f00e400, // movi	d0, #0
		0xaa0d03f3, // mov	x19, x13
		0xaa1103f4, // mov	x20, x17
		0xaa0f03f5, // mov	x21, x15
		0xfc4086a1, // ldr	d1, [x21], #8
		0xfd400282, // ldr	d2, [x20]
		0x1f420020, // fmadd	d0, d1, d2, d0
		0xfc2e7a00, // str	d0, [x16, x14, lsl #3]
		0x8b0c0294, // add	x20, x20, x12
		0xf1000673, // subs	x19, x19, #1
		0x54ffff41, // b.ne	0x88
		0x17ffffee, // b	0x5c
		0xf94013ea, // ldr	x10, [sp, #32]
		0x2a0303eb, // mov	w11, w3
		0x7100043f, // cmp	w1, #1
		0x540004cb, // b.lt	0x14c
		0x5280000c, // mov	w12, #0
		0xd280000d, // mov	x13, #0
		0xa942bbef, // ldp	x15, x14, [sp, #40]
		0xd37d7c70, // ubfiz	x16, x3, #3, #32
		0x2a0403f1, // mov	w17, w4
		0x14000005, // b	0xe0
		0x910005ad, // add	x13, x13, #1
		0x0b04018c, // add	w12, w12, w4
		0xeb0801bf, // cmp	x13, x8
		0x54000380, // b.eq	0x14c
		0x7100047f, // cmp	w3, #1
		0x54ffff6b, // b.lt	0xd0
		0xd2800002, // mov	x2, #0
		0x8b2c4de6, // add	x6, x15, w12, uxtw #3
		0x9b0b7da7, // mul	x7, x13, x11
		0x8b070d47, // add	x7, x10, x7, lsl #3
		0xaa0e03f3, // mov	x19, x14
		0x14000005, // b	0x110
		0x91000442, // add	x2, x2, #1
		0x91002273, // add	x19, x19, #8
		0xeb0b005f, // cmp	x2, x11
		0x54fffe20, // b.eq	0xd0
		0xf82278ff, // str	xzr, [x7, x2, lsl #3]
		0x7100049f, // cmp	w4, #1
		0x54ffff4b, // b.lt	0x100
		0x2f00e400, // movi	d0, #0
		0xaa1103f4, // mov	x20, x17
		0xaa1303f5, // mov	x21, x19
		0xaa0603f6, // mov	x22, x6
		0xfc4086c1, // ldr	d1, [x22], #8
		0xfd4002a2, // ldr	d2, [x21]
		0x1f420020, // fmadd	d0, d1, d2, d0
		0xfc2278e0, // str	d0, [x7, x2, lsl #3]
		0x8b1002b5, // add	x21, x21, x16
		0xf1000694, // subs	x20, x20, #1
		0x54ffff41, // b.ne	0x12c
		0x17ffffee, // b	0x100
		0x7100041f, // cmp	w0, #1
		0x540004cb, // b.lt	0x1e8
		0x5280000c, // mov	w12, #0
		0xd280000d, // mov	x13, #0
		0xf9401fee, // ldr	x14, [sp, #56]
		0xaa0303ef, // mov	x15, x3
		0xd37d7def, // ubfiz	x15, x15, #3, #32
		0x14000005, // b	0x17c
		0x910005ad, // add	x13, x13, #1
		0x0b01018c, // add	w12, w12, w1
		0xeb0901bf, // cmp	x13, x9
		0x54000380, // b.eq	0x1e8
		0x7100047f, // cmp	w3, #1
		0x54ffff6b, // b.lt	0x16c
		0xd2800010, // mov	x16, #0
		0x8b2c4cb1, // add	x17, x5, w12, uxtw #3
		0x9b0b7da0, // mul	x0, x13, x11
		0x8b000dc0, // add	x0, x14, x0, lsl #3
		0xaa0a03e2, // mov	x2, x10
		0x14000005, // b	0x1ac
		0x91000610, // add	x16, x16, #1
		0x91002042, // add	x2, x2, #8
		0xeb0b021f, // cmp	x16, x11
		0x54fffe20, // b.eq	0x16c
		0xf830781f, // str	xzr, [x0, x16, lsl #3]
		0x7100043f, // cmp	w1, #1
		0x54ffff4b, // b.lt	0x19c
		0x2f00e400, // movi	d0, #0
		0xaa0803e4, // mov	x4, x8
		0xaa0203e6, // mov	x6, x2
		0xaa1103e7, // mov	x7, x17
		0xfc4084e1, // ldr	d1, [x7], #8
		0xfd4000c2, // ldr	d2, [x6]
		0x1f420020, // fmadd	d0, d1, d2, d0
		0xfc307800, // str	d0, [x0, x16, lsl #3]
		0x8b0f00c6, // add	x6, x6, x15
		0xf1000484, // subs	x4, x4, #1
		0x54ffff41, // b.ne	0x1c8
		0x17ffffee, // b	0x19c
		0xa9414ff4, // ldp	x20, x19, [sp, #16]
		0xa8c257f6, // ldp	x22, x21, [sp], #32
		0xd65f03c0, // ret
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

	em := NewEmulator()
	em.MaxInst = 10_000_000

	// Load program
	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	// Load E matrix (output, initialized to zero)
	eBuf := make([]byte, ni*nj*8)
	for i, v := range E {
		binary.LittleEndian.PutUint64(eBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(eBase, eBuf)

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

	// Load F matrix (output, initialized to zero)
	fBuf := make([]byte, nj*nl*8)
	for i, v := range F {
		binary.LittleEndian.PutUint64(fBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(fBase, fBuf)

	// Load C matrix
	cBufData := make([]byte, nj*nm*8)
	for i, v := range C {
		binary.LittleEndian.PutUint64(cBufData[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(cBase, cBufData)

	// Load D matrix
	dBufData := make([]byte, nm*nl*8)
	for i, v := range D {
		binary.LittleEndian.PutUint64(dBufData[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(dBase, dBufData)

	// Load G matrix (output, initialized to zero)
	gBuf := make([]byte, ni*nl*8)
	for i, v := range G {
		binary.LittleEndian.PutUint64(gBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(gBase, gBuf)

	// Allocate stack space and write stack-passed args
	// After stp x22,x21,[sp,#-32]!, new SP = original SP - 32.
	// ldr x10,[sp,#32] reads original_SP+0 = F
	// ldp x15,x14,[sp,#40] reads original_SP+8 = C, original_SP+16 = D
	// ldr x14,[sp,#56] reads original_SP+24 = G
	stackBuf := make([]byte, 8192)
	binary.LittleEndian.PutUint64(stackBuf[4096:], fBase)    // stack[0] = F
	binary.LittleEndian.PutUint64(stackBuf[4096+8:], cBase)  // stack[8] = C
	binary.LittleEndian.PutUint64(stackBuf[4096+16:], dBase) // stack[16] = D
	binary.LittleEndian.PutUint64(stackBuf[4096+24:], gBase) // stack[24] = G
	em.LoadProgram(stackBase, stackBuf)

	// Set registers per ARM64 ABI
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(ni))
	em.Regs.WriteX(1, uint64(nj))
	em.Regs.WriteX(2, uint64(nk))
	em.Regs.WriteX(3, uint64(nl))
	em.Regs.WriteX(4, uint64(nm))
	em.Regs.WriteX(5, eBase)
	em.Regs.WriteX(6, aBase)
	em.Regs.WriteX(7, bBase)
	em.Regs.WriteX(30, haltPC) // LR = halt address
	em.Regs.SP = stackBase + 4096

	em.SetHaltPC(haltPC)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Read G[] from emulator memory and compare
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

	t.Logf("kernel_3mm completed in %d instructions, max relative error = %e", em.InstCount, maxRelErr)
}
