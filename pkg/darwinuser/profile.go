package darwinuser

import (
	"encoding/binary"
	"fmt"

	"github.com/sarchlab/poros/pkg/usermem"
)

// RuntimeProfile supplies selected nonsecret commpage fields captured on the
// native reference host. Unmodeled fields remain unmapped, not guessed as zero.
// Dynamic time offsets use a separate deterministic virtual epoch of zero.
type RuntimeProfile struct {
	CPUCapabilities   uint64 `json:"cpu_capabilities"`
	Version           uint16 `json:"version"`
	UserPageShift     uint8  `json:"user_page_shift_64_legacy"`
	CacheLineSize     uint16 `json:"cache_line_size"`
	CPUFamily         uint32 `json:"cpu_family"`
	UserTimebase      uint8  `json:"user_timebase"`
	ContinuousHWClock uint8  `json:"continuous_hwclock"`
	KDebug            uint32 `json:"kdebug"`
	TimebaseNumer     uint32 `json:"timebase_numer"`
	TimebaseDenom     uint32 `json:"timebase_denom"`
}

func (p RuntimeProfile) mapCommpage(m *usermem.AddressSpace) error {
	if p.Version != 3 || p.UserPageShift != 14 || p.CacheLineSize == 0 || p.CacheLineSize&(p.CacheLineSize-1) != 0 || p.TimebaseNumer == 0 || p.TimebaseDenom == 0 || p.UserTimebase > 3 || p.ContinuousHWClock > 1 {
		return fmt.Errorf("unsupported/invalid Darwin commpage profile")
	}
	if 1_000_000_000*uint64(p.TimebaseDenom)%uint64(p.TimebaseNumer) != 0 {
		return fmt.Errorf("non-integral virtual counter frequency")
	}
	const base uint64 = 0xfffffc000
	fields := []struct {
		offset uint64
		size   int
		value  uint64
	}{
		{0x10, 8, p.CPUCapabilities}, {0x1e, 2, uint64(p.Version)},
		{0x25, 1, uint64(p.UserPageShift)}, {0x26, 2, uint64(p.CacheLineSize)},
		{0x80, 4, uint64(p.CPUFamily)}, {0x90, 1, uint64(p.UserTimebase)},
		{0x91, 1, uint64(p.ContinuousHWClock)}, {0x100, 4, uint64(p.KDebug)},
		{0x98, 8, 0}, // virtual continuous-time offset; the guest never sleeps/suspends
		{0xa8, 8, 0}, // virtual continuous hardware-counter base
	}
	for _, f := range fields {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], f.value)
		if e := m.Map(base+f.offset, uint64(f.size), usermem.Read, b[:f.size]); e != nil {
			return e
		}
	}
	return nil
}
