package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPolyBenchMvt(t *testing.T) {
	// End-to-end test: PolyBench kernel_mvt with MINI dataset (N=40).
	//
	// The kernel computes:
	//   for i in 0..n:
	//     for j in 0..n: x1[i] += A[i*n+j] * y_1[j]
	//   for i in 0..n:
	//     for j in 0..n: x2[i] += A[j*n+i] * y_2[j]
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops
	//
	// Calling convention:
	//   X0=n, X1=x1, X2=x2, X3=y_1, X4=y_2, X5=A

	n := 40

	// Initialize arrays matching PolyBench MINI init_array
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

	// Compute expected output (golden reference)
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

	// Machine code bytes (40 instructions)
	code := []uint32{
		0x7100041f, // cmp	w0, #1
		0x540004cb, // b.lt	#0x9c
		0xd2800009, // mov	x9, #0
		0xd37d7c0a, // ubfiz	x10, x0, #3, #32
		0xaa0503eb, // mov	x11, x5
		0x2a0003e8, // mov	w8, w0
		0xfc697820, // ldr	d0, [x1, x9, lsl #3]
		0xaa0803ec, // mov	x12, x8
		0xaa0303ed, // mov	x13, x3
		0xaa0b03ee, // mov	x14, x11
		0xfc4085c1, // ldr	d1, [x14], #8
		0xfc4085a2, // ldr	d2, [x13], #8
		0x1f420020, // fmadd	d0, d1, d2, d0
		0xfc297820, // str	d0, [x1, x9, lsl #3]
		0xf100058c, // subs	x12, x12, #1
		0x54ffff61, // b.ne	#0x28
		0x91000529, // add	x9, x9, #1
		0x8b0a016b, // add	x11, x11, x10
		0xeb08013f, // cmp	x9, x8
		0x54fffe61, // b.ne	#0x18
		0x7100041f, // cmp	w0, #1
		0x5400024b, // b.lt	#0x9c
		0xd2800009, // mov	x9, #0
		0xd37d7c0a, // ubfiz	x10, x0, #3, #32
		0xfc697840, // ldr	d0, [x2, x9, lsl #3]
		0xaa0803eb, // mov	x11, x8
		0xaa0403ec, // mov	x12, x4
		0xaa0503ed, // mov	x13, x5
		0xfc408581, // ldr	d1, [x12], #8
		0xfd4001a2, // ldr	d2, [x13]
		0x1f410040, // fmadd	d0, d2, d1, d0
		0xfc297840, // str	d0, [x2, x9, lsl #3]
		0x8b0a01ad, // add	x13, x13, x10
		0xf100056b, // subs	x11, x11, #1
		0x54ffff41, // b.ne	#0x60
		0x91000529, // add	x9, x9, #1
		0x910020a5, // add	x5, x5, #8
		0xeb08013f, // cmp	x9, x8
		0x54fffe41, // b.ne	#0x54
		0xd65f03c0, // ret
	}

	codeBase := uint64(0x10000)
	x1Base := uint64(0x100000)
	x2Base := uint64(0x200000)
	y1Base := uint64(0x300000)
	y2Base := uint64(0x400000)
	aBase := uint64(0x500000)
	haltPC := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 10_000_000

	// Load program
	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	// Load x1 vector
	x1Buf := make([]byte, n*8)
	for i, v := range x1 {
		binary.LittleEndian.PutUint64(x1Buf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(x1Base, x1Buf)

	// Load x2 vector
	x2Buf := make([]byte, n*8)
	for i, v := range x2 {
		binary.LittleEndian.PutUint64(x2Buf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(x2Base, x2Buf)

	// Load y1 vector
	y1Buf := make([]byte, n*8)
	for i, v := range y1 {
		binary.LittleEndian.PutUint64(y1Buf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(y1Base, y1Buf)

	// Load y2 vector
	y2Buf := make([]byte, n*8)
	for i, v := range y2 {
		binary.LittleEndian.PutUint64(y2Buf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(y2Base, y2Buf)

	// Load A matrix
	aBuf := make([]byte, n*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Set registers per ARM64 ABI
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(n))
	em.Regs.WriteX(1, x1Base)
	em.Regs.WriteX(2, x2Base)
	em.Regs.WriteX(3, y1Base)
	em.Regs.WriteX(4, y2Base)
	em.Regs.WriteX(5, aBase)
	em.Regs.WriteX(30, haltPC) // LR = halt address
	em.Regs.SP = 0x600000 + 8192

	em.SetHaltPC(haltPC)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Read x1[] from emulator memory and compare
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

	// Read x2[] from emulator memory and compare
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

	t.Logf("kernel_mvt completed in %d instructions, max relative error = %e", em.InstCount, maxRelErr)
}
