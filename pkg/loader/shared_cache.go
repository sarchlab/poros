package loader

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxCacheFiles            = 128
	maxCacheMappings         = 256
	maxCacheDiskBytes uint64 = 32 << 30
	maxCacheVMBytes   uint64 = 64 << 30
	cachePageSize     uint64 = 16384
)

// CacheFileIdentity binds each open runtime input, including every subcache.
type CacheFileIdentity struct {
	Name   string `json:"name"`
	Size   uint64 `json:"size"`
	UUID   string `json:"uuid"`
	SHA256 string `json:"sha256"`
}

// SharedCache is an explicitly owned read-only runtime handle. It must outlive
// all processes borrowing it. Files are hashed once and must remain unchanged;
// lazy reads are not an eager whole-file snapshot. Close is idempotent.
type SharedCache struct {
	Base, Size                 uint64
	DynamicOffset, DynamicSize uint64
	UUID                       string
	Files                      []CacheFileIdentity
	mappings                   []CacheMapping
	files                      []*cacheFile
	closed                     bool
}
type cacheFile struct {
	f     *os.File
	size  uint64
	mod   time.Time
	owner *SharedCache
}

func (f *cacheFile) ReadAt(b []byte, off int64) (int, error) {
	if f.owner.closed {
		return 0, fmt.Errorf("shared cache handle is closed")
	}
	stat, err := f.f.Stat()
	if err != nil {
		return 0, err
	}
	if uint64(stat.Size()) != f.size || !stat.ModTime().Equal(f.mod) {
		return 0, fmt.Errorf("shared cache input changed after validation")
	}
	return f.f.ReadAt(b, off)
}

// CacheMapping records validated file/VM bounds and Mach protections. Its
// reader rebases slide-v5 pointers at zero slide for disabled authentication.
// It never modifies source files, shares mutable pages, or skips guest dyld.
type CacheMapping struct {
	Address, Size             uint64
	Protection, MaxProtection uint32
	source                    *cacheFile
	offset                    uint64
	slide                     *cacheSlide5
}
type cacheSlide5 struct {
	valueAdd uint64
	starts   []uint16
}

func (c *SharedCache) Mappings() []CacheMapping { return append([]CacheMapping(nil), c.mappings...) }
func (c *SharedCache) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	var result error
	for _, f := range c.files {
		result = errors.Join(result, f.f.Close())
	}
	return result
}
func (c *SharedCache) IsOpen() bool { return c != nil && !c.closed && len(c.files) != 0 }
func cacheRead(source io.ReaderAt, size, offset, count uint64) ([]byte, error) {
	if count > 16<<20 || offset > size || count > size-offset {
		return nil, fmt.Errorf("cache metadata range out of bounds")
	}
	b := make([]byte, int(count))
	n, err := source.ReadAt(b, int64(offset))
	if n != len(b) || (err != nil && err != io.EOF) {
		return nil, fmt.Errorf("cache metadata read: %d/%d: %v", n, len(b), err)
	}
	return b, nil
}
func cacheArchitecture(header []byte) string {
	if string(header[:7]) != "dyld_v1" {
		return ""
	}
	return strings.Trim(string(header[8:16]), " \x00")
}
func validCacheSuffix(s string) bool {
	if len(s) < 2 || s[0] != '.' || strings.Contains(s, "..") {
		return false
	}
	for _, ch := range s {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '.' || ch == '_' || ch == '-') {
			return false
		}
	}
	return true
}

