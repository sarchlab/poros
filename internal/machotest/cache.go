package machotest

import "encoding/binary"

// CacheImage constructs owned slide-v5 metadata, not an Apple runtime copy.
// The first chain starts at byte16, then advances one encoded stride of8.
func CacheImage() []byte {
	b := make([]byte, 0xc000)
	u := binary.LittleEndian
	copy(b, "dyld_v1  arm64e\x00")
	u.PutUint32(b[16:], 512)
	u.PutUint32(b[20:], 2)
	for i := 0; i < 16; i++ {
		b[88+i] = byte(i + 1)
	}
	u.PutUint64(b[224:], 0x180000000)
	u.PutUint64(b[232:], 0xc000)
	u.PutUint32(b[312:], 0x400)
	u.PutUint32(b[316:], 2)
	for i := 0; i < 2; i++ {
		m := b[0x400+i*56:]
		u.PutUint64(m, 0x180000000+uint64(i)*0x4000)
		u.PutUint64(m[8:], 0x4000)
		u.PutUint64(m[16:], uint64(i)*0x4000)
		u.PutUint32(m[48:], 3)
		u.PutUint32(m[52:], 1)
	}
	m := b[0x438:]
	u.PutUint64(m[24:], 0x8000)
	u.PutUint64(m[32:], 26)
	u.PutUint32(b[0x8000:], 5)
	u.PutUint32(b[0x8004:], 16384)
	u.PutUint32(b[0x8008:], 1)
	u.PutUint64(b[0x8010:], 0x180000000)
	u.PutUint16(b[0x8018:], 16)
	u.PutUint64(b[0x4010:], 0x100|1<<52)
	u.PutUint64(b[0x4018:], 0x200|1<<63|0xabcd<<34|1<<50|1<<51)
	return b
}
