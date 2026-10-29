// Package linuxuser constructs checked Linux process state for static ELF images.
// It does not yet execute processes or provide system calls.
package linuxuser

import (
	"debug/elf"
	"fmt"
)

// Mapping describes a page-aligned guest mapping. End is exclusive.
type Mapping struct {
	Start, End uint64
	Flags      elf.ProgFlag
}
type mappedRegion struct {
	Mapping
	data []byte
}

// AddressSpace owns mapped memory. Reads never allocate pages; access checks
// cover the entire request before copying, so failed writes are atomic.
type AddressSpace struct{ regions []mappedRegion }

// MemoryError reports the address and access type of a rejected request.
type MemoryError struct {
	Address        uint64
	Size           int
	Access, Reason string
}

func (e *MemoryError) Error() string {
	return fmt.Sprintf("guest %s at %#x (%d bytes): %s", e.Access, e.Address, e.Size, e.Reason)
}

// Mappings returns metadata copies, never writable backing storage.
func (m *AddressSpace) Mappings() []Mapping {
	out := make([]Mapping, len(m.regions))
	for i, r := range m.regions {
		out[i] = r.Mapping
	}
	return out
}
func (m *AddressSpace) check(addr uint64, n int, flag elf.ProgFlag, access string) error {
	if n == 0 {
		return nil
	}
	if n < 0 || uint64(n) > ^uint64(0)-addr {
		return &MemoryError{addr, n, access, "address overflow"}
	}
	end := addr + uint64(n)
	cursor := addr
	for _, r := range m.regions {
		if r.End <= cursor {
			continue
		}
		if r.Start > cursor {
			break
		}
		if r.Flags&flag == 0 {
			return &MemoryError{cursor, n, access, "permission denied"}
		}
		if end <= r.End {
			return nil
		}
		cursor = r.End
	}
	return &MemoryError{cursor, n, access, "unmapped address"}
}
func (m *AddressSpace) transfer(addr uint64, b []byte, write bool) {
	end := addr + uint64(len(b))
	cursor := addr
	for _, r := range m.regions {
		if r.End <= cursor || r.Start > cursor {
			continue
		}
		count := min(end, r.End) - cursor
		if write {
			copy(r.data[cursor-r.Start:cursor-r.Start+count], b[cursor-addr:cursor-addr+count])
		} else {
			copy(b[cursor-addr:cursor-addr+count], r.data[cursor-r.Start:cursor-r.Start+count])
		}
		cursor += count
		if cursor == end {
			return
		}
	}
}

// Read copies guest bytes into dst, enforcing read permission.
func (m *AddressSpace) Read(addr uint64, dst []byte) error {
	if e := m.check(addr, len(dst), elf.PF_R, "read"); e != nil {
		return e
	}
	m.transfer(addr, dst, false)
	return nil
}

// Fetch copies instruction bytes, enforcing execute rather than read permission.
func (m *AddressSpace) Fetch(addr uint64, dst []byte) error {
	if e := m.check(addr, len(dst), elf.PF_X, "fetch"); e != nil {
		return e
	}
	m.transfer(addr, dst, false)
	return nil
}

// Write modifies guest bytes, enforcing write permission.
func (m *AddressSpace) Write(addr uint64, src []byte) error {
	if e := m.check(addr, len(src), elf.PF_W, "write"); e != nil {
		return e
	}
	m.transfer(addr, src, true)
	return nil
}
