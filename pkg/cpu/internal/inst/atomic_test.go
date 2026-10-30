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
	}{{0x88e1fc62, 1, []int{1, 2, 3}}, {0xb8e18062, 2, []int{1, 3}}, {0x88fffc62, -1, []int{2, 3}}, {0xb8e1807f, -1, []int{1, 3}}} {
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

func TestPointerStripUnaryDependency(t *testing.T) {
	for _, word := range []uint32{0xdac143f1, 0xdac147f1, 0xdac147ff} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], word)
		i, err := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if err != nil {
			t.Fatal(err)
		}
		g, f := DestRegs(i)
		want := 17
		src := []int{17}
		if word&31 == 31 {
			want = -1
			src = nil
		}
		if g != want || f != -1 || !reflect.DeepEqual(SourceGPRs(i), src) {
			t.Fatal(g, f, SourceGPRs(i))
		}
	}
}

func TestOrderedMemoryDependencies(t *testing.T) {
	for _, word := range []uint32{0xc89ffc61, 0xc8dffc61} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], word)
		i, err := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if err != nil {
			t.Fatal(err)
		}
		load := word&(1<<22) != 0
		dest, _ := DestRegs(i)
		want := -1
		src := []int{1, 3}
		if load {
			want = 1
			src = []int{3}
		}
		if dest != want || !reflect.DeepEqual(SourceGPRs(i), src) || !IsBarrier(i.Op) || IsLoad(i.Op) != load || IsStore(i.Op) == load {
			t.Fatal(i, dest, SourceGPRs(i))
		}
	}
}

func TestExclusiveDependencyAndMemoryClassification(t *testing.T) {
	for _, tc := range []struct {
		word        uint32
		dst         int
		src         []int
		load, store bool
	}{
		{0x885f7c61, 1, []int{3}, true, false},
		{0x88027c65, 2, []int{5, 3}, false, true},
		{0x881f7c65, -1, []int{5, 3}, false, true},
	} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], tc.word)
		i, err := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if err != nil {
			t.Fatal(err)
		}
		g, f := DestRegs(i)
		if g != tc.dst || f != -1 || !reflect.DeepEqual(SourceGPRs(i), tc.src) || IsLoad(i.Op) != tc.load || IsStore(i.Op) != tc.store || !IsBarrier(i.Op) {
			t.Fatal(i, g, f, SourceGPRs(i))
		}
	}
}

func TestPartialDestinationDependencies(t *testing.T) {
	for _, tc := range []struct {
		word uint32
		src  []int
	}{{0xf2b579a1, []int{1}}, {0xb3703c41, []int{1, 2}}, {0xb340f041, []int{1, 2}}, {0xb340fc41, []int{2}}, {0x33007c41, []int{2}}} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], tc.word)
		i, err := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(SourceGPRs(i), tc.src) {
			t.Fatal(i, SourceGPRs(i))
		}
	}
}
func TestNonTemporalPairDependencies(t *testing.T) {
	// LDNP Q2,Q3,[X1] and STNP Q2,Q3,[X0]. Both vector lanes are tracked.
	for _, word := range []uint32{0xac400c22, 0xac000c02} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], word)
		i, err := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if err != nil {
			t.Fatal(err)
		}
		if IsLoad(i.Op) {
			_, a := DestRegs(i)
			_, b := DestRegs2(i)
			if a != 2 || b != 3 || !reflect.DeepEqual(SourceGPRs(i), []int{1}) {
				t.Fatal(i, a, b, SourceGPRs(i))
			}
		} else if !IsStore(i.Op) || !reflect.DeepEqual(SourceFPs(i), []int{2, 3}) || !reflect.DeepEqual(SourceGPRs(i), []int{0}) {
			t.Fatal(i, SourceFPs(i), SourceGPRs(i))
		}
	}
}

func TestLDADDDependenciesAndClassification(t *testing.T) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], 0xf8210062)
	i, err := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
	if err != nil {
		t.Fatal(err)
	}
	g, f := DestRegs(i)
	if g != 2 || f != -1 || !reflect.DeepEqual(SourceGPRs(i), []int{1, 3}) || !IsLoad(i.Op) || !IsStore(i.Op) || !IsBarrier(i.Op) {
		t.Fatal(i, g, f, SourceGPRs(i))
	}
}

func TestBSLReadsDestinationMask(t *testing.T) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], 0x6e621c20)
	decoded, e := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(SourceFPs(decoded), []int{0, 1, 2}) {
		t.Fatal("missing BSL mask/source", SourceFPs(decoded))
	}
	_, dest := DestRegs(decoded)
	if dest != 0 {
		t.Fatal(dest)
	}
}

func TestRCpcLoadClassification(t *testing.T) {
	for _, word := range []uint32{0xb8bfc128, 0xf8bfc128, 0xf8bfc3e8, 0xf8bfc13f} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], word)
		i, e := emu.DecodeChecked(b[:], emu.PointerAuthUnspecified)
		if e != nil {
			t.Fatal(e)
		}
		if !IsLoad(i.Op) || IsStore(i.Op) || !IsBarrier(i.Op) || BaseWritebackGPR(i) != -1 {
			t.Fatal("lost RCpc memory/order identity", i)
		}
		dest, fp := DestRegs(i)
		want := int(word & 31)
		if want == 31 {
			want = -1
		}
		if dest != want || fp != -1 {
			t.Fatal("RCpc destination", dest, fp)
		}
		src := SourceGPRs(i)
		base := int((word >> 5) & 31)
		if base == 31 {
			if len(src) != 0 {
				t.Fatal(src)
			}
		} else if !reflect.DeepEqual(src, []int{base}) {
			t.Fatal(src)
		}
	}
}
