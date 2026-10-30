package darwinuser

import (
	"encoding/binary"
	"errors"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"io"
	"sort"
)

type addressSpan struct{ lo, hi uint64 }

// Bound owned anonymous and private-file mappings independently of resident pages.
const maxOwnedMappingBytes uint64 = 8 << 30
const userAddressLimit uint64 = 1 << 47
const vmPage uint64 = 16384

type zeroMemory uint64

func (z zeroMemory) ReadAt(b []byte, offset int64) (int, error) {
	if offset < 0 || uint64(offset) > uint64(z) {
		return 0, io.EOF
	}
	n := min(uint64(len(b)), uint64(z)-uint64(offset))
	clear(b[:n])
	if n < uint64(len(b)) {
		return int(n), io.EOF
	}
	return int(n), nil
}

// mapAnonymous models the allocate/map kernel traps' copyin -> map -> copyout order. A failing
// copyout leaves the new mapping installed, just as the kernel trap does.
func (k *kernelState) mapAnonymous(r *emu.Registers, m *usermem.AddressSpace) bool {
	if !k.currentTaskRight(uint32(r.GPR[0])) {
		r.GPR[0] = 0x10000003
		return true
	}
	pointer, size, mask := r.GPR[1], r.GPR[2], r.GPR[3]
	flags, prot := uint32(r.GPR[4]), uint32(r.GPR[5])
	allocating := int32(r.GPR[16]) == -10
	if allocating {
		mask, flags, prot = 0, uint32(r.GPR[3]), 3
		if flags&0x4000 != 0 {
			return false // VM_FLAGS_OVERWRITE is supported for map, not allocate.
		}
	}
	// Allocation tags are accounting metadata. Other valid flag families require
	// actual semantics, so remain explicit unsupported interfaces.
	var b [8]byte
	if err := m.Read(pointer, b[:]); err != nil {
		r.GPR[0] = 10
		return true
	}
	hint := binary.LittleEndian.Uint64(b[:])
	// Copyout can partially modify a mixed-permission slot natively. Keep that
	// unmodeled path unsupported before allocation; a wholly readonly slot still
	// follows the native map-then-copyout-error order below.
	if m.Check(pointer, 8, usermem.Write, "VM address copyout") != nil && m.Check(pointer, 1, usermem.Write, "VM address copyout") == nil {
		return false
	}
	if allocating && size == 0 {
		if flags&^uint32(0xff000001) != 0 {
			return false
		}
		clear(b[:])
		r.GPR[0] = 0
		if m.Write(pointer, b[:]) != nil {
			r.GPR[0] = 10
		}
		return true
	}
	address, result, supported := k.allocateAnonymous(m, hint, size, mask, flags, prot, 7)
	if !supported {
		return false
	}
	r.GPR[0] = result
	if result == 0 {
		binary.LittleEndian.PutUint64(b[:], address)
		if m.Write(pointer, b[:]) != nil {
			r.GPR[0] = 10
		}
	}
	return true
}

// allocateAnonymous is shared by the fast Mach trap and the full MIG request.
// It takes actual VM policy, independent of either ABI's copyin/copyout framing.
func (k *kernelState) allocateAnonymous(m *usermem.AddressSpace, hint, size, mask uint64, flags, prot, maxProt uint32) (uint64, uint64, bool) {
	return k.allocateMemory(m, hint, size, mask, flags, prot, maxProt, nil)
}

