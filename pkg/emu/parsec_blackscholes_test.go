package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

// cndf computes the cumulative normal distribution function (Go golden reference).
func cndf(x float64) float64 {
	a1 := 0.31938153
	a2 := -0.356563782
	a3 := 1.781477937
	a4 := -1.821255978
	a5 := 1.330274429

	L := math.Abs(x)
	K := 1.0 / (1.0 + 0.2316419*L)
	w := 1.0 - 1.0/math.Sqrt(2.0*3.141592653589793)*
		math.Exp(-L*L/2.0)*
		(a1*K+a2*K*K+a3*K*K*K+a4*K*K*K*K+a5*K*K*K*K*K)
	if x < 0.0 {
		w = 1.0 - w
	}
	return w
}

// blkSchlsGolden computes the Black-Scholes option price (Go golden reference).
func blkSchlsGolden(S, X, r, v, T float64, otype int) float64 {
	sqrtT := math.Sqrt(T)
	d1 := (math.Log(S/X) + (r+v*v*0.5)*T) / (v * sqrtT)
	d2 := d1 - v*sqrtT
	NofXd1 := cndf(d1)
	NofXd2 := cndf(d2)
	if otype == 0 {
		return S*NofXd1 - X*math.Exp(-r*T)*NofXd2
	}
	return X*math.Exp(-r*T)*(1.0-NofXd2) - S*(1.0-NofXd1)
}

