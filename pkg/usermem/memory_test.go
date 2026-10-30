package usermem

import (
	"bytes"
	"testing"
)

func TestAtomicCrossRegionAccessAndOwnership(t *testing.T) {
	m := &AddressSpace{}
	data := []byte{1, 2, 3, 4}
	if e := m.Map(0x100, 4, Read|Write, data); e != nil {
		t.Fatal(e)
	}
	clear(data)
	if e := m.Map(0x104, 4, Read, []byte{5, 6, 7, 8}); e != nil {
		t.Fatal(e)
	}
	if e := m.Write(0x102, []byte{9, 9, 9, 9}); e == nil {
		t.Fatal("partial write accepted")
	}
	out := make([]byte, 8)
	if e := m.Read(0x100, out); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(out, []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatal(out)
	}
	maps := m.Mappings()
	maps[0].Flags = Execute
	if e := m.Fetch(0x100, make([]byte, 4)); e == nil {
		t.Fatal("metadata alias changed permissions")
	}
	for _, a := range []uint64{0x100, 0x102} {
		if e := m.Map(a, 4, Read, nil); e == nil {
			t.Fatal("overlap allowed")
		}
	}
	if e := m.Map(^uint64(0), 2, Read, nil); e == nil {
		t.Fatal("overflow allowed")
	}
	if e := m.Read(^uint64(0), make([]byte, 2)); e == nil {
		t.Fatal("read overflow allowed")
	}
}
