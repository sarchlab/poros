package usermem

import (
	"bytes"
	"testing"
)

func TestKernelReadOnlyRegionOwnershipPermissionsAndRevocation(t *testing.T) {
	m := &AddressSpace{}
	initial := []byte{1, 2, 3, 4}
	r, e := m.MapKernelReadOnly(0x1000, initial)
	if e != nil {
		t.Fatal(e)
	}
	clear(initial)
	out := make([]byte, 4)
	if e = m.Read(0x1000, out); e != nil || !bytes.Equal(out, []byte{1, 2, 3, 4}) {
		t.Fatal(out, e)
	}
	if m.Write(0x1000, []byte{9}) == nil || m.Fetch(0x1000, []byte{0}) == nil || m.Protect(0x1000, 4, Read|Write, false) == nil {
		t.Fatal("guest permission escaped")
	}
	next := []byte{4, 3, 2, 1}
	if e = r.Replace(next); e != nil {
		t.Fatal(e)
	}
	clear(next)
	if e = m.Read(0x1000, out); e != nil || !bytes.Equal(out, []byte{4, 3, 2, 1}) {
		t.Fatal(out, e)
	}
	if r.Replace([]byte{8}) == nil {
		t.Fatal("partial replacement")
	}
	if e = m.Protect(0x1001, 2, 0, false); e != nil {
		t.Fatal(e)
	}
	if e = r.Replace([]byte{9, 8, 7, 6}); e != nil {
		t.Fatal("protection split lost owner", e)
	}
	if e = m.Protect(0x1001, 2, Read, false); e != nil {
		t.Fatal(e)
	}
	if e = m.Read(0x1000, out); e != nil || !bytes.Equal(out, []byte{9, 8, 7, 6}) {
		t.Fatal(out, e)
	}
	if e = m.Unmap(0x1001, 2); e != nil {
		t.Fatal(e)
	}
	if r.Replace([]byte{1, 1, 1, 1}) == nil {
		t.Fatal("hole did not revoke")
	}
	if e = m.Map(0x1001, 2, Read, []byte{5, 5}); e != nil {
		t.Fatal(e)
	}
	if r.Replace([]byte{1, 1, 1, 1}) == nil {
		t.Fatal("replacement did not revoke")
	}
	if e = m.Read(0x1000, out); e != nil || !bytes.Equal(out, []byte{9, 5, 5, 6}) {
		t.Fatal("failed update modified partial region", out, e)
	}
	var empty ReadOnlyRegion
	if empty.Replace(nil) == nil {
		t.Fatal("empty handle")
	}
	if _, e = m.MapKernelReadOnly(0, nil); e == nil {
		t.Fatal("zero mapping")
	}
}
