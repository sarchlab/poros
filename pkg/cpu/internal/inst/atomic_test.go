package inst

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"reflect"
	"testing"
)

func TestAtomicDependenciesAndMemoryClass(t *testing.T) {
	for _, tc := range []struct {
		word uint32
		dest int
		src  []int
	}{{0x88e1fc62, 1, []int{1, 2, 3}}, {0xb8e18062, 2, []int{1, 3}}} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], tc.word)
		i, err := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if err != nil {
			t.Fatal(err)
		}
		g, f := DestRegs(i)
		if g != tc.dest || f != -1 || !reflect.DeepEqual(SourceGPRs(i), tc.src) || !IsLoad(i.Op) || !IsStore(i.Op) || !IsBarrier(i.Op) || BaseWritebackGPR(i) != -1 {
			t.Fatal(i, g, f, SourceGPRs(i))
		}
	}
}
func TestByteHalfwordSIMDDependencyChain(t *testing.T) {
	// CNT -> UADDLV H0 -> UMINV B1 -> ORR V2. Every scalar subregister
	// aliases its complete vector register in both timing scoreboards.
	words := []uint32{0x0e205800, 0x2e303800, 0x6e31a801, 0x4ea11c22}
	var previous int
	for n, w := range words {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], w)
		i, err := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if err != nil {
			t.Fatal(err)
		}
		_, dest := DestRegs(i)
		if dest < 0 {
			t.Fatal("lost B/H destination", i)
		}
		if n > 0 {
			found := false
			for _, source := range SourceFPs(i) {
				found = found || source == previous
			}
			if !found {
				t.Fatal("broken dependency", i, previous, SourceFPs(i))
			}
		}
		previous = dest
	}
}
