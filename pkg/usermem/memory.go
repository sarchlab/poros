// Package usermem provides checked memory shared by user-mode runtimes.
package usermem

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
)

var ErrProtection = errors.New("protection exceeds maximum")

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
type backing struct {
	data           []byte
	source         io.ReaderAt
	size, pageSize uint64
	pages          map[uint64][]byte
}
type mappedRegion struct {
	Mapping
	max     Permission
	backing *backing
	offset  uint64
}

// AddressSpace owns mutable guest bytes. Reader mappings materialize private
// pages lazily. Sources must stay open and unchanged for the address-space
// lifetime. Permission and backing-I/O failures leave writes atomic.
type AddressSpace struct {
	regions []mappedRegion
	// MaxLazyBytes bounds resident reader pages, independent of virtual extent.
	// Zero selects 256 MiB. Eager image/stack limits belong to their loader.
	MaxLazyBytes uint64
	lazyBytes    uint64
}

type MemoryError struct {
	Address        uint64
	Size           int
	Access, Reason string
}

func (e *MemoryError) Error() string {
	return fmt.Sprintf("guest %s at %#x (%d bytes): %s", e.Access, e.Address, e.Size, e.Reason)
}
func (m *AddressSpace) Mappings() []Mapping {
	out := make([]Mapping, len(m.regions))
	for i, r := range m.regions {
		out[i] = r.Mapping
	}
	return out
}
func (m *AddressSpace) first(addr uint64) int {
	return sort.Search(len(m.regions), func(i int) bool { return m.regions[i].End > addr })
}
func (m *AddressSpace) Check(addr uint64, n int, flag Permission, access string) error {
	if n == 0 {
		return nil
	}
	if n < 0 || uint64(n) > ^uint64(0)-addr {
		return &MemoryError{addr, n, access, "address overflow"}
	}
	end, cursor := addr+uint64(n), addr
	for i := m.first(addr); i < len(m.regions); i++ {
		r := m.regions[i]
		if r.Start > cursor {
			break
		}
		if flag != 0 && r.Flags&flag != flag {
			return &MemoryError{cursor, n, access, "permission denied"}
		}
		if end <= r.End {
			return nil
		}
		cursor = r.End
	}
	return &MemoryError{cursor, n, access, "unmapped address"}
}
func (m *AddressSpace) prepare(addr uint64, n int, access string) error {
	end := addr + uint64(n)
	for i := m.first(addr); i < len(m.regions) && m.regions[i].Start < end; i++ {
		r := &m.regions[i]
		b := r.backing
		if b.source == nil {
			continue
		}
		lo := r.offset + max(addr, r.Start) - r.Start
		hi := r.offset + min(end, r.End) - r.Start
		for page := lo / b.pageSize; page*b.pageSize < hi; page++ {
			if b.pages[page] != nil {
				continue
			}
			count := min(b.pageSize, b.size-page*b.pageSize)
			limit := m.MaxLazyBytes
			if limit == 0 {
				limit = 256 << 20
			}
			if m.lazyBytes > limit || count > limit-m.lazyBytes {
				return &MemoryError{addr, n, access, "resident reader-page limit"}
			}
			data := make([]byte, int(count))
			got, err := b.source.ReadAt(data, int64(page*b.pageSize))
			if got != len(data) || (err != nil && err != io.EOF) {
				return &MemoryError{addr, n, access, fmt.Sprintf("backing read: %d/%d bytes: %v", got, len(data), err)}
			}
			b.pages[page] = data
			m.lazyBytes += count
		}
	}
	return nil
}
func (m *AddressSpace) transfer(addr uint64, data []byte, write bool) {
	end, cursor := addr+uint64(len(data)), addr
	for i := m.first(addr); i < len(m.regions) && cursor < end; i++ {
		r := &m.regions[i]
		b := r.backing
		for cursor < min(end, r.End) {
			offset := r.offset + cursor - r.Start
			count := min(end, r.End) - cursor
			var span []byte
			if b.source == nil {
				span = b.data[offset : offset+count]
			} else {
				page := b.pages[offset/b.pageSize]
				within := offset % b.pageSize
				count = min(count, uint64(len(page))-within)
				span = page[within : within+count]
			}
			if write {
				copy(span, data[cursor-addr:cursor-addr+count])
			} else {
				copy(data[cursor-addr:cursor-addr+count], span)
			}
			cursor += count
		}
	}
}
func (m *AddressSpace) access(addr uint64, b []byte, flag Permission, name string, write bool) error {
	if err := m.Check(addr, len(b), flag, name); err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	if err := m.prepare(addr, len(b), name); err != nil {
		return err
	}
	m.transfer(addr, b, write)
	return nil
}
func (m *AddressSpace) Read(a uint64, b []byte) error { return m.access(a, b, Read, "read", false) }
func (m *AddressSpace) Fetch(a uint64, b []byte) error {
	return m.access(a, b, Execute, "fetch", false)
}
func (m *AddressSpace) Write(a uint64, b []byte) error { return m.access(a, b, Write, "write", true) }
func (m *AddressSpace) validateMap(start, size uint64, flags, maxFlags Permission) error {
	if size == 0 || size > uint64(^uint(0)>>1) || start > ^uint64(0)-size || maxFlags & ^(Read|Write|Execute) != 0 || flags & ^maxFlags != 0 {
		return fmt.Errorf("invalid guest mapping")
	}
	end := start + size
	for _, r := range m.regions {
		if start < r.End && r.Start < end {
			return fmt.Errorf("overlapping guest mapping")
		}
	}
	return nil
}
func (m *AddressSpace) install(r mappedRegion) {
	m.regions = append(m.regions, r)
	sort.Slice(m.regions, func(i, j int) bool { return m.regions[i].Start < m.regions[j].Start })
}

