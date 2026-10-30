package darwinuser

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"math/big"
	"testing"

	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/usermem"
)

func clockProfile() *RuntimeProfile {
	return &RuntimeProfile{Version: 3, UserPageShift: 14, CacheLineSize: 128, TimebaseNumer: 500000000, TimebaseDenom: 1, UserTimebase: 3, ContinuousHWClock: 1}
}
func clockFixture() ClockAnchor {
	return ClockAnchor{BootTimeMicroseconds: 1_000_000, InitialCounter: 100, TimeOfDay: TimeOfDayAnchor{Tick: 100, Seconds: 10, Scale: 1 << 63, TicksPerSecond: 2}}
}
func TestVirtualCalendarRefreshOwnershipAndFaults(t *testing.T) {
	anchor := clockFixture()
	m := &usermem.AddressSpace{}
	c, e := mapVirtualClock(m, clockProfile(), &anchor)
	if e != nil {
		t.Fatal(e)
	}
	anchor.TimeOfDay.Scale = 1
	for _, tc := range []struct{ counter, tick, seconds uint64 }{{100, 100, 10}, {101, 100, 10}, {102, 102, 11}, {103, 102, 11}, {108, 108, 14}} {
		if e = c.synchronize(tc.counter); e != nil {
			t.Fatal(e)
		}
		var out [40]byte
		if e = m.Read(0xfffffc120, out[:]); e != nil {
			t.Fatal(e)
		}
		if binary.LittleEndian.Uint64(out[:]) != tc.tick || binary.LittleEndian.Uint64(out[8:]) != tc.seconds || binary.LittleEndian.Uint64(out[24:]) != 1<<63 {
			t.Fatal(tc, out)
		}
	}
	var boot [8]byte
	if e = m.Read(0xfffffc0a0, boot[:]); e != nil || binary.LittleEndian.Uint64(boot[:]) != 1_000_000 {
		t.Fatal(boot, e)
	}
	for _, addr := range []uint64{0xfffffc0a0, 0xfffffc120, 0xfffffc147} {
		if m.Write(addr, []byte{1}) == nil {
			t.Fatal("guest clock write")
		}
	}
	if m.Read(0xfffffc148, []byte{0}) == nil {
		t.Fatal("unknown adjacent state synthesized")
	}
	before := c.tuple
	if c.synchronize(107) == nil || c.tuple != before || c.currentCounter != 108 {
		t.Fatal("backward mutation")
	}
	if e = m.Unmap(0xfffffc120, 40); e != nil {
		t.Fatal(e)
	}
	if c.synchronize(110) == nil || c.tuple != before || c.currentCounter != 108 {
		t.Fatal("stale mapping mutation")
	}
	if _, e = advanceCalendar(TimeOfDayAnchor{Tick: 1, Seconds: math.MaxInt64, Scale: math.MaxUint64, Fraction: math.MaxUint64}, 1); e == nil {
		t.Fatal("calendar overflow")
	}
}
func TestClockAnchorRejectsMissingAndInvalidInputs(t *testing.T) {
	base := clockFixture()
	mutations := []func(*ClockAnchor){func(a *ClockAnchor) { a.BootTimeMicroseconds = 0 }, func(a *ClockAnchor) { a.TimeOfDay.Tick = 0 }, func(a *ClockAnchor) { a.TimeOfDay.Seconds = 1 << 63 }, func(a *ClockAnchor) { a.TimeOfDay.Scale = 0 }, func(a *ClockAnchor) { a.TimeOfDay.TicksPerSecond = 0 }, func(a *ClockAnchor) { a.TimeOfDay.TicksPerSecond = 3 }, func(a *ClockAnchor) { a.InitialCounter = 99 }, func(a *ClockAnchor) { a.InitialCounter = 102 }, func(a *ClockAnchor) { a.ContinuousHardwareOffset = 1 }}
	for _, change := range mutations {
		a := base
		change(&a)
		if a.validate(clockProfile()) == nil {
			t.Fatal("invalid anchor", a)
		}
	}
	if base.validate(nil) == nil {
		t.Fatal("missing capabilities")
	}
	p := clockProfile()
	p.UserTimebase = 0
	if base.validate(p) == nil {
		t.Fatal("unavailable user counter")
	}
	// A negative absolute offset is represented as modular uint64 addition.
	a := base
	a.InitialCounter = 110
	a.AbsoluteOffset = ^uint64(9)
	a.ContinuousHardwareOffset = a.AbsoluteOffset
	if a.validate(clockProfile()) != nil {
		t.Fatal("native offset addition is modular")
	}
	no := &usermem.AddressSpace{}
	if _, e := mapVirtualClock(no, clockProfile(), nil); e != nil {
		t.Fatal(e)
	}
	if no.Read(0xfffffc0a0, make([]byte, 8)) == nil || no.Read(0xfffffc120, make([]byte, 40)) == nil {
		t.Fatal("uncaptured wall time guessed")
	}
}
func TestClockExecutionUsesOwnedCounterWithoutHostTime(t *testing.T) {
	a := clockFixture()
	p, e := Parse(machotest.Image(2), machotest.Image(7), Options{RuntimeProfile: clockProfile(), ClockAnchor: &a})
	if e != nil {
		t.Fatal(e)
	}
	if p.Registers.Counter.Value != 100 || p.Registers.Counter.Frequency != 2 {
		t.Fatal("counter anchor", p.Registers.Counter)
	}
	a.InitialCounter = 900
	x, e := p.NewExecution(context.Background(), RunOptions{})
	if e != nil {
		t.Fatal(e)
	}
	defer x.Close()
	for i := 0; i < 2; i++ {
		if _, _, e = x.StepDetailed(); e != nil {
			t.Fatal(e)
		}
	}
	if p.Registers.Counter.Value != 102 {
		t.Fatal("functional ticks", p.Registers.Counter)
	}
	p.Registers.Counter.Value = math.MaxUint64
	regs := p.Registers
	if _, _, e = x.StepDetailed(); e == nil || x.Result().Termination != "clock_fault" || p.Registers != regs {
		t.Fatal("counter overflow not precise", e)
	}
}
func TestNativeClockAnchorCalendarBrackets(t *testing.T) {
	var native struct {
		Numer, Denom uint32
		Samples      []struct {
			BootBefore       uint64 `json:"boot_before"`
			BootAfter        uint64 `json:"boot_after"`
			BootAPI          uint64 `json:"boot_api"`
			BootSysctl       uint64 `json:"boot_sysctl_usec"`
			AbsoluteOffset   uint64 `json:"absolute_offset"`
			ContinuousOffset uint64 `json:"continuous_offset"`
			HardwareOffset   uint64 `json:"continuous_hw_offset"`
			Tuple            [5]uint64
			Before           uint64 `json:"counter_before"`
			After            uint64 `json:"counter_after"`
			AbsoluteBefore   uint64 `json:"absolute_before"`
			AbsoluteAfter    uint64 `json:"absolute_after"`
			ContinuousBefore uint64 `json:"continuous_before"`
			ContinuousAfter  uint64 `json:"continuous_after"`
			Seconds          uint64 `json:"timeofday_sec"`
			Microseconds     uint64 `json:"timeofday_usec"`
		}
	}
	kernelNativeResult(t, "clock-anchor-probe.c", &native)
	if len(native.Samples) != 3 {
		t.Fatal("missing native clock samples")
	}
	p := clockProfile()
	p.TimebaseNumer = native.Numer
	p.TimebaseDenom = native.Denom
	for _, s := range native.Samples {
		if s.BootBefore == 0 || s.BootBefore != s.BootAfter || s.BootBefore != s.BootAPI || s.BootBefore != s.BootSysctl {
			t.Fatal("native boot epoch mismatch")
		}
		a := ClockAnchor{s.BootBefore, s.After, s.AbsoluteOffset, s.ContinuousOffset, s.HardwareOffset, TimeOfDayAnchor{s.Tuple[0], s.Tuple[1], s.Tuple[2], s.Tuple[3], s.Tuple[4]}}
		if e := a.validate(p); e != nil {
			t.Fatal(e)
		}
		if s.AbsoluteBefore < s.Before+s.AbsoluteOffset || s.AbsoluteAfter > s.After+s.AbsoluteOffset || s.ContinuousBefore < s.Before+s.HardwareOffset || s.ContinuousAfter > s.After+s.HardwareOffset {
			t.Fatal("native counter origins")
		}
		// Independent big-integer bintime calculation, compared against the real
		// API's wall time bracket, not against a second copy of the Go uint128 code.
		wall := func(counter uint64) uint64 {
			q := new(big.Int).SetUint64(s.Tuple[3])
			q.Mul(q, new(big.Int).SetUint64(counter+s.AbsoluteOffset-s.Tuple[0]))
			q.Add(q, new(big.Int).SetUint64(s.Tuple[2]))
			seconds := new(big.Int).Rsh(new(big.Int).Set(q), 64).Uint64() + s.Tuple[1]
			fracHigh := new(big.Int).Rsh(q, 32).Uint64() & math.MaxUint32
			return seconds*1_000_000 + (fracHigh * 1_000_000 >> 32)
		}
		observed := s.Seconds*1_000_000 + s.Microseconds
		if observed < wall(s.Before) || observed > wall(s.After) {
			t.Fatal("native calendar API outside captured bracket", s)
		}
		m := &usermem.AddressSpace{}
		c, e := mapVirtualClock(m, p, &a)
		if e != nil {
			t.Fatal(e)
		}
		for _, delta := range []uint64{0, s.Tuple[4], 3*s.Tuple[4] + 7} {
			if e = c.synchronize(a.InitialCounter + delta); e != nil {
				t.Fatal(e)
			}
			got, e := advanceCalendar(c.tuple, a.InitialCounter+delta+a.AbsoluteOffset-c.tuple.Tick)
			if e != nil {
				t.Fatal(e)
			}
			value := got.Seconds*1_000_000 + ((got.Fraction >> 32) * 1_000_000 >> 32)
			if value != wall(a.InitialCounter+delta) {
				t.Fatal("refresh changed captured NTP scale", value, wall(a.InitialCounter+delta))
			}
			if !bytes.Equal(calendarBytes(c.tuple)[24:32], calendarBytes(a.TimeOfDay)[24:32]) {
				t.Fatal("scale replaced")
			}
		}
	}
}

