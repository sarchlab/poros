package inst

import (
	"encoding/binary"
	"slices"
	"testing"

	"golang.org/x/arch/arm64/arm64asm"
)

func decodeWord(t *testing.T, word uint32) arm64asm.Inst {
	t.Helper()
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], word)
	inst, err := arm64asm.Decode(b[:])
	if err != nil {
		t.Fatalf("decode %08x: %v", word, err)
	}
	return inst
}

// Compare instructions have no destination; both operands are sources and the
// conditional-select aliases consume the flags they produce.
func TestCompareOperandsAndConditionalAliasesAreDependencies(t *testing.T) {
	for _, tc := range []struct {
		word       uint32
		sources    []int
		setsFlags  bool
		usesFlags  bool
		destGPR    int
		descriptor string
	}{
		{0x6b03001f, []int{0, 3}, true, false, -1, "CMP W0, W3"},
		{0x71000c1f, []int{0}, true, false, -1, "CMP W0, #3"},
		{0x2b03001f, []int{0, 3}, true, false, -1, "CMN W0, W3"},
		{0x6a03001f, []int{0, 3}, true, false, -1, "TST W0, W3"},
		{0x7a43b004, []int{0, 3}, true, true, -1, "CCMP W0, W3, #4, LT"},
		{0x3a43b004, []int{0, 3}, true, true, -1, "CCMN W0, W3, #4, LT"},
		{0x1a801400, []int{0}, false, true, 0, "CINC W0, W0, EQ"},
		{0x1a9f17e0, nil, false, true, 0, "CSET W0, EQ"},
		{0x5a9f13e0, nil, false, true, 0, "CSETM W0, EQ"},
		{0x5a801400, []int{0}, false, true, 0, "CNEG W0, W0, EQ"},
		{0x1a83a000, []int{0, 3}, false, true, 0, "CSEL W0, W0, W3, GE"},
		{0x3a030000, []int{0, 3}, true, true, 0, "ADCS W0, W0, W3"},
		{0x1a030000, []int{0, 3}, false, true, 0, "ADC W0, W0, W3"},
		{0x71000400, []int{0}, true, false, 0, "SUBS W0, W0, #1"},
	} {
		inst := decodeWord(t, tc.word)
		if got := SourceGPRs(inst); !slices.Equal(got, tc.sources) {
			t.Errorf("%s (%v): sources %v, want %v", tc.descriptor, inst, got, tc.sources)
		}
		if SetsFlags(inst.Op) != tc.setsFlags || UsesFlags(inst.Op) != tc.usesFlags {
			t.Errorf("%s (%v): sets=%v uses=%v, want sets=%v uses=%v", tc.descriptor, inst, SetsFlags(inst.Op), UsesFlags(inst.Op), tc.setsFlags, tc.usesFlags)
		}
		if gpr, fp := DestRegs(inst); gpr != tc.destGPR || fp != -1 {
			t.Errorf("%s (%v): dest gpr=%d fp=%d, want gpr=%d", tc.descriptor, inst, gpr, fp, tc.destGPR)
		}
	}
}
