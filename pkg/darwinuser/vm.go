package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"io"
	"sort"
)

type addressSpan struct{ lo, hi uint64 }

// Bound virtual anonymous reservations independently of the resident-page limit.
const maxAnonymousBytes uint64 = 8 << 30
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

// mapAnonymous models kernelrpc's copyin -> map -> copyout order. A failing
// copyout leaves the new mapping installed, just as the kernel trap does.
func (k *kernelState) mapAnonymous(r *emu.Registers, m *usermem.AddressSpace) bool {
	if !k.currentTaskRight(uint32(r.GPR[0])) {
		r.GPR[0] = 0x10000003
		return true
	}
	pointer, size, mask := r.GPR[1], r.GPR[2], r.GPR[3]
	flags, prot := uint32(r.GPR[4]), uint32(r.GPR[5])
	// Allocation tags are accounting metadata. Other valid flag families require
	// actual semantics, so remain explicit unsupported interfaces.
	var b [8]byte
	if err := m.Read(pointer, b[:]); err != nil {
		r.GPR[0] = 10
		return true
	}
	hint := binary.LittleEndian.Uint64(b[:])
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
	if flags&^uint32(0xff000001) != 0 {
		return 0, 0, false
	}

	if size == 0 || size > userAddressLimit-vmPage || (prot|maxProt)&^7 != 0 {
		return 0, 4, true
	}
	prot &= maxProt // XNU clamps requested current permissions to the maximum.
	if prot&4 != 0 {
		return 0, 0, false
	} // Executable anonymous mappings need Darwin JIT policy.
	size = (size + vmPage - 1) &^ (vmPage - 1)
	if size > maxAnonymousBytes-k.anonymousBytes || k.anonymousMappings >= 4096 {
		return 0, 3, true
	}
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
	if err := m.MapReader(address, size, machPermissions(prot), machPermissions(maxProt), zeroMemory(size), vmPage); err != nil {
		return 0, 3, true
	}
	k.anonymous = append(k.anonymous, addressSpan{address, address + size})
	k.anonymousBytes += size
	k.anonymousMappings++
	return address, 0, true
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
	k.anonymous = subtractSpan(k.anonymous, start, end)
	k.anonymousBytes = 0
	for _, v := range k.anonymous {
		k.anonymousBytes += v.hi - v.lo
	}
	k.anonymousMappings = uint32(len(k.anonymous))
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
