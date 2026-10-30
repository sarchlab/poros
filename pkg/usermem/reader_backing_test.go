package usermem

import (
	"bytes"
	"testing"
)

func TestReaderBackingIdentityAndReplacement(t *testing.T) {
	m := &AddressSpace{}
	source := bytes.NewReader(bytes.Repeat([]byte{7}, 0x4000))
	if e := m.MapReader(0x4000, 0x4000, Read|Write, Read|Write, source, 0x1000); e != nil {
		t.Fatal(e)
	}
	m.Protect(0x5000, 0x1000, Read, false)
	got, offset, ok := m.ReaderBacking(0x4800, 0x2800)
	if !ok || got != source || offset != 0x800 {
		t.Fatal("protection split lost source", offset, ok)
	}
	m.Write(0x4000, []byte{99})
	b := make([]byte, 1)
	source.ReadAt(b, 0)
	if b[0] != 7 {
		t.Fatal("write changed source")
	}
	replacement := bytes.NewReader(bytes.Repeat([]byte{8}, 0x4000))
	if e := m.ReplaceReader(0x4000, 0x4000, Read, Read, replacement, 3); e == nil {
		t.Fatal("invalid page accepted")
	}
	m.Read(0x4000, b)
	if b[0] != 99 {
		t.Fatal("failed replacement changed state")
	}
	if e := m.ReplaceReader(0x4000, 0x4000, Read, Read, replacement, 0x1000); e != nil {
		t.Fatal(e)
	}
	m.Read(0x4000, b)
	if b[0] != 8 || m.Protect(0x4000, 0x4000, Write, false) == nil {
		t.Fatal("replacement retained old bytes/maximum")
	}
	m.Unmap(0x5000, 0x1000)
	if _, _, ok := m.ReaderBacking(0x4000, 0x3000); ok {
		t.Fatal("hole retains source")
	}
	m.MapReader(0x5000, 0x1000, Read, Read, replacement, 0x1000)
	if _, _, ok := m.ReaderBacking(0x4000, 0x3000); ok {
		t.Fatal("independent remap merged backing")
	}
	for _, r := range [][2]uint64{{0, 1}, {0x4000, 0}, {^uint64(0), 2}, {0x7000, 0x2000}} {
		if _, _, ok := m.ReaderBacking(r[0], r[1]); ok {
			t.Fatal("bad range", r)
		}
	}
}
