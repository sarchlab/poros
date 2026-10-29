package loader

import (
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func fixtureELF(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/elf/startup.elf")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestELFFixture(t *testing.T) {
	b := fixtureELF(t)
	h := sha256.Sum256(b)
	var provenance struct {
		SHA256 map[string]string `json:"sha256"`
	}
	manifest, e := os.ReadFile("testdata/elf/provenance.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(manifest, &provenance); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"start.S", "layout.ld", "build.sh", "startup.elf"} {
		data, e := os.ReadFile("testdata/elf/" + name)
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != provenance.SHA256[name] {
			t.Fatalf("fixture provenance mismatch: %s", name)
		}
	}
	if hex.EncodeToString(h[:]) != provenance.SHA256["startup.elf"] {
		t.Fatal("fixture changed without provenance review")
	}
	image, e := ParseELF(b)
	if e != nil {
		t.Fatal(e)
	}
	if image.Entry != 0x400120 || image.ProgramHeaderAddress != 0x400040 || image.ProgramHeaderCount != 4 || len(image.Segments) != 3 {
		t.Fatalf("unexpected image: %+v", image)
	}
	for i, want := range []elf.ProgFlag{elf.PF_R | elf.PF_X, elf.PF_R, elf.PF_R | elf.PF_W} {
		s := image.Segments[i]
		if s.Flags != want || len(s.Data) != 4096 {
			t.Fatalf("segment%d %+v", i, s)
		}
	}
	data := image.Segments[2].Data
	if binary.LittleEndian.Uint64(data) != 0x0123456789abcdef {
		t.Fatal("initialized data")
	}
	for _, v := range data[8:] {
		if v != 0 {
			t.Fatal("BSS or page tail not zero")
		}
	}
	b[0] = 0
	if image.Segments[0].Data[0] != 0x7f {
		t.Fatal("image aliases input")
	}
	loaded, e := LoadELF("testdata/elf/startup.elf")
	if e != nil || loaded.Entry != image.Entry {
		t.Fatal(loaded, e)
	}
}
func TestELFWithoutSectionHeaders(t *testing.T) {
	b := fixtureELF(t)
	clear(b[40:48])
	clear(b[58:64])
	if _, e := ParseELF(b); e != nil {
		t.Fatal(e)
	}
}
func TestELFRejectsMalformedAndUnsupported(t *testing.T) {
	u := binary.LittleEndian
	tests := []struct {
		name, kind string
		mutate     func([]byte) []byte
	}{
		{"magic", "malformed", func(b []byte) []byte { b[0] = 0; return b }},
		{"class", "unsupported", func(b []byte) []byte { b[4] = 1; return b }},
		{"endian", "unsupported", func(b []byte) []byte { b[5] = 2; return b }},
		{"abi", "unsupported", func(b []byte) []byte { b[7] = 9; return b }},
		{"pie", "unsupported", func(b []byte) []byte { u.PutUint16(b[16:], 3); return b }},
		{"machine", "unsupported", func(b []byte) []byte { u.PutUint16(b[18:], 62); return b }},
		{"version", "malformed", func(b []byte) []byte { u.PutUint32(b[20:], 2); return b }},
		{"ehsize", "malformed", func(b []byte) []byte { u.PutUint16(b[52:], 1); return b }},
		{"phentsize", "malformed", func(b []byte) []byte { u.PutUint16(b[54:], 1); return b }},
		{"archflags", "unsupported", func(b []byte) []byte { u.PutUint32(b[48:], 1); return b }},
		{"noheaders", "limit", func(b []byte) []byte { u.PutUint16(b[56:], 0); return b }},
		{"manyheaders", "limit", func(b []byte) []byte { u.PutUint16(b[56:], 65535); return b }},
		{"phoverflow", "malformed", func(b []byte) []byte { u.PutUint64(b[32:], ^uint64(0)); return b }},
		{"phtruncated", "malformed", func(b []byte) []byte { return b[:200] }},
		{"filesize", "malformed", func(b []byte) []byte { u.PutUint64(b[64+32:], 0x100000); return b }},
		{"fileoffset", "malformed", func(b []byte) []byte { u.PutUint64(b[64+8:], ^uint64(0)); return b }},
		{"alignnotpower2", "malformed", func(b []byte) []byte { u.PutUint64(b[64+48:], 3); return b }},
		{"alignmismatch", "malformed", func(b []byte) []byte { u.PutUint64(b[120+48:], 0x20000); u.PutUint64(b[120+16:], 0x411000); return b }},
		{"pagemismatch-align0", "malformed", func(b []byte) []byte { u.PutUint64(b[64+48:], 0); u.PutUint64(b[64+16:], 0x400001); return b }},
		{"pagemismatch-align1", "malformed", func(b []byte) []byte { u.PutUint64(b[64+48:], 1); u.PutUint64(b[64+16:], 0x400001); return b }},
		{"flagbits", "unsupported", func(b []byte) []byte { u.PutUint32(b[64+4:], 0x100); return b }},
		{"address-wrap", "malformed", func(b []byte) []byte { u.PutUint64(b[64+16:], 0xffffffffffff0000); return b }},
		{"address-zero", "malformed", func(b []byte) []byte { u.PutUint64(b[64+16:], 0); return b }},
		{"memlimit", "limit", func(b []byte) []byte { u.PutUint64(b[64+40:], MaxELFImageBytes+1); return b }},
		{"page-overlap", "unsupported", func(b []byte) []byte { u.PutUint64(b[120+16:], 0x400000); u.PutUint64(b[120+48:], 4096); return b }},
		{"entry-alignment", "malformed", func(b []byte) []byte { u.PutUint64(b[24:], 0x400121); return b }},
		{"entry-readonly", "malformed", func(b []byte) []byte { u.PutUint64(b[24:], 0x401000); return b }},
		{"entry-lastbytes", "malformed", func(b []byte) []byte {
			u.PutUint64(b[24:], 0x40014c)
			u.PutUint64(b[64+32:], 0x14e)
			u.PutUint64(b[64+40:], 0x14e)
			return b
		}},
		{"entry-outside", "malformed", func(b []byte) []byte { u.PutUint64(b[24:], 0x800000); return b }},
		{"headers-unmapped", "unsupported", func(b []byte) []byte { u.PutUint32(b[64+4:], uint32(elf.PF_X)); return b }},
		{"interpreter", "unsupported", func(b []byte) []byte { u.PutUint32(b[232:], uint32(elf.PT_INTERP)); return b }},
		{"dynamic", "unsupported", func(b []byte) []byte { u.PutUint32(b[232:], uint32(elf.PT_DYNAMIC)); return b }},
		{"tls", "unsupported", func(b []byte) []byte { u.PutUint32(b[232:], uint32(elf.PT_TLS)); return b }},
		{"relro", "unsupported", func(b []byte) []byte { u.PutUint32(b[232:], uint32(elf.PT_GNU_RELRO)); return b }},
		{"unknown-type", "unsupported", func(b []byte) []byte { u.PutUint32(b[232:], 0x12345678); return b }},
		{"exec-stack", "unsupported", func(b []byte) []byte { u.PutUint32(b[232+4:], 7); return b }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image, e := ParseELF(tt.mutate(fixtureELF(t)))
			var typed *ELFError
			if image != nil || !errors.As(e, &typed) || typed.Kind != tt.kind {
				t.Fatalf("image=%v error=%v wantkind=%s", image, e, tt.kind)
			}
		})
	}
	for _, n := range []int{0, 1, 63} {
		if _, e := ParseELF(make([]byte, n)); e == nil {
			t.Fatal("short header accepted")
		}
	}
	if _, e := ParseELF(make([]byte, MaxELFFileBytes+1)); e == nil {
		t.Fatal("large file accepted")
	}
}

