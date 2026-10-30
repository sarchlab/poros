package emu

import "testing"

func TestUnalignedMemoryAccessSplitsWordBoundary(t *testing.T) {
	m := TrackedMemory{}
	m.record(63, 8, false)
	if len(m.Accesses) != 2 || m.Accesses[0].Addr != 63 || m.Accesses[0].Size != 1 || m.Accesses[1].Addr != 64 || m.Accesses[1].Size != 7 {
		t.Fatalf("lost crossing access: %+v", m.Accesses)
	}
}