// allocateMemory shares address selection and ownership accounting, not syscall ABI.
func (k *kernelState) allocateMemory(m *usermem.AddressSpace, hint, size, mask uint64, flags, prot, maxProt uint32, source io.ReaderAt) (uint64, uint64, bool) {
	if flags&^uint32(0xff004001) != 0 || (flags&0x4000 != 0 && source != nil) {
		return 0, 0, false
	}

	if size == 0 || size > userAddressLimit-vmPage || (prot|maxProt)&^7 != 0 {
		return 0, 4, true
	}
	prot &= maxProt // XNU clamps requested current permissions to the maximum.
	if prot&4 != 0 {
		return 0, 0, false
	} // Owned executable mappings need explicit Darwin signing/JIT policy.
	size = (size + vmPage - 1) &^ (vmPage - 1)
	alignmentMask := mask | (vmPage - 1)
	align := func(value uint64) (uint64, bool) {
		if alignmentMask > ^uint64(0)-value {
			return 0, false
		}
		result := (value + alignmentMask) &^ alignmentMask
		return result, result < userAddressLimit
	}
	anywhere := flags&1 != 0
	address := hint &^ (vmPage - 1)
	if anywhere {
		address = max(address, uint64(1)<<32)
	}
	if anywhere {
		var ok bool
		address, ok = align(address)
		if !ok {
			return 0, 3, true
		}
	} else if address&mask != 0 {
		return 0, 3, true
	}
	if address > userAddressLimit-size {
		return 0, 3, true
	}
	if !anywhere && flags&0x4000 != 0 {
		return k.overwriteAnonymous(m, address, size, prot, maxProt)
	}
	if k.ownedBytes > maxOwnedMappingBytes || size > maxOwnedMappingBytes-k.ownedBytes || k.ownedMappings >= 4096 {
		return 0, 3, true
	}
	ranges := append([]addressSpan(nil), k.reserved...)
	for _, v := range m.Mappings() {
		ranges = append(ranges, addressSpan{v.Start, v.End})
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].lo < ranges[j].lo })
	for _, v := range ranges {
		if address > userAddressLimit-size {
			return 0, 3, true
		}
		if address < v.hi && v.lo < address+size {
			if !anywhere {
				return 0, 3, true
			}
			var ok bool
			address, ok = align(v.hi)
			if !ok {
				return 0, 3, true
			}
		}
	}
	if address > userAddressLimit-size {
		return 0, 3, true
	}
	if source == nil {
		source = zeroMemory(size)
	}
	if err := m.MapReader(address, size, machPermissions(prot), machPermissions(maxProt), source, vmPage); err != nil {
		return 0, 3, true
	}
	k.ownedRanges = append(k.ownedRanges, addressSpan{address, address + size})
	k.ownedBytes += size
	k.ownedMappings++
	return address, 0, true
}

func (k *kernelState) overwriteAnonymous(m *usermem.AddressSpace, start, size uint64, prot, maxProt uint32) (uint64, uint64, bool) {
	end := start + size // already range-checked by allocateMemory
	for _, spans := range [][]addressSpan{k.protected, k.reserved} {
		for _, span := range spans {
			if start < span.hi && span.lo < end {
				return 0, 0, false
			}
		}
	}
	// Only existing owned private mappings may be replaced. Unknown mappings
	// (including kernel-maintained regions) cannot be erased by a generic flag.
	for _, mapping := range m.Mappings() {
		lo, hi := max(start, mapping.Start), min(end, mapping.End)
		if lo >= hi {
			continue
		}
		unowned := []addressSpan{{lo, hi}}
		for _, owned := range k.ownedRanges {
			unowned = subtractSpan(unowned, owned.lo, owned.hi)
		}
		if len(unowned) != 0 {
			return 0, 0, false
		}
	}
	ranges := append(subtractSpan(k.ownedRanges, start, end), addressSpan{start, end})
	total := uint64(0)
	for _, span := range ranges {
		if span.hi-span.lo > maxOwnedMappingBytes-total {
			return 0, 3, true
		}
		total += span.hi - span.lo
	}
	if len(ranges) > 4096 {
		return 0, 3, true
	}
	if err := m.OverwriteReader(start, size, machPermissions(prot), machPermissions(maxProt), zeroMemory(size), vmPage); err != nil {
		return 0, 3, true
	}
	k.ownedRanges, k.ownedBytes, k.ownedMappings = ranges, total, uint32(len(ranges))
	return start, 0, true
}

