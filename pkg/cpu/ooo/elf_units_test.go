package ooo

import (
	"encoding/binary"
	"golang.org/x/arch/arm64/arm64asm"
	"testing"
)

func TestELFVectorExecutionUnits(t *testing.T) {
	for _, tc := range []struct {
		encoding uint32
		unit     unitType
	}{
		{0x4ea11c00, unitFP},   // ORR V0.16B, V0.16B, V1.16B
		{0x4f000400, unitFP},   // MOVI V0.4S, #0
		{0x6ee0f820, unitFP},   // FNEG V0.2D, V1.2D
		{0xaa010000, unitINT},  // ORR X0, X0, X1
		{0xf9400020, unitLDST}, // LDR X0, [X1]
	} {
		var bytes [4]byte
		binary.LittleEndian.PutUint32(bytes[:], tc.encoding)
		instruction, err := arm64asm.Decode(bytes[:])
		if err != nil {
			t.Fatal(err)
		}
		if got := classifyUnit(instruction); got != tc.unit {
			t.Fatalf("%s classified as %v, want %v", instruction, got, tc.unit)
		}
	}
}
