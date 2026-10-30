// Package machotest constructs small, owned Mach-O fixtures for portable tests.
// These bytes are not Apple runtime binaries and cannot stand in for native dyld.
package machotest

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

const EntryOffset = 0x400

// Image returns an ARM64 executable (2) or an arm64e dyld-shaped image (7).
// The executable loops; the runtime establishes entry registers then encounters
// PACIBSP followed by BRK, providing a deterministic unsupported-feature witness.
func Image(kind uint32) []byte {
	u := binary.LittleEndian
	b := make([]byte, 16384)
	put := func(p int, v uint32) { u.PutUint32(b[p:], v) }
	q := func(p int, v uint64) { u.PutUint64(b[p:], v) }
	put(0, 0xfeedfacf)
	put(4, 0x100000c)
	put(12, kind)
	put(24, 4)
	base := uint64(0x100000000)
	if kind == 7 {
		base = 0
		put(8, 0x80000002)
	}
	off := 32
	segment := func(name string, a, size, filesize uint64, prot uint32) {
		put(off, 0x19)
		put(off+4, 72)
		copy(b[off+8:off+24], name)
		q(off+24, a)
		q(off+32, size)
		q(off+48, filesize)
		put(off+56, prot)
		put(off+60, prot)
		off += 72
	}
	segment("__TEXT", base, 16384, 16384, 5)
	segment("__DATA", base+16384, 16384, 0, 3)
	if kind == 2 {
		put(off, 0x80000028)
		put(off+4, 24)
		q(off+8, EntryOffset)
		off += 24
		put(off, 0xe)
		put(off+4, 32)
		put(off+8, 12)
		copy(b[off+12:], "/usr/lib/dyld\x00")
		off += 32
		put(EntryOffset, 0x14000000)
	} else {
		put(off, 5)
		put(off+4, 288)
		put(off+8, 6)
		put(off+12, 68)
		q(off+272, EntryOffset)
		off += 288
		put(off, 0xf)
		put(off+4, 32)
		put(off+8, 12)
		copy(b[off+12:], "/usr/lib/dyld\x00")
		off += 32
		// mov x0,sp; and sp,x0,#~15; mov x29,#0; mov x30,#0; b +4; pacibsp.
		for i, w := range []uint32{0x910003e0, 0x927cec1f, 0xd280001d, 0xd280001e, 0x14000001, 0xd503237f, 0xd4200000} {
			put(EntryOffset+i*4, w)
		}
	}
	put(16, 4)
	put(20, uint32(off-32))
	return b
}
func Universal(slice []byte) []byte {
	b := make([]byte, 16384+len(slice))
	u := binary.BigEndian
	l := binary.LittleEndian
	u.PutUint32(b, 0xcafebabe)
	u.PutUint32(b[4:], 1)
	u.PutUint32(b[8:], l.Uint32(slice[4:]))
	u.PutUint32(b[12:], l.Uint32(slice[8:]))
	u.PutUint32(b[16:], 16384)
	u.PutUint32(b[20:], uint32(len(slice)))
	u.PutUint32(b[24:], 14)
	copy(b[16384:], slice)
	return b
}

// WithAllImageInfo adds an owned zero-fill section to the fixture's data segment.
func WithAllImageInfo(image []byte) []byte {
	b := append([]byte(nil), image...)
	u := binary.LittleEndian
	const segment = 104
	size := int(u.Uint32(b[20:]))
	copy(b[256:32+size+80], b[176:32+size])
	clear(b[176:256])
	u.PutUint32(b[segment+4:], 152)
	u.PutUint32(b[segment+64:], 1)
	u.PutUint32(b[20:], uint32(size+80))
	copy(b[176:192], "__all_image_info")
	copy(b[192:208], "__DATA")
	u.PutUint64(b[208:], u.Uint64(b[segment+24:])+0x100)
	u.PutUint64(b[216:], 0x170)
	u.PutUint32(b[240:], 1)
	return b
}

// SignedDataImage constructs a repo-owned integrity fixture. Its ad-hoc
// CodeDirectory is not a native trust or execution oracle.
func SignedDataImage(data []byte) ([]byte, string) {
	b := append(Image(2), make([]byte, 16384)...)
	copy(b[16384:], data)
	l := binary.LittleEndian
	l.PutUint64(b[144:], 16384)
	l.PutUint64(b[152:], 16384)
	end := 32 + int(l.Uint32(b[20:]))
	l.PutUint32(b[16:], 5)
	l.PutUint32(b[20:], uint32(end+16-32))
	l.PutUint32(b[end:], 0x1d)
	l.PutUint32(b[end+4:], 16)
	l.PutUint32(b[end+8:], 32768)
	l.PutUint32(b[end+12:], 20+88+8*32)
	u := binary.BigEndian
	cd := make([]byte, 88+8*32)
	u.PutUint32(cd, 0xfade0c02)
	u.PutUint32(cd[4:], uint32(len(cd)))
	u.PutUint32(cd[8:], 0x20400)
	u.PutUint32(cd[16:], 88)
	u.PutUint32(cd[28:], 8)
	u.PutUint32(cd[32:], 32768)
	cd[36] = 32
	cd[37] = 2
	cd[39] = 12
	for p := 0; p < 8; p++ {
		hash := sha256.Sum256(b[p*4096 : (p+1)*4096])
		copy(cd[88+p*32:], hash[:])
	}
	hash := sha256.Sum256(cd)
	super := make([]byte, 20)
	u.PutUint32(super, 0xfade0cc0)
	u.PutUint32(super[4:], uint32(20+len(cd)))
	u.PutUint32(super[8:], 1)
	u.PutUint32(super[16:], 20)
	b = append(b, super...)
	b = append(b, cd...)
	return b, hex.EncodeToString(hash[:20])
}