func TestELFPagePrefixAndBSS(t *testing.T) {
	b := fixtureELF(t)
	u := binary.LittleEndian
	// Move the writable segment start sixteen bytes into the same file/guest page.
	// Its file-backed prefix must remain visible; the BSS suffix must be zero.
	u.PutUint64(b[176+8:], 0x2010)
	u.PutUint64(b[176+16:], 0x402010)
	u.PutUint64(b[176+32:], 1)
	b[0x2010] = 0xa5
	u.PutUint64(b[176+40:], 64)
	image, e := ParseELF(b)
	if e != nil {
		t.Fatal(e)
	}
	if u.Uint64(image.Segments[2].Data) != 0x0123456789abcdef {
		t.Fatal("lost file-backed page prefix")
	}
	if image.Segments[2].Data[16] != 0xa5 {
		t.Fatal("file byte")
	}
	for _, v := range image.Segments[2].Data[17:] {
		if v != 0 {
			t.Fatal("nonzero BSS")
		}
	}
	u.PutUint64(b[176+32:], 0)
	image, e = ParseELF(b)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range image.Segments[2].Data {
		if v != 0 {
			t.Fatal("pure BSS mapping has file prefix bytes")
		}
	}
}
func TestELFExplicitPHDR(t *testing.T) {
	b := fixtureELF(t)
	u := binary.LittleEndian
	// Replace GNU_STACK with an explicit PHDR placed before the three load entries.
	copy(b[120:288], append([]byte(nil), b[64:232]...))
	clear(b[64:120])
	u.PutUint32(b[64:], uint32(elf.PT_PHDR))
	u.PutUint32(b[68:], uint32(elf.PF_R))
	u.PutUint64(b[72:], 64)
	u.PutUint64(b[80:], 0x400040)
	u.PutUint64(b[96:], 224)
	u.PutUint64(b[104:], 224)
	if _, e := ParseELF(b); e != nil {
		t.Fatal(e)
	}
	// A second readable mapping of the header bytes must not override PT_PHDR.
	u.PutUint64(b[232+8:], 0)
	u.PutUint64(b[232+32:], 0x150)
	u.PutUint64(b[232+40:], 0x150)
	u.PutUint64(b[232+48:], 4096)
	if image, e := ParseELF(b); e != nil || image.ProgramHeaderAddress != 0x400040 {
		t.Fatal("PHDR alias selection", e)
	}
	u.PutUint64(b[80:], 0x400048)
	if _, e := ParseELF(b); e == nil {
		t.Fatal("wrong PHDR accepted")
	}
}