func subtractSpan(spans []addressSpan, start, end uint64) []addressSpan {
	out := make([]addressSpan, 0, len(spans)+1)
	for _, v := range spans {
		if v.hi <= start || v.lo >= end {
			out = append(out, v)
			continue
		}
		if v.lo < start {
			out = append(out, addressSpan{v.lo, start})
		}
		if v.hi > end {
			out = append(out, addressSpan{end, v.hi})
		}
	}
	return out
}

func (k *kernelState) deallocate(r *emu.Registers, m *usermem.AddressSpace) bool {
	if !k.currentTaskRight(uint32(r.GPR[0])) {
		r.GPR[0] = 0x10000003
		return true
	}
	start, size := r.GPR[1], r.GPR[2]
	if size == 0 {
		r.GPR[0] = 0
		return true
	}
	if size > ^uint64(0)-start || start+size > ^uint64(0)-(vmPage-1) {
		r.GPR[0] = 4
		return true
	}
	end := (start + size + vmPage - 1) &^ (vmPage - 1)
	start &= ^(vmPage - 1)
	if !k.unmapRange(m, start, end) {
		return false
	}
	r.GPR[0] = 0
	return true
}

// unmapRange applies a prevalidated half-open range to mappings and reservations.
func (k *kernelState) unmapRange(m *usermem.AddressSpace, start, end uint64) bool {
	for _, v := range k.protected {
		if start < v.hi && v.lo < end {
			return false
		}
	}
	if err := m.Unmap(start, end-start); err != nil {
		return false
	}
	k.reserved = subtractSpan(k.reserved, start, end)
	k.ownedRanges = subtractSpan(k.ownedRanges, start, end)
	k.ownedBytes = 0
	for _, v := range k.ownedRanges {
		k.ownedBytes += v.hi - v.lo
	}
	k.ownedMappings = uint32(len(k.ownedRanges))
	return true
}
func (k *kernelState) munmap(r *emu.Registers, m *usermem.AddressSpace) bool {
	start, size := r.GPR[0], r.GPR[1]
	if start%vmPage != 0 || size == 0 || size > ^uint64(0)-start || start+size > ^uint64(0)-(vmPage-1) {
		r.GPR[0] = 22
		r.GPR[1] = 0
		r.C = true
		return true
	}
	end := (start + size + vmPage - 1) &^ (vmPage - 1)
	if !k.unmapRange(m, start, end) {
		return false
	}
	r.GPR[0], r.GPR[1] = 0, 0
	r.C = false
	return true
}

// mprotect models BSD page rounding and write-implies-read separately from the
// Mach protect trap. Execute transitions and extra protection flags need an
// explicit signing/guarded-page policy and remain unsupported.
func (k *kernelState) mprotect(r *emu.Registers, m *usermem.AddressSpace) bool {
	address, size, prot := r.GPR[0], r.GPR[1], uint32(r.GPR[2])
	finish := func(errno uint64) bool { r.GPR[0] = errno; r.GPR[1] = 0; r.C = errno != 0; return true }
	if address%vmPage != 0 {
		return finish(22)
	}
	if size == 0 {
		return finish(0)
	}
	if size > ^uint64(0)-address || address+size > ^uint64(0)-(vmPage-1) {
		return finish(22)
	}
	if prot&^uint32(3) != 0 {
		return false
	}
	if prot&2 != 0 {
		prot |= 1
	}
	size = (size + vmPage - 1) &^ (vmPage - 1)
	if k.overlapsReclaim(address, address+size) {
		return false
	}
	e := m.Protect(address, size, machPermissions(prot), false)
	if errors.Is(e, usermem.ErrProtection) {
		return finish(13)
	}
	if e != nil {
		return finish(12)
	}
	return finish(0)
}