// OpenSharedCache validates the modern split ARM64 cache tables and all files.
// Unknown slide formats fail explicitly rather than exposing encoded pointers.
func OpenSharedCache(path string) (result *SharedCache, err error) {
	c := &SharedCache{}
	defer func() {
		if err != nil {
			c.Close()
		}
	}()
	root, header, err := c.openFile(path)
	if err != nil {
		return nil, err
	}
	u := binary.LittleEndian
	architecture := cacheArchitecture(header)
	c.Base = u.Uint64(header[224:])
	c.Size = u.Uint64(header[232:])
	c.UUID = hex.EncodeToString(header[88:104])
	c.DynamicOffset = u.Uint64(header[496:])
	c.DynamicSize = u.Uint64(header[504:])
	if c.Base == 0 || c.Size == 0 || c.Size > maxCacheVMBytes || c.Base > ^uint64(0)-c.Size {
		return nil, fmt.Errorf("invalid shared cache VM range")
	}
	if c.DynamicSize != 0 && (c.DynamicOffset > c.Size || c.DynamicSize > c.Size-c.DynamicOffset || c.DynamicSize > 1<<20 || c.DynamicOffset%cachePageSize != 0 || c.DynamicSize%cachePageSize != 0) {
		return nil, fmt.Errorf("invalid cache dynamic region")
	}
	if err = c.addMappings(root, header); err != nil {
		return nil, err
	}
	if c.mappings[0].Address != c.Base || c.mappings[0].offset != 0 || c.mappings[0].Protection&1 == 0 {
		return nil, fmt.Errorf("shared cache header is not mapped at region base")
	}
	table, count := uint64(u.Uint32(header[392:])), uint64(u.Uint32(header[396:]))
	if count >= maxCacheFiles {
		return nil, fmt.Errorf("too many shared cache files")
	}
	entries, err := cacheRead(root, root.size, table, count*56)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for i := uint64(0); i < count; i++ {
		entry := entries[i*56 : (i+1)*56]
		rawSuffix := entry[24:56]
		zero := strings.IndexByte(string(rawSuffix), 0)
		if zero < 0 {
			return nil, fmt.Errorf("unterminated subcache suffix")
		}
		suffix := string(rawSuffix[:zero])
		if !validCacheSuffix(suffix) || names[suffix] {
			return nil, fmt.Errorf("invalid/duplicate subcache suffix")
		}
		names[suffix] = true
		file, h, e := c.openFile(path + suffix)
		if e != nil {
			return nil, e
		}
		if cacheArchitecture(h) != architecture || string(h[88:104]) != string(entry[:16]) {
			return nil, fmt.Errorf("subcache architecture/UUID mismatch: %s", suffix)
		}
		first := len(c.mappings)
		if e = c.addMappings(file, h); e != nil {
			return nil, e
		}
		vmOffset := u.Uint64(entry[16:])
		if vmOffset >= c.Size || c.mappings[first].Address != c.Base+vmOffset {
			return nil, fmt.Errorf("subcache VM offset mismatch")
		}
	}
	for _, m := range c.mappings {
		if c.DynamicSize != 0 && m.Address < c.Base+c.DynamicOffset+c.DynamicSize && c.Base+c.DynamicOffset < m.Address+m.Size {
			return nil, fmt.Errorf("dynamic cache region overlaps file mapping")
		}
	}
	return c, nil
}
func (c *SharedCache) openFile(path string) (*cacheFile, []byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() < 512 || uint64(stat.Size()) > maxCacheDiskBytes {
		f.Close()
		return nil, nil, fmt.Errorf("invalid shared cache file size/type")
	}
	source := &cacheFile{f: f, size: uint64(stat.Size()), mod: stat.ModTime(), owner: c}
	c.files = append(c.files, source)
	total := source.size
	for _, id := range c.Files {
		total += id.Size
	}
	if total > maxCacheDiskBytes {
		return nil, nil, fmt.Errorf("shared cache disk-byte limit")
	}
	h, err := cacheRead(source, source.size, 0, 512)
	if err != nil {
		return nil, nil, err
	}
	arch := cacheArchitecture(h)
	if arch != "arm64" && arch != "arm64e" {
		return nil, nil, fmt.Errorf("unsupported shared cache architecture")
	}
	if binary.LittleEndian.Uint32(h[16:]) < 512 {
		return nil, nil, fmt.Errorf("unsupported old shared cache header")
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return nil, nil, err
	}
	after, err := f.Stat()
	if err != nil || after.Size() != stat.Size() || !after.ModTime().Equal(stat.ModTime()) {
		return nil, nil, fmt.Errorf("cache changed while hashing")
	}
	c.Files = append(c.Files, CacheFileIdentity{filepath.Base(path), source.size, hex.EncodeToString(h[88:104]), hex.EncodeToString(hash.Sum(nil))})
	return source, h, nil
}
func (c *SharedCache) addMappings(file *cacheFile, header []byte) error {
	u := binary.LittleEndian
	offset, count := uint64(u.Uint32(header[312:])), uint64(u.Uint32(header[316:]))
	if count == 0 || count > maxCacheMappings || count > 4096-uint64(len(c.mappings)) {
		return fmt.Errorf("invalid cache mapping count")
	}
	table, err := cacheRead(file, file.size, offset, count*56)
	if err != nil {
		return err
	}
	for i := uint64(0); i < count; i++ {
		b := table[i*56 : (i+1)*56]
		m := CacheMapping{Address: u.Uint64(b), Size: u.Uint64(b[8:]), offset: u.Uint64(b[16:]), MaxProtection: u.Uint32(b[48:]), Protection: u.Uint32(b[52:]), source: file}
		if m.Size == 0 || m.Size%cachePageSize != 0 || m.Address%cachePageSize != 0 || m.offset%cachePageSize != 0 || m.Address < c.Base || m.Address-c.Base > c.Size || m.Size > c.Size-(m.Address-c.Base) || m.offset > file.size || m.Size > file.size-m.offset || m.MaxProtection&^7 != 0 || m.Protection&^m.MaxProtection != 0 {
			return fmt.Errorf("invalid cache file/VM mapping")
		}
		for _, prior := range c.mappings {
			if m.Address < prior.Address+prior.Size && prior.Address < m.Address+m.Size {
				return fmt.Errorf("overlapping cache mappings")
			}
		}
		slideOffset, slideSize := u.Uint64(b[24:]), u.Uint64(b[32:])
		if slideSize != 0 {
			info, e := cacheRead(file, file.size, slideOffset, slideSize)
			if e != nil {
				return e
			}
			if len(info) < 24 || u.Uint32(info) != 5 || u.Uint32(info[4:]) != uint32(cachePageSize) {
				return fmt.Errorf("unsupported cache slide-info format (requires v5/16KiB)")
			}
			pages := uint64(u.Uint32(info[8:]))
			if pages != m.Size/cachePageSize || pages > (uint64(len(info))-24)/2 {
				return fmt.Errorf("invalid slide-info page table")
			}
			slide := &cacheSlide5{valueAdd: u.Uint64(info[16:]), starts: make([]uint16, int(pages))}
			for j := range slide.starts {
				start := u.Uint16(info[24+j*2:])
				if start != 0xffff && (uint64(start)+8 > cachePageSize || start%8 != 0) {
					return fmt.Errorf("invalid slide-info page start")
				}
				slide.starts[j] = start
			}
			m.slide = slide
		}
		c.mappings = append(c.mappings, m)
	}
	return nil
}

