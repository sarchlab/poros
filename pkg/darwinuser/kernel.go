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
	directories                                []RuntimeDirectory
	descriptors                                map[uint32]guestDescriptor
	amfi                                       *AMFIPolicy
	files                                      []FileIdentity
	sysctlNames                                []SysctlName
	sandboxSyscalls                            []SandboxSyscallPermission
	reserved                                   []addressSpan
	protected                                  []addressSpan
	anonymous                                  []addressSpan
	anonymousBytes                             uint64
	anonymousMappings                          uint32
	dyldInfoAddress, dyldInfoSize              uint64
	dyldInfoFinal                              bool
	taskSendRefs, hostSendRefs, threadSendRefs uint32
	receivePorts                               map[uint32]receivePort
	nextPort                                   uint32
	entropyState                               uint64
	sharedCacheBase                            uint64
	csrConfig                                  *uint32
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
	case 5:
		return k.open(r, m)
	case 41:
		return k.duplicateDescriptor(r)
	case 92:
		return k.fcntl(r, m)
	case 470:
		return k.statAt(r, m)
	case 463:
		return k.openAt(r, m)
	case 6:
		return k.closeDescriptor(r)
	case 202:
		return k.sysctl(r, m)
	case 381:
		return k.macSyscall(r, m)
	case 427:
		return k.fsgetpath(r, m)
	case 73:
		return k.munmap(r, m)
	case -47:
		return k.message2(r, m)
	case -24:
		return k.constructPort(r, m)
	case -19:
		return k.modifyPortRefs(r)
	case -12:
		return k.deallocate(r, m)
	case -15:
		return k.mapAnonymous(r, m)
	case -14: // mach_vm_protect, on this process's task map only.
		if !k.currentTaskRight(uint32(r.GPR[0])) {
			r.GPR[0] = 0x10000003 // MACH_SEND_INVALID_DEST; kernelrpc rejects foreign task rights.
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
		if !copySelfRight(&k.taskSendRefs) {
			return false
		}
		r.GPR[0] = 0x103 // task_self_trap, a new send reference
	case -29:
		if !copySelfRight(&k.hostSendRefs) {
			return false
		}
		r.GPR[0] = 0x107 // host_self_trap, a new send reference
	case -27:
		if !copySelfRight(&k.threadSendRefs) {
			return false
		}
		r.GPR[0] = 0x10b // thread_self_trap, a new send reference
	case -26: // mach_reply_port, a new ordinary receive right
		r.GPR[0] = uint64(k.allocateReceivePort(receivePort{queueLimit: 5}))
	case 336: // proc_info: registration of this task's dyld image-info range.
		if uint32(r.GPR[0]) != 15 {
			return false
		}
		address, size := r.GPR[4], uint64(uint32(r.GPR[5]))
		errno := uint64(0)
		if int32(r.GPR[1]) != 100 || address == 0 || size > ^uint64(0)-address || k.dyldInfoFinal {
			errno = 22
		} else {
			k.dyldInfoFinal = k.dyldInfoAddress != 0 || k.dyldInfoSize != 0
			k.dyldInfoAddress, k.dyldInfoSize = address, size
		}
		r.GPR[0] = errno
		r.C = errno != 0
	case 20: // getpid in this private single-process namespace.
		r.GPR[0] = 100
		r.C = false
	case 294: // warm kernel shared region; dyld still performs discovery and binding.
		addr := r.GPR[0]
		if addr == 0 {
			return false
		} // null means detach/unmap; not modeled yet.
		if k.sharedCacheBase == 0 {
			return false
		} // no implicit host-cache discovery.
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], k.sharedCacheBase)
		r.GPR[0] = 0
		r.C = false
		if err := m.Write(addr, b[:]); err != nil {
			r.GPR[0] = 14
			r.C = true
		}
	case 500: // getentropy: deterministic guest kernel stream, not host randomness.
		addr, n := r.GPR[0], r.GPR[1]
		errno := uint64(0)
		if n > 256 {
			errno = 22
		} else if n != 0 {
			var b [256]byte
			state := k.entropyState
			for i := uint64(0); i < n; i += 8 {
				v := nextEntropyWord(&state)
				for j := uint64(0); j < 8 && i+j < n; j++ {
					b[i+j] = byte(v >> (8 * j))
				}
			}
			if err := m.Write(addr, b[:n]); err != nil {
				errno = 14
			} else {
				k.entropyState = state
			}
		}
		r.GPR[0] = errno
		r.C = errno != 0
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
	if number >= 0 {
		r.GPR[1] = 0
	} // Current BSD return types have no second result.
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
