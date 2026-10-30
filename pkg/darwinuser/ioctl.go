package darwinuser

import (
	"encoding/binary"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

const fioDType uint64 = 0x4004667a
const tiocGetA uint64 = 0x40487413

// ioctl dispatches only observed command contracts to private descriptor kinds.
// It never forwards a command or descriptor to the host kernel.
func (k *kernelState) ioctl(r *emu.Registers, m *usermem.AddressSpace) bool {
	switch r.GPR[1] { // ioctl commands are full-width; high bits are not truncated.
	case fioDType, tiocGetA:
		return k.ioctlDescriptorQuery(r, m)
	case dtraceAddDOF, dtraceRemoveDOF, 0x200068ff:
		return k.ioctlDTrace(r, m)
	default:
		return false
	}
}

func (k *kernelState) ioctlDescriptorQuery(r *emu.Registers, m *usermem.AddressSpace) bool {
	fd := uint32(r.GPR[0])
	if fd < 3 && !k.standardIO {
		return false
	}
	finish := func(errno uint64) bool { r.GPR[0] = errno; r.GPR[1] = 0; r.C = errno != 0; return true }
	node, ok := k.descriptors[fd]
	if !ok {
		return finish(9)
	}
	// VREG/VDIR and POSIX shared memory reject these queries before copyout.
	// The result is not a generic descriptor-class number.
	if node.sharedMemory != nil {
		return finish(25)
	}
	if node.file != nil && node.file.stat.Mode&0xf000 == 0x8000 {
		return finish(25)
	}
	if node.path != "" && node.file == nil && !node.dtrace {
		return finish(25)
	}
	if node.file == nil || !node.file.nullDevice {
		return false
	}
	if r.GPR[1] == tiocGetA {
		return finish(19)
	} // /dev/null has no terminal device.
	address := r.GPR[2]
	if m.Check(address, 4, usermem.Write, "FIODTYPE copyout") != nil && m.Check(address, 1, usermem.Write, "FIODTYPE copyout") == nil {
		return false
	}
	var output [4]byte
	binary.LittleEndian.PutUint32(output[:], 2) // Native /dev/null device d_type.
	if m.Write(address, output[:]) != nil {
		return finish(14)
	}
	return finish(0)
}
