// Package usermem provides checked memory shared by user-mode runtimes.
package usermem

import (
	"fmt"
	"sort"
)

// Permission is a format-independent access mask.
type Permission uint8

const (
	Execute Permission = 1
	Write   Permission = 2
	Read    Permission = 4
)

// Mapping describes a guest mapping. End is exclusive.
type Mapping struct {
	Start, End uint64
	Flags      Permission
}
type mappedRegion struct {
	Mapping
	data []byte
}

// AddressSpace owns mapped memory. Reads never allocate pages; access checks
// cover the entire request before copying, so failed writes are atomic.
type AddressSpace struct{ regions []mappedRegion }

// MemoryError reports a rejected request. Address is the first faulting byte
// for unmapped/permission faults, or the original start for address overflow.
// Size is the original requested length, not the remaining length at Address.
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
func (m *AddressSpace) Check(addr uint64, n int, flag Permission, access string) error {
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
	if e := m.Check(addr, len(dst), Read, "read"); e != nil {
		return e
	}
	m.transfer(addr, dst, false)
	return nil
}

// Fetch copies instruction bytes, enforcing execute rather than read permission.
func (m *AddressSpace) Fetch(addr uint64, dst []byte) error {
	if e := m.Check(addr, len(dst), Execute, "fetch"); e != nil {
		return e
	}
	m.transfer(addr, dst, false)
	return nil
}

// Write modifies guest bytes, enforcing write permission.
func (m *AddressSpace) Write(addr uint64, src []byte) error {
	if e := m.Check(addr, len(src), Write, "write"); e != nil {
		return e
	}
	m.transfer(addr, src, true)
	return nil
}

// Map installs an owned zero-filled region and copies its initial bytes. It
// rejects overlap before allocating. Resource limits belong to the process loader.
func (m *AddressSpace) Map(start, size uint64, flags Permission, initial []byte) error {
	if size == 0 || size > uint64(^uint(0)>>1) || start > ^uint64(0)-size || uint64(len(initial)) > size || flags & ^(Read|Write|Execute) != 0 {
		return fmt.Errorf("invalid guest mapping")
	}
	end := start + size
	for _, r := range m.regions {
		if start < r.End && r.Start < end {
			return fmt.Errorf("overlapping guest mapping")
		}
	}
	data := make([]byte, int(size))
	copy(data, initial)
	m.regions = append(m.regions, mappedRegion{Mapping{start, end, flags}, data})
	sort.Slice(m.regions, func(i, j int) bool { return m.regions[i].Start < m.regions[j].Start })
	return nil
}
