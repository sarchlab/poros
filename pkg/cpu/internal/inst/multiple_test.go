package inst

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"reflect"
	"testing"
)

func TestLD1AllDestinationsAndPostRegisterDependencies(t *testing.T) {
	for _, tc := range []struct {
		word      uint32
		dest, src []int
		base      int
	}{
		{0x4c40207f, []int{31, 0, 1, 2}, []int{3}, -1},
		{0x4ccb6863, []int{3, 4, 5}, []int{3, 11}, 3},
		{0x4cdfa01f, []int{31, 0}, []int{0}, 0},
		{0xad400460, []int{0, 1}, []int{3}, -1},
	} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], tc.word)
		i, e := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if e != nil {
			t.Fatal(e)
		}
		dest, n := DestFPs(i)
		if !IsLoad(i.Op) || !reflect.DeepEqual(dest[:n], tc.dest) || !reflect.DeepEqual(SourceGPRs(i), tc.src) || len(SourceFPs(i)) != 0 || BaseWritebackGPR(i) != tc.base {
			t.Fatal(i, dest, n, SourceGPRs(i), SourceFPs(i), BaseWritebackGPR(i))
		}
	}
}
func TestCryptoSIMDReadModifyWriteAndTableSources(t *testing.T) {
	for _, tc := range []struct {
		word    uint32
		fp, gpr []int
		dest    int
	}{
		{0x4e284827, []int{7, 1}, nil, 7},
		{0x4e0363e0, []int{31, 0, 1, 2, 3}, nil, 0},
		{0x4e041c67, []int{7}, []int{3}, 7},
		{0x5e010461, []int{3}, nil, 1},
		{0x6e016002, []int{0, 1}, nil, 2},
	} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], tc.word)
		i, e := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if e != nil {
			t.Fatal(e)
		}
		regs, n := DestFPs(i)
		gpr, _ := DestRegs(i)
		if n != 1 || regs[0] != tc.dest || gpr != -1 || !reflect.DeepEqual(SourceFPs(i), tc.fp) || !reflect.DeepEqual(SourceGPRs(i), tc.gpr) || SetsFlags(i.Op) || UsesFlags(i.Op) {
			t.Fatal(i, regs, n, SourceFPs(i), SourceGPRs(i))
		}
	}
}

func TestST1AllSourcesAndNoFPDestinations(t *testing.T) {
	var bytes [4]byte
	binary.LittleEndian.PutUint32(bytes[:], 0x4c8b20bf)
	i, e := emu.DecodeChecked(bytes[:], emu.PointerAuthUnspecified)
	if e != nil {
		t.Fatal(e)
	}
	_, count := DestFPs(i)
	gpr, fp := DestRegs(i)
	if !IsStore(i.Op) || IsLoad(i.Op) || gpr != -1 || fp != -1 || count != 0 || !reflect.DeepEqual(SourceFPs(i), []int{31, 0, 1, 2}) || !reflect.DeepEqual(SourceGPRs(i), []int{5, 11}) || PostIndexGPR(i) != 11 || BaseWritebackGPR(i) != 5 {
		t.Fatal(i, SourceFPs(i), SourceGPRs(i), count)
	}
}
