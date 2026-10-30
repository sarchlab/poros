package inst

import (
	"encoding/binary"
	"golang.org/x/arch/arm64/arm64asm"
	"reflect"
	"testing"
)

func TestRegSPOperandClassTracksOrdinaryGPRs(t *testing.T) {
	for _, tc := range []struct {
		word uint32
		dst  int
		src  []int
	}{
		{0x91000422, 2, []int{1}},  // ADD X2,X1,#1
		{0xd1000c64, 4, []int{3}},  // SUB X4,X3,#3
		{0x11000422, 2, []int{1}},  // ADD W2,W1,#1
		{0xb1000422, 2, []int{1}},  // ADDS X2,X1,#1: Reg destination, RegSP source
		{0x910043ff, -1, nil},      // ADD SP,SP,#16: SP remains outside the GPR scoreboard
		{0x910043e2, 2, nil},       // ADD X2,SP,#16
		{0x9100403f, -1, []int{1}}, // ADD SP,X1,#16
	} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], tc.word)
		i, err := arm64asm.Decode(b[:])
		if err != nil {
			t.Fatal(err)
		}
		dst, fp := DestRegs(i)
		src := SourceGPRs(i)
		if dst != tc.dst || fp != -1 || !reflect.DeepEqual(src, tc.src) {
			t.Fatalf("%s: dst=%d fp=%d src=%v; want %d %v", i, dst, fp, src, tc.dst, tc.src)
		}
	}
	for _, r := range []arm64asm.Reg{arm64asm.W0, arm64asm.W30, arm64asm.X0, arm64asm.X30} {
		if ArgGPR(arm64asm.RegSP(r)) != GprIdx(r) {
			t.Fatal(r)
		}
	}
	if ArgGPR(arm64asm.RegSP(arm64asm.XZR)) != -1 || ArgGPR(arm64asm.RegSP(arm64asm.WZR)) != -1 {
		t.Fatal("SP aliases must not be tracked as ZR")
	}
}
