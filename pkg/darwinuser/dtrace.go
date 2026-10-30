package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

const dtraceHelperPath = "/dev/dtracehelper"
const dtraceAddDOF = 0x80086804
const dtraceRemoveDOF = 0x20006802

// ioctlDTrace implements only the captured lazy helper contract. Registrations belong
// to this process, survive descriptor close, and never reach a host device.
// XNU dtrace_lazy_dofs_add stores helper metadata without reading a DOF header.
func (k *kernelState) ioctlDTrace(r *emu.Registers, m *usermem.AddressSpace) bool {
	fd, command, arg := uint32(r.GPR[0]), r.GPR[1], r.GPR[2]
	if command != dtraceAddDOF && command != dtraceRemoveDOF && command != 0x200068ff {
		return false
	}
	if fd < 3 && !k.standardIO {
		return false
	}
	finish := func(errno uint64) bool { r.GPR[0] = errno; r.GPR[1] = 0; r.C = errno != 0; return true }
	node, ok := k.descriptors[fd]
	if !ok {
		return finish(9)
	}
	if !node.dtrace || k.dtrace == nil {
		return false
	}
	if command == 0x200068ff {
		return finish(25)
	}
	if command == dtraceRemoveDOF {
		// The native _IO argument is an integer generation, not a user pointer.
		generation := uint64(int64(int32(arg)))
		if _, ok := k.dofs[generation]; !ok {
			return finish(22)
		}
		delete(k.dofs, generation)
		return finish(0)
	}
	var word [8]byte
	if m.Read(arg, word[:]) != nil {
		return finish(14)
	}
	address := binary.LittleEndian.Uint64(word[:])
	if m.Read(address, word[:]) != nil {
		return finish(14)
	}
	count := binary.LittleEndian.Uint64(word[:])
	if count == 0 || count > 1024 {
		return finish(22)
	}
	data := make([]byte, 8+80*int(count))
	if m.Read(address, data) != nil {
		return finish(14)
	}
	// Partial copyout can commit a prefix on native kernels; that unmodeled
	// boundary must stop before registrations or output bytes change.
	if m.Check(address, len(data), usermem.Write, "DTrace helper copyout") != nil && m.Check(address, 1, usermem.Write, "DTrace helper copyout") == nil {
		return false
	}
	if uint64(len(k.dofs))+count > 1024 {
		_ = m.Write(address, data) // XNU preserves the prior lazy-add error.
		return finish(22)
	}
	var last uint64
	for generation := range k.dofs {
		if generation > last {
			last = generation
		}
	}
	if last > uint64(^uint32(0)>>1)-count {
		return false
	}
	for i := 0; i < int(count); i++ {
		entry := data[8+i*80 : 8+(i+1)*80]
		if binary.LittleEndian.Uint64(entry[64:72]) != binary.LittleEndian.Uint64(entry[72:80]) {
			return false
		}
	}
	if k.dofs == nil {
		k.dofs = make(map[uint64][80]byte)
	}
	for i := 0; i < int(count); i++ {
		entry := data[8+i*80 : 8+(i+1)*80]
		generation := last + uint64(i) + 1
		var saved [80]byte
		copy(saved[:], entry)
		k.dofs[generation] = saved
		binary.LittleEndian.PutUint64(entry[72:80], generation)
	}
	// Native registrations remain installed if the final copyout fails.
	if m.Write(address, data) != nil {
		return finish(14)
	}
	return finish(0)
}
