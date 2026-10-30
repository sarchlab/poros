package darwinuser

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"io"
	"strings"
)

const maxSharedMemorySnapshotBytes = 64 << 10

// ReadOnlySharedMemory is a native launch snapshot, not a live connection to a
// host daemon. The collector requires unchanged bytes before/after measurement.
// Names belong to the POSIX shared-memory namespace, not filesystem paths.
type ReadOnlySharedMemory struct {
	Name     string   `json:"name"`
	Stat     FileStat `json:"stat"`
	SHA256   string   `json:"sha256"`
	Contents []byte   `json:"contents_base64"`
}

func validateSharedMemory(entries []ReadOnlySharedMemory) error {
	if len(entries) > 8 {
		return fmt.Errorf("too many captured shared-memory objects")
	}
	names := make(map[string]bool)
	total := 0
	for _, entry := range entries {
		if entry.Name == "" || len(entry.Name) > 31 || strings.IndexByte(entry.Name, 0) >= 0 || names[entry.Name] {
			return fmt.Errorf("invalid or duplicate shared-memory name")
		}
		names[entry.Name] = true
		if entry.Stat.Size <= 0 || entry.Stat.Size > maxSharedMemorySnapshotBytes || int64(len(entry.Contents)) != entry.Stat.Size {
			return fmt.Errorf("shared-memory snapshot size mismatch or exceeds bound")
		}
		total += len(entry.Contents)
		if total > maxSharedMemorySnapshotBytes {
			return fmt.Errorf("shared-memory snapshots exceed aggregate bound")
		}
		if err := entry.Stat.validate(); err != nil {
			return err
		}
		hash := sha256.Sum256(entry.Contents)
		if entry.SHA256 != hex.EncodeToString(hash[:]) {
			return fmt.Errorf("shared-memory snapshot SHA256 mismatch")
		}
	}
	return nil
}

func copySharedMemory(entries []ReadOnlySharedMemory) []ReadOnlySharedMemory {
	copied := append([]ReadOnlySharedMemory(nil), entries...)
	for i := range copied {
		copied[i].Contents = append([]byte(nil), copied[i].Contents...)
	}
	return copied
}

func (k *kernelState) openSharedMemory(r *emu.Registers, m *usermem.AddressSpace) bool {
	if uint32(r.GPR[1]) != 0 {
		return false
	} // Creation, truncation and writes are unmodeled.
	finish := func(value uint64, failed bool) bool { r.GPR[0] = value; r.GPR[1] = 0; r.C = failed; return true }
	var name [32]byte
	for i := range name {
		if r.GPR[0] > ^uint64(0)-uint64(i) || m.Read(r.GPR[0]+uint64(i), name[i:i+1]) != nil {
			return finish(14, true)
		}
		if name[i] != 0 {
			continue
		}
		for j := range k.sharedMemory {
			if k.sharedMemory[j].Name != string(name[:i]) {
				continue
			}
			fd, ok := k.allocateDescriptor(guestDescriptor{sharedMemory: &k.sharedMemory[j]})
			if !ok {
				return false
			}
			return finish(uint64(fd), false)
		}
		return false // Unknown objects do not imply native ENOENT.
	}
	return false // Overlong names need their own exact kernel error contract.
}

// sharedMemoryPages owns a fixed launch snapshot. Read-only aliases therefore
// share identical bytes without pretending to synchronize with a host daemon.
type sharedMemoryPages struct {
	snapshot     *ReadOnlySharedMemory
	offset, size uint64
}

func (p sharedMemoryPages) ReadAt(dst []byte, offset int64) (int, error) {
	if offset < 0 || uint64(offset) > p.size {
		return 0, io.EOF
	}
	n := min(uint64(len(dst)), p.size-uint64(offset))
	start := p.offset + uint64(offset)
	if start > uint64(len(p.snapshot.Contents)) || n > uint64(len(p.snapshot.Contents))-start {
		return 0, io.ErrUnexpectedEOF
	}
	copy(dst, p.snapshot.Contents[start:start+n])
	if n < uint64(len(dst)) {
		return int(n), io.EOF
	}
	return int(n), nil
}

func (k *kernelState) mmapSharedMemory(r *emu.Registers, m *usermem.AddressSpace) bool {
	hint, length, offset := r.GPR[0], r.GPR[1], r.GPR[5]
	prot, flags, fd := uint32(r.GPR[2]), uint32(r.GPR[3]), uint32(r.GPR[4])
	finish := func(value uint64, failed bool) bool { r.GPR[0] = value; r.GPR[1] = 0; r.C = failed; return true }
	if flags != 1 && flags != 0x40001 && flags != 2 && flags != 0x40002 {
		return false
	}
	if hint != 0 || (prot != 1 && prot != 3) {
		return false
	}
	if offset>>63 != 0 {
		return finish(22, true)
	}
	// Legacy zero length and unaligned-offset mappings have different pointer
	// and cleanup rules. They remain explicit unsupported forms here.
	if flags&0x40000 == 0 && (length == 0 || offset%vmPage != 0) {
		return false
	}
	if length == 0 || length > userAddressLimit-vmPage || offset>>63 != 0 || offset%vmPage != 0 {
		return finish(22, true)
	}
	descriptor, ok := k.descriptors[fd]
	if !ok {
		if fd < 3 && !k.standardIO {
			return false
		}
		return finish(9, true)
	}
	snapshot := descriptor.sharedMemory
	if snapshot == nil {
		return false
	}
	if flags&2 != 0 {
		return finish(22, true)
	} // Darwin POSIX shm supports shared mappings only.
	if prot&2 != 0 {
		return finish(1, true)
	} // A read-only descriptor cannot create a writable map.
	size := (length + vmPage - 1) &^ (vmPage - 1)
	if offset >= uint64(len(snapshot.Contents)) {
		return finish(22, true)
	}
	if size > uint64(len(snapshot.Contents))-offset {
		return false
	} // No invented final-page bytes.
	source := sharedMemoryPages{snapshot: snapshot, offset: offset, size: size}
	address, result, handled := k.allocateMemory(m, 0, size, 0, 1, 1, 1, source)
	if !handled {
		return false
	}
	if result != 0 {
		if result == 3 {
			return finish(12, true)
		}
		return false
	}
	return finish(address, false)
}
