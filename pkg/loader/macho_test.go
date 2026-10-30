package loader

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/sarchlab/poros/internal/machotest"
)

func TestMachOExecutableAndDyld(t *testing.T) {
	for _, kind := range []uint32{2, 7} {
		for _, fat := range []bool{false, true} {
			b := machotest.Image(kind)
			if fat {
				b = machotest.Universal(b)
			}
			i, e := ParseMachO(b, kind)
			if e != nil {
				t.Fatal(e)
			}
			if len(i.Segments) != 2 || i.Segments[1].FileSize != 0 || len(i.SHA256) != 64 {
				t.Fatal(i)
			}
			if kind == 2 && (i.MainEntry != 0x100000400 || i.Dylinker != "/usr/lib/dyld") {
				t.Fatal(i)
			}
			if kind == 7 && i.ThreadEntry != 0x400 {
				t.Fatal(i)
			}
			data, e := i.SegmentData(0)
			if e != nil {
				t.Fatal(e)
			}
			before := data[0]
			clear(b)
			clear(data)
			data, e = i.SegmentData(0)
			if e != nil {
				t.Fatal(e)
			}
			if data[0] != before {
				t.Fatal("image aliases input")
			}
		}
	}
}
func TestMachORejectsMalformed(t *testing.T) {
	u := binary.LittleEndian
	cases := []struct {
		name string
		edit func([]byte) []byte
	}{
		{"short", func(b []byte) []byte { return b[:10] }},
		{"commands outside file", func(b []byte) []byte { u.PutUint32(b[20:], ^uint32(0)); return b }},
		{"command zero", func(b []byte) []byte { u.PutUint32(b[36:], 0); return b }},
		{"section count", func(b []byte) []byte { u.PutUint32(b[96:], ^uint32(0)); return b }},
		{"segment wraps", func(b []byte) []byte { u.PutUint64(b[56:], ^uint64(0)-1); return b }},
		{"file outside", func(b []byte) []byte { u.PutUint64(b[72:], 16384); return b }},
		{"file exceeds memory", func(b []byte) []byte { u.PutUint64(b[80:], 32768); return b }},
		{"protection", func(b []byte) []byte { u.PutUint32(b[92:], 8); return b }},
		{"overlap", func(b []byte) []byte { u.PutUint64(b[128:], 0x100000000); return b }},
		{"entry BSS", func(b []byte) []byte { u.PutUint64(b[184:], 16384); return b }},
		{"entry tail", func(b []byte) []byte { u.PutUint64(b[184:], 16382); return b }},
		{"wrong CPU", func(b []byte) []byte { u.PutUint32(b[4:], 7); return b }},
		{"wrong type", func(b []byte) []byte { u.PutUint32(b[12:], 7); return b }},
		{"required command", func(b []byte) []byte { u.PutUint32(b[200:], 0x80001234); return b }},
		{"bad dylinker offset", func(b []byte) []byte { u.PutUint32(b[208:], 32); return b }},
		{"unterminated dylinker", func(b []byte) []byte { copy(b[212:232], bytes.Repeat([]byte{'x'}, 20)); return b }},
		{"excess image", func(b []byte) []byte { u.PutUint64(b[136:], MaxMachOImageBytes); return b }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, e := ParseMachO(tc.edit(machotest.Image(2)), 2)
			var me *MachOError
			if !errors.As(e, &me) {
				t.Fatalf("wanted classified error, got %v", e)
			}
		})
	}
}
func TestMachOUniversalBoundsAndArchitectureAgreement(t *testing.T) {
	for _, edit := range []func([]byte){
		func(b []byte) { binary.BigEndian.PutUint32(b[16:], ^uint32(0)) },
		func(b []byte) { binary.BigEndian.PutUint32(b[20:], 0) },
		func(b []byte) { binary.BigEndian.PutUint32(b[24:], 64) },
		func(b []byte) { binary.BigEndian.PutUint32(b[12:], 2) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[16384+4:], 7) },
	} {
		b := machotest.Universal(machotest.Image(2))
		edit(b)
		if _, e := ParseMachO(b, 2); e == nil {
			t.Fatal("malformed universal accepted")
		}
	}
}
func FuzzParseMachO(f *testing.F) {
	f.Add(machotest.Image(2))
	f.Add(machotest.Universal(machotest.Image(7)))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseMachO(b, 2); _, _ = ParseMachO(b, 7) })
}

func TestMachOAllImageInfoSectionMetadata(t *testing.T) {
	b := machotest.WithAllImageInfo(machotest.Image(7))
	i, err := ParseMachO(b, 7)
	if err != nil || i.AllImageInfoAddress != 0x4100 || i.AllImageInfoSize != 0x170 {
		t.Fatal(i, err)
	}
	u := binary.LittleEndian
	size := int(u.Uint32(b[20:]))
	copy(b[336:32+size+80], b[256:32+size])
	copy(b[256:336], b[176:256])
	u.PutUint32(b[108:], 232)
	u.PutUint32(b[168:], 2)
	u.PutUint32(b[20:], uint32(size+80))
	if _, err := ParseMachO(b, 7); err == nil {
		t.Fatal("ambiguous all-image-info sections accepted")
	}
}

func TestMachOOriginalFileSnapshotAndSegmentCopies(t *testing.T) {
	for _, fat := range []bool{false, true} {
		input := machotest.Image(2)
		if fat {
			input = machotest.Universal(input)
		}
		original := append([]byte(nil), input...)
		image, err := ParseMachO(input, 2)
		if err != nil {
			t.Fatal(err)
		}
		clear(input)
		data, err := image.SegmentData(0)
		if err != nil {
			t.Fatal(err)
		}
		clear(data)
		got := make([]byte, len(original))
		n, err := image.ReadAt(got, 0)
		if err != nil || n != len(got) || !bytes.Equal(got, original) || image.FileSize != uint64(len(original)) {
			t.Fatal("file snapshot aliases caller or segment", n, err)
		}
		var tail [8]byte
		n, err = image.ReadAt(tail[:], int64(len(original)-3))
		if n != 3 || err != io.EOF || !bytes.Equal(tail[:3], original[len(original)-3:]) {
			t.Fatal("short EOF semantics", n, err)
		}
		if _, err = image.ReadAt(tail[:], -1); err == nil {
			t.Fatal("negative offset accepted")
		}
		for _, index := range []int{-1, len(image.Segments)} {
			if _, err = image.SegmentData(index); err == nil {
				t.Fatal("bad segment index")
			}
		}
		image.Segments[0].FileOffset = ^uint64(0)
		if _, err = image.SegmentData(0); err == nil {
			t.Fatal("caller-mutated extent accepted")
		}
	}
}
