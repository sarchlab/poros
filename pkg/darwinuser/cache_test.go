package darwinuser

import (
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ownedCache(t *testing.T, dynamic bool) *loader.SharedCache {
	t.Helper()
	b := machotest.CacheImage()
	if dynamic {
		binary.LittleEndian.PutUint64(b[496:], 0x8000)
		binary.LittleEndian.PutUint64(b[504:], 0x4000)
	}
	path := filepath.Join(t.TempDir(), "cache")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := loader.OpenSharedCache(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}
func TestBorrowedCacheAndSharedRegionQuery(t *testing.T) {
	c := ownedCache(t, false)
	p := process(t, Options{SharedCache: c})
	q := process(t, Options{SharedCache: c})
	if word(t, p, c.Base+0x4010) != c.Base+0x100 {
		t.Fatal("cache rebase")
	}
	if err := p.Memory.Protect(c.Base+0x4000, 0x4000, 6, false); err != nil {
		t.Fatal(err)
	}
	if err := p.Memory.Write(c.Base+0x4010, []byte{0xff}); err != nil {
		t.Fatal(err)
	}
	if word(t, q, c.Base+0x4010) != c.Base+0x100 {
		t.Fatal("processes share mutable cache")
	}
	r := emu.Registers{}
	r.GPR[16] = 294
	r.GPR[0] = p.Registers.SP
	if !p.kernel.trap(&r, p.Memory) || r.GPR[0] != 0 || r.C || word(t, p, p.Registers.SP) != c.Base {
		t.Fatal("shared region result")
	}
	r.GPR[0] = 1
	if !p.kernel.trap(&r, p.Memory) || r.GPR[0] != 14 || !r.C {
		t.Fatal("copyout error")
	}
	r.GPR[0] = 0
	before := r
	if p.kernel.trap(&r, p.Memory) || r != before {
		t.Fatal("unmap side effect silently ignored")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(machotest.Image(2), machotest.Image(7), Options{SharedCache: c}); err == nil {
		t.Fatal("borrowed closed cache accepted")
	}
	if err := p.Memory.Read(c.Base, make([]byte, 8)); err == nil {
		t.Fatal("unloaded closed cache page read")
	}
}
func TestDynamicProfileIdentityKnownFieldsAndUnknownGap(t *testing.T) {
	c := ownedCache(t, true)
	path := "/system/cache"
	zero := hex.EncodeToString(make([]byte, 16))
	d := CacheDynamicProfile{UUID: c.UUID, Offset: c.DynamicOffset, Size: c.DynamicSize, MagicHex: hex.EncodeToString([]byte("dyld_data    v4\x00")), FileIDHex: zero, SystemFlagsHex: zero, ProcessorFlagsHex: zero, CachePathOffset: 112, CachePath: &path}
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, CacheDynamic: &d}
	p := process(t, Options{SharedCache: c, RuntimeProfile: &profile})
	if textAt(t, p, c.Base+c.DynamicOffset+112) != path {
		t.Fatal("cache path lost")
	}
	if err := p.Memory.Read(c.Base+c.DynamicOffset+80, make([]byte, 1)); err == nil {
		t.Fatal("unknown v4 field guessed")
	}
	if err := p.Memory.Write(c.Base+c.DynamicOffset, []byte{0}); err == nil {
		t.Fatal("dynamic region writable")
	}
	d.UUID = "bad"
	if _, err := Parse(machotest.Image(2), machotest.Image(7), Options{SharedCache: c, RuntimeProfile: &profile}); err == nil {
		t.Fatal("profile from different runtime accepted")
	}
}

func TestDynamicCacheRespectsUnmappedReservedSpan(t *testing.T) {
	c := ownedCache(t, true)
	main := machotest.Image(2)
	// Reserve an inaccessible segment at the dynamic region without mapping it.
	copy(main[112:128], "__PAGEZERO")
	binary.LittleEndian.PutUint64(main[128:], c.Base+c.DynamicOffset)
	binary.LittleEndian.PutUint32(main[160:], 0)
	binary.LittleEndian.PutUint32(main[164:], 0)
	if _, err := Parse(main, machotest.Image(7), Options{SharedCache: c}); err == nil || !strings.Contains(err.Error(), "dynamic cache overlaps reserved") {
		t.Fatal("reserved gap accepted", err)
	}
}
