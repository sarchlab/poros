package ooo

import (
	"context"
	"testing"
	"time"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/messaging/twowaybuffered"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem/timed"
	"golang.org/x/arch/arm64/arm64asm"
)

func asyncProbe(t *testing.T, code []uint32, setup func(*emu.Emulator), tweak func(*AsyncSpec, *timed.Config)) *AsyncComp {
	t.Helper()
	em := emu.NewEmulator()
	em.Regs.PC = 0x10000
	em.Regs.WriteX(30, 0xffff0000)
	em.SetHaltPC(0xffff0000)
	for i, word := range append(append([]uint32(nil), code...), 0xd65f03c0) {
		em.Mem.WriteU32(0x10000+uint64(4*i), word)
	}
	if setup != nil {
		setup(em)
	}
	em.Mem.TrackAccesses = true
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	spec := DefaultAsyncSpec()
	cfg := timed.DefaultConfig(spec.Freq)
	if tweak != nil {
		tweak(&spec, &cfg)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := modeling.MakeBuilder[*AsyncComp](spec).WithSimulation(s).WithResources(Resources{Emu: em, Context: ctx}).Build("CPU")
	c.BindPort("Memory", twowaybuffered.MakeBuilder().WithSimulation(s).WithSpec(twowaybuffered.Spec{IncomingCapacity: cfg.PortCapacity, OutgoingCapacity: cfg.PortCapacity}).Build())
	if _, err := timed.Build(s, c.Ports.Memory, cfg); err != nil {
		t.Fatal(err)
	}
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	c.TickLater()
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	if c.State.Err != "" {
		t.Fatal(c.State.Err)
	}
	if c.State.InstDone != uint64(len(code)+1) || len(c.State.Pending) != 0 || len(c.State.Stores) != 0 {
		t.Fatalf("unfinished pipeline: retired=%d want=%d pending=%d stores=%d", c.State.InstDone, len(code)+1, len(c.State.Pending), len(c.State.Stores))
	}
	return c
}

func TestAsyncDrainedStorePrefixDoesNotWaitForYoungerRetirement(t *testing.T) {
	c := &AsyncComp{State: AsyncState{Stores: []asyncStore{{Tag: 1, Retired: true, Sent: 1, Accesses: []emu.MemAccess{{Addr: 0x100, Size: 8, IsWrite: true}}}, {Tag: 2}}}}
	p := &asyncMiddleware{c: c}
	p.drainStores()
	if len(c.State.Stores) != 1 || c.State.Stores[0].Tag != 2 {
		t.Fatal("completed prefix retained behind unretired store")
	}
}

func TestAsyncPairLoadCannotReadUndrainedOverlappingPart(t *testing.T) {
	c := &AsyncComp{State: AsyncState{Cycles: 10, Stores: []asyncStore{{Tag: 1, Ready: 2, AddressReady: 1, Accesses: []emu.MemAccess{{Addr: 0x100, Size: 8, IsWrite: true}}}}}}
	p := &asyncMiddleware{c: c}
	load := asyncEntry{Tag: 2, Accesses: []emu.MemAccess{{Addr: 0x100, Size: 8}, {Addr: 0x108, Size: 8}}}
	if ready, _, _ := p.loadOrder(&load); ready {
		t.Fatal("mixed forwarding/cache pair bypassed older store drain")
	}
	c.State.Stores = nil
	if ready, forward, _ := p.loadOrder(&load); !ready || forward {
		t.Fatal("pair should issue after old store drains")
	}
}

func TestAsyncStoreCoalescingMaskAndOrdering(t *testing.T) {
	write := func(addr uint64, size int) emu.MemAccess { return emu.MemAccess{Addr: addr, Size: size, IsWrite: true} }
	c := &AsyncComp{Spec: DefaultAsyncSpec(), State: AsyncState{Stores: []asyncStore{
		{Tag: 1, Retired: true, Accesses: []emu.MemAccess{write(0x108, 8)}},
		{Tag: 2, Retired: true, Accesses: []emu.MemAccess{write(0x118, 8), write(0x110, 8)}},
		{Tag: 3, Retired: true, Accesses: []emu.MemAccess{write(0x200, 8)}},
		{Tag: 4, Retired: true, Accesses: []emu.MemAccess{write(0x120, 8)}},
	}}}
	g := (&asyncMiddleware{c: c}).storeGroup()
	if g.Address != 0x108 || len(g.Mask) != 24 || len(g.Parts) != 2 || g.Parts[1].Count != 2 {
		t.Fatalf("incorrect same-line grouping: %+v", g)
	}
	for _, set := range g.Mask {
		if !set {
			t.Fatal("covered bytes not marked")
		}
	}
	c.State.Stores[1].Accesses = c.State.Stores[1].Accesses[:1]
	g = (&asyncMiddleware{c: c}).storeGroup()
	for i, set := range g.Mask {
		if set != (i < 8 || i >= 16) {
			t.Fatal("coalescing wrote an untouched gap")
		}
	}
	c.State.Stores[1].Retired = false
	if g = (&asyncMiddleware{c: c}).storeGroup(); len(g.Parts) != 1 {
		t.Fatal("coalescing crossed an unretired store")
	}
}

func TestAsyncCoalescedAcknowledgementReleasesAllStores(t *testing.T) {
	var body []uint32
	for i := 0; i < 16; i++ {
		body = append(body, 0xf9000020+uint32(i)*1024)
	}
	c := asyncProbe(t, body, func(e *emu.Emulator) { e.Regs.WriteX(1, 0x20000); e.Regs.WriteX(0, 7) }, func(s *AsyncSpec, c *timed.Config) { c.PortCapacity = 1 })
	if c.State.Memory.StoreAccesses != 16 || c.State.Memory.CoalescedAccesses == 0 || c.State.Memory.Responses != c.State.Memory.Writes {
		t.Fatalf("coalesced store acknowledgement accounting: %+v", c.State.Memory)
	}
}

type cancelExecution struct {
	*emu.Emulator
	cancel context.CancelFunc
	steps  int
}

func (e *cancelExecution) StepDetailed() (arm64asm.Inst, bool, error) {
	i, b, err := e.Emulator.StepDetailed()
	e.steps++
	if e.steps == 32 {
		e.cancel()
	}
	return i, b, err
}
func TestAsyncCancellationDrainsMoreResponsesThanPortCapacity(t *testing.T) {
	em := emu.NewEmulator()
	em.Regs.PC = 0x10000
	em.Regs.WriteX(1, 0x20000)
	em.Mem.TrackAccesses = true
	for i := 0; i < 80; i++ {
		em.Mem.WriteU32(0x10000+uint64(i*4), 0xf9400020+uint32(i)*1024)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	x := &cancelExecution{Emulator: em, cancel: cancel}
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	spec := DefaultAsyncSpec()
	cfg := timed.DefaultConfig(spec.Freq)
	cfg.PortCapacity = 1
	c := modeling.MakeBuilder[*AsyncComp](spec).WithSimulation(s).WithResources(Resources{Execution: x, Context: ctx}).Build("CPU")
	c.BindPort("Memory", twowaybuffered.MakeBuilder().WithSimulation(s).Build())
	if _, err := timed.Build(s, c.Ports.Memory, cfg); err != nil {
		t.Fatal(err)
	}
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	c.TickLater()
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	if !c.State.Cancelled || !c.State.Done || len(c.State.Pending) != 0 || c.State.Memory.Reads < 2 || c.State.Memory.Responses != c.State.Memory.Reads {
		t.Fatalf("cancellation left queued traffic: %+v", c.State.Memory)
	}
}

func TestAsyncIndependentMissesOverlap(t *testing.T) {
	setup := func(e *emu.Emulator) {
		e.Regs.WriteX(1, 0x20000)
		e.Regs.WriteX(2, 0x40000)
		e.Mem.WriteU64(0x20000, 0x40000)
		e.Mem.WriteU64(0x40000, 7)
	}
	independent := asyncProbe(t, []uint32{0xf9400020, 0xf9400043}, setup, nil) // LDR x0,[x1];LDR x3,[x2]
	dependent := asyncProbe(t, []uint32{0xf9400020, 0xf9400003}, setup, nil)   // LDR x0,[x1];LDR x3,[x0]
	if dependent.State.Cycles < independent.State.Cycles+200 {
		t.Fatalf("misses did not overlap: independent %d dependent %d", independent.State.Cycles, dependent.State.Cycles)
	}
	if independent.State.Memory.MaxOutstanding < 2 {
		t.Fatal("independent request was serialized")
	}
	if dependent.Resources.Emu.Regs.ReadX(3) != 7 {
		t.Fatal("functional result corrupted by timing-only data")
	}
}

func TestAsyncL1DependentLatency(t *testing.T) {
	setup := func(e *emu.Emulator) { e.Regs.WriteX(1, 0x20000); e.Mem.WriteU64(0x20000, 0x20000) }
	var body []uint32
	for i := 0; i < 64; i++ {
		body = append(body, 0xf9400021)
	} // LDR x1,[x1]
	a := asyncProbe(t, body[:1], setup, nil)
	b := asyncProbe(t, body, setup, nil)
	t.Logf("first miss %d cycles; dependent L1 %.3f cycles/load", a.State.Cycles, float64(b.State.Cycles-a.State.Cycles)/63)
	if b.State.Memory.Reads != 64 || b.State.Memory.Responses != 64 {
		t.Fatal("load completed without actual response")
	}
	for _, ratio := range []int{1, 2, 3, 4} {
		tweak := func(s *AsyncSpec, c *timed.Config) {
			c.L1.Freq = s.Freq * timing.Freq(ratio)
			c.L2.Freq = c.L1.Freq
			c.Memory.Freq = c.L1.Freq
		}
		x := asyncProbe(t, body[:1], setup, tweak)
		y := asyncProbe(t, body, setup, tweak)
		t.Logf("hierarchy tick ratio%d: miss%d L1%.3f CPU cycles/load", ratio, x.State.Cycles, float64(y.State.Cycles-x.State.Cycles)/63)
	}
}

func TestAsyncForwardingAndStoreDrain(t *testing.T) {
	setup := func(e *emu.Emulator) { e.Regs.WriteX(1, 0x20000); e.Regs.WriteX(0, 42) }
	c := asyncProbe(t, []uint32{0xf9000020, 0xf9400022}, setup, nil)
	if c.Resources.Emu.Regs.ReadX(2) != 42 || c.State.Memory.ForwardedLoads != 1 || c.State.Memory.Writes != 1 || c.State.Memory.Responses != 1 {
		t.Fatalf("forwarding/draining failed: %+v", c.State.Memory)
	}
}

func TestAsyncROBWrapAndBackpressure(t *testing.T) {
	var body []uint32
	for i := 0; i < 80; i++ {
		body = append(body, 0xf9000020, 0x91020021, 0xf9400022)
	} // store; add address128;load
	c := asyncProbe(t, body, func(e *emu.Emulator) { e.Regs.WriteX(1, 0x20000) }, func(s *AsyncSpec, c *timed.Config) {
		s.ROBCapacity = 8
		s.StoreBufferSize = 2
		c.PortCapacity = 1
		c.L1.NumMSHREntries = 1
	})
	if c.State.Memory.Writes != 80 || c.State.Memory.Reads != 80 {
		t.Fatalf("lost request at wrap/backpressure: %+v", c.State.Memory)
	}
}
