package darwinuser

import (
	"encoding/binary"
	"fmt"

	"github.com/sarchlab/poros/pkg/usermem"
)

// RuntimeProfile supplies selected nonsecret kernel/runtime state captured on the
// native reference host. Unmodeled commpage fields remain unmapped, not guessed as zero.
// Dynamic time offsets use a separate deterministic virtual epoch of zero.
type RuntimeProfile struct {
	Directories         []RuntimeDirectory   `json:"directories,omitempty"`
	DevFirmware         *uint32              `json:"development_firmware,omitempty"`
	ReadOnlyDevFirmware *uint32              `json:"read_only_development_firmware,omitempty"`
	SysctlNames         []SysctlName         `json:"sysctl_names,omitempty"`
	ExclusiveMonitor    string               `json:"exclusive_monitor,omitempty"`
	NativeDyld          *RuntimeDyldIdentity `json:"native_dyld,omitempty"`
	PointerStrip        string               `json:"pointer_strip,omitempty"`
	CacheDynamic        *CacheDynamicProfile `json:"cache_dynamic,omitempty"`
	AtomicAlignment     string               `json:"atomic_alignment,omitempty"`
	CSRConfig           *uint32              `json:"csr_config,omitempty"`
	DyldFlags           *uint64              `json:"dyld_flags,omitempty"`
	KernelPageShift     uint8                `json:"kernel_page_shift,omitempty"`
	ReadOnlyPageShifts  *PageShifts          `json:"read_only_page_shifts,omitempty"`
	CPUCapabilities     uint64               `json:"cpu_capabilities"`
	Version             uint16               `json:"version"`
	UserPageShift       uint8                `json:"user_page_shift_64_legacy"`
	CacheLineSize       uint16               `json:"cache_line_size"`
	CPUFamily           uint32               `json:"cpu_family"`
	UserTimebase        uint8                `json:"user_timebase"`
	ContinuousHWClock   uint8                `json:"continuous_hwclock"`
	KDebug              uint32               `json:"kdebug"`
	TimebaseNumer       uint32               `json:"timebase_numer"`
	TimebaseDenom       uint32               `json:"timebase_denom"`
}

type PageShifts struct {
	User   uint8 `json:"user"`
	Kernel uint8 `json:"kernel"`
}

func (p RuntimeProfile) mapCommpage(m *usermem.AddressSpace) error {
	if p.ExclusiveMonitor != "" && p.ExclusiveMonitor != "m2-local-64-byte" {
		return fmt.Errorf("unsupported exclusive monitor profile")
	}
	if p.PointerStrip != "" && p.PointerStrip != "darwin-47-bit" {
		return fmt.Errorf("unsupported pointer strip profile")
	}
	if p.AtomicAlignment != "" && p.AtomicAlignment != "natural" && p.AtomicAlignment != "within-16-byte" {
		return fmt.Errorf("unsupported atomic alignment profile")
	}
	if p.Version != 3 || p.UserPageShift != 14 || p.CacheLineSize == 0 || p.CacheLineSize&(p.CacheLineSize-1) != 0 || p.TimebaseNumer == 0 || p.TimebaseDenom == 0 || p.UserTimebase > 3 || p.ContinuousHWClock > 1 {
		return fmt.Errorf("unsupported/invalid Darwin commpage profile")
	}
	if 1_000_000_000*uint64(p.TimebaseDenom)%uint64(p.TimebaseNumer) != 0 {
		return fmt.Errorf("non-integral virtual counter frequency")
	}
	if p.KernelPageShift != 0 {
		if p.KernelPageShift != 14 {
			return fmt.Errorf("unsupported kernel page shift")
		}
		if err := m.Map(0xfffffc037, 1, usermem.Read, []byte{p.KernelPageShift}); err != nil {
			return err
		}
	}
	if p.ReadOnlyPageShifts != nil {
		if p.ReadOnlyPageShifts.User != 14 || p.ReadOnlyPageShifts.Kernel != 14 {
			return fmt.Errorf("unsupported read-only page shifts")
		}
		if err := m.Map(0xfffff4025, 1, usermem.Read, []byte{p.ReadOnlyPageShifts.User}); err != nil {
			return err
		}
		if err := m.Map(0xfffff4037, 1, usermem.Read, []byte{p.ReadOnlyPageShifts.Kernel}); err != nil {
			return err
		}
	}
	// XNU _COMM_PAGE_DEV_FIRM is a selected uint32 in both commpage views.
	for _, field := range []struct {
		address uint64
		value   *uint32
	}{{0xfffffc084, p.DevFirmware}, {0xfffff4084, p.ReadOnlyDevFirmware}} {
		address, value := field.address, field.value
		if value != nil {
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], *value)
			if err := m.Map(address, 4, usermem.Read, b[:]); err != nil {
				return err
			}
		}
	}
	if p.DyldFlags != nil {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], *p.DyldFlags)
		if err := m.Map(0xfffffc160, 8, usermem.Read, b[:]); err != nil {
			return err
		}
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
		{0x88, 8, 0}, // virtual absolute-time counter offset
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

// RuntimeDyldIdentity identifies the runtime actually mapped by the native
// reference process, rather than assuming the first universal file slice ran.
type RuntimeDyldIdentity struct {
	UUID    string `json:"uuid"`
	Subtype uint32 `json:"subtype"`
}
