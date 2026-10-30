package darwinuser

import (
	"github.com/sarchlab/poros/pkg/loader"
	"io"
)

// imageSegment is an immutable original-file view with explicit BSS. AddressSpace
// owns all private writes; the type also preserves kernel file identity through
// protection splits and provides the whole-file offsets required by file traps.
type imageSegment struct {
	image   *loader.MachOImage
	segment loader.MachOSegment
}

func (s imageSegment) ReadAt(dst []byte, offset int64) (int, error) {
	if offset < 0 || uint64(offset) > s.segment.Size {
		return 0, io.EOF
	}
	n := min(uint64(len(dst)), s.segment.Size-uint64(offset))
	actual := uint64(0)
	if uint64(offset) < s.segment.FileSize {
		actual = min(n, s.segment.FileSize-uint64(offset))
	}
	if actual != 0 {
		base := s.image.SliceOffset() + s.segment.FileOffset
		if base > s.image.FileSize || uint64(offset) > s.image.FileSize-base || actual > s.image.FileSize-base-uint64(offset) {
			return 0, io.ErrUnexpectedEOF
		}
		got, e := s.image.ReadAt(dst[:actual], int64(base+uint64(offset)))
		if got != int(actual) || (e != nil && e != io.EOF) {
			return got, e
		}
	}
	clear(dst[actual:n])
	if n < uint64(len(dst)) {
		return int(n), io.EOF
	}
	return int(n), nil
}
