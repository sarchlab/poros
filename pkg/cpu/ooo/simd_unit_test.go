package ooo

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"testing"
)

func TestSIMDFormsUseFPResourcesAndLoadsStayLDST(t *testing.T) {
	for _, tc := range []struct {
		word uint32
		unit unitType
	}{
		{0x4ee18422, unitFP}, {0x4e286800, unitFP}, {0x4e284827, unitFP}, {0x6e016002, unitFP}, {0x4e0363e0, unitFP}, {0x5e010461, unitFP}, {0x4e041c67, unitFP},
		{0x4c40207f, unitLDST}, {0x3dc00067, unitLDST}, {0xad400460, unitLDST},
		{0xaa0103e0, unitINT},
	} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], tc.word)
		i, e := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if e != nil {
			t.Fatal(e)
		}
		if got := classifyUnit(i); got != tc.unit {
			t.Fatal(i, got, tc.unit)
		}
	}
}