func TestCalendarGuestReaderRetriesAcrossVirtualSecondRefresh(t *testing.T) {
	// Owned equivalent of the documented tuple reader: tick, counter, tuple,
	// tick/retry, then 128-bit fraction arithmetic including real guest ADC.
	words := []uint32{0xf9400141, 0xd53be040, 0xaa0003ec, 0xa9408d42, 0xa9419544, 0xf9400146, 0xeb06003f, 0x54ffff21, 0xcb010000, 0x9b007c87, 0x9bc07c88, 0xab070063, 0x9a080042, 0xd360fc63, 0x9b0b7c63, 0xd360fc63, 0xd503201f}
	m := &usermem.AddressSpace{}
	code := make([]byte, 4*len(words))
	for i, w := range words {
		binary.LittleEndian.PutUint32(code[i*4:], w)
	}
	if e := m.Map(0x1000, uint64(len(code)), usermem.Execute, code); e != nil {
		t.Fatal(e)
	}
	profile := clockProfile()
	profile.TimebaseNumer = 10_000_000
	a := clockFixture()
	a.InitialCounter = 198
	a.TimeOfDay.TicksPerSecond = 100
	a.TimeOfDay.Scale = 184467440737095516
	c, e := mapVirtualClock(m, profile, &a)
	if e != nil {
		t.Fatal(e)
	}
	p := &Process{Memory: m, clock: c, Registers: emu.Registers{PC: 0x1000, Counter: emu.CounterState{Enabled: true, Frequency: 100, Value: 198}}}
	p.Registers.GPR[10] = 0xfffffc120
	p.Registers.GPR[11] = 1_000_000
	x, e := p.NewExecution(context.Background(), RunOptions{MaxInstructions: 80})
	if e != nil {
		t.Fatal(e)
	}
	defer x.Close()
	attempts := 0
	for p.Registers.PC != 0x1000+uint64(len(code)) {
		if p.Registers.PC == 0x1000 {
			attempts++
		}
		if _, _, e = x.StepDetailed(); e != nil {
			t.Fatal(e)
		}
	}
	if attempts != 2 || c.tuple.Tick != 200 {
		t.Fatal("missing/busy retry", attempts, c.tuple)
	}
	// Independent exact integer result for the counter sampled in the accepted
	// second attempt; final counter is later and must not be used for the result.
	q := new(big.Int).SetUint64(a.TimeOfDay.Scale)
	q.Mul(q, new(big.Int).SetUint64(p.Registers.GPR[12]-a.TimeOfDay.Tick))
	seconds := a.TimeOfDay.Seconds + new(big.Int).Rsh(new(big.Int).Set(q), 64).Uint64()
	microseconds := ((new(big.Int).Rsh(q, 32).Uint64() & math.MaxUint32) * 1_000_000) >> 32
	if p.Registers.GPR[2] != seconds || p.Registers.GPR[3] != microseconds {
		t.Fatal("guest calendar arithmetic", p.Registers.GPR[2:4], seconds, microseconds)
	}
}
