package linuxuser

import (
	"debug/elf"
	"github.com/sarchlab/poros/pkg/usermem"
)

// Mapping retains the Linux public API and ELF permission representation.
type Mapping struct {
	Start, End uint64
	Flags      elf.ProgFlag
}
type MemoryError = usermem.MemoryError

// AddressSpace uses the same checked memory implementation as other user runtimes.
type AddressSpace struct{ space usermem.AddressSpace }

func (m *AddressSpace) Mappings() []Mapping {
	mappings := m.space.Mappings()
	out := make([]Mapping, len(mappings))
	for i, r := range mappings {
		out[i] = Mapping{r.Start, r.End, elf.ProgFlag(r.Flags)}
	}
	return out
}
func (m *AddressSpace) check(a uint64, n int, f elf.ProgFlag, access string) error {
	return m.space.Check(a, n, usermem.Permission(f), access)
}
func (m *AddressSpace) Read(a uint64, b []byte) error  { return m.space.Read(a, b) }
func (m *AddressSpace) Write(a uint64, b []byte) error { return m.space.Write(a, b) }
func (m *AddressSpace) Fetch(a uint64, b []byte) error { return m.space.Fetch(a, b) }
func (m *AddressSpace) mapRegion(start, size uint64, flags elf.ProgFlag, b []byte) error {
	return m.space.Map(start, size, usermem.Permission(flags), b)
}