func TestParsecBlackScholes(t *testing.T) {
	// End-to-end test: PARSEC BlackScholes option pricing kernel.
	//
	// The kernel computes Black-Scholes European option prices for numOptions
	// options. It calls exp() and log() via BL instructions which are intercepted
	// by Go hooks. FSQRT is used inline (no hook needed).
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize
	//                -fno-slp-vectorize -fno-unroll-loops
	//
	// ARM64 ABI:
	//   X0=prices, X1=S, X2=X, X3=r, X4=v, X5=T, X6=otype, W7=numOptions

	numOptions := 16

	// Test data: 16 options with varying parameters
	stockPrices := []float64{
		42.0, 100.0, 50.0, 80.0, 120.0, 30.0, 95.0, 70.0,
		55.0, 110.0, 45.0, 85.0, 60.0, 75.0, 90.0, 65.0,
	}
	strikePrices := []float64{
		40.0, 105.0, 48.0, 82.0, 115.0, 32.0, 100.0, 68.0,
		52.0, 108.0, 50.0, 80.0, 58.0, 72.0, 88.0, 70.0,
	}
	riskFreeRate := []float64{
		0.05, 0.04, 0.06, 0.03, 0.05, 0.04, 0.05, 0.06,
		0.03, 0.05, 0.04, 0.06, 0.05, 0.03, 0.04, 0.05,
	}
	volatility := []float64{
		0.20, 0.25, 0.30, 0.15, 0.22, 0.35, 0.18, 0.28,
		0.25, 0.20, 0.30, 0.22, 0.25, 0.18, 0.20, 0.32,
	}
	timeToExp := []float64{
		0.50, 1.00, 0.25, 0.75, 1.50, 0.30, 0.80, 0.60,
		1.00, 0.50, 0.75, 1.25, 0.40, 0.90, 1.00, 0.50,
	}
	optionType := []int32{
		0, 1, 0, 1, 0, 1, 0, 1,
		0, 1, 0, 1, 0, 1, 0, 1,
	}

	// Compute golden reference
	expectedPrices := make([]float64, numOptions)
	for i := 0; i < numOptions; i++ {
		expectedPrices[i] = blkSchlsGolden(
			stockPrices[i], strikePrices[i],
			riskFreeRate[i], volatility[i], timeToExp[i],
			int(optionType[i]),
		)
	}

	// Machine code: 173 instructions from compiled BlkSchlsEqEuroNoDiv.
	// BL instructions patched with offsets to hook addresses:
	//   0x104: BL _log  → target 0xF000
	//   0x154: BL _exp  → target 0xF100
	//   0x1d8: BL _exp  → target 0xF100
	//   0x240: BL _exp  → target 0xF100
	code := []uint32{
		0x710004ff, // 0x000: cmp   w7, #1
		0x5400156b, // 0x004: b.lt  0x2b0
		0xd10443ff, // 0x008: sub   sp, sp, #0x110
		0x6d073bef, // 0x00c: stp   d15, d14, [sp, #0x70]
		0x6d0833ed, // 0x010: stp   d13, d12, [sp, #0x80]
		0x6d092beb, // 0x014: stp   d11, d10, [sp, #0x90]
		0x6d0a23e9, // 0x018: stp   d9, d8, [sp, #0xa0]
		0xa90b6ffc, // 0x01c: stp   x28, x27, [sp, #0xb0]
		0xa90c67fa, // 0x020: stp   x26, x25, [sp, #0xc0]
		0xa90d5ff8, // 0x024: stp   x24, x23, [sp, #0xd0]
		0xa90e57f6, // 0x028: stp   x22, x21, [sp, #0xe0]
		0xa90f4ff4, // 0x02c: stp   x20, x19, [sp, #0xf0]
		0xa9107bfd, // 0x030: stp   x29, x30, [sp, #0x100]
		0x910403fd, // 0x034: add   x29, sp, #0x100
		0xaa0603f3, // 0x038: mov   x19, x6
		0xaa0503f4, // 0x03c: mov   x20, x5
		0xaa0403f5, // 0x040: mov   x21, x4
		0xaa0303f6, // 0x044: mov   x22, x3
		0xaa0203f7, // 0x048: mov   x23, x2
		0xaa0103f8, // 0x04c: mov   x24, x1
		0xaa0003f9, // 0x050: mov   x25, x0
		0xd28201c8, // 0x054: mov   x8, #0x100e
		0xf2a30e28, // 0x058: movk  x8, #0x1871, lsl #16
		0xf2d4ce28, // 0x05c: movk  x8, #0xa671, lsl #32
		0xf2e7f9a8, // 0x060: movk  x8, #0x3fcd, lsl #48
		0xf9001be8, // 0x064: str   x8, [sp, #0x30]
		0xd2864b68, // 0x068: mov   x8, #0x325b
		0xf2bcb508, // 0x06c: movk  x8, #0xe5a8, lsl #16
		0xf2da3e08, // 0x070: movk  x8, #0xd1f0, lsl #32
		0xf2f7fac8, // 0x074: movk  x8, #0xbfd6, lsl #48
		0xf90017e8, // 0x078: str   x8, [sp, #0x28]
		0xd29f1d88, // 0x07c: mov   x8, #0xf8ec
		0xf2a75248, // 0x080: movk  x8, #0x3a92, lsl #16
		0xf2ce17e8, // 0x084: movk  x8, #0x70bf, lsl #32
		0xf2e7fa88, // 0x088: movk  x8, #0x3fcd, lsl #48  (a2 = -0.356563782 is wrong comment; this is a different const)
		0xf90013e8, // 0x08c: str   x8, [sp, #0x20]
		0xd28bcd08, // 0x090: mov   x8, #0x5e68
		0xf2a04be8, // 0x094: movk  x8, #0x25f, lsl #16
		0xf2d01de8, // 0x098: movk  x8, #0x80ef, lsl #32
		0xf2e7ff88, // 0x09c: movk  x8, #0x3ffc, lsl #48
		0xf9000fe8, // 0x0a0: str   x8, [sp, #0x18]
		0xd28f1a08, // 0x0a4: mov   x8, #0x78d0
		0xf2a9de48, // 0x0a8: movk  x8, #0x4ef2, lsl #16
		0xf2c47ba8, // 0x0ac: movk  x8, #0x23dd, lsl #32
		0xf2f7ffa8, // 0x0b0: movk  x8, #0xbffd, lsl #48
		0xf9000be8, // 0x0b4: str   x8, [sp, #0x10]
		0xd2852868, // 0x0b8: mov   x8, #0x2943
		0xf2bade88, // 0x0bc: movk  x8, #0xd6f4, lsl #16
		0xf2c919a8, // 0x0c0: movk  x8, #0x48cd, lsl #32
		0xf2e7fea8, // 0x0c4: movk  x8, #0x3ff5, lsl #48
		0xf90033e8, // 0x0c8: str   x8, [sp, #0x60]
		0xd286ca28, // 0x0cc: mov   x8, #0x3651
		0xf2a67a88, // 0x0d0: movk  x8, #0x33d4, lsl #16
		0xf2d108a8, // 0x0d4: movk  x8, #0x8845, lsl #32
		0xf2f7fb28, // 0x0d8: movk  x8, #0xbfd9, lsl #48
		0xf90007e8, // 0x0dc: str   x8, [sp, #0x8]
		0x2a0703fa, // 0x0e0: mov   w26, w7
		0xfd400280, // 0x0e4: ldr   d0, [x20]
		0x1e61c00e, // 0x0e8: fsqrt d14, d0
		0x1e604008, // 0x0ec: fmov  d8, d0
		0xfd0023e0, // 0x0f0: str   d0, [sp, #0x40]
		0xfc408700, // 0x0f4: ldr   d0, [x24], #0x8
		0xfc4086e1, // 0x0f8: ldr   d1, [x23], #0x8
		0x6d0503e1, // 0x0fc: stp   d1, d0, [sp, #0x50] (wrong comment for disassembly but correct encoding)
		0x1e611800, // 0x100: fdiv  d0, d0, d1
		0x940037bf, // 0x104: bl    _log (→ 0xF000)
		0xfc4086c3, // 0x108: ldr   d3, [x22], #0x8
		0xfd0027e3, // 0x10c: str   d3, [sp, #0x48]
		0xfc4086a1, // 0x110: ldr   d1, [x21], #0x8
		0x1e610822, // 0x114: fmul  d2, d1, d1
		0x1e6c1004, // 0x118: fmov  d4, #0.5
		0x1f440c42, // 0x11c: fmadd d2, d2, d4, d3
		0x1e6c1003, // 0x120: fmov  d3, #0.5
		0x1f480040, // 0x124: fmadd d0, d2, d8, d0
		0x1e6109c2, // 0x128: fmul  d2, d14, d1
		0x1e62180f, // 0x12c: fdiv  d15, d0, d2
		0x1f4ebc20, // 0x130: fmsub d0, d1, d14, d15
		0xfd0037e0, // 0x134: str   d0, [sp, #0x68]
		0x1e60c1e0, // 0x138: fabs  d0, d15
		0xfd401beb, // 0x13c: ldr   d11, [sp, #0x30]
		0x1e6e1008, // 0x140: fmov  d8, #1.0
		0x1f4b2001, // 0x144: fmadd d1, d0, d11, d8
		0x1e611909, // 0x148: fdiv  d9, d8, d1
		0x1e608800, // 0x14c: fnmul d0, d0, d0
		0x1e630800, // 0x150: fmul  d0, d0, d3
		0x940037eb, // 0x154: bl    _exp (→ 0xF100)
		0x6d4237ea, // 0x158: ldp   d10, d13, [sp, #0x20]
		0x1e6d0921, // 0x15c: fmul  d1, d9, d13
		0x1e610921, // 0x160: fmul  d1, d9, d1
		0x1f4a0521, // 0x164: fmadd d1, d9, d10, d1
		0x6d413bec, // 0x168: ldp   d12, d14, [sp, #0x10]
		0x1e6e0922, // 0x16c: fmul  d2, d9, d14
		0x1e620922, // 0x170: fmul  d2, d9, d2
		0x1f490441, // 0x174: fmadd d1, d2, d9, d1
		0x1e6c0922, // 0x178: fmul  d2, d9, d12
		0x1e620922, // 0x17c: fmul  d2, d9, d2
		0x1e620922, // 0x180: fmul  d2, d9, d2
		0x1f490441, // 0x184: fmadd d1, d2, d9, d1
		0xfd4033e2, // 0x188: ldr   d2, [sp, #0x60]
		0x1e620922, // 0x18c: fmul  d2, d9, d2
		0x1e620922, // 0x190: fmul  d2, d9, d2
		0x1e620922, // 0x194: fmul  d2, d9, d2
		0x1e620922, // 0x198: fmul  d2, d9, d2
		0x1f490441, // 0x19c: fmadd d1, d2, d9, d1
		0xfd4007e9, // 0x1a0: ldr   d9, [sp, #0x8]
		0x1e690800, // 0x1a4: fmul  d0, d0, d9
		0x1f412000, // 0x1a8: fmadd d0, d0, d1, d8
		0x1e603901, // 0x1ac: fsub  d1, d8, d0
		0x1e6021e8, // 0x1b0: fcmp  d15, #0.0
		0x1e604c20, // 0x1b4: fcsel d0, d1, d0, mi
		0xfd001fe0, // 0x1b8: str   d0, [sp, #0x38]
		0xfd4037e0, // 0x1bc: ldr   d0, [sp, #0x68]
		0x1e60c000, // 0x1c0: fabs  d0, d0
		0x1f4b2001, // 0x1c4: fmadd d1, d0, d11, d8
		0x1e61190f, // 0x1c8: fdiv  d15, d8, d1
		0x1e608800, // 0x1cc: fnmul d0, d0, d0
		0x1e6c1001, // 0x1d0: fmov  d1, #0.5
		0x1e610800, // 0x1d4: fmul  d0, d0, d1
		0x940037ca, // 0x1d8: bl    _exp (→ 0xF100)
		0x1e6d09e1, // 0x1dc: fmul  d1, d15, d13
		0x1e6109e1, // 0x1e0: fmul  d1, d15, d1
		0x1f4a05e1, // 0x1e4: fmadd d1, d15, d10, d1
		0x1e6e09e2, // 0x1e8: fmul  d2, d15, d14
		0x1e6209e2, // 0x1ec: fmul  d2, d15, d2
		0x1f4f0441, // 0x1f0: fmadd d1, d2, d15, d1
		0x1e6c09e2, // 0x1f4: fmul  d2, d15, d12
		0x1e6209e2, // 0x1f8: fmul  d2, d15, d2
		0x1e6209e2, // 0x1fc: fmul  d2, d15, d2
		0xfd4033e3, // 0x200: ldr   d3, [sp, #0x60]
		0x1e6309e3, // 0x204: fmul  d3, d15, d3
		0x1f4f0441, // 0x208: fmadd d1, d2, d15, d1
		0x1e6309e2, // 0x20c: fmul  d2, d15, d3
		0x1e6209e2, // 0x210: fmul  d2, d15, d2
		0x1e6209e2, // 0x214: fmul  d2, d15, d2
		0x1f4f0441, // 0x218: fmadd d1, d2, d15, d1
		0x1e690800, // 0x21c: fmul  d0, d0, d9
		0x1f412001, // 0x220: fmadd d1, d0, d1, d8  (note: encoding was 0x1f412001 not 0x1f412000)
		0x1e613902, // 0x224: fsub  d2, d8, d1
		0xfd4037e0, // 0x228: ldr   d0, [sp, #0x68]
		0x1e602008, // 0x22c: fcmp  d0, #0.0
		0xb840467b, // 0x230: ldr   w27, [x19], #0x4
		0x6d4403e3, // 0x234: ldp   d3, d0, [sp, #0x40]
		0x1e638800, // 0x238: fnmul d0, d0, d3
		0x1e614c4c, // 0x23c: fcsel d12, d2, d1, mi
		0x940037b0, // 0x240: bl    _exp (→ 0xF100)
		0x6d4513e5, // 0x244: ldp   d5, d4, [sp, #0x50]
		0x1e6008a1, // 0x248: fmul  d1, d5, d0
		0x1e6c3902, // 0x24c: fsub  d2, d8, d12
		0xfd401fe6, // 0x250: ldr   d6, [sp, #0x38]
		0x1e663903, // 0x254: fsub  d3, d8, d6
		0x1e638883, // 0x258: fnmul d3, d4, d3
		0x1f420c21, // 0x25c: fmadd d1, d1, d2, d3
		0x1e6088a0, // 0x260: fnmul d0, d5, d0
		0x1e6c0800, // 0x264: fmul  d0, d0, d12
		0x1f460080, // 0x268: fmadd d0, d4, d6, d0
		0x7100037f, // 0x26c: cmp   w27, #0
		0x1e601c20, // 0x270: fcsel d0, d1, d0, ne
		0xfc008720, // 0x274: str   d0, [x25], #0x8
		0x91002294, // 0x278: add   x20, x20, #0x8
		0xf100075a, // 0x27c: subs  x26, x26, #1
		0x54fff321, // 0x280: b.ne  0xe4
		0xa9507bfd, // 0x284: ldp   x29, x30, [sp, #0x100]
		0xa94f4ff4, // 0x288: ldp   x20, x19, [sp, #0xf0]
		0xa94e57f6, // 0x28c: ldp   x22, x21, [sp, #0xe0]
		0xa94d5ff8, // 0x290: ldp   x24, x23, [sp, #0xd0]
		0xa94c67fa, // 0x294: ldp   x26, x25, [sp, #0xc0]
		0xa94b6ffc, // 0x298: ldp   x28, x27, [sp, #0xb0]
		0x6d4a23e9, // 0x29c: ldp   d9, d8, [sp, #0xa0]
		0x6d492beb, // 0x2a0: ldp   d11, d10, [sp, #0x90]
		0x6d4833ed, // 0x2a4: ldp   d13, d12, [sp, #0x80]
		0x6d473bef, // 0x2a8: ldp   d15, d14, [sp, #0x70]
		0x910443ff, // 0x2ac: add   sp, sp, #0x110
		0xd65f03c0, // 0x2b0: ret
	}

	codeBase := uint64(0x1000)
	logAddr := uint64(0xF000)  // BL target for log()
	expAddr := uint64(0xF100)  // BL target for exp()

	pricesBase := uint64(0x100000)
	sBase := uint64(0x200000)
	xBase := uint64(0x300000)
	rBase := uint64(0x400000)
	vBase := uint64(0x500000)
	tBase := uint64(0x600000)
	otypeBase := uint64(0x700000)
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

	// Helper to write float64 array to emulator memory
	writeF64 := func(base uint64, data []float64) {
		buf := make([]byte, len(data)*8)
		for i, v := range data {
			binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(base, buf)
	}

	// Helper to write int32 array to emulator memory
	writeI32 := func(base uint64, data []int32) {
		buf := make([]byte, len(data)*4)
		for i, v := range data {
			binary.LittleEndian.PutUint32(buf[i*4:], uint32(v))
		}
		em.LoadProgram(base, buf)
	}

	// Load input data
	writeF64(pricesBase, make([]float64, numOptions)) // output (zeroed)
	writeF64(sBase, stockPrices)
	writeF64(xBase, strikePrices)
	writeF64(rBase, riskFreeRate)
	writeF64(vBase, volatility)
	writeF64(tBase, timeToExp)
	writeI32(otypeBase, optionType)

	// Allocate stack space
	stackBuf := make([]byte, 8192)
	em.LoadProgram(stackBase, stackBuf)

	// Set registers per ARM64 ABI
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, pricesBase)
	em.Regs.WriteX(1, sBase)
	em.Regs.WriteX(2, xBase)
	em.Regs.WriteX(3, rBase)
	em.Regs.WriteX(4, vBase)
	em.Regs.WriteX(5, tBase)
	em.Regs.WriteX(6, otypeBase)
	em.Regs.WriteX(7, uint64(numOptions))
	em.Regs.WriteX(30, haltPC) // LR = halt address
	em.Regs.SP = stackBase + 4096

	em.SetHaltPC(haltPC)

	// Register BL-intercept hooks for exp and log
	em.RegisterFuncHook(logAddr, func(em *Emulator) {
		x := em.Regs.ReadD(0)
		em.Regs.WriteD(0, math.Log(x))
	})
	em.RegisterFuncHook(expAddr, func(em *Emulator) {
		x := em.Regs.ReadD(0)
		em.Regs.WriteD(0, math.Exp(x))
	})

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Read prices[] from emulator memory and compare
	maxRelErr := 0.0
	for i := 0; i < numOptions; i++ {
		bits := em.Mem.ReadU64(pricesBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := expectedPrices[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("prices[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	t.Logf("BlackScholes completed in %d instructions, max relative error = %e",
		em.InstCount, maxRelErr)
}
