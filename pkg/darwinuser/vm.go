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
	if uint32(r.GPR[0]) != 0x103 {
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
	if flags&^uint32(0xff000001) != 0 {
		return false
	}

	if size == 0 || size > userAddressLimit-vmPage || prot&^7 != 0 {
		r.GPR[0] = 4
		return true
	}
	if prot&4 != 0 {
		return false
	} // Executable anonymous mappings need Darwin JIT policy.
	size = (size + vmPage - 1) &^ (vmPage - 1)
	if size > maxAnonymousBytes-k.anonymousBytes || k.anonymousMappings >= 4096 {
		r.GPR[0] = 3
		return true
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
			r.GPR[0] = 3
			return true
		}
	} else if address&mask != 0 {
		r.GPR[0] = 3
		return true
	}
	ranges := append([]addressSpan(nil), k.reserved...)
	for _, v := range m.Mappings() {
		ranges = append(ranges, addressSpan{v.Start, v.End})
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].lo < ranges[j].lo })
	for _, v := range ranges {
		if address > userAddressLimit-size {
			r.GPR[0] = 3
			return true
		}
		if address < v.hi && v.lo < address+size {
			if !anywhere {
				r.GPR[0] = 3
				return true
			}
			var ok bool
			address, ok = align(v.hi)
			if !ok {
				r.GPR[0] = 3
				return true
			}
		}
	}
	if address > userAddressLimit-size {
		r.GPR[0] = 3
		return true
	}
	if err := m.MapReader(address, size, machPermissions(prot), usermem.Read|usermem.Write|usermem.Execute, zeroMemory(size), vmPage); err != nil {
		r.GPR[0] = 3
		return true
	}
	k.anonymousBytes += size
	k.anonymousMappings++
	binary.LittleEndian.PutUint64(b[:], address)
	r.GPR[0] = 0
	if err := m.Write(pointer, b[:]); err != nil {
		r.GPR[0] = 10
	}
	return true
}
