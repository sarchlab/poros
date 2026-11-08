package loader

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Bounds define the initial static Linux ELF profile, not host resource limits.
const (
	ELFPageSize          uint64 = 4096
	ELFAddressLimit      uint64 = 1 << 48
	MaxELFFileBytes             = 16 << 20
	MaxELFImageBytes     uint64 = 256 << 20
	MaxELFImageLimit     uint64 = 512 << 20
	MaxELFProgramHeaders        = 128
)

// ELFError classifies an invalid image independently of its diagnostic message.
type ELFError struct{ Kind, Detail string }

func (e *ELFError) Error() string { return "ELF " + e.Kind + ": " + e.Detail }
func elfError(kind, format string, args ...any) error {
	return &ELFError{kind, fmt.Sprintf(format, args...)}
}

// ELFSegment owns page-rounded mapping bytes, including the file-backed page
// prefix and zero BSS. Data starts at Address rounded down to ELFPageSize.
// Overlapping mapping pages are unsupported.
type ELFSegment struct {
	Address, MemorySize uint64
	Flags               elf.ProgFlag
	Data                []byte
}

// ELFImage is a validated static AArch64 executable snapshot.
// Callers own this value and its bytes; edits are not revalidated automatically.
type ELFImage struct {
	Entry, ProgramHeaderAddress uint64
	ProgramHeaderCount          uint16
	Segments                    []ELFSegment
}

// ELFOptions sets a bounded image allocation budget. Zero retains the 256 MiB default.
type ELFOptions struct{ MaxImageBytes uint64 }

// LoadELF reads a bounded file and validates program headers. Sections are unused.
func LoadELF(path string) (*ELFImage, error) {
	return LoadELFWithOptions(path, ELFOptions{})
}

// LoadELFWithOptions keeps file and address limits unchanged.
func LoadELFWithOptions(path string, options ELFOptions) (*ELFImage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxELFFileBytes+1))
	if err != nil {
		return nil, err
	}
	return ParseELFWithOptions(b, options)
}

// ParseELF accepts ELF64 LE AArch64 ET_EXEC with no dynamic runtime requirements.
// Validation completes before any segment bytes are copied into the result.
func ParseELF(b []byte) (*ELFImage, error) {
	return ParseELFWithOptions(b, ELFOptions{})
}

