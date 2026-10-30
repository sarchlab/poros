package darwinuser

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"

	"github.com/sarchlab/poros/pkg/usermem"
)

// TimeOfDayAnchor is the five-u64 Darwin commpage calendar tuple. Fraction and
// Scale are binary fractions (units 2^-64 seconds), including captured NTP scale.
type TimeOfDayAnchor struct {
	Tick           uint64 `json:"tick"`
	Seconds        uint64 `json:"seconds"`
	Fraction       uint64 `json:"fraction"`
	Scale          uint64 `json:"scale"`
	TicksPerSecond uint64 `json:"ticks_per_second"`
}

// ClockAnchor describes one explicit launch-time clock snapshot, separate from
// immutable runtime capabilities. After launch only virtual elapsed ticks advance
// this state: no host time, sleep, calendar adjustment or NTP daemon is consulted.
type ClockAnchor struct {
	BootTimeMicroseconds     uint64          `json:"boot_time_microseconds"`
	InitialCounter           uint64          `json:"initial_counter"`
	AbsoluteOffset           uint64          `json:"absolute_offset"`
	ContinuousOffset         uint64          `json:"continuous_offset"`
	ContinuousHardwareOffset uint64          `json:"continuous_hardware_offset"`
	TimeOfDay                TimeOfDayAnchor `json:"time_of_day"`
}

type virtualClock struct {
	anchor         ClockAnchor
	currentCounter uint64
	tuple          TimeOfDayAnchor
	region         *usermem.ReadOnlyRegion
}

func (a ClockAnchor) validate(p *RuntimeProfile) error {
	if p == nil || p.TimebaseNumer == 0 || p.TimebaseDenom == 0 || p.UserTimebase == 0 {
		return fmt.Errorf("clock anchor requires captured user timebase")
	}
	frequency := uint64(1_000_000_000) * uint64(p.TimebaseDenom) / uint64(p.TimebaseNumer)
	t := a.TimeOfDay
	// Native offsets are uint64 modular additions, including a possible negative
	// hardware-to-absolute offset; do not reject a legitimate wrap in that sum.
	now := a.InitialCounter + a.AbsoluteOffset
	if a.BootTimeMicroseconds == 0 || t.Tick == 0 || t.Seconds > math.MaxInt64 || t.Scale == 0 || t.TicksPerSecond == 0 || t.TicksPerSecond != frequency || now < t.Tick || now-t.Tick >= t.TicksPerSecond {
		return fmt.Errorf("invalid or stale clock anchor")
	}
	if p.ContinuousHWClock != 0 && a.AbsoluteOffset+a.ContinuousOffset != a.ContinuousHardwareOffset {
		return fmt.Errorf("inconsistent continuous clock origins")
	}
	if _, e := advanceCalendar(t, now-t.Tick); e != nil {
		return e
	}
	return nil
}
func calendarBytes(t TimeOfDayAnchor) []byte {
	b := make([]byte, 40)
	for i, value := range []uint64{t.Tick, t.Seconds, t.Fraction, t.Scale, t.TicksPerSecond} {
		binary.LittleEndian.PutUint64(b[8*i:], value)
	}
	return b
}
func advanceCalendar(t TimeOfDayAnchor, delta uint64) (TimeOfDayAnchor, error) {
	hi, lo := bits.Mul64(t.Scale, delta)
	frac, carry := bits.Add64(t.Fraction, lo, 0)
	seconds, c1 := bits.Add64(t.Seconds, hi, carry)
	if c1 != 0 || seconds > math.MaxInt64 || delta > math.MaxUint64-t.Tick {
		return t, fmt.Errorf("virtual calendar overflow")
	}
	t.Tick += delta
	t.Seconds = seconds
	t.Fraction = frac
	return t, nil
}
func mapVirtualClock(m *usermem.AddressSpace, p *RuntimeProfile, anchor *ClockAnchor) (*virtualClock, error) {
	if p == nil {
		if anchor != nil {
			return nil, fmt.Errorf("clock anchor requires runtime profile")
		}
		return nil, nil
	}
	var a ClockAnchor
	if anchor != nil {
		a = *anchor
		if e := a.validate(p); e != nil {
			return nil, e
		}
	}
	for _, field := range []struct{ offset, value uint64 }{{0x88, a.AbsoluteOffset}, {0x98, a.ContinuousOffset}, {0xa8, a.ContinuousHardwareOffset}} {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], field.value)
		if e := m.Map(0xfffffc000+field.offset, 8, usermem.Read, b[:]); e != nil {
			return nil, e
		}
	}
	if anchor == nil {
		return nil, nil
	} // Boot epoch/calendar remain absent without input.
	var boot [8]byte
	binary.LittleEndian.PutUint64(boot[:], a.BootTimeMicroseconds)
	if e := m.Map(0xfffffc0a0, 8, usermem.Read, boot[:]); e != nil {
		return nil, e
	}
	region, e := m.MapKernelReadOnly(0xfffffc120, calendarBytes(a.TimeOfDay))
	if e != nil {
		return nil, e
	}
	return &virtualClock{a, a.InitialCounter, a.TimeOfDay, region}, nil
}
func (c *virtualClock) synchronize(counter uint64) error {
	if counter < c.currentCounter {
		return fmt.Errorf("virtual counter moved backward")
	}
	absolute := counter + c.anchor.AbsoluteOffset
	if absolute < c.tuple.Tick {
		return fmt.Errorf("virtual absolute clock wrapped")
	}
	delta := absolute - c.tuple.Tick
	if delta >= c.tuple.TicksPerSecond {
		// Refresh only at deterministic whole virtual-second boundaries. The guest
		// sees a nonzero, valid tuple, and its existing seqlock retry handles a boundary
		// crossed between reads. Captured scale is preserved (not replaced by 1/freq).
		elapsed := delta / c.tuple.TicksPerSecond * c.tuple.TicksPerSecond
		next, e := advanceCalendar(c.tuple, elapsed)
		if e != nil {
			return e
		}
		if e = c.region.Replace(calendarBytes(next)); e != nil {
			return e
		}
		c.tuple = next
	}
	c.currentCounter = counter
	return nil
}
