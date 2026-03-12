package emu

import (
	"encoding/binary"
	"testing"
)

// x264GoldenSAD16x16 computes the Sum of Absolute Differences for a 16x16
// macroblock between cur and ref, using the given stride.
func x264GoldenSAD16x16(cur, ref []byte, stride int) int {
	sad := 0
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			diff := int(cur[y*stride+x]) - int(ref[y*stride+x])
			if diff < 0 {
				diff = -diff
			}
			sad += diff
		}
	}
	return sad
}

// x264GoldenHadamard4x4 performs a 4x4 Hadamard transform on src, writing
// the result to dst. Both slices must have length 16.
func x264GoldenHadamard4x4(src, dst []int16) {
	var tmp [16]int16

	// Horizontal transform
	for i := 0; i < 4; i++ {
		s0 := int(src[i*4+0])
		s1 := int(src[i*4+1])
		s2 := int(src[i*4+2])
		s3 := int(src[i*4+3])
		d0 := s0 + s1
		d1 := s0 - s1
		d2 := s2 + s3
		d3 := s2 - s3
		tmp[i*4+0] = int16(d0 + d2)
		tmp[i*4+1] = int16(d1 + d3)
		tmp[i*4+2] = int16(d0 - d2)
		tmp[i*4+3] = int16(d1 - d3)
	}

	// Vertical transform
	for i := 0; i < 4; i++ {
		s0 := int(tmp[0*4+i])
		s1 := int(tmp[1*4+i])
		s2 := int(tmp[2*4+i])
		s3 := int(tmp[3*4+i])
		d0 := s0 + s1
		d1 := s0 - s1
		d2 := s2 + s3
		d3 := s2 - s3
		dst[0*4+i] = int16(d0 + d2)
		dst[1*4+i] = int16(d1 + d3)
		dst[2*4+i] = int16(d0 - d2)
		dst[3*4+i] = int16(d1 - d3)
	}
}

// x264GoldenSATD4x4 computes the Sum of Absolute Transformed Differences
// for a 4x4 block.
func x264GoldenSATD4x4(cur, ref []byte, stride int) int {
	var diff, hada [16]int16
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			diff[y*4+x] = int16(int(cur[y*stride+x]) - int(ref[y*stride+x]))
		}
	}
	x264GoldenHadamard4x4(diff[:], hada[:])
	satd := 0
	for i := 0; i < 16; i++ {
		v := int(hada[i])
		if v < 0 {
			v = -v
		}
		satd += v
	}
	return (satd + 1) >> 1
}

// x264GoldenMotionSearch performs motion estimation matching the C kernel.
// Returns bestCost, bestMx, bestMy.
func x264GoldenMotionSearch(
	cur []byte, curOff int,
	ref []byte, refOff int,
	stride, searchRange int,
) (int, int, int) {
	best := 0x7FFFFFFF
	bmx, bmy := 0, 0

	for my := -searchRange; my <= searchRange; my++ {
		for mx := -searchRange; mx <= searchRange; mx++ {
			rOff := refOff + my*stride + mx
			cost := x264GoldenSAD16x16(cur[curOff:], ref[rOff:], stride)
			if cost < best {
				best = cost
				bmx = mx
				bmy = my
			}
		}
	}

	// Refine with SATD
	bestRefOff := refOff + bmy*stride + bmx
	totalSATD := 0
	for by := 0; by < 16; by += 4 {
		for bx := 0; bx < 16; bx += 4 {
			off1 := curOff + by*stride + bx
			off2 := bestRefOff + by*stride + bx
			totalSATD += x264GoldenSATD4x4(cur[off1:], ref[off2:], stride)
		}
	}

	return totalSATD, bmx, bmy
}

