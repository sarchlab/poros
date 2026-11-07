package inst

import (
	"encoding/binary"
	"reflect"
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

func TestImmediateGeneralRegisterDependencies(t *testing.T) {
	for _, tc := range []struct {
		encoding uint32
		dest     int
		sources  []int
	}{
		{0x91000442, 2, []int{2}},  // ADD X2, X2, #1
		{0x11000442, 2, []int{2}},  // ADD W2, W2, #1 (same physical GPR)
		{0xd1000442, 2, []int{2}},  // SUB X2, X2, #1
		{0x51000442, 2, []int{2}},  // SUB W2, W2, #1
		{0x9100045f, -1, []int{2}}, // ADD SP, X2, #1: actual SP remains untracked
		{0x910007e2, 2, nil},       // ADD X2, SP, #1
		{0x8b1f0042, 2, []int{2}},  // ADD X2, X2, XZR
	} {
		var code [4]byte
		binary.LittleEndian.PutUint32(code[:], tc.encoding)
		in, err := arm64asm.Decode(code[:])
		if err != nil {
			t.Fatal(err)
		}
		dest, fp := DestRegs(in)
		if dest != tc.dest || fp != -1 || !reflect.DeepEqual(SourceGPRs(in), tc.sources) {
			t.Fatalf("%s: dest=%d fp=%d sources=%v; want dest=%d sources=%v", in, dest, fp, SourceGPRs(in), tc.dest, tc.sources)
		}
	}
}
