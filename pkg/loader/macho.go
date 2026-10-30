package loader

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const (
	MachOPageSize      uint64 = 16384
	MaxMachOFileBytes         = 64 << 20
	MaxMachOImageBytes uint64 = 128 << 20
	machoCPUARM64             = 0x100000c
	MachOExecutable    uint32 = 2
	MachODylinker      uint32 = 7
)

// MachOError distinguishes malformed input, unsupported features and resource limits.
type MachOError struct{ Kind, Detail string }

func (e *MachOError) Error() string { return "Mach-O " + e.Kind + ": " + e.Detail }
func machoError(kind, format string, args ...any) error {
	return &MachOError{kind, fmt.Sprintf(format, args...)}
}

// MachOSegment preserves virtual and file extents. Data owns the file bytes;
// the process mapper supplies zero-filled BSS. Protections use Mach VM bits R=1,W=2,X=4.
type MachOSegment struct {
	Name                      string
	Address, Size, FileOffset uint64
	Protection, MaxProtection uint32
	Data                      []byte
}
type MachOLinkedit struct {
	Command      uint32
	Offset, Size uint32
}

// MachOImage describes the selected ARM64-family slice. Parsing does not bind
// imports, execute initializers, apply fixups, or verify an Apple code signature.
// MainEntry belongs to dyld; ThreadEntry is the kernel entry for MH_DYLINKER.
type MachOImage struct {
	FileSize                                         uint64 // complete supplied file, including all universal slices
	Type, Subtype, Flags                             uint32
	HeaderAddress, MainEntry, ThreadEntry, StackSize uint64
	AllImageInfoAddress, AllImageInfoSize            uint64
	Dylinker                                         string
	Libraries                                        []string
	Segments                                         []MachOSegment
	Linkedit                                         []MachOLinkedit
	UUID                                             string
	SHA256                                           string // entire supplied file, including all universal slices
}

func LoadMachO(path string, fileType uint32) (*MachOImage, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, MaxMachOFileBytes+1))
	if e != nil {
		return nil, e
	}
	return ParseMachO(b, fileType)
}