// ParseELFWithOptions validates the allocation budget before parsing or allocating.
func ParseELFWithOptions(b []byte, options ELFOptions) (*ELFImage, error) {
	limit := options.MaxImageBytes
	if limit == 0 {
		limit = MaxELFImageBytes
	}
	if limit < ELFPageSize || limit > MaxELFImageLimit || limit%ELFPageSize != 0 {
		return nil, elfError("limit", "image budget must be page-aligned and between 4 KiB and 512 MiB")
	}
	if len(b) > MaxELFFileBytes {
		return nil, elfError("limit", "file exceeds %d bytes", MaxELFFileBytes)
	}
	if len(b) < 64 {
		return nil, elfError("malformed", "truncated header")
	}
	if string(b[:4]) != "\x7fELF" {
		return nil, elfError("malformed", "bad magic")
	}
	if b[4] != 2 || b[5] != 1 || b[6] != 1 {
		return nil, elfError("unsupported", "requires ELF64 little-endian version 1")
	}
	if (b[7] != 0 && b[7] != 3) || b[8] != 0 {
		return nil, elfError("unsupported", "OS ABI/version")
	}
	u := binary.LittleEndian
	if u.Uint16(b[16:]) != uint16(elf.ET_EXEC) || u.Uint16(b[18:]) != uint16(elf.EM_AARCH64) {
		return nil, elfError("unsupported", "requires AArch64 ET_EXEC (no PIE)")
	}
	if u.Uint32(b[20:]) != 1 || u.Uint16(b[52:]) != 64 || u.Uint16(b[54:]) != 56 {
		return nil, elfError("malformed", "header version or size")
	}
	if u.Uint32(b[48:]) != 0 {
		return nil, elfError("unsupported", "nonzero architecture flags")
	}
	entry, phoff := u.Uint64(b[24:]), u.Uint64(b[32:])
	count := u.Uint16(b[56:])
	if count == 0 || count > MaxELFProgramHeaders {
		return nil, elfError("limit", "program header count %d", count)
	}
	tableSize := uint64(count) * 56
	if phoff < 64 || phoff > uint64(len(b)) || tableSize > uint64(len(b))-phoff {
		return nil, elfError("malformed", "program header table outside file")
	}
	image := &ELFImage{Entry: entry, ProgramHeaderCount: count}
	var total, previousEnd uint64
	var phdr *elf.ProgHeader
	var stackSeen bool
	var headerAddresses []uint64
	for i := uint16(0); i < count; i++ {
		h := b[phoff+uint64(i)*56:]
		p := elf.ProgHeader{Type: elf.ProgType(u.Uint32(h)), Flags: elf.ProgFlag(u.Uint32(h[4:])), Off: u.Uint64(h[8:]), Vaddr: u.Uint64(h[16:]), Filesz: u.Uint64(h[32:]), Memsz: u.Uint64(h[40:]), Align: u.Uint64(h[48:])}
		if p.Type != elf.PT_NULL && (p.Off > uint64(len(b)) || p.Filesz > uint64(len(b))-p.Off) {
			return nil, elfError("malformed", "program header %d file bounds", i)
		}
		switch p.Type {
		case elf.PT_INTERP, elf.PT_DYNAMIC, elf.PT_TLS, elf.PT_GNU_RELRO:
			return nil, elfError("unsupported", "program header %d requires %s", i, p.Type)
		case elf.PT_GNU_STACK:
			if stackSeen {
				return nil, elfError("malformed", "duplicate GNU_STACK")
			}
			stackSeen = true
			if p.Flags != elf.PF_R|elf.PF_W {
				return nil, elfError("unsupported", "stack must be read/write and non-executable")
			}
			continue
		case elf.PT_PHDR:
			if phdr != nil || len(image.Segments) > 0 {
				return nil, elfError("malformed", "PHDR must be unique and precede load segments")
			}
			header := p
			phdr = &header
			continue
		case elf.PT_NULL, elf.PT_NOTE:
			continue
		case elf.PT_LOAD:
		default:
			return nil, elfError("unsupported", "program header %d type %#x", i, uint32(p.Type))
		}
		if p.Filesz > p.Memsz || p.Off > uint64(len(b)) || p.Filesz > uint64(len(b))-p.Off {
			return nil, elfError("malformed", "segment %d file bounds", i)
		}
		if p.Align > 1 && (p.Align&(p.Align-1) != 0 || p.Vaddr%p.Align != p.Off%p.Align) {
			return nil, elfError("malformed", "segment %d alignment", i)
		}
		if p.Vaddr%ELFPageSize != p.Off%ELFPageSize {
			return nil, elfError("malformed", "segment %d page offset mismatch", i)
		}
		if p.Flags & ^(elf.PF_R|elf.PF_W|elf.PF_X) != 0 {
			return nil, elfError("unsupported", "segment %d permission flags", i)
		}
		if p.Memsz == 0 {
			continue
		}
		if p.Vaddr < ELFPageSize || p.Vaddr >= ELFAddressLimit || p.Memsz > ELFAddressLimit-p.Vaddr {
			return nil, elfError("malformed", "segment %d address range", i)
		}
		start := p.Vaddr & ^(ELFPageSize - 1)
		end := (p.Vaddr + p.Memsz + ELFPageSize - 1) &^ (ELFPageSize - 1)
		if start < previousEnd {
			return nil, elfError("unsupported", "segment %d has overlapping or unordered mapping pages", i)
		}
		previousEnd = end
		if end-start > limit-total {
			return nil, elfError("limit", "mapped image exceeds %d bytes", limit)
		}
		total += end - start
		// Copy the file-backed mapping, not just bytes from p_vaddr. Linux
		// maps the first file page including its prefix. The final file page
		// remains file-backed unless a BSS tail requires zeroing it.
		fileStart := p.Off & ^(ELFPageSize - 1)
		fileEnd := min(uint64(len(b)), fileStart+end-start)
		if p.Memsz > p.Filesz {
			fileEnd = p.Off + p.Filesz
		}
		if p.Filesz == 0 {
			fileEnd = fileStart
		} // Pure BSS maps anonymous zero pages, including the prefix.
		image.Segments = append(image.Segments, ELFSegment{Address: p.Vaddr, MemorySize: p.Memsz, Flags: p.Flags, Data: b[fileStart:fileEnd]})
		if phoff >= p.Off && phoff-p.Off <= p.Filesz && tableSize <= p.Filesz-(phoff-p.Off) && p.Flags&elf.PF_R != 0 {
			headerAddresses = append(headerAddresses, p.Vaddr+(phoff-p.Off))
		}
	}
	if len(image.Segments) == 0 {
		return nil, elfError("malformed", "no nonempty load segments")
	}
	if len(headerAddresses) == 0 {
		return nil, elfError("unsupported", "program header table must be file-backed in a readable load segment")
	}
	image.ProgramHeaderAddress = headerAddresses[0]
	if phdr != nil {
		found := false
		for _, addr := range headerAddresses {
			if addr == phdr.Vaddr {
				found = true
			}
		}
		if !found || phdr.Off != phoff || phdr.Filesz != tableSize || phdr.Memsz != tableSize {
			return nil, elfError("malformed", "PHDR does not describe mapped program header table")
		}
		image.ProgramHeaderAddress = phdr.Vaddr
	}
	executable := false
	for _, s := range image.Segments {
		if entry >= s.Address && entry-s.Address <= s.MemorySize && 4 <= s.MemorySize-(entry-s.Address) && s.Flags&elf.PF_X != 0 {
			executable = true
		}
	}
	if entry%4 != 0 || !executable {
		return nil, elfError("malformed", "entry must address a complete aligned executable instruction")
	}
	for i := range image.Segments {
		s := &image.Segments[i]
		start := s.Address & ^(ELFPageSize - 1)
		end := (s.Address + s.MemorySize + ELFPageSize - 1) &^ (ELFPageSize - 1)
		data := make([]byte, end-start)
		copy(data, s.Data)
		s.Data = data
	}
	return image, nil
}
