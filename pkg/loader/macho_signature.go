package loader

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// VerifyCodeSignatureRange checks a SHA256 CodeDirectory against the CDHash
// captured from the same native executable and validates every covered file-page
// hash. This is a bounded integrity/coverage check, not certificate trust or a
// replacement for native code-signing policy. Unsupported formats fail closed.
// Offsets are relative to the complete file, including a universal container.
func (i *MachOImage) VerifyCodeSignatureRange(offset, size uint64, cdhash string) error {
	fail := func() error { return fmt.Errorf("unsupported or invalid captured Mach-O code-signature coverage") }
	if size == 0 || offset < i.sliceOffset || offset-i.sliceOffset > uint64(len(i.slice)) || size > uint64(len(i.slice))-(offset-i.sliceOffset) {
		return fail()
	}
	var signature []byte
	for _, entry := range i.Linkedit {
		if entry.Command == 0x1d {
			if signature != nil {
				return fail()
			}
			a, z := uint64(entry.Offset), uint64(entry.Size)
			if a > uint64(len(i.slice)) || z > uint64(len(i.slice))-a {
				return fail()
			}
			signature = i.slice[a : a+z]
		}
	}
	u := binary.BigEndian
	if len(signature) < 12 || u.Uint32(signature) != 0xfade0cc0 {
		return fail()
	}
	length, count := uint64(u.Uint32(signature[4:])), uint64(u.Uint32(signature[8:]))
	if length > uint64(len(signature)) || length < 12 || count > (length-12)/8 || count > 64 {
		return fail()
	}
	var directory []byte
	for n := uint64(0); n < count; n++ {
		slot, at := u.Uint32(signature[12+n*8:]), uint64(u.Uint32(signature[16+n*8:]))
		if at < 12+count*8 || at > length || length-at < 8 {
			return fail()
		}
		z := uint64(u.Uint32(signature[at+4:]))
		if z < 8 || z > length-at {
			return fail()
		}
		if slot == 0 {
			if directory != nil {
				return fail()
			}
			directory = signature[at : at+z]
		}
	}
	if len(directory) < 88 || u.Uint32(directory) != 0xfade0c02 || u.Uint32(directory[8:]) != 0x20400 || directory[36] != 32 || directory[37] != 2 || directory[39] != 12 || u.Uint32(directory[44:]) != 0 || u.Uint64(directory[56:]) != 0 {
		return fail()
	}
	sum := sha256.Sum256(directory)
	if hex.EncodeToString(sum[:20]) != cdhash {
		return fail()
	}
	hashOffset, pages, limit := uint64(u.Uint32(directory[16:])), uint64(u.Uint32(directory[28:])), uint64(u.Uint32(directory[32:]))
	if limit > uint64(len(i.slice)) || pages != (limit+4095)/4096 || hashOffset < 88 || hashOffset > uint64(len(directory)) || pages > (uint64(len(directory))-hashOffset)/32 || offset-i.sliceOffset > limit || size > limit-(offset-i.sliceOffset) {
		return fail()
	}
	first, last := (offset-i.sliceOffset)/4096, (offset-i.sliceOffset+size-1)/4096
	for page := first; page <= last; page++ {
		start := page * 4096
		sum := sha256.Sum256(i.slice[start:min(start+4096, limit)])
		expected := directory[hashOffset+page*32 : hashOffset+(page+1)*32]
		for n, b := range sum {
			if b != expected[n] {
				return fail()
			}
		}
	}
	return nil
}