// ParseMachO selects generic ARM64 for an executable and arm64e (then ARM64)
// for dyld. CPU subtype capability/version bits are retained. Unknown required
// load commands fail; ordinary dynamic metadata is preserved for the runtime.
func ParseMachO(input []byte, fileType uint32) (*MachOImage, error) {
	if fileType != MachOExecutable && fileType != MachODylinker {
		return nil, machoError("unsupported", "file type %d", fileType)
	}
	if len(input) > MaxMachOFileBytes {
		return nil, machoError("limit", "file exceeds %d bytes", MaxMachOFileBytes)
	}
	b, e := machoSlice(input, fileType)
	if e != nil {
		return nil, e
	}
	if len(b) < 32 || binary.LittleEndian.Uint32(b) != 0xfeedfacf {
		return nil, machoError("malformed", "requires 64-bit little-endian Mach-O header")
	}
	u := binary.LittleEndian
	cpu, sub, typ := u.Uint32(b[4:]), u.Uint32(b[8:]), u.Uint32(b[12:])
	if cpu != machoCPUARM64 || (sub&0xffffff != 0 && sub&0xffffff != 2) || typ != fileType {
		return nil, machoError("unsupported", "architecture/subtype/type %#x/%#x/%d", cpu, sub, typ)
	}
	count, size := u.Uint32(b[16:]), u.Uint32(b[20:])
	if count == 0 || count > 1024 {
		return nil, machoError("limit", "load command count %d", count)
	}
	if uint64(size) > uint64(len(b)-32) {
		return nil, machoError("malformed", "load commands outside file")
	}
	hash := sha256.Sum256(input)
	img := &MachOImage{FileSize: uint64(len(input)), Type: typ, Subtype: sub, Flags: u.Uint32(b[24:]), SHA256: hex.EncodeToString(hash[:])}
	var mainSeen, threadSeen, headerSeen, dylinkerSeen, allImageInfoSeen bool
	var entryOffset, total uint64
	off, end := uint64(32), uint64(32)+uint64(size)
	within := func(a, n uint64) bool { return a <= uint64(len(b)) && n <= uint64(len(b))-a }
	for i := uint32(0); i < count; i++ {
		if end-off < 8 {
			return nil, machoError("malformed", "truncated command %d", i)
		}
		cmd, n := u.Uint32(b[off:]), uint64(u.Uint32(b[off+4:]))
		if n < 8 || n%8 != 0 || n > end-off {
			return nil, machoError("malformed", "command %d size", i)
		}
		c := b[off : off+n]
		switch cmd {
		case 0x19: // LC_SEGMENT_64
			if n < 72 || uint64(u.Uint32(c[64:])) > (n-72)/80 || n != 72+uint64(u.Uint32(c[64:]))*80 {
				return nil, machoError("malformed", "segment command/section count")
			}
			s := MachOSegment{Name: strings.TrimRight(string(c[8:24]), "\x00"), Address: u.Uint64(c[24:]), Size: u.Uint64(c[32:]), FileOffset: u.Uint64(c[40:]), MaxProtection: u.Uint32(c[56:]), Protection: u.Uint32(c[60:])}
			fs := u.Uint64(c[48:])
			if fs > s.Size || !within(s.FileOffset, fs) || s.Address >= 1<<48 || s.Size > (1<<48)-s.Address || s.Protection & ^uint32(7) != 0 || s.MaxProtection & ^uint32(7) != 0 || s.Protection & ^s.MaxProtection != 0 {
				return nil, machoError("malformed", "segment %q bounds/protection", s.Name)
			}
			if s.Address%MachOPageSize != 0 || s.Size%MachOPageSize != 0 || s.FileOffset%MachOPageSize != 0 {
				return nil, machoError("unsupported", "segment %q is not 16 KiB aligned", s.Name)
			}

			for section := uint64(72); section < n; section += 80 {
				h := c[section : section+80]
				address, length := u.Uint64(h[32:]), u.Uint64(h[40:])
				offset, align := uint64(u.Uint32(h[48:])), u.Uint32(h[52:])
				if address < s.Address || address-s.Address > s.Size || length > s.Size-(address-s.Address) || align > 63 {
					return nil, machoError("malformed", "section outside segment")
				}
				if (s.Name == "__DATA" || s.Name == "__DATA_DIRTY") && strings.TrimRight(string(h[:16]), "\x00") == "__all_image_info" {
					if allImageInfoSeen {
						return nil, machoError("malformed", "duplicate all-image-info section")
					}
					allImageInfoSeen = true
					img.AllImageInfoAddress = address
					img.AllImageInfoSize = length
				}
				sectionType := u.Uint32(h[64:]) & 0xff
				if sectionType != 1 && sectionType != 0xc && sectionType != 0x12 && length != 0 {
					if !within(offset, length) || offset < s.FileOffset || offset-s.FileOffset > fs || length > fs-(offset-s.FileOffset) {
						return nil, machoError("malformed", "section outside file-backed segment")
					}
				}
				if !within(uint64(u.Uint32(h[56:])), uint64(u.Uint32(h[60:]))*8) {
					return nil, machoError("malformed", "section relocations outside file")
				}
			}
			// __PAGEZERO is a reserved inaccessible range, not a multi-gigabyte allocation.
			if s.Protection != 0 {
				if s.Size > MaxMachOImageBytes-total {
					return nil, machoError("limit", "mapped image exceeds %d bytes", MaxMachOImageBytes)
				}
				total += s.Size
			} else if fs != 0 {
				return nil, machoError("unsupported", "file-backed inaccessible segment %q", s.Name)
			}
			if s.FileOffset == 0 && fs >= end && s.Protection&1 != 0 {
				if headerSeen {
					return nil, machoError("malformed", "multiple header mappings")
				}
				headerSeen = true
				img.HeaderAddress = s.Address
			}
			s.Data = append([]byte(nil), b[s.FileOffset:s.FileOffset+fs]...)
			img.Segments = append(img.Segments, s)
		case 0x80000028: // LC_MAIN
			if n != 24 || mainSeen {
				return nil, machoError("malformed", "duplicate/invalid LC_MAIN")
			}
			mainSeen = true
			entryOffset = u.Uint64(c[8:])
			img.StackSize = u.Uint64(c[16:])
		case 5: // LC_UNIXTHREAD: ARM_THREAD_STATE64, 68 32-bit words
			if n != 288 || threadSeen || u.Uint32(c[8:]) != 6 || u.Uint32(c[12:]) != 68 {
				return nil, machoError("unsupported", "LC_UNIXTHREAD requires one ARM_THREAD_STATE64")
			}
			threadSeen = true
			img.ThreadEntry = u.Uint64(c[272:])
		case 0xe, 0xf, 0xc, 0x80000018, 0x8000001f, 0x20, 0x80000023:
			minimum := uint64(24)
			if cmd == 0xe || cmd == 0xf {
				minimum = 12
			}
			if n < minimum {
				return nil, machoError("malformed", "short path command")
			}
			start := uint64(u.Uint32(c[8:]))
			if start < minimum || start >= n {
				return nil, machoError("malformed", "path offset")
			}
			tail := string(c[start:])
			z := strings.IndexByte(tail, 0)
			if z <= 0 {
				return nil, machoError("malformed", "empty/unterminated path")
			}
			name := tail[:z]
			if cmd == 0xe {
				if dylinkerSeen {
					return nil, machoError("malformed", "duplicate dylinker")
				}
				dylinkerSeen = true
				img.Dylinker = name
			} else if cmd != 0xf {
				img.Libraries = append(img.Libraries, name)
			}
		case 0x80000034, 0x80000033, 0x1d, 0x1e, 0x26, 0x29: // fixups, exports, signature, split info, function starts, data-in-code
			if n != 16 {
				return nil, machoError("malformed", "linkedit command size")
			}
			a, z := u.Uint32(c[8:]), u.Uint32(c[12:])
			if !within(uint64(a), uint64(z)) {
				return nil, machoError("malformed", "linkedit outside file")
			}
			img.Linkedit = append(img.Linkedit, MachOLinkedit{cmd, a, z})

		case 2: // LC_SYMTAB
			if n != 24 || !within(uint64(u.Uint32(c[8:])), uint64(u.Uint32(c[12:]))*16) || !within(uint64(u.Uint32(c[16:])), uint64(u.Uint32(c[20:]))) {
				return nil, machoError("malformed", "symbol/string table bounds")
			}
		case 0x21, 0x2c: // LC_ENCRYPTION_INFO[_64]
			expected := uint64(20)
			if cmd == 0x2c {
				expected = 24
			}
			if n != expected || !within(uint64(u.Uint32(c[8:])), uint64(u.Uint32(c[12:]))) {
				return nil, machoError("malformed", "encryption metadata")
			}
			if u.Uint32(c[16:]) != 0 {
				return nil, machoError("unsupported", "encrypted image")
			}
		case 0x1b:
			if n != 24 {
				return nil, machoError("malformed", "UUID command size")
			}
			img.UUID = hex.EncodeToString(c[8:24])
		case 0x80000022: // LC_DYLD_INFO_ONLY: rebase/bind/export extents
			if n != 48 {
				return nil, machoError("malformed", "dyld info size")
			}
			for j := 8; j < 48; j += 8 {
				if !within(uint64(u.Uint32(c[j:])), uint64(u.Uint32(c[j+4:]))) {
					return nil, machoError("malformed", "dyld info outside file")
				}
			}
		default:
			if cmd&0x80000000 != 0 {
				return nil, machoError("unsupported", "required load command %#x", cmd)
			}
		}
		off += n
	}
	if off != end {
		return nil, machoError("malformed", "load command size/count mismatch")
	}
	sort.Slice(img.Segments, func(i, j int) bool { return img.Segments[i].Address < img.Segments[j].Address })
	var prev uint64
	for _, s := range img.Segments {
		if s.Size == 0 {
			continue
		}
		if s.Address < prev {
			return nil, machoError("malformed", "overlapping segments")
		}
		prev = s.Address + s.Size
	}
	if !headerSeen {
		return nil, machoError("malformed", "no readable mapped header")
	}
	if typ == MachOExecutable {
		if !mainSeen || threadSeen || !dylinkerSeen {
			return nil, machoError("unsupported", "executable needs LC_MAIN and LC_LOAD_DYLINKER")
		}
		found := false
		for _, s := range img.Segments {
			if entryOffset >= s.FileOffset && entryOffset-s.FileOffset < uint64(len(s.Data)) && s.Protection&4 != 0 {
				img.MainEntry = s.Address + entryOffset - s.FileOffset
				found = true
				break
			}
		}
		if !found {
			return nil, machoError("malformed", "LC_MAIN outside executable file bytes")
		}
	} else if !threadSeen || mainSeen {
		return nil, machoError("unsupported", "dyld needs LC_UNIXTHREAD")
	}
	entry := img.MainEntry
	if typ == MachODylinker {
		entry = img.ThreadEntry
	}
	valid := false
	for _, s := range img.Segments {
		if entry >= s.Address && entry-s.Address < s.Size && s.Size-(entry-s.Address) >= 4 && s.Protection&4 != 0 {
			valid = true
		}
	}
	if !valid || entry%4 != 0 {
		return nil, machoError("malformed", "entry is not a complete executable instruction")
	}
	return img, nil
}

