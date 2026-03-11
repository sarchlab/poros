package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

// sphComputeGolden is the Go golden reference for the SPH kernel.
func sphComputeGolden(
	px, py, pz []float64,
	vx, vy, vz []float64,
	n int,
	h, mass, rho0, k, mu float64,
) (density, pressure, fx, fy, fz []float64) {
	h2 := h * h
	poly6Coeff := 315.0 / (64.0 * 3.141592653589793 * h * h * h * h * h * h * h * h * h)
	spikyCoeff := -45.0 / (3.141592653589793 * h * h * h * h * h * h)
	viscCoeff := 45.0 / (3.141592653589793 * h * h * h * h * h * h)

	density = make([]float64, n)
	pressure = make([]float64, n)
	fx = make([]float64, n)
	fy = make([]float64, n)
	fz = make([]float64, n)

	// Phase 1: densities
	for i := 0; i < n; i++ {
		rho := 0.0
		for j := 0; j < n; j++ {
			dx := px[i] - px[j]
			dy := py[i] - py[j]
			dz := pz[i] - pz[j]
			r2 := dx*dx + dy*dy + dz*dz
			if r2 < h2 {
				diff := h2 - r2
				rho += mass * poly6Coeff * diff * diff * diff
			}
		}
		density[i] = rho
		pressure[i] = k * (rho - rho0)
	}

	// Phase 2: forces
	for i := 0; i < n; i++ {
		fxi, fyi, fzi := 0.0, 0.0, 0.0
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			dx := px[i] - px[j]
			dy := py[i] - py[j]
			dz := pz[i] - pz[j]
			r2 := dx*dx + dy*dy + dz*dz
			if r2 < h2 && r2 > 1e-12 {
				r := math.Sqrt(r2)
				diff := h - r
				pterm := -mass * (pressure[i] + pressure[j]) / (2.0 * density[j])
				fp := spikyCoeff * diff * diff * pterm / r
				fxi += fp * dx
				fyi += fp * dy
				fzi += fp * dz
				vterm := mu * mass / density[j]
				fv := viscCoeff * diff * vterm
				fxi += fv * (vx[j] - vx[i])
				fyi += fv * (vy[j] - vy[i])
				fzi += fv * (vz[j] - vz[i])
			}
		}
		fx[i] = fxi
		fy[i] = fyi
		fz[i] = fzi
	}
	return
}

