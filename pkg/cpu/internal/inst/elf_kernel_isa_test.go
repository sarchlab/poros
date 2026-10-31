package inst

import (
	"encoding/binary"
	"golang.org/x/arch/arm64/arm64asm"
	"reflect"
	"testing"
)

func TestELFKernelDependencies(t *testing.T) {
	for _, tc := range []struct {
		op    uint32
		dest  int
		src   []int
		flags bool
	}{
		{0x7e61d821, 1, []int{1}, false}, {0x1e612010, -1, []int{0, 1}, true},
		{0x6ee0f820, 0, []int{1}, false}, {0x4ea11c00, 0, []int{0, 1}, false},
	} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], tc.op)
		in, e := arm64asm.Decode(b[:])
		if e != nil {
			t.Fatal(e)
		}
		g, f := DestRegs(in)
		if g != -1 || f != tc.dest || !reflect.DeepEqual(SourceFPs(in), tc.src) || SetsFlags(in.Op) != tc.flags {
			t.Fatalf("%s dest %d,%d src%v flags%v", in, g, f, SourceFPs(in), SetsFlags(in.Op))
		}
	}
	if !SetsFlags(arm64asm.NEGS) {
		t.Fatal("NEGS flags dependency")
	}
}
