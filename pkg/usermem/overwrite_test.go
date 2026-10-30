package usermem

import (
	"bytes"
	"reflect"
	"testing"
)

func TestOverwriteReaderPrevalidationSplitsAndRevokes(t *testing.T) {
	m := &AddressSpace{}
	old := bytes.Repeat([]byte{5}, 64)
	if err := m.MapReader(0x1000, 64, Read|Write, Read|Write, bytes.NewReader(old), 16); err != nil {
		t.Fatal(err)
	}
	if err := m.Write(0x1000, bytes.Repeat([]byte{9}, 64)); err != nil {
		t.Fatal(err)
	}
	if err := m.Unmap(0x1020, 16); err != nil {
		t.Fatal(err)
	}
	handle, err := m.MapKernelReadOnly(0x1040, []byte{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	before := m.Mappings()
	for _, pageSize := range []uint64{0, 3} {
		if m.OverwriteReader(0x1010, 52, Read, Read, bytes.NewReader(make([]byte, 52)), pageSize) == nil {
			t.Fatal("invalid page size")
		}
		if !reflect.DeepEqual(before, m.Mappings()) || m.lazyBytes != 48 {
			t.Fatal("prevalidation mutated mappings/residency")
		}
	}
	if err := m.OverwriteReader(0x1010, 52, Read, Read, bytes.NewReader(make([]byte, 52)), 16); err != nil {
		t.Fatal(err)
	}
	if m.lazyBytes != 16 {
		t.Fatal("obsolete private pages retained", m.lazyBytes)
	}
	data := make([]byte, 68)
	if err := m.Read(0x1000, data); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data[:16], bytes.Repeat([]byte{9}, 16)) || !bytes.Equal(data[16:], make([]byte, 52)) || !bytes.Equal(old, bytes.Repeat([]byte{5}, 64)) {
		t.Fatal("replacement/private source isolation", data)
	}
	if handle.Replace([]byte{7, 7, 7, 7}) == nil {
		t.Fatal("overwritten kernel handle remained valid")
	}
	if m.Write(0x1010, []byte{1}) == nil {
		t.Fatal("replacement permissions")
	}
}
