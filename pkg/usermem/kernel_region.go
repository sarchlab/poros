package usermem

import "fmt"

// ReadOnlyRegion permits the owning virtual kernel to replace one complete
// read-only mapping. It grants no guest write/execute permission and exposes no
// backing slices. Like AddressSpace, it is used only between sequential steps.
type ReadOnlyRegion struct {
	space       *AddressSpace
	start, size uint64
	backing     *backing
}

// MapKernelReadOnly copies initial bytes and returns an update capability scoped
// to that backing and address range. Removing/replacing any part revokes updates.
func (m *AddressSpace) MapKernelReadOnly(start uint64, initial []byte) (*ReadOnlyRegion, error) {
	size := uint64(len(initial))
	if err := m.validateMap(start, size, Read, Read); err != nil {
		return nil, err
	}
	b := &backing{data: append([]byte(nil), initial...), size: size}
	m.install(mappedRegion{Mapping{start, start + size, Read}, Read, b, 0})
	return &ReadOnlyRegion{m, start, size, b}, nil
}

// Replace validates the complete original mapping before changing any byte.
// Protection splits retain ownership; unmap/remap, holes and size changes do not.
func (r *ReadOnlyRegion) Replace(data []byte) error {
	if r == nil || r.space == nil || r.backing == nil || uint64(len(data)) != r.size {
		return fmt.Errorf("invalid kernel region update")
	}
	cursor, end := r.start, r.start+r.size
	for i := r.space.first(cursor); i < len(r.space.regions) && cursor < end; i++ {
		mapping := r.space.regions[i]
		if mapping.Start > cursor || mapping.backing != r.backing || mapping.offset+cursor-mapping.Start != cursor-r.start {
			return fmt.Errorf("kernel region backing was replaced")
		}
		cursor = min(end, mapping.End)
	}
	if cursor != end {
		return fmt.Errorf("kernel region was unmapped")
	}
	copy(r.backing.data, data)
	return nil
}