// ReadAt returns private rebased bytes. This mapping is only valid when address
// authentication is disabled; callers must reject enabled arm64e processes.
func (m CacheMapping) ReadAt(dst []byte, offset int64) (int, error) {
	if m.source == nil || m.offset > m.source.size || m.Size > m.source.size-m.offset || offset < 0 || uint64(offset) > m.Size {
		return 0, fmt.Errorf("cache mapping offset out of bounds")
	}
	n := min(uint64(len(dst)), m.Size-uint64(offset))
	done := uint64(0)
	for done < n {
		position := uint64(offset) + done
		page := position / cachePageSize
		within := position % cachePageSize
		var data [16384]byte
		got, err := m.source.ReadAt(data[:], int64(m.offset+page*cachePageSize))
		if got != len(data) || (err != nil && err != io.EOF) {
			return int(done), fmt.Errorf("cache backing page read: %d/%d: %v", got, len(data), err)
		}
		if m.slide != nil {
			if err = m.rebasePage(page, data[:]); err != nil {
				return int(done), err
			}
		}
		count := min(n-done, cachePageSize-within)
		copy(dst[done:done+count], data[within:within+count])
		done += count
	}
	if n < uint64(len(dst)) {
		return int(n), io.EOF
	}
	return int(n), nil
}
func (m CacheMapping) rebasePage(page uint64, data []byte) error {
	if page >= uint64(len(m.slide.starts)) {
		return fmt.Errorf("slide-info page index out of bounds")
	}
	start := m.slide.starts[page]
	if start == 0xffff {
		return nil
	}
	offset := uint64(start)
	u := binary.LittleEndian
	for {
		if offset+8 > cachePageSize {
			return fmt.Errorf("cache pointer chain escapes page")
		}
		encoded := u.Uint64(data[offset:])
		next := (encoded >> 52) & 0x7ff
		target := encoded & ((1 << 34) - 1)
		if target > ^uint64(0)-m.slide.valueAdd {
			return fmt.Errorf("cache pointer rebase overflows")
		}
		value := target + m.slide.valueAdd
		if encoded>>63 == 0 {
			if encoded&(((1<<10)-1)<<42) != 0 {
				return fmt.Errorf("reserved cache pointer bits")
			}
			value |= ((encoded >> 34) & 255) << 56
		}
		u.PutUint64(data[offset:], value)
		if next == 0 {
			return nil
		}
		offset += next * 8
	}
}