func machoSlice(b []byte, typ uint32) ([]byte, error) {
	if len(b) < 4 {
		return nil, machoError("malformed", "truncated magic")
	}
	magic := binary.BigEndian.Uint32(b)
	if magic != 0xcafebabe && magic != 0xcafebabf {
		return b, nil
	}
	if len(b) < 8 {
		return nil, machoError("malformed", "truncated universal header")
	}
	u := binary.BigEndian
	count := uint64(u.Uint32(b[4:]))
	width := uint64(20)
	if magic == 0xcafebabf {
		width = 32
	}
	if count == 0 || count > 32 || count > (uint64(len(b))-8)/width {
		return nil, machoError("malformed", "universal architecture table")
	}
	type extent struct{ a, z uint64 }
	var extents []extent
	var chosen []byte
	best := -1
	for i := uint64(0); i < count; i++ {
		c := b[8+i*width:]
		cpu, sub := u.Uint32(c), u.Uint32(c[4:])
		a, z := uint64(u.Uint32(c[8:])), uint64(u.Uint32(c[12:]))
		align := u.Uint32(c[16:])
		if width == 32 {
			a, z, align = u.Uint64(c[8:]), u.Uint64(c[16:]), u.Uint32(c[24:])
		}
		if z < 32 || align > 63 || a < 8+count*width || a > uint64(len(b)) || z > uint64(len(b))-a || a%(uint64(1)<<align) != 0 {
			return nil, machoError("malformed", "universal slice bounds/alignment")
		}
		for _, e := range extents {
			if a < e.a+e.z && e.a < a+z {
				return nil, machoError("malformed", "overlapping universal slices")
			}
		}
		extents = append(extents, extent{a, z})
		if cpu != machoCPUARM64 || (sub&0xffffff != 0 && sub&0xffffff != 2) {
			continue
		}
		if binary.LittleEndian.Uint32(b[a:]) != 0xfeedfacf || binary.LittleEndian.Uint32(b[a+4:]) != cpu || binary.LittleEndian.Uint32(b[a+8:]) != sub {
			return nil, machoError("malformed", "universal table/header architecture mismatch")
		}
		score := 0
		if (typ == MachODylinker && sub&0xffffff == 2) || (typ == MachOExecutable && sub&0xffffff == 0) {
			score = 1
		}
		if score > best {
			chosen = b[a : a+z]
			best = score
		}
	}
	if best < 0 {
		return nil, machoError("unsupported", "universal file has no supported ARM64 slice")
	}
	return chosen, nil
}
