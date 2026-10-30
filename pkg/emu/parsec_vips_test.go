package emu

import (
	"encoding/binary"
	"math"
	"testing"
)

// convolve2dGolden is the Go golden reference for 2D convolution.
func convolve2dGolden(output, input, kernel []float64, width, height, ksize int) {
	half := ksize / 2
	for y := half; y < height-half; y++ {
		for x := half; x < width-half; x++ {
			sum := 0.0
			for ky := -half; ky <= half; ky++ {
				for kx := -half; kx <= half; kx++ {
					sum += input[(y+ky)*width+(x+kx)] * kernel[(ky+half)*ksize+(kx+half)]
				}
			}
			output[y*width+x] = sum
		}
	}
}

func TestParsecVips(t *testing.T) {
	// End-to-end test: PARSEC VIPS 2D convolution kernel.
	//
	// The kernel applies a KxK convolution filter to a WxH image.
	// No external function calls (no BL instructions).
	//
	// Compiled with: clang -target arm64-apple-macos -O1 -fno-vectorize
	//                -fno-slp-vectorize -fno-unroll-loops -fno-stack-protector
	//
	// Native timing reference (M2, -O1, W=64 H=64 K=3, median of 7 runs):
	//   ~89,800 ns → adjusted 1.976e-04 seconds (×2.2 E/P-core)
	//
	// ARM64 ABI:
	//   X0=output, X1=input, X2=kernel, W3=width, W4=height, W5=ksize

	W, H, K := 32, 32, 3
	half := K / 2

	// Initialize input image
	input := make([]float64, W*H)
	for i := range input {
		input[i] = float64(i%256) / 255.0
	}

	// Initialize convolution kernel (Gaussian-like 3x3)
	kern := []float64{
		1.0 / 16, 2.0 / 16, 1.0 / 16,
		2.0 / 16, 4.0 / 16, 2.0 / 16,
		1.0 / 16, 2.0 / 16, 1.0 / 16,
	}

	// Golden reference
	goldenOutput := make([]float64, W*H)
	convolve2dGolden(goldenOutput, input, kern, W, H, K)

	// Machine code: convolve2d function (66 instructions)
	// Compiled from parsec/kernels/vips.c
	//
	// Original architectural CNEG conditions; no emulator compensation.
	code := []uint32{
		0x0b457ca8, // add   w8, w5, w5, lsr #31
		0x13017d08, // asr   w8, w8, #1
		0x4b08008d, // sub   w13, w4, w8
		0x6b0d011f, // cmp   w8, w13
		0x540007aa, // b.ge  0x104
		0xa9bd5ff8, // stp   x24, x23, [sp, #-0x30]!
		0xa90157f6, // stp   x22, x21, [sp, #0x10]
		0xa9024ff4, // stp   x20, x19, [sp, #0x20]
		0x52800009, // mov   w9, #0
		0x4b08006e, // sub   w14, w3, w8
		0x7100011f, // cmp   w8, #0
		0x5a88550a, // cneg  w10, w8, mi
		0x93407d0b, // sxtw  x11, w8
		0x93407c6c, // sxtw  x12, w3
		0x93407dad, // sxtw  x13, w13
		0x93407dce, // sxtw  x14, w14
		0x4b0803ef, // neg   w15, w8
		0x6b0f011f, // cmp   w8, w15
		0x5a88c510, // cneg  w16, w8, le
		0x0b100110, // add   w16, w8, w16
		0x11000610, // add   w16, w16, #1
		0xaa0b03f1, // mov   x17, x11
		0x14000005, // b     0x6c
		0x91000631, // add   x17, x17, #1
		0x0b030129, // add   w9, w9, w3
		0xeb0d023f, // cmp   x17, x13
		0x54000480, // b.eq  0xf8
		0x6b0e011f, // cmp   w8, w14
		0x54ffff6a, // b.ge  0x5c
		0x9b0c7e24, // mul   x4, x17, x12
		0x8b040c04, // add   x4, x0, x4, lsl #3
		0xaa0903e6, // mov   x6, x9
		0xaa0b03e7, // mov   x7, x11
		0x14000006, // b     0x9c
		0xfc277880, // str   d0, [x4, x7, lsl #3]
		0x910004e7, // add   x7, x7, #1
		0x110004c6, // add   w6, w6, #1
		0xeb0e00ff, // cmp   x7, x14
		0x54fffe20, // b.eq  0x5c
		0x2f00e400, // movi  d0, #0
		0x310004bf, // cmn   w5, #1
		0x54ffff2b, // b.lt  0x88
		0x52800013, // mov   w19, #0
		0xaa0603f4, // mov   x20, x6
		0xaa0f03f5, // mov   x21, x15
		0xaa1003f6, // mov   x22, x16
		0xaa1403f7, // mov   x23, x20
		0xaa1303f8, // mov   x24, x19
		0xfc77d821, // ldr   d1, [x1, w23, sxtw #3]
		0xfc78d842, // ldr   d2, [x2, w24, sxtw #3]
		0x1f420020, // fmadd d0, d1, d2, d0
		0x11000718, // add   w24, w24, #1
		0x110006f7, // add   w23, w23, #1
		0x710006d6, // subs  w22, w22, #1
		0x54ffff41, // b.ne  0xc0
		0x110006b6, // add   w22, w21, #1
		0x0b050273, // add   w19, w19, w5
		0x0b030294, // add   w20, w20, w3
		0x6b0a02bf, // cmp   w21, w10
		0xaa1603f5, // mov   x21, x22
		0x54fffe21, // b.ne  0xb4
		0x17ffffe5, // b     0x88
		0xa9424ff4, // ldp   x20, x19, [sp, #0x20]
		0xa94157f6, // ldp   x22, x21, [sp, #0x10]
		0xa8c35ff8, // ldp   x24, x23, [sp], #0x30
		0xd65f03c0, // ret
	}

	codeBase := uint64(0x10000)
	outputBase := uint64(0x100000)
	inputBase := uint64(0x200000)
	kernelBase := uint64(0x300000)
	haltPC := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 100_000_000

	// Load program
	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	// Load input data
	inputBuf := make([]byte, W*H*8)
	for i, v := range input {
		binary.LittleEndian.PutUint64(inputBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(inputBase, inputBuf)

	// Load kernel data
	kernBuf := make([]byte, K*K*8)
	for i, v := range kern {
		binary.LittleEndian.PutUint64(kernBuf[i*8:], math.Float64bits(v))
	}
	em.LoadProgram(kernelBase, kernBuf)

	// Output buffer (zeros)
	em.LoadProgram(outputBase, make([]byte, W*H*8))

	// Stack
	stackBase := uint64(0x400000)
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set up registers (match C function ABI)
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, outputBase) // output
	em.Regs.WriteX(1, inputBase)  // input
	em.Regs.WriteX(2, kernelBase) // kernel
	em.Regs.WriteX(3, uint64(W))  // width (W3)
	em.Regs.WriteX(4, uint64(H))  // height (W4)
	em.Regs.WriteX(5, uint64(K))  // ksize (W5)
	em.Regs.WriteX(30, haltPC)    // return address
	em.Regs.SP = stackBase + 8192
	em.SetHaltPC(haltPC)

	// Run
	err := em.Run()
	if err != nil {
		t.Fatalf("emulator error: %v", err)
	}

	if !em.Halted {
		t.Fatal("emulator should have halted")
	}

	// Read output and compare
	maxRelErr := 0.0
	for y := half; y < H-half; y++ {
		for x := half; x < W-half; x++ {
			idx := y*W + x
			addr := outputBase + uint64(idx*8)
			bits := em.Mem.ReadU64(addr)
			got := math.Float64frombits(bits)
			want := goldenOutput[idx]

			denom := math.Max(1.0, math.Abs(want))
			relErr := math.Abs(got-want) / denom
			if relErr > maxRelErr {
				maxRelErr = relErr
			}
			if relErr > 1e-10 {
				t.Errorf("output[%d][%d]: got %v, want %v (relErr=%e)", y, x, got, want, relErr)
			}
		}
	}

	t.Logf("vips: %d instructions executed, max relative error = %e",
		em.InstCount, maxRelErr)
}
