package loader

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/usermem"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cacheFixture(t *testing.T, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cache")
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestSharedCacheRebasesByteStartsAndPreservesPrivatePages(t *testing.T) {
	original := machotest.CacheImage()
	path := cacheFixture(t, original)
	c, err := OpenSharedCache(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	hash := sha256.Sum256(original)
	if len(c.Files) != 1 || c.Files[0].SHA256 != hex.EncodeToString(hash[:]) {
		t.Fatal("identity")
	}
	mappings := c.Mappings()
	if len(mappings) != 2 {
		t.Fatal(len(mappings))
	}
	a, b := &usermem.AddressSpace{}, &usermem.AddressSpace{}
	for _, m := range mappings {
		for _, space := range []*usermem.AddressSpace{a, b} {
			if err := space.MapReader(m.Address, m.Size, usermem.Read|usermem.Write, usermem.Read|usermem.Write, m, 16384); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, space := range []*usermem.AddressSpace{a, b} {
		var got [16]byte
		if err := space.Read(c.Base+0x4010, got[:]); err != nil {
			t.Fatal(err)
		}
		if binary.LittleEndian.Uint64(got[:8]) != c.Base+0x100 || binary.LittleEndian.Uint64(got[8:]) != c.Base+0x200 {
			t.Fatalf("wrong byte-start/next/auth rebase: %x", got)
		}
	}
	if err := a.Write(c.Base+0x4010, []byte{0xff}); err != nil {
		t.Fatal(err)
	}
	var got [1]byte
	b.Read(c.Base+0x4010, got[:])
	if got[0] == 0xff {
		t.Fatal("private pages shared")
	}
	onDisk, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(onDisk, original) {
		t.Fatal("source modified")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := mappings[1].ReadAt(got[:], 0); err == nil {
		t.Fatal("closed source accepted")
	}
	if err := c.Close(); err != nil {
		t.Fatal("close not idempotent")
	}
}
func TestSharedCacheRejectsMalformedBounds(t *testing.T) {
	u := binary.LittleEndian
	for name, mutate := range map[string]func([]byte){
		"header-base":     func(b []byte) { u.PutUint64(b[0x400:], 0x180008000) },
		"header-offset":   func(b []byte) { u.PutUint64(b[0x410:], 0x8000) },
		"header":          func(b []byte) { copy(b, "bad") },
		"old-header":      func(b []byte) { u.PutUint32(b[16:], 100) },
		"file-overrun":    func(b []byte) { u.PutUint64(b[0x410:], 0xc000) },
		"overlap":         func(b []byte) { u.PutUint64(b[0x438:], 0x180000000) },
		"protection":      func(b []byte) { u.PutUint32(b[0x434:], 7) },
		"slide-version":   func(b []byte) { u.PutUint32(b[0x8000:], 3) },
		"slide-count":     func(b []byte) { u.PutUint32(b[0x8008:], 0xffffffff) },
		"start-outside":   func(b []byte) { u.PutUint16(b[0x8018:], 16384) },
		"start-unaligned": func(b []byte) { u.PutUint16(b[0x8018:], 1) },
		"mapping-count":   func(b []byte) { u.PutUint32(b[316:], 0xffffffff) },
		"subcache-count":  func(b []byte) { u.PutUint32(b[396:], 0xffffffff) },
		"subcache-oob":    func(b []byte) { u.PutUint32(b[392:], 0xfffffff0); u.PutUint32(b[396:], 1) },
	} {
		t.Run(name, func(t *testing.T) {
			b := machotest.CacheImage()
			mutate(b)
			if c, err := OpenSharedCache(cacheFixture(t, b)); err == nil {
				c.Close()
				t.Fatal("accepted malformed cache")
			}
		})
	}
}
func TestSharedCacheLazyChainFailures(t *testing.T) {
	for _, raw := range []uint64{1 << 42, 0x7ff << 52} {
		b := machotest.CacheImage()
		binary.LittleEndian.PutUint64(b[0x4010:], raw)
		c, err := OpenSharedCache(cacheFixture(t, b))
		if err != nil {
			t.Fatal(err)
		}
		var out [8]byte
		if _, err := c.Mappings()[1].ReadAt(out[:], 16); err == nil {
			t.Fatal("invalid chain accepted")
		}
		c.Close()
	}
	var zero CacheMapping
	if _, err := zero.ReadAt(make([]byte, 8), 0); err == nil {
		t.Fatal("zero descriptor accepted")
	}
}
func TestSharedCacheSubcacheMetadata(t *testing.T) {
	u := binary.LittleEndian
	root := machotest.CacheImage()
	child := machotest.CacheImage()
	u.PutUint64(root[232:], 0x18000)
	u.PutUint32(root[392:], 0x9000)
	u.PutUint32(root[396:], 1)
	entry := root[0x9000:]
	copy(entry, child[88:104])
	u.PutUint64(entry[16:], 0xc000)
	copy(entry[24:], ".01")
	for _, at := range []int{0x400, 0x438} {
		u.PutUint64(child[at:], u.Uint64(child[at:])+0xc000)
	}
	path := cacheFixture(t, root)
	if err := os.WriteFile(path+".01", child, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := OpenSharedCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Files) != 2 || len(c.Mappings()) != 4 {
		t.Fatal("missing child")
	}
	c.Close()
	for _, suffix := range []string{"../secret", ".x/secret", ".x\\secret", "..01", ".01.."} {
		copy(root[0x9018:0x9038], make([]byte, 32))
		copy(root[0x9018:], suffix)
		os.WriteFile(path, root, 0600)
		if c, err := OpenSharedCache(path); err == nil {
			c.Close()
			t.Fatal("unsafe suffix", suffix)
		}
	}
	copy(root[0x9018:0x9038], make([]byte, 32))
	copy(root[0x9018:], ".01")
	os.WriteFile(path, root, 0600)
	child[88] ^= 1
	os.WriteFile(path+".01", child, 0600)
	if c, err := OpenSharedCache(path); err == nil {
		c.Close()
		t.Fatal("UUID mismatch")
	}
}
func TestSharedCacheChangedInputFails(t *testing.T) {
	path := cacheFixture(t, machotest.CacheImage())
	c, err := OpenSharedCache(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte{0})
	f.Close()
	if _, err := c.Mappings()[0].ReadAt(make([]byte, 8), 0); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatal(err)
	}
}
func FuzzSlide5PageBounds(f *testing.F) {
	f.Add(uint16(16), uint64(0x100|1<<52), uint64(0x200|1<<63))
	f.Fuzz(func(t *testing.T, start uint16, first, second uint64) {
		data := make([]byte, 16384)
		binary.LittleEndian.PutUint64(data[16:], first)
		binary.LittleEndian.PutUint64(data[24:], second)
		m := CacheMapping{slide: &cacheSlide5{valueAdd: 0x180000000, starts: []uint16{start}}}
		_ = m.rebasePage(0, data)
	})
}

var _ io.ReaderAt = CacheMapping{}

func TestSharedCachePartialOpenFailureClosesFiles(t *testing.T) {
	// /dev/fd exists on supported Darwin/Linux hosts. Repeated missing-child
	// failures must not retain the successfully opened root descriptor.
	if _, err := os.ReadDir("/dev/fd"); err != nil {
		t.Skip("no descriptor inventory")
	}
	root := machotest.CacheImage()
	binary.LittleEndian.PutUint32(root[392:], 0x9000)
	binary.LittleEndian.PutUint32(root[396:], 1)
	copy(root[0x9018:], ".missing")
	path := cacheFixture(t, root)
	before, _ := os.ReadDir("/dev/fd")
	for i := 0; i < 20; i++ {
		if c, err := OpenSharedCache(path); err == nil {
			c.Close()
			t.Fatal("missing subcache accepted")
		}
	}
	after, _ := os.ReadDir("/dev/fd")
	if len(after) > len(before) {
		t.Fatalf("descriptor leak: %d -> %d", len(before), len(after))
	}
}