func TestParsecFluidanimate(t *testing.T) {
	// End-to-end test: PARSEC fluidanimate SPH kernel.
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize
	//                -fno-slp-vectorize -fno-unroll-loops -fno-stack-protector
	//                -fno-math-errno
	//
	// ARM64 ABI for sph_compute:
	//   x0=px, x1=py, x2=pz, x3=vx, x4=vy, x5=vz, x6=density, x7=pressure
	//   [SP+0]=fx, [SP+8]=fy, [SP+16]=fz
	//   [SP+24]=n (int32)
	//   d0=h, d1=mass, d2=rho0, d3=k, d4=mu

	numParticles := 8

	// Particle positions: arranged in a grid within smoothing radius
	h := 1.0
	px := []float64{0.0, 0.3, 0.6, 0.0, 0.3, 0.6, 0.0, 0.3}
	py := []float64{0.0, 0.0, 0.0, 0.3, 0.3, 0.3, 0.6, 0.6}
	pz := []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}

	// Particle velocities
	vx := []float64{0.1, -0.1, 0.2, -0.2, 0.1, -0.1, 0.2, -0.2}
	vy := []float64{0.0, 0.1, -0.1, 0.2, -0.2, 0.1, -0.1, 0.2}
	vz := []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}

	mass := 1.0
	rho0 := 1000.0
	kConst := 3.0
	mu := 3.5

	// Compute golden reference
	goldenDensity, goldenPressure, goldenFx, goldenFy, goldenFz :=
		sphComputeGolden(px, py, pz, vx, vy, vz, numParticles, h, mass, rho0, kConst, mu)

	_ = goldenPressure // used for validation but not directly checked here

	// Machine code from compiled fluidanimate.c
	code := []uint32{
		0xb9401be8, // 0x000: ldr   w8, [sp, #0x18]
		0x7100051f, // 0x004: cmp   w8, #1
		0x5400130b, // 0x008: b.lt  0x268
		0xd280000c, // 0x00c: mov   x12, #0
		0xd285a30a, // 0x010: mov   x10, #0x2d18
		0xf2aa888a, // 0x014: movk  x10, #0x5444, lsl #16
		0xf2c43f6a, // 0x018: movk  x10, #0x21fb, lsl #32
		0xf2e8012a, // 0x01c: movk  x10, #0x4009, lsl #48
		0x9e670145, // 0x020: fmov  d5, x10
		0xd2d0000a, // 0x024: mov   x10, #0x800000000000
		0xf2f808ca, // 0x028: movk  x10, #0xc046, lsl #48
		0x9e670146, // 0x02c: fmov  d6, x10
		0xd2d0000a, // 0x030: mov   x10, #0x800000000000
		0xf2e808ca, // 0x034: movk  x10, #0x4046, lsl #48
		0x9e670147, // 0x038: fmov  d7, x10
		0xd285a30a, // 0x03c: mov   x10, #0x2d18
		0xf2aa888a, // 0x040: movk  x10, #0x5444, lsl #16
		0xf2c43f6a, // 0x044: movk  x10, #0x21fb, lsl #32
		0xf2e80d2a, // 0x048: movk  x10, #0x4069, lsl #48
		0x9e670150, // 0x04c: fmov  d16, x10
		0xd2d6000a, // 0x050: mov   x10, #0xb00000000000
		0xf2e80e6a, // 0x054: movk  x10, #0x4073, lsl #48
		0x9e670151, // 0x058: fmov  d17, x10
		0xa940a7ea, // 0x05c: ldp   x10, x9, [sp, #0x8]
		0xf94003eb, // 0x060: ldr   x11, [sp]
		0x1e650805, // 0x064: fmul  d5, d0, d5
		0x1e6008a5, // 0x068: fmul  d5, d5, d0
		0x1e6008a5, // 0x06c: fmul  d5, d5, d0
		0x1e6008a5, // 0x070: fmul  d5, d5, d0
		0x1e6008a5, // 0x074: fmul  d5, d5, d0
		0x1e6008b2, // 0x078: fmul  d18, d5, d0
		0x1e7218c5, // 0x07c: fdiv  d5, d6, d18
		0x1e7218e6, // 0x080: fdiv  d6, d7, d18
		0x1e600807, // 0x084: fmul  d7, d0, d0
		0x1e700810, // 0x088: fmul  d16, d0, d16
		0x1e600a10, // 0x08c: fmul  d16, d16, d0
		0x1e600a10, // 0x090: fmul  d16, d16, d0
		0x1e600a10, // 0x094: fmul  d16, d16, d0
		0x1e600a10, // 0x098: fmul  d16, d16, d0
		0x1e600a10, // 0x09c: fmul  d16, d16, d0
		0x1e600a10, // 0x0a0: fmul  d16, d16, d0
		0x1e600a10, // 0x0a4: fmul  d16, d16, d0
		0x1e600a10, // 0x0a8: fmul  d16, d16, d0
		0x1e701a30, // 0x0ac: fdiv  d16, d17, d16
		0x1e610a10, // 0x0b0: fmul  d16, d16, d1
		0xfc6c7811, // 0x0b4: ldr   d17, [x0, x12, lsl #3]
		0xfc6c7833, // 0x0b8: ldr   d19, [x1, x12, lsl #3]
		0x2f00e412, // 0x0bc: movi  d18, #0
		0xaa0803ed, // 0x0c0: mov   x13, x8
		0xfc6c7854, // 0x0c4: ldr   d20, [x2, x12, lsl #3]
		0xaa0003ee, // 0x0c8: mov   x14, x0
		0xaa0103ef, // 0x0cc: mov   x15, x1
		0xaa0203f0, // 0x0d0: mov   x16, x2
		0xfc4085d5, // 0x0d4: ldr   d21, [x14], #0x8
		0x1e753a35, // 0x0d8: fsub  d21, d17, d21
		0xfc4085f6, // 0x0dc: ldr   d22, [x15], #0x8
		0xfc408617, // 0x0e0: ldr   d23, [x16], #0x8
		0x1e763a76, // 0x0e4: fsub  d22, d19, d22
		0x1e773a97, // 0x0e8: fsub  d23, d20, d23
		0x1e760ad6, // 0x0ec: fmul  d22, d22, d22
		0x1f555ab5, // 0x0f0: fmadd d21, d21, d21, d22
		0x1f5756f5, // 0x0f4: fmadd d21, d23, d23, d21
		0x1e7538f6, // 0x0f8: fsub  d22, d7, d21
		0x1e760a17, // 0x0fc: fmul  d23, d16, d22
		0x1e770ad7, // 0x100: fmul  d23, d22, d23
		0x1f564af6, // 0x104: fmadd d22, d23, d22, d18
		0x1e6722a0, // 0x108: fcmp  d21, d7
		0x1e724ed2, // 0x10c: fcsel d18, d22, d18, mi
		0xf10005ad, // 0x110: subs  x13, x13, #1
		0x54fffe01, // 0x114: b.ne  0xd4
		0xfc2c78d2, // 0x118: str   d18, [x6, x12, lsl #3]
		0x1e623a51, // 0x11c: fsub  d17, d18, d2
		0x1e630a31, // 0x120: fmul  d17, d17, d3
		0xfc2c78f1, // 0x124: str   d17, [x7, x12, lsl #3]
		0x9100058c, // 0x128: add   x12, x12, #1
		0xeb08019f, // 0x12c: cmp   x12, x8
		0x54fffc21, // 0x130: b.ne  0xb4
		0x7100051f, // 0x134: cmp   w8, #1
		0x5400098b, // 0x138: b.lt  0x268
		0xd280000c, // 0x13c: mov   x12, #0
		0xd280000d, // 0x140: mov   x13, #0
		0x1e614022, // 0x144: fneg  d2, d1
		0x1e640821, // 0x148: fmul  d1, d1, d4
		0xd37d7d0e, // 0x14c: ubfiz x14, x8, #3, #32
		0xd29d422f, // 0x150: mov   x15, #0xea11
		0xf2b025af, // 0x154: movk  x15, #0x812d, lsl #16
		0xf2d2f32f, // 0x158: movk  x15, #0x9799, lsl #32
		0xf2e7ae2f, // 0x15c: movk  x15, #0x3d71, lsl #48
		0x14000008, // 0x160: b     0x180
		0xfc2d7963, // 0x164: str   d3, [x11, x13, lsl #3]
		0xfc2d7944, // 0x168: str   d4, [x10, x13, lsl #3]
		0xfc2d7930, // 0x16c: str   d16, [x9, x13, lsl #3]
		0x910005ad, // 0x170: add   x13, x13, #1
		0x9100218c, // 0x174: add   x12, x12, #8
		0xeb0801bf, // 0x178: cmp   x13, x8
		0x54000760, // 0x17c: b.eq  0x268
		0xd2800010, // 0x180: mov   x16, #0
		0x2f00e403, // 0x184: movi  d3, #0
		0x2f00e404, // 0x188: movi  d4, #0
		0x2f00e410, // 0x18c: movi  d16, #0
		0x14000004, // 0x190: b     0x1a0
		0x91002210, // 0x194: add   x16, x16, #8
		0xeb1001df, // 0x198: cmp   x14, x16
		0x54fffe40, // 0x19c: b.eq  0x164
		0xeb10019f, // 0x1a0: cmp   x12, x16
		0x54ffff80, // 0x1a4: b.eq  0x194
		0xfc6d7811, // 0x1a8: ldr   d17, [x0, x13, lsl #3]
		0xfc706812, // 0x1ac: ldr   d18, [x0, x16]
		0x1e723a31, // 0x1b0: fsub  d17, d17, d18
		0xfc6d7832, // 0x1b4: ldr   d18, [x1, x13, lsl #3]
		0xfc706833, // 0x1b8: ldr   d19, [x1, x16]
		0x1e733a52, // 0x1bc: fsub  d18, d18, d19
		0xfc6d7853, // 0x1c0: ldr   d19, [x2, x13, lsl #3]
		0xfc706854, // 0x1c4: ldr   d20, [x2, x16]
		0x1e743a73, // 0x1c8: fsub  d19, d19, d20
		0x1e720a54, // 0x1cc: fmul  d20, d18, d18
		0x1f515234, // 0x1d0: fmadd d20, d17, d17, d20
		0x1f535274, // 0x1d4: fmadd d20, d19, d19, d20
		0x1e672280, // 0x1d8: fcmp  d20, d7
		0x9e6701f5, // 0x1dc: fmov  d21, x15
		0x1e754684, // 0x1e0: fccmp d20, d21, #4, mi
		0x54fffd8d, // 0x1e4: b.le  0x194
		0x1e61c294, // 0x1e8: fsqrt d20, d20
		0x1e743815, // 0x1ec: fsub  d21, d0, d20
		0xfc6d78f6, // 0x1f0: ldr   d22, [x7, x13, lsl #3]
		0xfc7068f7, // 0x1f4: ldr   d23, [x7, x16]
		0x1e772ad6, // 0x1f8: fadd  d22, d22, d23
		0x1e620ad6, // 0x1fc: fmul  d22, d22, d2
		0xfc7068d7, // 0x200: ldr   d23, [x6, x16]
		0x1e772af8, // 0x204: fadd  d24, d23, d23
		0x1e781ad6, // 0x208: fdiv  d22, d22, d24
		0x1e7508b8, // 0x20c: fmul  d24, d5, d21
		0x1e780ab8, // 0x210: fmul  d24, d21, d24
		0x1e760b16, // 0x214: fmul  d22, d24, d22
		0x1e741ad4, // 0x218: fdiv  d20, d22, d20
		0x1f510e83, // 0x21c: fmadd d3, d20, d17, d3
		0x1f521284, // 0x220: fmadd d4, d20, d18, d4
		0x1f534290, // 0x224: fmadd d16, d20, d19, d16
		0x1e771831, // 0x228: fdiv  d17, d1, d23
		0x1e7508d2, // 0x22c: fmul  d18, d6, d21
		0x1e710a51, // 0x230: fmul  d17, d18, d17
		0xfc706872, // 0x234: ldr   d18, [x3, x16]
		0xfc6d7873, // 0x238: ldr   d19, [x3, x13, lsl #3]
		0x1e733a52, // 0x23c: fsub  d18, d18, d19
		0x1f520e23, // 0x240: fmadd d3, d17, d18, d3
		0xfc706892, // 0x244: ldr   d18, [x4, x16]
		0xfc6d7893, // 0x248: ldr   d19, [x4, x13, lsl #3]
		0x1e733a52, // 0x24c: fsub  d18, d18, d19
		0x1f521224, // 0x250: fmadd d4, d17, d18, d4
		0xfc7068b2, // 0x254: ldr   d18, [x5, x16]
		0xfc6d78b3, // 0x258: ldr   d19, [x5, x13, lsl #3]
		0x1e733a52, // 0x25c: fsub  d18, d18, d19
		0x1f524230, // 0x260: fmadd d16, d17, d18, d16
		0x17ffffcc, // 0x264: b     0x194
		0xd65f03c0, // 0x268: ret
	}

	codeBase := uint64(0x1000)
	n := numParticles

	// Memory layout
	pxBase := uint64(0x100000)
	pyBase := pxBase + uint64(n)*8
	pzBase := pyBase + uint64(n)*8
	vxBase := pzBase + uint64(n)*8
	vyBase := vxBase + uint64(n)*8
	vzBase := vyBase + uint64(n)*8
	densityBase := vzBase + uint64(n)*8
	pressureBase := densityBase + uint64(n)*8
	fxBase := pressureBase + uint64(n)*8
	fyBase := fxBase + uint64(n)*8
	fzBase := fyBase + uint64(n)*8
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

	// Helper to write float64 array
	writeF64 := func(base uint64, data []float64) {
		buf := make([]byte, len(data)*8)
		for i, v := range data {
			binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
		}
		em.LoadProgram(base, buf)
	}

	// Load input data
	writeF64(pxBase, px)
	writeF64(pyBase, py)
	writeF64(pzBase, pz)
	writeF64(vxBase, vx)
	writeF64(vyBase, vy)
	writeF64(vzBase, vz)

	// Zero output arrays
	writeF64(densityBase, make([]float64, n))
	writeF64(pressureBase, make([]float64, n))
	writeF64(fxBase, make([]float64, n))
	writeF64(fyBase, make([]float64, n))
	writeF64(fzBase, make([]float64, n))

	// Allocate stack
	stackBuf := make([]byte, 8192)
	em.LoadProgram(stackBase, stackBuf)

	// Set up stack arguments
	sp := stackBase + 4096
	// [SP+0]=fx, [SP+8]=fy, [SP+16]=fz, [SP+24]=n (int32)
	binary.LittleEndian.PutUint64(stackBuf[4096:], fxBase)
	binary.LittleEndian.PutUint64(stackBuf[4096+8:], fyBase)
	binary.LittleEndian.PutUint64(stackBuf[4096+16:], fzBase)
	binary.LittleEndian.PutUint32(stackBuf[4096+24:], uint32(n))
	em.LoadProgram(stackBase, stackBuf)

	// Set registers per ARM64 ABI
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, pxBase)
	em.Regs.WriteX(1, pyBase)
	em.Regs.WriteX(2, pzBase)
	em.Regs.WriteX(3, vxBase)
	em.Regs.WriteX(4, vyBase)
	em.Regs.WriteX(5, vzBase)
	em.Regs.WriteX(6, densityBase)
	em.Regs.WriteX(7, pressureBase)
	em.Regs.WriteX(30, haltPC) // LR = halt
	em.Regs.SP = sp

	// FP registers: d0=h, d1=mass, d2=rho0, d3=k, d4=mu
	em.Regs.WriteD(0, h)
	em.Regs.WriteD(1, mass)
	em.Regs.WriteD(2, rho0)
	em.Regs.WriteD(3, kConst)
	em.Regs.WriteD(4, mu)

	em.SetHaltPC(haltPC)

	if err := em.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Read and compare density results
	maxRelErr := 0.0
	for i := 0; i < n; i++ {
		bits := em.Mem.ReadU64(densityBase + uint64(i)*8)
		got := math.Float64frombits(bits)
		want := goldenDensity[i]

		denom := math.Max(1.0, math.Abs(want))
		relErr := math.Abs(got-want) / denom
		if relErr > maxRelErr {
			maxRelErr = relErr
		}
		if relErr > 1e-10 {
			t.Errorf("density[%d] = %g, want %g (relErr=%e)", i, got, want, relErr)
		}
	}

	// Read and compare force results
	for i := 0; i < n; i++ {
		gotFx := math.Float64frombits(em.Mem.ReadU64(fxBase + uint64(i)*8))
		gotFy := math.Float64frombits(em.Mem.ReadU64(fyBase + uint64(i)*8))
		gotFz := math.Float64frombits(em.Mem.ReadU64(fzBase + uint64(i)*8))

		wantFx := goldenFx[i]
		wantFy := goldenFy[i]
		wantFz := goldenFz[i]

		checkForce := func(name string, got, want float64) {
			denom := math.Max(1.0, math.Abs(want))
			relErr := math.Abs(got-want) / denom
			if relErr > maxRelErr {
				maxRelErr = relErr
			}
			if relErr > 1e-6 {
				t.Errorf("%s[%d] = %g, want %g (relErr=%e)", name, i, got, want, relErr)
			}
		}

		checkForce("fx", gotFx, wantFx)
		checkForce("fy", gotFy, wantFy)
		checkForce("fz", gotFz, wantFz)
	}

	t.Logf("Fluidanimate SPH completed in %d instructions, max relative error = %e",
		em.InstCount, maxRelErr)
}
