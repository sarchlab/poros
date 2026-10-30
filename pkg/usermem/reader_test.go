package usermem

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"
)

type faultyReader struct {
	data   []byte
	failAt int64
	reads  int
}

func (f *faultyReader) ReadAt(b []byte, o int64) (int, error) {
	f.reads++
	if o >= f.failAt {
		return 0, io.ErrUnexpectedEOF
	}
	return bytes.NewReader(f.data).ReadAt(b, o)
}
func TestLazyWritesAtomicOnIOFailureAndPrivate(t *testing.T) {
	source := &faultyReader{data: bytes.Repeat([]byte{0x55}, 32), failAt: 16}
	m := &AddressSpace{}
	if err := m.MapReader(0x1000, 32, Read|Write, Read|Write, source, 16); err != nil {
		t.Fatal(err)
	}
	if source.reads != 0 {
		t.Fatal("mapping eagerly read source")
	}
	if err := m.Write(0x1008, bytes.Repeat([]byte{9}, 16)); err == nil {
		t.Fatal("failed backing accepted")
	}
	out := make([]byte, 16)
	if err := m.Read(0x1000, out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, source.data[:16]) {
		t.Fatal("partial write")
	}
	source.failAt = 32
	if err := m.Write(0x1008, bytes.Repeat([]byte{9}, 16)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(source.data, bytes.Repeat([]byte{0x55}, 32)) {
		t.Fatal("host source changed")
	}
	all := make([]byte, 32)
	if err := m.Read(0x1000, all); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(all[8:24], bytes.Repeat([]byte{9}, 16)) {
		t.Fatal(all)
	}
}
func TestLazyLimitAndProtectionSplit(t *testing.T) {
	m := &AddressSpace{MaxLazyBytes: 16}
	source := &faultyReader{data: bytes.Repeat([]byte{3}, 48), failAt: 48}
	if err := m.MapReader(0x1000, 48, Read|Write, Read|Write, source, 16); err != nil {
		t.Fatal(err)
	}
	if err := m.Write(0x1008, make([]byte, 16)); err == nil {
		t.Fatal("resident limit ignored")
	}
	if err := m.Protect(0x1010, 16, Read, false); err != nil {
		t.Fatal(err)
	}
	before := source.reads
	if err := m.Write(0x100f, []byte{1, 2}); err == nil || source.reads != before {
		t.Fatal("permissions checked after IO")
	}
	m.MaxLazyBytes = 48
	if err := m.Read(0x1010, make([]byte, 16)); err != nil {
		t.Fatal(err)
	}
	if err := m.Protect(0x1010, 16, Read, true); err != nil {
		t.Fatal(err)
	}
	if err := m.Protect(0x1000, 48, Read|Write, false); !errors.Is(err, ErrProtection) {
		t.Fatal(err)
	}
	if m.Mappings()[0].Flags != Read|Write || m.Mappings()[1].Flags != Read {
		t.Fatal("partial protect")
	}
	if err := m.Write(0x1020, []byte{7}); err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	if err := m.Read(0x1020, b[:]); err != nil || b[0] != 7 {
		t.Fatal("split view offset wrong")
	}
	if err := m.Read(0x1000, b[:]); err != nil || b[0] != 3 {
		t.Fatal("split backing alias wrong")
	}
	if err := m.Protect(0x1020, 32, 0, false); err == nil {
		t.Fatal("unmapped protect accepted")
	}
}
func TestProtectionEagerSplitsPreserveData(t *testing.T) {
	m := &AddressSpace{}
	data := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	if err := m.MapProtected(0x100, 8, Read, Read|Write, data); err != nil {
		t.Fatal(err)
	}
	if err := m.Protect(0x102, 3, Read|Write, false); err != nil {
		t.Fatal(err)
	}
	if err := m.Write(0x102, []byte{8, 9, 10}); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 8)
	if err := m.Read(0x100, got); err != nil || !bytes.Equal(got, []byte{0, 1, 8, 9, 10, 5, 6, 7}) {
		t.Fatal(got, err)
	}
	if err := m.Protect(0x102, 3, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := m.Read(0x102, make([]byte, 1)); err == nil {
		t.Fatal("no-access map readable")
	}
}

type exactEOFReader struct{}

func (exactEOFReader) ReadAt(b []byte, offset int64) (int, error) {
	for i := range b {
		b[i] = 5
	}
	return len(b), io.EOF
}
func TestReaderAtFullEOFIsValid(t *testing.T) {
	m := &AddressSpace{}
	if err := m.MapReader(0x1000, 16, Read, Read, exactEOFReader{}, 16); err != nil {
		t.Fatal(err)
	}
	var b [16]byte
	if err := m.Read(0x1000, b[:]); err != nil || b[15] != 5 {
		t.Fatal(err, b)
	}
}

func TestUnmapSplitsAndReclaimsPrivatePages(t *testing.T) {
	m := &AddressSpace{MaxLazyBytes: 8}
	if err := m.MapReader(0x1000, 12, Read|Write, Read|Write, bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}), 4); err != nil {
		t.Fatal(err)
	}
	var b [8]byte
	if err := m.Read(0x1000, b[:]); err != nil {
		t.Fatal(err)
	}
	if m.lazyBytes != 8 {
		t.Fatal(m.lazyBytes)
	}
	if err := m.Unmap(0x1002, 4); err != nil {
		t.Fatal(err)
	}
	// Each partial surviving page is retained until its last mapped byte is gone.
	if m.lazyBytes != 8 {
		t.Fatal(m.lazyBytes)
	}
	if err := m.Read(0x1002, b[:1]); err == nil {
		t.Fatal("unmapped bytes readable")
	}
	if err := m.Unmap(0x1000, 6); err != nil {
		t.Fatal(err)
	}
	if m.lazyBytes != 4 {
		t.Fatal(m.lazyBytes)
	}
	if err := m.Read(0x1008, b[:4]); err != nil {
		t.Fatal("resident budget not reclaimed", err)
	}
	if !bytes.Equal(b[:4], []byte{9, 10, 11, 12}) {
		t.Fatal("split backing offset", b)
	}
	if err := m.Unmap(0x1000, 0x100); err != nil || m.lazyBytes != 0 || len(m.Mappings()) != 0 {
		t.Fatal(err, m.lazyBytes)
	}
	if err := m.Map(0x1000, 12, Read|Write, nil); err != nil {
		t.Fatal(err)
	}
	m.Read(0x1000, b[:])
	if b != [8]byte{} {
		t.Fatal("reused VA retained old bytes", b)
	}
	before := m.Mappings()
	if err := m.Unmap(^uint64(0), 2); err == nil || !reflect.DeepEqual(before, m.Mappings()) {
		t.Fatal("overflow changed mappings")
	}
}

func TestCheckProtectionDoesNotReadOrChangePrivateMappings(t *testing.T) {
	source := &faultyReader{data: make([]byte, 32), failAt: 0}
	m := &AddressSpace{}
	m.MapReader(0x1000, 32, Read, Read|Write, source, 16)
	before := m.Mappings()
	if m.CheckProtection(0x1008, 16, Write) != nil {
		t.Fatal("available maximum rejected")
	}
	if !errors.Is(m.CheckProtection(0x1008, 16, Execute), ErrProtection) {
		t.Fatal("maximum ignored")
	}
	if m.CheckProtection(0x1018, 16, Read) == nil {
		t.Fatal("hole ignored")
	}
	if source.reads != 0 || m.lazyBytes != 0 || !reflect.DeepEqual(before, m.Mappings()) {
		t.Fatal("read-only protection check changed backing/mappings")
	}
	if m.Check(0x1000, 1, Write, "test") == nil {
		t.Fatal("current protection changed")
	}
}
