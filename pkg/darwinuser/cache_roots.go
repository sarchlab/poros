package darwinuser

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/sarchlab/poros/pkg/usermem"
)

// CacheRootsProfile records the selected v4 fields consumed by the matching
// installed dyld's rootsBitmapsValid/isImageRooted/isImageCryptexRooted methods.
// This layout is binary-derived, not a claimed public cross-version ABI. The
// enclosing profile binds it to the loaded shared-cache UUID and dynamic range.
type CacheRootsProfile struct {
	Flags         uint8  `json:"roots_flags"`
	ImageCount    uint32 `json:"image_count"`
	ImageOffset   uint32 `json:"image_roots_offset"`
	ImageSize     uint32 `json:"image_roots_size"`
	CryptexOffset uint32 `json:"cryptex_roots_offset"`
	CryptexSize   uint32 `json:"cryptex_roots_size"`
	ImageHex      string `json:"image_roots_hex"`
	CryptexHex    string `json:"cryptex_roots_hex"`
}

func (p CacheRootsProfile) mapRegion(m *usermem.AddressSpace, base, size uint64) error {
	if p.Flags&1 == 0 || p.ImageCount == 0 || p.ImageCount > 65536 {
		return fmt.Errorf("unsupported captured roots bitmap state")
	}
	n := (p.ImageCount + 7) / 8
	a, b := uint64(p.ImageOffset), uint64(p.CryptexOffset)
	if p.ImageSize != n || p.CryptexSize != n || a < 112 || b < 112 || a+uint64(n) > size || b+uint64(n) > size || (a < b+uint64(n) && b < a+uint64(n)) {
		return fmt.Errorf("invalid roots bitmap descriptor ranges")
	}
	if len(p.ImageHex) != int(n)*2 || len(p.CryptexHex) != int(n)*2 {
		return fmt.Errorf("invalid captured roots bitmap length")
	}
	image, err := hex.DecodeString(p.ImageHex)
	if err != nil {
		return fmt.Errorf("invalid image roots bitmap: %w", err)
	}
	cryptex, err := hex.DecodeString(p.CryptexHex)
	if err != nil {
		return fmt.Errorf("invalid cryptex roots bitmap: %w", err)
	}
	var header [5]byte
	binary.LittleEndian.PutUint32(header[:4], p.ImageCount)
	header[4] = p.Flags
	var descriptors [16]byte
	for i, v := range []uint32{p.ImageOffset, p.ImageSize, p.CryptexOffset, p.CryptexSize} {
		binary.LittleEndian.PutUint32(descriptors[i*4:], v)
	}
	for _, v := range []struct {
		offset uint64
		data   []byte
	}{{80, header[:]}, {96, descriptors[:]}, {a, image}, {b, cryptex}} {
		if err := m.Map(base+v.offset, uint64(len(v.data)), usermem.Read, v.data); err != nil {
			return err
		}
	}
	return nil
}
