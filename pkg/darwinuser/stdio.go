package darwinuser

import (
	"fmt"

	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

// InitialDescriptor captures one of the launch's explicit standard descriptors.
// The supported launch uses /dev/null for stdin and distinct empty regular
// temporary files for stdout/stderr. Paths and host descriptors are not exposed.
type InitialDescriptor struct {
	FD    uint32   `json:"fd"`
	Flags uint32   `json:"flags"`
	Stat  FileStat `json:"stat"`
}

func validateInitialDescriptors(entries []InitialDescriptor) error {
	if len(entries) == 0 {
		return nil
	}
	if len(entries) != 3 {
		return fmt.Errorf("standard IO profile requires all three descriptors")
	}
	var seen [3]bool
	for _, entry := range entries {
		if entry.FD > 2 || seen[entry.FD] {
			return fmt.Errorf("invalid/duplicate initial descriptor")
		}
		seen[entry.FD] = true
		if err := entry.Stat.validate(); err != nil {
			return err
		}
		if entry.FD == 0 {
			if entry.Flags != 0 || entry.Stat.Mode&0xf000 != 0x2000 {
				return fmt.Errorf("stdin requires captured read-only null device")
			}
		} else {
			if entry.Flags != 2 || entry.Stat.Mode&0xf000 != 0x8000 || entry.Stat.Size != 0 || entry.Stat.Blocks != 0 {
				return fmt.Errorf("output requires captured empty read/write regular file")
			}
		}
	}
	for i := range entries {
		for j := 0; j < i; j++ {
			if entries[i].Stat.Device == entries[j].Stat.Device && entries[i].Stat.Inode == entries[j].Stat.Inode {
				return fmt.Errorf("initial descriptors must refer to distinct files")
			}
		}
	}
	return nil
}

// guestOpenFile is a private open-file description. Dup aliases share its
// metadata and, once IO occurs, its byte content and current offset.
type guestOpenFile struct {
	initialFD uint32
	stat      FileStat
	flags     uint32
	offset    uint64
	data      []byte
}

func (k *kernelState) fstat(r *emu.Registers, m *usermem.AddressSpace) bool {
	fd := uint32(r.GPR[0])
	if fd < 3 && !k.standardIO {
		return false
	}
	finish := func(errno uint64) bool { r.GPR[0] = errno; r.GPR[1] = 0; r.C = errno != 0; return true }
	node, ok := k.descriptors[fd]
	if !ok {
		return finish(9)
	}
	var stat *FileStat
	if node.file != nil {
		if len(node.file.data) != 0 {
			return false
		} // Post-write filesystem timestamps/blocks are not yet modeled.
		stat = &node.file.stat
	} else {
		for _, entry := range k.directories {
			if entry.CanonicalPath == node.path {
				stat = entry.Stat
				break
			}
		}
	}
	if stat == nil {
		return false
	}
	output := r.GPR[1]
	if m.Check(output, 144, usermem.Write, "fstat64 copyout") != nil && m.Check(output, 1, usermem.Write, "fstat64 copyout") == nil {
		return false
	}
	b := stat.bytes()
	if m.Write(output, b[:]) != nil {
		return finish(14)
	}
	return finish(0)
}

// write copies guest bytes into a private captured output file. It never writes
// the host descriptor used by the command's own JSON report.
func (k *kernelState) write(r *emu.Registers, m *usermem.AddressSpace) bool {
	fd := uint32(r.GPR[0])
	if fd < 3 && !k.standardIO {
		return false
	}
	finish := func(value uint64, err bool) bool { r.GPR[0] = value; r.GPR[1] = 0; r.C = err; return true }
	node, ok := k.descriptors[fd]
	if !ok {
		return finish(9, true)
	}
	if node.file == nil {
		return false
	}
	file := node.file
	if file.flags&3 == 0 {
		return finish(9, true)
	}
	count := r.GPR[2]
	if count == 0 {
		return finish(0, false)
	}
	limit := k.outputLimit
	if limit == 0 {
		limit = 1 << 20
	}
	used := 0
	for _, out := range k.outputFiles {
		if out != nil {
			used += len(out.data)
		}
	}
	if limit < 0 || used > limit || count > uint64(limit-used) || file.offset != uint64(len(file.data)) {
		return false
	}
	address := r.GPR[1]
	if m.Check(address, int(count), usermem.Read, "write input") != nil && m.Check(address, 1, usermem.Read, "write input") == nil {
		return false
	}
	data := make([]byte, int(count))
	if m.Read(address, data) != nil {
		return finish(14, true)
	}
	file.data = append(file.data, data...)
	file.offset += count
	return finish(count, false)
}