// Map installs owned bytes; its permissions are also the maximum protection.
func (m *AddressSpace) Map(start, size uint64, flags Permission, initial []byte) error {
	return m.MapProtected(start, size, flags, flags, initial)
}
func (m *AddressSpace) MapProtected(start, size uint64, flags, maxFlags Permission, initial []byte) error {
	if err := m.validateMap(start, size, flags, maxFlags); err != nil {
		return err
	}
	if uint64(len(initial)) > size {
		return fmt.Errorf("initial bytes exceed mapping")
	}
	data := make([]byte, int(size))
	copy(data, initial)
	m.install(mappedRegion{Mapping{start, start + size, flags}, maxFlags, &backing{data: data, size: size}, 0})
	return nil
}

// MapReader installs a bounded virtual view of a stable source. Reads, fetches,
// and writes use private resident pages; the source is never modified.
func (m *AddressSpace) MapReader(start, size uint64, flags, maxFlags Permission, source io.ReaderAt, pageSize uint64) error {
	if err := m.validateMap(start, size, flags, maxFlags); err != nil {
		return err
	}
	if source == nil || pageSize == 0 || pageSize > 1<<20 || pageSize&(pageSize-1) != 0 {
		return fmt.Errorf("invalid reader mapping")
	}
	m.install(mappedRegion{Mapping{start, start + size, flags}, maxFlags, &backing{source: source, size: size, pageSize: pageSize, pages: map[uint64][]byte{}}, 0})
	return nil
}

// Protect changes a fully mapped range atomically. The caller supplies its OS
// page rounding. setMaximum also reduces current protection to the new maximum.
func (m *AddressSpace) Protect(start, size uint64, flags Permission, setMaximum bool) error {
	if size == 0 {
		return nil
	}
	if size > uint64(^uint(0)>>1) || flags & ^(Read|Write|Execute) != 0 {
		return fmt.Errorf("invalid protection range")
	}
	if err := m.Check(start, int(size), 0, "protect"); err != nil {
		return err
	}
	end := start + size
	for _, r := range m.regions {
		if start < r.End && r.Start < end && flags & ^r.max != 0 {
			return ErrProtection
		}
	}
	out := make([]mappedRegion, 0, len(m.regions)+2)
	for _, r := range m.regions {
		if r.End <= start || r.Start >= end {
			out = append(out, r)
			continue
		}
		orig := r
		if r.Start < start {
			prefix := r
			prefix.End = start
			out = append(out, prefix)
			r.offset += start - r.Start
			r.Start = start
		}
		r.End = min(r.End, end)
		if setMaximum {
			r.max = flags
			r.Flags &= flags
		} else {
			r.Flags = flags
		}
		out = append(out, r)
		if orig.End > end {
			orig.offset += end - orig.Start
			orig.Start = end
			out = append(out, orig)
		}
	}
	m.regions = out
	return nil
}

// CompareExchange is indivisible in the address-space owner's execution stream.
// Both access permissions are checked even when the comparison fails.
func (m *AddressSpace) CompareExchange(addr uint64, expected, replacement, old []byte) (bool, error) {
	n := len(expected)
	if (n != 4 && n != 8) || len(replacement) != n || len(old) != n || addr%16+uint64(n) > 16 {
		return false, &MemoryError{addr, n, "atomic", "invalid width or alignment"}
	}
	if err := m.Check(addr, n, Read|Write, "atomic"); err != nil {
		return false, err
	}
	if err := m.prepare(addr, n, "atomic"); err != nil {
		return false, err
	}
	var compare, value, current [8]byte
	copy(compare[:], expected)
	copy(value[:], replacement)
	m.transfer(addr, current[:n], false)
	swapped := bytes.Equal(current[:n], compare[:n])
	if swapped {
		m.transfer(addr, value[:n], true)
	}
	copy(old, current[:n])
	return swapped, nil
}

func (m *AddressSpace) Exchange(addr uint64, replacement, old []byte) error {
	n := len(replacement)
	if (n != 4 && n != 8) || len(old) != n || addr%16+uint64(n) > 16 {
		return &MemoryError{addr, n, "atomic", "invalid width or alignment"}
	}
	if err := m.Check(addr, n, Read|Write, "atomic"); err != nil {
		return err
	}
	if err := m.prepare(addr, n, "atomic"); err != nil {
		return err
	}
	var value, current [8]byte
	copy(value[:], replacement)
	m.transfer(addr, current[:n], false)
	m.transfer(addr, value[:n], true)
	copy(old, current[:n])
	return nil
}

// WriteExclusive preserves the native permission check even for a failed local
// reservation. Only successful exclusive stores materialize/write data pages.
func (m *AddressSpace) WriteExclusive(address uint64,data []byte,enabled bool) error {
 if err:=m.Check(address,len(data),Write,"exclusive write");err!=nil{return err}
 if !enabled{return nil}
 return m.Write(address,data)
}
