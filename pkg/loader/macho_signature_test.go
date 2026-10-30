package loader

import (
	"encoding/binary"
	"github.com/sarchlab/poros/internal/machotest"
	"testing"
)

func TestMachOSignaturePageCoverageAndFatOffset(t *testing.T) {
	raw, hash := machotest.SignedDataImage([]byte("covered data"))
	for _, fat := range []bool{false, true} {
		b := append([]byte(nil), raw...)
		base := uint64(0)
		if fat {
			b = machotest.Universal(b)
			base = 16384
		}
		img, e := ParseMachO(b, 2)
		if e != nil {
			t.Fatal(e)
		}
		if img.SliceOffset() != base {
			t.Fatal("slice offset", img.SliceOffset())
		}
		if e = img.VerifyCodeSignatureRange(base+16384, 16384, hash); e != nil {
			t.Fatal(e)
		}
		for _, r := range [][2]uint64{{base + 32767, 2}, {base + 32768, 1}, {base + 16384, 0}, {^uint64(0), 8}} {
			if img.VerifyCodeSignatureRange(r[0], r[1], hash) == nil {
				t.Fatal("coverage escaped", r)
			}
		}
		if img.VerifyCodeSignatureRange(base+16384, 16384, "bad") == nil {
			t.Fatal("unbound CDHash")
		}
		b[base+16384] ^= 1
		changed, e := ParseMachO(b, 2)
		if e != nil {
			t.Fatal(e)
		}
		if changed.VerifyCodeSignatureRange(base+16384, 16384, hash) == nil {
			t.Fatal("corrupt page accepted")
		}
	}
	for _, change := range []func([]byte){func(b []byte) { binary.BigEndian.PutUint32(b[32768+16:], ^uint32(0)) }, func(b []byte) { b[32768+20+37] = 1 }, func(b []byte) { binary.BigEndian.PutUint32(b[32768+20+44:], 1) }, func(b []byte) { binary.BigEndian.PutUint32(b[32768+20+28:], ^uint32(0)) }} {
		b := append([]byte(nil), raw...)
		change(b)
		img, e := ParseMachO(b, 2)
		if e != nil {
			t.Fatal(e)
		}
		if img.VerifyCodeSignatureRange(16384, 16384, hash) == nil {
			t.Fatal("malformed signature accepted")
		}
	}
}
