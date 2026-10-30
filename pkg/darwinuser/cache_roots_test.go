package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/usermem"
)

func TestNativeCacheRootsCapture(t *testing.T) {
	var p CacheRootsProfile
	kernelNativeResult(t, "cache-roots-bitmaps-probe.c", &p)
	m := &usermem.AddressSpace{}
	const base = 0x10000
	if err := p.mapRegion(m, base, 16384); err != nil {
		t.Fatal(err)
	}
	var header [5]byte
	if err := m.Read(base+80, header[:]); err != nil || binary.LittleEndian.Uint32(header[:4]) != 4115 || header[4] != 1 {
		t.Fatal(header, err)
	}
	for _, field := range []struct {
		offset uint32
		hex    string
	}{{p.ImageOffset, p.ImageHex}, {p.CryptexOffset, p.CryptexHex}} {
		want, _ := hex.DecodeString(field.hex)
		got := make([]byte, len(want))
		if err := m.Read(base+uint64(field.offset), got); err != nil || !bytes.Equal(got, want) {
			t.Fatal("captured bitmap changed", err)
		}
	}
	if m.Read(base+85, make([]byte, 3)) == nil {
		t.Fatal("unknown header bytes fabricated")
	}
	if m.Write(base+uint64(p.ImageOffset), []byte{1}) == nil {
		t.Fatal("roots bitmap writable")
	}
}

func TestCacheRootsValidateAndPreservePathEnvelopeOverlap(t *testing.T) {
	c := ownedCache(t, true)
	path := "/system/cache"
	length, zero := uint32(len(path)+1), uint32(0)
	empty := hex.EncodeToString(make([]byte, 16))
	window := append([]byte(path), 0, 0x81, 0x01)
	roots := CacheRootsProfile{Flags: 1, ImageCount: 9, ImageOffset: 126, ImageSize: 2, CryptexOffset: 128, CryptexSize: 2, ImageHex: "8101", CryptexHex: "0200"}
	p := CacheDynamicProfile{Roots: &roots, UUID: c.UUID, Offset: c.DynamicOffset, Size: c.DynamicSize, MagicHex: hex.EncodeToString([]byte("dyld_data    v4\x00")), FileIDHex: empty, SystemFlagsHex: empty, ProcessorFlagsHex: empty, CachePathOffset: 112, CachePath: &path, CryptexLength: &zero, CachePathLength: &length, CachePathWindowHex: hex.EncodeToString(window)}
	m := &usermem.AddressSpace{}
	if err := p.mapRegion(m, c); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 18)
	if err := m.Read(c.Base+c.DynamicOffset+112, got); err != nil || !bytes.Equal(got, append(window, 2, 0)) {
		t.Fatal(got, err)
	}
	var descriptors [16]byte
	if err := m.Read(c.Base+c.DynamicOffset+96, descriptors[:]); err != nil {
		t.Fatal(err)
	}
	for i, want := range []uint32{126, 2, 128, 2} {
		if binary.LittleEndian.Uint32(descriptors[i*4:]) != want {
			t.Fatal("wrong descriptor")
		}
	}
	for _, mutate := range []func(*CacheRootsProfile){
		func(r *CacheRootsProfile) { r.Flags = 0 }, func(r *CacheRootsProfile) { r.ImageCount = 0 }, func(r *CacheRootsProfile) { r.ImageCount = 65537 },
		func(r *CacheRootsProfile) { r.ImageSize = 1 }, func(r *CacheRootsProfile) { r.CryptexSize = 3 }, func(r *CacheRootsProfile) { r.ImageOffset = 100 },
		func(r *CacheRootsProfile) { r.CryptexOffset = 127 }, func(r *CacheRootsProfile) { r.CryptexOffset = 16383 }, func(r *CacheRootsProfile) { r.ImageHex = "zz01" },
		func(r *CacheRootsProfile) { r.CryptexHex = "00" }, func(r *CacheRootsProfile) { r.ImageHex = "8001" },
	} {
		badRoots := roots
		mutate(&badRoots)
		bad := p
		bad.Roots = &badRoots
		if bad.mapRegion(&usermem.AddressSpace{}, c) == nil {
			t.Fatal("invalid roots state accepted", badRoots)
		}
	}
	p.MagicHex = hex.EncodeToString([]byte("dyld_data    v3\x00"))
	if p.mapRegion(&usermem.AddressSpace{}, c) == nil {
		t.Fatal("v4 fields accepted for v3")
	}
}

func TestCacheFileIdentityResolvesOnlyCapturedPath(t *testing.T) {
	var native struct {
		FileIDHex     string `json:"file_id_hex"`
		PathHex       string `json:"path_hex"`
		X0, X1        uint64
		Carry         bool
		TailUnchanged bool `json:"tail_unchanged"`
		StatMatches   bool `json:"stat_identity_matches"`
	}
	kernelNativeResult(t, "cache-fsgetpath-probe.c", &native)
	id, err := hex.DecodeString(native.FileIDHex)
	if err != nil || len(id) != 16 {
		t.Fatal("invalid native file identity", err)
	}
	pathBytes, err := hex.DecodeString(native.PathHex)
	if err != nil || len(pathBytes) < 2 || pathBytes[len(pathBytes)-1] != 0 {
		t.Fatal("invalid native path", err)
	}
	path := string(pathBytes[:len(pathBytes)-1])
	c := ownedCache(t, true)
	length, zero := uint32(len(path)+1), uint32(0)
	empty := hex.EncodeToString(make([]byte, 16))
	d := CacheDynamicProfile{UUID: c.UUID, Offset: c.DynamicOffset, Size: c.DynamicSize, MagicHex: hex.EncodeToString([]byte("dyld_data    v4\x00")), FileIDHex: hex.EncodeToString(id[:]), SystemFlagsHex: empty, ProcessorFlagsHex: empty, CachePathOffset: 112, CachePath: &path, CryptexLength: &zero, CachePathLength: &length}
	profile := RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 125, TimebaseDenom: 3, CacheDynamic: &d}
	p := process(t, Options{SharedCache: c, RuntimeProfile: &profile})
	if err := p.Memory.Map(0x10000, 1024, usermem.Read|usermem.Write, nil); err != nil {
		t.Fatal(err)
	}
	if err := p.Memory.Map(0x20000, 8, usermem.Read, id[:8]); err != nil {
		t.Fatal(err)
	}
	p.Registers.GPR[0] = 0x10000
	p.Registers.GPR[1] = 1024
	p.Registers.GPR[2] = 0x20000
	p.Registers.GPR[3] = binary.LittleEndian.Uint64(id[8:])
	if !p.kernel.fsgetpath(&p.Registers, p.Memory) || p.Registers.C || p.Registers.GPR[0] != native.X0 || p.Registers.GPR[1] != native.X1 || !native.TailUnchanged || !native.StatMatches || textAt(t, p, 0x10000) != path {
		t.Fatal("cache file identity not registered")
	}
	startup := startupFixture(t)
	startup.ExecutableFile.FSID = binary.LittleEndian.Uint64(id[:8])
	startup.ExecutableFile.ObjectID = binary.LittleEndian.Uint64(id[8:])
	if _, err := Parse(machotest.Image(2), machotest.Image(7), Options{SharedCache: c, RuntimeProfile: &profile, StartupProfile: startup}); err == nil {
		t.Fatal("cache/startup identity collision accepted")
	}
	for _, badPath := range []string{"relative", "/system/../cache", "/bad\x00path"} {
		d.CachePath = &badPath
		if _, _, err := d.fileIdentity(); err == nil {
			t.Fatal("invalid cache identity path", badPath)
		}
	}
}
