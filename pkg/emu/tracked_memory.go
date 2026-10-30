package emu

import "fmt"

// TrackedMemory records committed data accesses for CPU timing consumers.
type TrackedMemory struct {
	Space    CheckedMemory
	Accesses []MemAccess
}

func (m *TrackedMemory) Fetch(a uint64, b []byte) error { return m.Space.Fetch(a, b) }
func (m *TrackedMemory) Read(a uint64, b []byte) error {
	if err := m.Space.Read(a, b); err != nil {
		return err
	}
	m.record(a, len(b), false)
	return nil
}
func (m *TrackedMemory) Write(a uint64, b []byte) error {
	if err := m.Space.Write(a, b); err != nil {
		return err
	}
	m.record(a, len(b), true)
	return nil
}
func (m *TrackedMemory) record(a uint64, n int, w bool) {
	for n > 0 {
		// Split at natural word boundaries so an unaligned operation crossing
		// a cache line is represented on both sides.
		width := min(n, 8-int(a%8))
		m.Accesses = append(m.Accesses, MemAccess{Addr: a, Size: width, IsWrite: w})
		a += uint64(width)
		n -= width
	}
}

func (m *TrackedMemory) CompareExchange(a uint64, expected, replacement, old []byte) (bool, error) {
	memory, ok := m.Space.(AtomicMemory)
	if !ok {
		return false, fmt.Errorf("memory does not implement atomic RMW")
	}
	swapped, err := memory.CompareExchange(a, expected, replacement, old)
	if err != nil {
		return false, err
	}
	m.record(a, len(old), false)
	if swapped {
		m.record(a, len(old), true)
	}
	return swapped, nil
}

func (m *TrackedMemory) Exchange(a uint64, replacement, old []byte) error {
	memory, ok := m.Space.(AtomicMemory)
	if !ok {
		return fmt.Errorf("memory does not implement atomic RMW")
	}
	if err := memory.Exchange(a, replacement, old); err != nil {
		return err
	}
	m.record(a, len(old), false)
	m.record(a, len(old), true)
	return nil
}
