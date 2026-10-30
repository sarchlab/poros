package darwinuser

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/sarchlab/poros/pkg/loader"
	"github.com/sarchlab/poros/pkg/usermem"
	"path"
	"strings"
)

// CacheDynamicProfile contains documented fields and selected version-4 fields
// identified from the matching dyld binary. Other unknown fields stay unmapped.
type CacheDynamicProfile struct {
	Roots              *CacheRootsProfile `json:"roots,omitempty"`
	UUID               string             `json:"cache_uuid"`
	Offset             uint64             `json:"dynamic_offset"`
	Size               uint64             `json:"dynamic_size"`
	MagicHex           string             `json:"magic_hex"`
	FileIDHex          string             `json:"file_id_hex"`
	SystemFlagsHex     string             `json:"system_flags_hex"`
	ProcessorFlagsHex  string             `json:"processor_flags_hex"`
	CryptexOffset      uint32             `json:"cryptex_offset"`
	CachePathOffset    uint32             `json:"cache_path_offset"`
	CryptexPath        *string            `json:"cryptex_path"`
	CachePath          *string            `json:"cache_path"`
	CryptexLength      *uint32            `json:"cryptex_length,omitempty"`
	CachePathLength    *uint32            `json:"cache_path_length,omitempty"`
	CryptexWindowHex   string             `json:"cryptex_window_hex,omitempty"`
	CachePathWindowHex string             `json:"cache_path_window_hex,omitempty"`
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
	if version == 4 {
		if p.Size < 112 {
			return fmt.Errorf("version-4 dynamic region too short")
		}
		for _, v := range []struct {
			field  uint64
			offset uint32
			length *uint32
			path   *string
		}{{88, p.CryptexOffset, p.CryptexLength, p.CryptexPath}, {92, p.CachePathOffset, p.CachePathLength, p.CachePath}} {
			if v.length == nil {
				return fmt.Errorf("missing version-4 path length")
			}
			if v.offset == 0 {
				if *v.length != 0 || v.path != nil {
					return fmt.Errorf("invalid null version-4 path")
				}
			} else if v.path == nil || uint64(*v.length) != uint64(len(*v.path))+1 || v.offset < 112 || uint64(v.offset)+uint64(*v.length) > p.Size {
				return fmt.Errorf("invalid version-4 path length/range")
			}
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], *v.length)
			if err := m.Map(base+v.field, 4, usermem.Read, b[:]); err != nil {
				return err
			}
		}
	}
	if p.Roots != nil {
		if version != 4 {
			return fmt.Errorf("roots bitmaps require version-4 dynamic region")
		}
		if err := p.Roots.mapRegion(m, base, p.Size); err != nil {
			return err
		}
	}
	mapPath := func(field uint64, offset uint32, path *string, windowHex string) error {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], offset)
		if err := m.Map(base+field, 4, usermem.Read, b[:]); err != nil {
			return err
		}
		if offset == 0 {
			if path != nil || windowHex != "" {
				return fmt.Errorf("cache path supplied for null offset")
			}
			return nil
		}
		if path == nil || len(*path) == 0 || len(*path) > 1024 || !strings.HasPrefix(*path, "/") || strings.IndexByte(*path, 0) >= 0 || uint64(offset) >= p.Size || uint64(len(*path)+1) > p.Size-uint64(offset) {
			return fmt.Errorf("invalid dynamic cache path")
		}
		data := append([]byte(*path), 0)
		if windowHex == "" {
			return m.Map(base+uint64(offset), uint64(len(data)), usermem.Read, data)
		}
		start := uint64(offset) &^ 15
		end := (uint64(offset) + uint64(len(data)) + 15) &^ 15
		window, err := hex.DecodeString(windowHex)
		if err != nil || end > p.Size || uint64(len(window)) != end-start || !bytes.Equal(window[uint64(offset)-start:uint64(offset)-start+uint64(len(data))], data) {
			return fmt.Errorf("invalid captured cache path window")
		}
		// The native strlen implementation reads 16-byte aligned vectors. Preserve
		// the captured envelope, including real padding; never manufacture zeroes.
		cursor := base + start
		limit := base + end
		for _, existing := range m.Mappings() {
			if existing.End <= cursor || existing.Start >= limit {
				continue
			}
			if cursor < existing.Start {
				n := existing.Start - cursor
				if err := m.Map(cursor, n, usermem.Read, window[cursor-base-start:cursor-base-start+n]); err != nil {
					return err
				}
				cursor = existing.Start
			}
			hi := min(existing.End, limit)
			got := make([]byte, hi-cursor)
			if err := m.Read(cursor, got); err != nil || !bytes.Equal(got, window[cursor-base-start:hi-base-start]) {
				return fmt.Errorf("path envelope disagrees with known cache field")
			}
			cursor = hi
		}
		if cursor < limit {
			return m.Map(cursor, limit-cursor, usermem.Read, window[cursor-base-start:])
		}
		return nil
	}
	if version >= 1 {
		if err = mapPath(32, p.CryptexOffset, p.CryptexPath, p.CryptexWindowHex); err != nil {
			return err
		}
	}
	if version >= 2 {
		if err = mapPath(36, p.CachePathOffset, p.CachePath, p.CachePathWindowHex); err != nil {
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

// fileIdentity reuses the native dynamic region's FileIdTuple and canonical
// cache path; it grants identity lookup only, never host file access.
func (p CacheDynamicProfile) fileIdentity() (FileIdentity, bool, error) {
	raw, err := hex.DecodeString(p.FileIDHex)
	if err != nil || len(raw) != 16 {
		return FileIdentity{}, false, fmt.Errorf("invalid cache file identity")
	}
	fsid, inode := binary.LittleEndian.Uint64(raw[:8]), binary.LittleEndian.Uint64(raw[8:])
	if fsid == 0 && inode == 0 {
		return FileIdentity{}, false, nil
	}
	if uint32(fsid) == 0 || inode == 0 || p.CachePath == nil || !path.IsAbs(*p.CachePath) || path.Clean(*p.CachePath) != *p.CachePath || strings.IndexByte(*p.CachePath, 0) >= 0 {
		return FileIdentity{}, false, fmt.Errorf("invalid cache identity path")
	}
	return FileIdentity{FSID: fsid, ObjectID: inode, Path: *p.CachePath}, true, nil
}
