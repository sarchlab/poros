package darwinuser

import (
	"fmt"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
	"io"
)

// filePages supplies immutable file bytes and the native zero tail of its final
// partial page. Whole pages beyond EOF are deliberately not admitted by mmap.
type filePages struct {
	source                 io.ReaderAt
	offset, size, fileSize uint64
}

func (f filePages) ReadAt(dst []byte, offset int64) (int, error) {
	if offset < 0 || uint64(offset) > f.size {
		return 0, io.EOF
	}
	n := min(uint64(len(dst)), f.size-uint64(offset))
	position := f.offset + uint64(offset)
	fileEnd := (f.fileSize + vmPage - 1) &^ (vmPage - 1)
	if position > fileEnd || n > fileEnd-position {
		return 0, fmt.Errorf("file mapping exceeds final file page")
	}
	actual := uint64(0)
	if position < f.fileSize {
		actual = min(n, f.fileSize-position)
	}
	if actual != 0 {
		got, err := f.source.ReadAt(dst[:actual], int64(position))
		if got != int(actual) || (err != nil && err != io.EOF) {
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			return got, fmt.Errorf("file mapping backing read: %w", err)
		}
	}
	clear(dst[actual:n])
	if n < uint64(len(dst)) {
		return int(n), io.EOF
	}
	return int(n), nil
}

func (k *kernelState) mmapFile(r *emu.Registers, m *usermem.AddressSpace) bool {
	hint, length, offset := r.GPR[0], r.GPR[1], r.GPR[5]
	prot, flags, fd := uint32(r.GPR[2]), uint32(r.GPR[3]), uint32(r.GPR[4])
	if flags == 1 || flags == 0x40001 {
		return k.mmapSharedMemory(r, m)
	}
	if descriptor, ok := k.descriptors[fd]; ok && descriptor.sharedMemory != nil {
		return k.mmapSharedMemory(r, m)
	}
	finish := func(value uint64, failed bool) bool { r.GPR[0] = value; r.GPR[1] = 0; r.C = failed; return true }
	if flags != 2 && flags != 0x40002 {
		if flags == 0x40000 {
			return finish(22, true)
		}
		return false
	}
	if prot != 1 && prot != 3 {
		return false
	} // executable/JIT file mappings need code-signing policy.
	if hint != 0 {
		return false
	} // Nonzero hint rounding awaits its own native contract.
	if flags == 2 && (offset%vmPage != 0 || offset>>63 != 0 || length == 0) {
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
	file := descriptor.file
	if file == nil || file.reader == nil || file.stat.Size < 0 {
		return false
	}
	if file.flags&3 == 1 {
		return false
	}
	size := (length + vmPage - 1) &^ (vmPage - 1)
	fileSize := uint64(file.stat.Size)
	fileEnd := (fileSize + vmPage - 1) &^ (vmPage - 1)
	if offset > fileEnd || size > fileEnd-offset {
		return false
	} // native whole-EOF-page access faults are not yet modeled.
	source := filePages{source: file.reader, offset: offset, size: size, fileSize: fileSize}
	address, result, handled := k.allocateMemory(m, hint, size, 0, 1, prot, 7, source)
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
