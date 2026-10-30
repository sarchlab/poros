package cpu

import (
	"encoding/binary"
	"fmt"
	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/mem"
	"github.com/sarchlab/poros/pkg/usermem"
	"strings"
	"testing"
)

func TestLD1AllDestinationsAcrossPipelines(t *testing.T) {
	words := []uint32{0x4c40207f, 0x4e083fe8, 0x4e083c09, 0x4e083c2a, 0x4e083c4b, 0xd503201f}
	for _, mode := range []string{"functional", "inorder", "ooo"} {
		t.Run(mode, func(t *testing.T) {
			x := newExtensionExecution(t, words)
			data := make([]byte, 64)
			for i := 0; i < 4; i++ {
				binary.LittleEndian.PutUint64(data[16*i:], uint64(17+i))
				binary.LittleEndian.PutUint64(data[16*i+8:], uint64(91+i))
			}
			if e := x.memory.Space.Write(0x2000, data); e != nil {
				t.Fatal(e)
			}
			if mode == "functional" {
				for !x.Stopped() {
					if _, _, e := x.StepDetailed(); e != nil {
						t.Fatal(e)
					}
				}
			} else {
				s := sim.MakeBuilder().Build()
				defer s.Terminate()
				if mode == "inorder" {
					spec := inorder.DefaultSpec()
					spec.LoadLatency = 40
					c := modeling.MakeBuilder[*inorder.Comp](spec).WithSimulation(s).WithResources(inorder.Resources{Execution: x}).Build("CPU")
					if e := s.Initialize(); e != nil {
						t.Fatal(e)
					}
					for n := 0; n < 1000 && !c.State.Done; n++ {
						c.Middlewares.Pipeline.Handle(nil)
					}
					if !c.State.Done || c.State.Err != "" || c.State.InstDone != uint64(len(words)) {
						t.Fatal(c.State)
					}
					for _, n := range c.State.FpPending {
						if n != 0 {
							t.Fatal("leaked FP scoreboard")
						}
					}
				} else {
					spec := ooo.DefaultSpec()
					spec.LoadLatency = 40
					c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
					if e := s.Initialize(); e != nil {
						t.Fatal(e)
					}
					for n := 0; n < 1000 && !c.State.Done; n++ {
						c.Middlewares.Pipeline.Handle(nil)
					}
					if !c.State.Done || c.State.Err != "" || c.State.InstDone != uint64(len(words)) || c.State.FpInFlight != 0 {
						t.Fatal(c.State)
					}
					for i, fp := range []int{31, 0, 1, 2} {
						if c.State.GprReady[8+i] <= c.State.FpReady[fp] {
							t.Fatal("consumer did not wait", i, c.State.GprReady, c.State.FpReady)
						}
					}
				}
			}
			for i := 0; i < 4; i++ {
				if x.regs.GPR[8+i] != uint64(17+i) {
					t.Fatal(mode, i, x.regs.GPR)
				}
			}
		})
	}
}
func TestOoOMultipleFPLoadWaitsForEnoughPhysicalSlots(t *testing.T) {
	for _, tc := range []struct {
		word  uint32
		count int
	}{{0x4c40207f, 4}, {0xad400460, 2}} {
		for free := 1; free < tc.count; free++ {
			t.Run(fmt.Sprintf("%d/%d", tc.count, free), func(t *testing.T) {
				x := newExtensionExecution(t, []uint32{tc.word, 0xd503201f})
				before := x.regs
				s := sim.MakeBuilder().Build()
				defer s.Terminate()
				spec := ooo.DefaultSpec()
				spec.FpPhysLimit = 4
				c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
				if e := s.Initialize(); e != nil {
					t.Fatal(e)
				}
				// One older instruction owns the occupied slots until cycle 10.
				c.State.RobCount = 1
				c.State.RobTail = 1
				c.State.Rob[0] = 10
				c.State.RobFPDests[0] = 4 - free
				c.State.FpInFlight = 4 - free
				for c.State.Cycles < 9 {
					c.Middlewares.Pipeline.Handle(nil)
					if x.regs != before || len(x.MemoryAccesses()) != 0 || c.State.Done {
						t.Fatal("load stepped before slots were available", c.State)
					}
				}
				for n := 0; n < 1000 && !c.State.Done; n++ {
					c.Middlewares.Pipeline.Handle(nil)
				}
				if !c.State.Done || c.State.Err != "" || c.State.InstDone != 3 || c.State.FpInFlight != 0 || x.regs.PC != x.end {
					t.Fatal(c.State)
				}
			})
		}
	}
}
func TestOoOImpossibleFPLoadDrainsOlderWorkBeforeError(t *testing.T) {
	for _, word := range []uint32{0x4c40207f, 0xad400460} {
		x := newExtensionExecution(t, []uint32{0xf9400061, word, 0xd503201f})
		s := sim.MakeBuilder().Build()
		t.Cleanup(func() { _ = s.Terminate() })
		spec := ooo.DefaultSpec()
		spec.FpPhysLimit = 1
		spec.LoadLatency = 30
		c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
		if e := s.Initialize(); e != nil {
			t.Fatal(e)
		}
		for n := 0; n < 1000 && !c.State.Done; n++ {
			c.Middlewares.Pipeline.Handle(nil)
		}
		if !c.State.Done || !strings.Contains(c.State.Err, "FP destinations") || c.State.InstDone != 1 || c.State.RobCount != 0 || c.State.FpInFlight != 0 || x.regs.PC != 0x1004 || x.regs.GPR[1] != 7 {
			t.Fatal(c.State, x.regs)
		}
		for _, v := range x.regs.FP {
			if v.Lo != 0 || v.Hi != 0 {
				t.Fatal("oversize load executed")
			}
		}
	}
}
func TestOoOLD1PostRegisterWaitsForIncrement(t *testing.T) {
	x := newExtensionExecution(t, []uint32{0xf940006b, 0x4ccb6863, 0x91000064, 0xd503201f})
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	spec := ooo.DefaultSpec()
	spec.LoadLatency = 40
	c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
	if e := s.Initialize(); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 1000 && !c.State.Done; n++ {
		c.Middlewares.Pipeline.Handle(nil)
	}
	if !c.State.Done || c.State.Err != "" || x.regs.GPR[3] != 0x2007 || x.regs.GPR[4] != 0x2007 || c.State.GprReady[3] <= c.State.GprReady[11] || c.State.GprReady[4] <= c.State.GprReady[3] {
		t.Fatal(c.State, x.regs)
	}
}

