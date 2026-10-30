package darwinuser

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
)

// CacheDynamicProfile contains only documented dynamic-region fields read from
// the native reference process. Unknown version-4 fields remain unmapped.
type CacheDynamicProfile struct {
	UUID              string  `json:"cache_uuid"`
	Offset            uint64  `json:"dynamic_offset"`
	Size              uint64  `json:"dynamic_size"`
	MagicHex          string  `json:"magic_hex"`
	FileIDHex         string  `json:"file_id_hex"`
	SystemFlagsHex    string  `json:"system_flags_hex"`
	ProcessorFlagsHex string  `json:"processor_flags_hex"`
	CryptexOffset     uint32  `json:"cryptex_offset"`
	CachePathOffset   uint32  `json:"cache_path_offset"`
	CryptexPath       *string `json:"cryptex_path"`
	CachePath         *string `json:"cache_path"`
}

func (p CacheDynamicProfile) mapRegion(m *usermem.AddressSpace, c *loader.SharedCache) error {
	if p.UUID != c.UUID || p.Offset != c.DynamicOffset || p.Size != c.DynamicSize || p.Size < 80 {
		return fmt.Errorf("native cache dynamic profile identity/range mismatch")
	}
	base := c.Base + p.Offset
	mapHex := func(offset uint64, value string) error {
		b, err := hex.DecodeString(value)
		if err != nil || len(b) != 16 {
			return fmt.Errorf("invalid 16-byte cache profile field")
		}
		return m.Map(base+offset, 16, usermem.Read, b)
	}
	magic, err := hex.DecodeString(p.MagicHex)
	if err != nil || len(magic) != 16 || string(magic[:14]) != "dyld_data    v" || magic[15] != 0 || magic[14] < '0' || magic[14] > '4' {
		return fmt.Errorf("unsupported dynamic cache magic")
	}
	if err = mapHex(0, p.MagicHex); err != nil {
		return err
	}
	if err = mapHex(16, p.FileIDHex); err != nil {
		return err
	}
	version := magic[14] - '0'
	mapPath := func(field uint64, offset uint32, path *string) error {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], offset)
		if err := m.Map(base+field, 4, usermem.Read, b[:]); err != nil {
			return err
		}
		if offset == 0 {
			if path != nil {
				return fmt.Errorf("cache path supplied for null offset")
			}
			return nil
		}
		if path == nil || len(*path) == 0 || len(*path) > 1024 || !strings.HasPrefix(*path, "/") || strings.IndexByte(*path, 0) >= 0 || uint64(offset) >= p.Size || uint64(len(*path)+1) > p.Size-uint64(offset) {
			return fmt.Errorf("invalid dynamic cache path")
		}
		return m.Map(base+uint64(offset), uint64(len(*path)+1), usermem.Read, append([]byte(*path), 0))
	}
	if version >= 1 {
		if err = mapPath(32, p.CryptexOffset, p.CryptexPath); err != nil {
			return err
		}
	}
	if version >= 2 {
		if err = mapPath(36, p.CachePathOffset, p.CachePath); err != nil {
			return err
		}
	}
	if version >= 3 {
		if err = mapHex(48, p.SystemFlagsHex); err != nil {
			return err
		}
		if err = mapHex(64, p.ProcessorFlagsHex); err != nil {
			return err
		}
	}
	return nil
}
