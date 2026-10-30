package emu

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/usermem"
	"testing"
)

func TestOrderedMemoryNativeAlignmentAndValues(t *testing.T) {
	var native struct {
		Cases []struct {
			Width, Offset, Exit, Signal int
			Load, SP                    bool
		}
	}
	nativeAtomicObservation(t, "acquire-release-probe.c", &native)
	if len(native.Cases) != 192 {
		t.Fatal("missing native cases")
	}
	for _, tc := range native.Cases {
		word := uint32(0x889ffc61)
		if tc.Width == 8 {
			word |= 1 << 30
		}
		if tc.Load {
			word |= 1 << 22
		}
		if tc.SP {
			word = (word &^ uint32(31<<5)) | (31 << 5)
		}
		m := atomicSpace(t, word, usermem.Read|usermem.Write)
		addr := uint64(0x2000 + tc.Offset)
		var initial [8]byte
		binary.LittleEndian.PutUint64(initial[:], 9)
		m.Space.Write(addr, initial[:tc.Width])
		r := Registers{PC: 0x1000, SPAlignmentCheck: true, AtomicAlignment: AtomicWithin16Bytes, SP: addr}
		r.GPR[3] = addr
		r.GPR[1] = 15
		before := r
		_, err := StepChecked(&r, m)
		if tc.Signal != 0 {
			if err == nil || r != before || len(m.Accesses) != 0 {
				t.Fatal("native fault mismatch", tc, err)
			}
			continue
		}
		if tc.Exit != 0 || err != nil {
			t.Fatal(tc, err)
		}
		var after [8]byte
		m.Space.Read(addr, after[:tc.Width])
		want := uint64(15)
		if tc.Load {
			want = 9
			if r.GPR[1] != 9 {
				t.Fatal("load value", r.GPR[1])
			}
		}
		total := 0
		for _, access := range m.Accesses {
			total += access.Size
			if access.IsWrite == tc.Load {
				t.Fatal("wrong memory direction")
			}
		}
		if binary.LittleEndian.Uint64(after[:]) != want || total != tc.Width {
			t.Fatal(tc, after, m.Accesses)
		}
	}
}
func TestOrderedMemoryPermissionsFaultAtomically(t *testing.T) {
	for _, word := range []uint32{0x889ffc61, 0x88dffc61, 0xc89ffc61, 0xc8dffc61} {
		load := word&(1<<22) != 0
		flags := usermem.Read
		if load {
			flags = usermem.Write
		}
		m := atomicSpace(t, word, flags)
		r := Registers{PC: 0x1000}
		r.GPR[3] = 0x2000
		r.GPR[1] = 99
		before := r
		if _, err := StepChecked(&r, m); err == nil || r != before || len(m.Accesses) != 0 {
			t.Fatal("permission failure changed state", err)
		}
	}
}