func TestOoOST1WaitsForLastVectorAndPostIncrement(t *testing.T) {
	x := newExtensionExecution(t, []uint32{0x3dc00062, 0xf940006b, 0x4c8b20bf, 0x910000a6, 0xd503201f})
	space := x.memory.Space.(*usermem.AddressSpace)
	if e := space.Map(0x3000, 128, usermem.Read|usermem.Write, nil); e != nil {
		t.Fatal(e)
	}
	x.regs.GPR[5] = 0x303d
	x.regs.WriteQ(31, 11, 0)
	x.regs.WriteQ(0, 12, 0)
	x.regs.WriteQ(1, 13, 0)
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	spec := ooo.DefaultSpec()
	spec.LoadLatency = 40
	c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: x}).Build("CPU")
	if e := s.Initialize(); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 1000 && !c.State.Done; n++ {
		c.Middlewares.Pipeline.Handle(nil)
	}
	if !c.State.Done || c.State.Err != "" || x.regs.GPR[5] != 0x3044 || x.regs.GPR[6] != 0x3044 || c.State.GprReady[5] <= c.State.GprReady[11] || c.State.GprReady[6] <= c.State.GprReady[5] {
		t.Fatal(c.State, x.regs.GPR)
	}
	data := make([]byte, 64)
	if e := space.Read(0x303d, data); e != nil {
		t.Fatal(e)
	}
	for i, want := range []uint64{11, 12, 13, 7} {
		if got := binary.LittleEndian.Uint64(data[16*i:]); got != want {
			t.Fatal(i, got, want)
		}
	}
}
func TestMultipleSIMDTracksBothCacheLines(t *testing.T) {
	for _, word := range []uint32{0x4c40207f, 0x4c00207f} {
		for _, mode := range []string{"inorder", "ooo"} {
			x := newExtensionExecution(t, []uint32{word})
			x.regs.GPR[3] = 0x203d
			if e := x.memory.Space.(*usermem.AddressSpace).Map(0x2040, 64, usermem.Read|usermem.Write, nil); e != nil {
				t.Fatal(e)
			}
			cache := mem.NewCache(mem.DefaultCacheConfig())
			s := sim.MakeBuilder().Build()
			t.Cleanup(func() { _ = s.Terminate() })
			if mode == "ooo" {
				c := modeling.MakeBuilder[*ooo.Comp](ooo.DefaultSpec()).WithSimulation(s).WithResources(ooo.Resources{Execution: x, Cache: cache}).Build("CPU")
				if e := s.Initialize(); e != nil {
					t.Fatal(e)
				}
				for n := 0; n < 1000 && !c.State.Done; n++ {
					c.Middlewares.Pipeline.Handle(nil)
				}
				if !c.State.Done || c.State.Err != "" {
					t.Fatal(c.State)
				}
			} else {
				c := modeling.MakeBuilder[*inorder.Comp](inorder.DefaultSpec()).WithSimulation(s).WithResources(inorder.Resources{Execution: x, Cache: cache}).Build("CPU")
				if e := s.Initialize(); e != nil {
					t.Fatal(e)
				}
				for n := 0; n < 1000 && !c.State.Done; n++ {
					c.Middlewares.Pipeline.Handle(nil)
				}
				if !c.State.Done || c.State.Err != "" {
					t.Fatal(c.State)
				}
			}
			// 64 unaligned bytes are nine natural-word transfers. The next-line
			// prefetch may satisfy the second line; it must still receive its accesses.
			if stats := cache.Stats(); stats.L1Hits+stats.L1Misses != 9 {
				t.Fatal(mode, word, stats)
			}
		}
	}
}