func FuzzParseELF(f *testing.F) {
	b, e := os.ReadFile("testdata/elf/startup.elf")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Add([]byte("\x7fELF"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > MaxELFFileBytes+1 {
			return
		}
		image, e := ParseELF(b)
		if (image == nil) != (e != nil) {
			t.Fatal("partial image returned")
		}
	})
}

func TestELFAcceptedHeaderEdges(t *testing.T) {
	u := binary.LittleEndian
	t.Run("128-headers-with-null-and-note", func(t *testing.T) {
		b := fixtureELF(t)
		u.PutUint16(b[56:], 128)
		// Expand the first mapping to hold the larger program-header table;
		// relocate later mappings so the page ranges remain distinct.
		u.PutUint64(b[64+32:], 0x2000)
		u.PutUint64(b[64+40:], 0x2000)
		u.PutUint64(b[120+16:], 0x402000)
		u.PutUint64(b[120+48:], 4096)
		u.PutUint64(b[176+16:], 0x403000)
		u.PutUint64(b[176+48:], 4096)
		clear(b[288 : 64+128*56])
		u.PutUint32(b[288:], uint32(elf.PT_NOTE))
		image, e := ParseELF(b)
		if e != nil || image.ProgramHeaderCount != 128 {
			t.Fatal(image, e)
		}
		u.PutUint16(b[56:], 129)
		if image, e := ParseELF(b); image != nil || e == nil {
			t.Fatal("129 headers accepted")
		}
	})
	t.Run("zero-memsz-load", func(t *testing.T) {
		b := fixtureELF(t)
		u.PutUint64(b[176+32:], 0)
		u.PutUint64(b[176+40:], 0)
		image, e := ParseELF(b)
		if e != nil || len(image.Segments) != 2 {
			t.Fatal(image, e)
		}
	})
	t.Run("64KiB-alignment", func(t *testing.T) {
		b := fixtureELF(t)
		u.PutUint64(b[120+16:], 0x411000)
		u.PutUint64(b[176+16:], 0x412000)
		for _, off := range []int{64, 120, 176} {
			u.PutUint64(b[off+48:], 65536)
		}
		if _, e := ParseELF(b); e != nil {
			t.Fatal(e)
		}
	})
}
