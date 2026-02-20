package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPolyBenchBicg(t *testing.T) {
	// End-to-end test: PolyBench kernel_bicg with MINI dataset (M=38, N=42).
	//
	// The kernel computes:
	//   for i in 0..m:
	//     q[i] = 0
	//     for j in 0..n:
	//       s[j] += r[i] * A[i*n+j]
	//       q[i] += A[i*n+j] * p[j]
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops
	//
	// Calling convention (ARM64 AAPCS):
	//   X0=m, X1=n, X2=A, X3=s, X4=q, X5=p, X6=r

	m, n := 38, 42

	// Initialize arrays matching PolyBench MINI init_array
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

	// Compute expected output (golden reference)
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

	// Machine code bytes (32 instructions)
	code := []uint32{
		0x7100041f, // cmp	w0, #1
		0x540003cb, // b.lt	0x7c
		0xd2800008, // mov	x8, #0
		0x2a0103e9, // mov	w9, w1
		0xd37d7c2a, // ubfiz	x10, x1, #3, #32
		0x2a0003eb, // mov	w11, w0
		0x14000005, // b	0x2c
		0x91000508, // add	x8, x8, #1
		0x8b0a0042, // add	x2, x2, x10
		0xeb0b011f, // cmp	x8, x11
		0x540002a0, // b.eq	0x7c
		0xf828789f, // str	xzr, [x4, x8, lsl #3]
		0xaa0903ec, // mov	x12, x9
		0xaa0303ed, // mov	x13, x3
		0xaa0503ee, // mov	x14, x5
		0xaa0203ef, // mov	x15, x2
		0x7100043f, // cmp	w1, #1
		0x54fffecb, // b.lt	0x1c
		0xfd4001a0, // ldr	d0, [x13]
		0xfc6878c1, // ldr	d1, [x6, x8, lsl #3]
		0xfd4001e2, // ldr	d2, [x15]
		0x1f420020, // fmadd	d0, d1, d2, d0
		0xfc0085a0, // str	d0, [x13], #8
		0xfc687880, // ldr	d0, [x4, x8, lsl #3]
		0xfc4085e1, // ldr	d1, [x15], #8
		0xfc4085c2, // ldr	d2, [x14], #8
		0x1f420020, // fmadd	d0, d1, d2, d0
		0xfc287880, // str	d0, [x4, x8, lsl #3]
		0xf100058c, // subs	x12, x12, #1
		0x54fffea1, // b.ne	0x48
		0x17ffffe9, // b	0x1c
		0xd65f03c0, // ret
	}

	codeBase := uint64(0x10000)
	aBase := uint64(0x100000)
	sBase := uint64(0x200000)
	qBase := uint64(0x300000)
	pBase := uint64(0x400000)
	rBase := uint64(0x500000)
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

	// Load A matrix
	aBuf := make([]byte, m*n*8)
	for i, v := range A {
		binary.LittleEndian.PutUint64(aBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(aBase, aBuf)

	// Load s vector
	sBuf := make([]byte, n*8)
	for i, v := range s {
		binary.LittleEndian.PutUint64(sBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(sBase, sBuf)

	// Load q vector
	qBuf := make([]byte, m*8)
	for i, v := range q {
		binary.LittleEndian.PutUint64(qBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(qBase, qBuf)

	// Load p vector
	pBuf := make([]byte, n*8)
	for i, v := range p {
		binary.LittleEndian.PutUint64(pBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(pBase, pBuf)

	// Load r vector
	rBuf := make([]byte, m*8)
	for i, v := range r {
		binary.LittleEndian.PutUint64(rBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(rBase, rBuf)

	// Allocate stack space
	stackBuf := make([]byte, 8192)
	em.LoadProgram(stackBase, stackBuf)

	// Set registers per ARM64 ABI
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, uint64(m))
	em.Regs.WriteX(1, uint64(n))
	em.Regs.WriteX(2, aBase)
	em.Regs.WriteX(3, sBase)
	em.Regs.WriteX(4, qBase)
	em.Regs.WriteX(5, pBase)
	em.Regs.WriteX(6, rBase)
	em.Regs.WriteX(30, haltPC) // LR = halt address
	em.Regs.SP = stackBase + 8192

	em.SetHaltPC(haltPC)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Read s[] from emulator memory and compare
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

	// Read q[] from emulator memory and compare
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

	t.Logf("kernel_bicg completed in %d instructions, max relative error = %e", em.InstCount, maxRelErr)
}
