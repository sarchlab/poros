package darwinuser

import (
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
)

// Root is structural; descriptors never mount or expose the host filesystem.
type guestDescriptor struct {
	path  string
	flags uint32
}

const maxGuestDescriptors = 256

func (k *kernelState) open(r *emu.Registers, m *usermem.AddressSpace) bool {
	return k.openDirectory(r, m, r.GPR[0], uint32(r.GPR[1]), 0, false)
}
func (k *kernelState) openAt(r *emu.Registers, m *usermem.AddressSpace) bool {
	return k.openDirectory(r, m, r.GPR[1], uint32(r.GPR[2]), uint32(r.GPR[0]), true)
}

func (k *kernelState) openDirectory(r *emu.Registers, m *usermem.AddressSpace, address uint64, flags, dirfd uint32, relative bool) bool {
	if flags&^uint32(0x20100000) != 0 {
		return false
	}
	finish := func(value uint64, errno bool) bool { r.GPR[0] = value; r.GPR[1] = 0; r.C = errno; return true }
	name, errno, handled := k.directoryPath(m, address, dirfd, relative)
	if !handled {
		return false
	}
	if errno != 0 {
		return finish(errno, true)
	}
	canonical := ""
	symlink := false
	if name == "/" {
		canonical = "/"
	} else {
		for _, node := range k.directories {
			if name == node.Path || (flags&0x20000000 == 0 && name == node.Path+"/") {
				canonical = node.CanonicalPath
				symlink = node.Symlink
				break
			}
			if name == node.CanonicalPath || (flags&0x20000000 == 0 && name == node.CanonicalPath+"/") {
				canonical = node.CanonicalPath
				break
			}
		}
	}
	if canonical == "" {
		return false
	} // Unknown traversal is not an invented ENOENT.
	if symlink && flags&0x20000000 != 0 {
		if flags&0x100000 == 0 {
			return false
		}
		return finish(62, true)
	}
	fd, ok := k.allocateDescriptor(guestDescriptor{path: canonical, flags: flags})
	if !ok {
		return false
	}
	return finish(uint64(fd), false)
}

func (k *kernelState) allocateDescriptor(entry guestDescriptor) (uint32, bool) {
	if k.descriptors == nil {
		k.descriptors = make(map[uint32]guestDescriptor)
	}
	for fd := uint32(3); fd < maxGuestDescriptors; fd++ {
		if _, used := k.descriptors[fd]; !used {
			k.descriptors[fd] = entry
			return fd, true
		}
	}
	return 0, false // Resource bound, not an invented native RLIMIT_NOFILE errno.
}

func (k *kernelState) duplicateDescriptor(r *emu.Registers) bool {
	source := uint32(r.GPR[0])
	if source < 3 {
		return false
	}
	node, ok := k.descriptors[source]
	if !ok {
		r.GPR[0] = 9
		r.GPR[1] = 0
		r.C = true
		return true
	}
	fd, ok := k.allocateDescriptor(node)
	if !ok {
		return false
	}
	r.GPR[0] = uint64(fd)
	r.GPR[1] = 0
	r.C = false
	return true
}

func (k *kernelState) closeDescriptor(r *emu.Registers) bool {
	fd := uint32(r.GPR[0])
	if fd < 3 {
		return false
	} // Console lifetime is not modeled yet.
	_, ok := k.descriptors[fd]
	r.GPR[0] = 9
	r.GPR[1] = 0
	r.C = true
	if ok {
		delete(k.descriptors, fd)
		r.GPR[0] = 0
		r.C = false
	}
	return true
}

// directoryPath resolves only descriptor-relative concatenation and structural
// root dot entries. It deliberately does not clean arbitrary traversal paths.
func (k *kernelState) directoryPath(m *usermem.AddressSpace, address uint64, dirfd uint32, relative bool) (string, uint64, bool) {
	name := ""
	for i := uint64(0); i < 1024; i++ {
		var c [1]byte
		if address > ^uint64(0)-i || m.Read(address+i, c[:]) != nil {
			return "", 14, true
		}
		if c[0] == 0 {
			break
		}
		name += string(c[:])
		if i == 1023 {
			return "", 0, false
		}
	}
	if name == "" {
		return "", 2, true
	}
	if !strings.HasPrefix(name, "/") {
		if !relative {
			return "", 0, false
		} // No implicit host/current-directory namespace.
		if dirfd == 0xfffffffe {
			return "", 0, false
		} // Darwin AT_FDCWD; no implicit guest cwd.
		directory, ok := k.descriptors[dirfd]
		if !ok {
			if dirfd < 3 {
				return "", 0, false
			}
			return "", 9, true
		}
		if directory.path == "/" && (name == "." || name == "..") {
			name = "/"
		} else {
			name = strings.TrimSuffix(directory.path, "/") + "/" + name
		}
	}
	return name, 0, true
}

func (k *kernelState) fcntl(r *emu.Registers, m *usermem.AddressSpace) bool {
	if uint32(r.GPR[1]) != 50 {
		return false
	} // F_GETPATH only.
	fd := uint32(r.GPR[0])
	if fd < 3 {
		return false
	}
	finish := func(errno uint64) bool { r.GPR[0] = errno; r.GPR[1] = 0; r.C = errno != 0; return true }
	node, ok := k.descriptors[fd]
	if !ok {
		return finish(9)
	}
	value := append([]byte(node.path), 0)
	if m.Check(r.GPR[2], len(value), usermem.Write, "F_GETPATH copyout") != nil && m.Check(r.GPR[2], 1, usermem.Write, "F_GETPATH copyout") == nil {
		return false
	}
	if m.Write(r.GPR[2], value) != nil {
		return finish(14)
	}
	return finish(0)
}
