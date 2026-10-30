package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
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
	length, empty := uint32(len(path)+1), uint32(0)
	zero := hex.EncodeToString(make([]byte, 16))
	d := CacheDynamicProfile{UUID: c.UUID, Offset: c.DynamicOffset, Size: c.DynamicSize, MagicHex: hex.EncodeToString([]byte("dyld_data    v4\x00")), FileIDHex: zero, SystemFlagsHex: zero, ProcessorFlagsHex: zero, CachePathOffset: 112, CachePath: &path, CryptexLength: &empty, CachePathLength: &length}
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
	if _, err := Parse(main, machotest.Image(7), Options{SharedCache: c}); err == nil || !strings.Contains(err.Error(), "overlaps reserved process region") {
		t.Fatal("reserved gap accepted", err)
	}
}

func TestDynamicPathEnvelopePreservesPaddingAndChecksKnownFields(t *testing.T) {
	c := ownedCache(t, true)
	path := "/system/cache"
	length, zero := uint32(len(path)+1), uint32(0)
	empty := hex.EncodeToString(make([]byte, 16))
	window := append([]byte(path), 0, 0xa5, 0x5a)
	d := CacheDynamicProfile{UUID: c.UUID, Offset: c.DynamicOffset, Size: c.DynamicSize, MagicHex: hex.EncodeToString([]byte("dyld_data    v4\x00")), FileIDHex: empty, SystemFlagsHex: empty, ProcessorFlagsHex: empty, CachePathOffset: 112, CachePath: &path, CryptexLength: &zero, CachePathLength: &length, CachePathWindowHex: hex.EncodeToString(window)}
	m := &usermem.AddressSpace{}
	if err := d.mapRegion(m, c); err != nil {
		t.Fatal(err)
	}
	var got [16]byte
	if err := m.Read(c.Base+c.DynamicOffset+112, got[:]); err != nil || !bytes.Equal(got[:], window) {
		t.Fatal("padding changed", err, got)
	}
	for _, mutate := range []func(*CacheDynamicProfile){func(p *CacheDynamicProfile) { v := length + 1; p.CachePathLength = &v }, func(p *CacheDynamicProfile) { p.CachePathWindowHex = "00" }, func(p *CacheDynamicProfile) { p.CachePathLength = nil }, func(p *CacheDynamicProfile) { p.CachePathOffset = 80 }} {
		bad := d
		mutate(&bad)
		if err := bad.mapRegion(&usermem.AddressSpace{}, c); err == nil {
			t.Fatal("invalid path accepted")
		}
	}
	// An older unaligned path envelope can overlap already known header fields.
	// Those bytes must agree before the remaining captured bytes are mapped.
	older := d
	older.MagicHex = hex.EncodeToString([]byte("dyld_data    v2\x00"))
	older.CachePathOffset = 41
	w := make([]byte, 32)
	binary.LittleEndian.PutUint32(w[4:], 41)
	copy(w[9:], path)
	w[30] = 0xa5
	older.CachePathWindowHex = hex.EncodeToString(w)
	if err := older.mapRegion(&usermem.AddressSpace{}, c); err != nil {
		t.Fatal("consistent known overlap", err)
	}
	w[0] = 1
	older.CachePathWindowHex = hex.EncodeToString(w)
	if err := older.mapRegion(&usermem.AddressSpace{}, c); err == nil {
		t.Fatal("inconsistent known overlap accepted")
	}
}
