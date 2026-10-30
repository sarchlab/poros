package darwinuser

import (
	"encoding/binary"
	"errors"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// KernelState is a private, single-thread process namespace. Port names are
// guest values, never host Mach rights. Reply ports receive distinct names.
type kernelState struct {
	nextPort  uint32
	csrConfig *uint32
}

func (k *kernelState) trap(r *emu.Registers, m *usermem.AddressSpace) bool {
	number := int32(r.GPR[16]) // XNU decodes the low signed 32 bits of x16.
	if uint32(number) == 0x80000000 {
		switch uint32(r.GPR[3]) {
		case 2: // machine-dependent set_cthread_self; x0 and NZCV are preserved.
			r.ThreadPointer = emu.ThreadPointerState{Enabled: true, Value: r.GPR[0]}
		case 3:
			if !r.ThreadPointer.Enabled {
				return false
			}
			r.GPR[0] = r.ThreadPointer.Value
		default:
			return false
		}
		return true
	}
	switch number {
	case -14: // mach_vm_protect, on this process's task map only.
		if uint32(r.GPR[0]) != 0x103 {
			r.GPR[0] = 4
			break
		}
		addr, size, prot := r.GPR[1], r.GPR[2], r.GPR[4]
		if prot&^7 != 0 || size > ^uint64(0)-addr || addr+size > ^uint64(0)-0x3fff {
			r.GPR[0] = 4
			break
		}
		end := (addr + size + 0x3fff) &^ uint64(0x3fff)
		addr &^= 0x3fff
		if size == 0 {
			r.GPR[0] = 0
			break
		}
		err := m.Protect(addr, end-addr, machPermissions(uint32(prot)), r.GPR[3] != 0)
		r.GPR[0] = 0
		if err != nil {
			r.GPR[0] = 1
			if errors.Is(err, usermem.ErrProtection) {
				r.GPR[0] = 2
			}
		}
	case -28:
		r.GPR[0] = 0x103 // task_self_trap, send right
	case -29:
		r.GPR[0] = 0x107 // host_self_trap, send right
	case -27:
		r.GPR[0] = 0x10b // thread_self_trap, send right
	case -26: // mach_reply_port, a new receive right
		if k.nextPort == 0 {
			k.nextPort = 0x10f
		}
		if k.nextPort >= 0x10000 {
			r.GPR[0] = 0
		} else {
			r.GPR[0] = uint64(k.nextPort)
			k.nextPort += 4
		}
	case 483:
		if k.csrConfig == nil {
			return false
		}
		op, addr, size := r.GPR[0], r.GPR[1], r.GPR[2]
		errno := uint64(0)
		var b [4]byte
		switch {
		case op > 1:
			errno = 78 // ENOSYS
		case addr == 0 || size != 4:
			errno = 22
		case op == 1:
			binary.LittleEndian.PutUint32(b[:], *k.csrConfig)
			if err := m.Write(addr, b[:]); err != nil {
				errno = 14
			}
		default:
			if err := m.Read(addr, b[:]); err != nil {
				errno = 14
			} else {
				allowed := *k.csrConfig
				if allowed&(1|16) != 0 {
					allowed |= 8
				}
				if binary.LittleEndian.Uint32(b[:])&^allowed != 0 {
					errno = 1
				}
			}
		}
		r.GPR[0] = errno
		r.C = errno != 0
	case 38: // crossarch namespace query; this process is native ARM64, not Rosetta.
		r.C = true
		if uint32(r.GPR[0]) == 0 {
			r.GPR[0] = 45
		} else {
			r.GPR[0] = 22
		} // ENOTSUP / EINVAL
	case 372: // thread_selfid, BSD return convention
		r.GPR[0] = 1
		r.C = false
	default:
		return false
	}
	return true
}

func machPermissions(prot uint32) usermem.Permission {
	var flags usermem.Permission
	if prot&1 != 0 {
		flags |= usermem.Read
	}
	if prot&2 != 0 {
		flags |= usermem.Write
	}
	if prot&4 != 0 {
		flags |= usermem.Execute
	}
	return flags
}
