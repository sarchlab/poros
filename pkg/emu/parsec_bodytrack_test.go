package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestParsecBodytrack(t *testing.T) {
	// End-to-end test: PARSEC Bodytrack particle filter likelihood kernel.
	//
	// The kernel computes particle likelihoods:
	//   likelihood[p] = exp(-0.5 * sum_i((proj * state - obs)[i]^2))
	//
	// It calls exp() via a BL instruction which is intercepted by a Go hook.
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize
	//                -fno-slp-vectorize -fno-unroll-loops -fno-stack-protector
	//
	// ARM64 ABI:
	//   X0 = likelihood output (double*)
	//   X1 = states (double[nParticles * stateDim])
	//   X2 = observations (double[obsDim])
	//   X3 = projMatrix (double[obsDim * stateDim])
	//   W4 = nParticles
	//   W5 = stateDim
	//   W6 = obsDim

	nParticles := 16
	stateDim := 6
	obsDim := 4

	// Initialize states
	states := make([]float64, nParticles*stateDim)
	for p := 0; p < nParticles; p++ {
		for j := 0; j < stateDim; j++ {
			states[p*stateDim+j] = 0.1*float64(p+1) + 0.05*float64(j)
		}
	}

	// Initialize observations
	observations := make([]float64, obsDim)
	for i := 0; i < obsDim; i++ {
		observations[i] = 1.0 + 0.5*float64(i)
	}

	// Initialize projection matrix
	projMatrix := make([]float64, obsDim*stateDim)
	for i := 0; i < obsDim; i++ {
		for j := 0; j < stateDim; j++ {
			projMatrix[i*stateDim+j] = float64((i+j)%5+1) * 0.1
		}
	}

	// Golden reference
	expectedLikelihood := make([]float64, nParticles)
	for p := 0; p < nParticles; p++ {
		sum := 0.0
		state := states[p*stateDim : (p+1)*stateDim]
		for i := 0; i < obsDim; i++ {
			predicted := 0.0
			for j := 0; j < stateDim; j++ {
				predicted += projMatrix[i*stateDim+j] * state[j]
			}
			diff := predicted - observations[i]
			sum += diff * diff
		}
		expectedLikelihood[p] = math.Exp(-0.5 * sum)
	}

	// Machine code: 71 instructions from compiled bodytrack_likelihood.
	// BL _exp at offset 0x06c is patched to target hook at 0xF100.
	//
	// BL at PC = 0x1006c, target = 0xF100
	// offset = 0xF100 - 0x1006c = -0xF6C => imm26 = 0x3FFFC25
	// Patched: 0x97FFFC25
	code := []uint32{
		0xd10203ff, // 0x000: sub sp, sp, #0x80
		0x6d0123e9, // 0x004: stp d9, d8, [sp, #0x10]
		0xa9026ffc, // 0x008: stp x28, x27, [sp, #0x20]
		0xa90367fa, // 0x00c: stp x26, x25, [sp, #0x30]
		0xa9045ff8, // 0x010: stp x24, x23, [sp, #0x40]
		0xa90557f6, // 0x014: stp x22, x21, [sp, #0x50]
		0xa9064ff4, // 0x018: stp x20, x19, [sp, #0x60]
		0xa9077bfd, // 0x01c: stp x29, x30, [sp, #0x70]
		0x9101c3fd, // 0x020: add x29, sp, #0x70
		0xb9000fe6, // 0x024: str w6, [sp, #0xc]
		0xf90003e3, // 0x028: str x3, [sp]
		0x7100049f, // 0x02c: cmp w4, #1
		0x5400064b, // 0x030: b.lt epilogue
		0xaa0503f4, // 0x034: mov x20, x5
		0xaa0203f6, // 0x038: mov x22, x2
		0xaa0103f7, // 0x03c: mov x23, x1
		0xaa0003f8, // 0x040: mov x24, x0
		0xd2800019, // 0x044: mov x25, #0
		0x2a0503fa, // 0x048: mov w26, w5
		0x2a0403fb, // 0x04c: mov w27, w4
		0xd37d7e9c, // 0x050: ubfiz x28, x20, #3, #32
		0x937d7e95, // 0x054: sbfiz x21, x20, #3, #32
		0x1e7c1008, // 0x058: fmov d8, #-0.5
		0xb9400fe8, // 0x05c: ldr w8, [sp, #0xc]
		0x2a0803f3, // 0x060: mov w19, w8
		0x14000008, // 0x064: b outer_check
		0x1e680800, // 0x068: fmul d0, d0, d8
		0x97fffc25, // 0x06c: bl _exp hook at 0xF100 (PATCHED)
		0xfc397b00, // 0x070: str d0, [x24, x25, lsl #3]
		0x91000739, // 0x074: add x25, x25, #1
		0x8b1502f7, // 0x078: add x23, x23, x21
		0xeb1b033f, // 0x07c: cmp x25, x27
		0x540003c0, // 0x080: b.eq epilogue
		0xb9400fe8, // 0x084: ldr w8, [sp, #0xc]
		0x7100051f, // 0x088: cmp w8, #1
		0x5400030b, // 0x08c: b.lt exp_negzero
		0xd2800008, // 0x090: mov x8, #0
		0x2f00e400, // 0x094: movi d0, #0
		0xf94003e9, // 0x098: ldr x9, [sp]
		0x14000008, // 0x09c: b inner_outer_check
		0xfc687ac2, // 0x0a0: ldr d2, [x22, x8, lsl #3]
		0x1e623821, // 0x0a4: fsub d1, d1, d2
		0x1f410020, // 0x0a8: fmadd d0, d1, d1, d0
		0x91000508, // 0x0ac: add x8, x8, #1
		0x8b1c0129, // 0x0b0: add x9, x9, x28
		0xeb13011f, // 0x0b4: cmp x8, x19
		0x54fffd80, // 0x0b8: b.eq fmul_exp
		0x2f00e401, // 0x0bc: movi d1, #0
		0x7100069f, // 0x0c0: cmp w20, #1 (was w20 cmp #1, stateDim check)
		0x54fffeeb, // 0x0c4: b.lt load_obs_sub
		0xaa1a03ea, // 0x0c8: mov x10, x26
		0xaa1703eb, // 0x0cc: mov x11, x23
		0xaa0903ec, // 0x0d0: mov x12, x9
		0xfc408582, // 0x0d4: ldr d2, [x12], #8
		0xfc408563, // 0x0d8: ldr d3, [x11], #8
		0x1f430441, // 0x0dc: fmadd d1, d2, d3, d1
		0xf100054a, // 0x0e0: subs x10, x10, #1
		0x54ffff81, // 0x0e4: b.ne inner_matvec
		0x17ffffee, // 0x0e8: b load_obs_sub
		0xd2f00008, // 0x0ec: mov x8, #0x8000000000000000
		0x9e670100, // 0x0f0: fmov d0, x8
		0x17ffffde, // 0x0f4: b bl_exp
		0xa9477bfd, // 0x0f8: ldp x29, x30, [sp, #0x70]
		0xa9464ff4, // 0x0fc: ldp x20, x19, [sp, #0x60]
		0xa94557f6, // 0x100: ldp x22, x21, [sp, #0x50]
		0xa9445ff8, // 0x104: ldp x24, x23, [sp, #0x40]
		0xa94367fa, // 0x108: ldp x26, x25, [sp, #0x30]
		0xa9426ffc, // 0x10c: ldp x28, x27, [sp, #0x20]
		0x6d4123e9, // 0x110: ldp d9, d8, [sp, #0x10]
		0x910203ff, // 0x114: add sp, sp, #0x80
		0xd65f03c0, // 0x118: ret
	}

	codeBase := uint64(0x10000)
	expHookAddr := uint64(0xF100)

	likelihoodBase := uint64(0x100000)
	statesBase := uint64(0x200000)
	obsBase := uint64(0x300000)
	projBase := uint64(0x400000)
	stackBase := uint64(0x500000)
	haltPC := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 10_000_000

	// Encode and load code
	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	// Load states
	statesBuf := make([]byte, nParticles*stateDim*8)
	for i, v := range states {
		binary.LittleEndian.PutUint64(statesBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(statesBase, statesBuf)

	// Load observations
	obsBuf := make([]byte, obsDim*8)
	for i, v := range observations {
		binary.LittleEndian.PutUint64(obsBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(obsBase, obsBuf)

	// Load projection matrix
	projBuf := make([]byte, obsDim*stateDim*8)
	for i, v := range projMatrix {
		binary.LittleEndian.PutUint64(projBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(projBase, projBuf)

	// Output buffer (zeros)
	em.LoadProgram(likelihoodBase, make([]byte, nParticles*8))

	// Stack
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set registers
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, likelihoodBase)  // likelihood output
	em.Regs.WriteX(1, statesBase)      // states
	em.Regs.WriteX(2, obsBase)         // observations
	em.Regs.WriteX(3, projBase)        // projMatrix
	em.Regs.WriteX(4, uint64(nParticles))
	em.Regs.WriteX(5, uint64(stateDim))
	em.Regs.WriteX(6, uint64(obsDim))
	em.Regs.WriteX(30, haltPC)
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	// Register exp function hook
	em.RegisterFuncHook(expHookAddr, func(emInner *Emulator) {
		x := emInner.Regs.ReadD(0)
		emInner.Regs.WriteD(0, math.Exp(x))
	})

	// Run
	err := em.Run()
	if err != nil {
		t.Fatalf("emulation failed: %v", err)
	}

	// Read results and compare
	const tol = 1e-9
	for p := 0; p < nParticles; p++ {
		addr := likelihoodBase + uint64(p*8)
		bits := em.Mem.ReadU64(addr)
		got := math.Float64frombits(bits)
		want := expectedLikelihood[p]
		if math.Abs(got-want) > tol && math.Abs(got-want)/math.Abs(want) > tol {
			t.Errorf("likelihood[%d]: got %.12e, want %.12e (diff=%.4e)",
				p, got, want, math.Abs(got-want))
		}
	}
	t.Logf("bodytrack: %d particles verified, %d instructions executed",
		nParticles, em.InstCount)
}