func TestParsecX264(t *testing.T) {
	// End-to-end test: PARSEC x264 motion estimation kernel.
	//
	// 100% integer — no floating-point instructions.
	// Uses SAD (Sum of Absolute Differences) for full-pel search,
	// then refines with SATD (Sum of Absolute Transformed Differences)
	// using 4x4 Hadamard transforms.
	//
	// ARM64 ABI:
	//   X0 = cur (unsigned char*)
	//   X1 = ref (unsigned char*)
	//   W2 = stride
	//   W3 = range (search range)
	//   X4 = best_cost output (int*)
	//   X5 = best_mx output (int*)
	//   X6 = best_my output (int*)

	// Frame parameters for test: 64x64 frame, 16x16 macroblock, search range 4
	frameW := 64
	searchRange := 4

	// Create frame data: current and reference frames
	// Current macroblock at center of frame
	curOffY := 24 // row offset for current block
	curOffX := 24 // col offset for current block
	curOff := curOffY*frameW + curOffX

	// Reference frame: same as current but shifted by (2, -1)
	// This means the best match should be at mx=2, my=-1
	frameSize := frameW * frameW

	curFrame := make([]byte, frameSize)
	refFrame := make([]byte, frameSize)

	// Fill current frame with pseudo-random data
	for i := 0; i < frameSize; i++ {
		curFrame[i] = byte((i*73 + 17) % 256)
	}

	// Reference frame: shift the current block by (2, -1)
	// Copy the current macroblock to ref at offset (curOffY-1, curOffX+2)
	copy(refFrame, curFrame) // start with same data
	bestMxExpected := 2
	bestMyExpected := -1
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			srcIdx := (curOffY+y)*frameW + (curOffX + x)
			dstIdx := (curOffY+bestMyExpected+y)*frameW + (curOffX + bestMxExpected + x)
			refFrame[dstIdx] = curFrame[srcIdx]
		}
	}

	// Compute golden result
	goldenCost, goldenMx, goldenMy := x264GoldenMotionSearch(
		curFrame, curOff,
		refFrame, curOff,
		frameW, searchRange,
	)

	t.Logf("Golden: cost=%d, mx=%d, my=%d", goldenCost, goldenMx, goldenMy)

	// Verify the golden result finds the expected motion vector
	if goldenMx != bestMxExpected || goldenMy != bestMyExpected {
		t.Fatalf("Golden motion vector mismatch: got (%d, %d), want (%d, %d)",
			goldenMx, goldenMy, bestMxExpected, bestMyExpected)
	}

	// Machine code from x264.o compiled with:
	//   clang -target arm64-apple-macos -O1 -fno-vectorize
	//         -fno-slp-vectorize -fno-unroll-loops -fno-stack-protector
	//
	// CNEG conditions patched to match emulator semantics:
	//   cneg mi → cneg pl (condition field 5→4 in CSNEG encoding)
	code := []uint32{
		0xd10283ff, 0xa90667fa, 0xa9075ff8, 0xa90857f6, 0xa9094ff4, 0x37f805a3, 0x52800009, 0x52800008,
		0x4b0303eb, 0xaa0203ea, 0x93407d4a, 0x93407d6b, 0x1100046c, 0xaa2a03ed, 0x9b2305ad, 0x12b0000e,
		0xaa0b03ef, 0xaa0d03f0, 0xaa0b03f1, 0xd2800007, 0x52800003, 0xaa0003f3, 0xaa1003f4, 0xd2800015,
		0x38756a76, 0x38756a97, 0x6b1702d6, 0x5a9646d6, 0x0b0302c3, 0x910006b5, 0xf10042bf, 0x54ffff21,
		0x910004e7, 0x8b0a0294, 0x8b0a0273, 0xf10040ff, 0x54fffe61, 0x6b0e007f, 0x1a88b1e8, 0x1a89b229,
		0x1a8eb06e, 0x91000631, 0x91000610, 0x6b11019f, 0x54fffce1, 0x910005ef, 0x8b0a01ad, 0x6b0f019f,
		0x54fffc21, 0x14000003, 0x52800008, 0x52800009, 0xd280000b, 0x5280000a, 0x93407c4c, 0x9b22050d,
		0x8b29c1ad, 0x937e7c4e, 0x910083ef, 0x910103f0, 0x910003f1, 0xd2800001, 0xaa0003e2, 0xaa0d03e3,
		0xd2800007, 0x910083f3, 0xaa0203f4, 0xaa0303f5, 0xd2800016, 0x38766a97, 0x38766ab8, 0x4b1802f7,
		0x78367a77, 0x910006d6, 0xf10012df, 0x54ffff41, 0x910004e7, 0x91002273, 0x8b0c02b5, 0x8b0c0294,
		0xf10010ff, 0x54fffe61, 0xd2800007, 0x8b0701f3, 0x79400274, 0x79400675, 0x79400a76, 0x79400e73,
		0x0b1402b7, 0x4b150294, 0x0b160275, 0x0b1702b8, 0x8b070219, 0x79000338, 0x4b1302d3, 0x0b140276,
		0x79000736, 0x4b1502f5, 0x79000b35, 0x4b130293, 0x79000f33, 0x910020e7, 0xf10080ff, 0x54fffd81,
		0x928000e7, 0x8b070213, 0x79401274, 0x79402275, 0x79403276, 0x79404273, 0x0b1402b7, 0x4b150294,
		0x0b160275, 0x0b1702b8, 0x8b070239, 0x79001338, 0x4b1302d3, 0x0b140276, 0x79002336, 0x4b1502f5,
		0x79003335, 0x4b130293, 0x79004333, 0xb10008e7, 0x54fffda1, 0xd2800013, 0x78f36a34, 0x7100029f,
		0x5a944694, 0x0b1400e7, 0x91000a73, 0xf100827f, 0x54ffff41, 0x110004e7, 0x0b47054a, 0x91001027,
		0x91001063, 0x91001042, 0xf100303f, 0xaa0703e1, 0x54fff683, 0x91001161, 0x8b0e01ad, 0x8b0e0000,
		0xf100317f, 0xaa0103eb, 0x54fff563, 0xb900008a, 0xb90000a9, 0xb90000c8, 0xa9494ff4, 0xa94857f6,
		0xa9475ff8, 0xa94667fa, 0x910283ff, 0xd65f03c0,
	}

	codeBase := uint64(0x10000)
	curBase := uint64(0x100000)
	refBase := uint64(0x200000)
	outCostBase := uint64(0x300000) // 4 bytes for best_cost
	outMxBase := uint64(0x300010)   // 4 bytes for best_mx
	outMyBase := uint64(0x300020)   // 4 bytes for best_my
	stackBase := uint64(0x400000)
	haltPC := uint64(0xFFFF0000)

	em := NewEmulator()
	em.MaxInst = 100_000_000

	// Load program
	codeBuf := make([]byte, len(code)*4)
	for i, inst := range code {
		binary.LittleEndian.PutUint32(codeBuf[i*4:], inst)
	}
	em.LoadProgram(codeBase, codeBuf)

	// Load frame data
	em.LoadProgram(curBase, curFrame)
	em.LoadProgram(refBase, refFrame)

	// Output areas (zeroed)
	em.LoadProgram(outCostBase, make([]byte, 64))
	em.LoadProgram(outMxBase, make([]byte, 64))
	em.LoadProgram(outMyBase, make([]byte, 64))

	// Stack
	em.LoadProgram(stackBase, make([]byte, 8192))

	// Set up registers (match C function ABI)
	em.Regs.PC = codeBase
	em.Regs.WriteX(0, curBase+uint64(curOff))    // cur pointer
	em.Regs.WriteX(1, refBase+uint64(curOff))    // ref pointer (same offset)
	em.Regs.WriteX(2, uint64(frameW))            // stride (W2)
	em.Regs.WriteX(3, uint64(searchRange))       // range (W3)
	em.Regs.WriteX(4, outCostBase)               // best_cost output
	em.Regs.WriteX(5, outMxBase)                 // best_mx output
	em.Regs.WriteX(6, outMyBase)                 // best_my output
	em.Regs.WriteX(30, haltPC)                   // return address
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

	// Read results
	gotCost := int32(em.Mem.ReadU32(outCostBase))
	gotMx := int32(em.Mem.ReadU32(outMxBase))
	gotMy := int32(em.Mem.ReadU32(outMyBase))

	t.Logf("x264: %d instructions executed", em.InstCount)
	t.Logf("Results: cost=%d, mx=%d, my=%d", gotCost, gotMx, gotMy)
	t.Logf("Golden:  cost=%d, mx=%d, my=%d", goldenCost, goldenMx, goldenMy)

	if int(gotCost) != goldenCost {
		t.Errorf("best_cost: got %d, want %d", gotCost, goldenCost)
	}
	if int(gotMx) != goldenMx {
		t.Errorf("best_mx: got %d, want %d", gotMx, goldenMx)
	}
	if int(gotMy) != goldenMy {
		t.Errorf("best_my: got %d, want %d", gotMy, goldenMy)
	}
}
