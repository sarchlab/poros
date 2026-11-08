package cpu

import (
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
)

func runWidthProbe(t *testing.T, code []uint32, width, retire, units int) *ooo.Comp {
	t.Helper()
	em := setupEmu(append(append([]uint32{}, code...), 0xd65f03c0), 0x10000, 0xffff0000)
	em.Mem.TrackAccesses = true
	spec := ooo.DefaultSpec()
	spec.FetchWidth = width
	spec.RetireWidth = retire
	spec.NumINT = units
	simulation := sim.MakeBuilder().Build()
	core := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(simulation).WithResources(ooo.Resources{Emu: em}).Build("Probe")
	runTestSimulation(t, simulation, core)
	if core.State.Err != "" || core.State.InstDone != uint64(len(code)+1) {
		t.Fatalf("probe failed: %+v", core.State)
	}
	return core
}

func TestFrontendWidthPreservesIntegerDependencies(t *testing.T) {
	for _, word := range []uint32{0x91000442, 0x11000442} { // X2/W2 immediate forms
		var dependent, independent []uint32
		for i := 0; i < 128; i++ {
			dependent = append(dependent, word)
			reg := uint32(2 + i%8)
			independent = append(independent, (word&^0x3ff)|reg|reg<<5)
		}
		slow := runWidthProbe(t, dependent, 3, 8, 6)
		wide := runWidthProbe(t, dependent, 8, 8, 6)
		parallel := runWidthProbe(t, independent, 8, 8, 6)
		narrowParallel := runWidthProbe(t, independent, 3, 8, 6)
		if parallel.State.Cycles >= narrowParallel.State.Cycles {
			t.Fatal("independent throughput did not respond to frontend width")
		}
		if slow.State.Cycles < 128 || wide.State.Cycles < 128 || wide.State.Cycles > slow.State.Cycles+2 {
			t.Fatalf("dependency chain changed with width: narrow=%d wide=%d", slow.State.Cycles, wide.State.Cycles)
		}
		if parallel.State.Cycles*3 >= wide.State.Cycles {
			t.Fatalf("independent work did not overlap: %d vs %d", parallel.State.Cycles, wide.State.Cycles)
		}
		if wide.Resources.Emu.Regs.ReadX(2) != 128 {
			t.Fatal("dependent ADD result changed")
		}
		for r := uint32(2); r < 10; r++ {
			if parallel.Resources.Emu.Regs.ReadX(int(r)) != 16 {
				t.Fatal("independent ADD result changed")
			}
		}
	}
}

func TestNOPUsesFrontendAndRetirementButNotIntegerUnits(t *testing.T) {
	code := make([]uint32, 256)
	for i := range code {
		code[i] = 0xd503201f
	}
	narrow := runWidthProbe(t, code, 3, 8, 1)
	wide := runWidthProbe(t, code, 8, 8, 1)
	manyUnits := runWidthProbe(t, code, 8, 8, 6)
	if wide.State.Cycles != manyUnits.State.Cycles {
		t.Fatal("NOP throughput depends on integer unit count")
	}
	serialRetire := runWidthProbe(t, code, 8, 1, 1)
	if wide.State.Cycles*2 >= narrow.State.Cycles {
		t.Fatalf("NOP still execution-limited: narrow=%d wide=%d", narrow.State.Cycles, wide.State.Cycles)
	}
	if serialRetire.State.Cycles < 257 || wide.State.Cycles < 33 {
		t.Fatal("NOP bypassed architectural accounting")
	}
	// NOPs don't mark the integer unit busy; RET is the only issuing instruction.
	if len(wide.State.IntUnits[0]) != 0 {
		t.Fatal("NOP resource state did not drain")
	}
}

func TestInOrderImmediateDependenciesStallAndDrain(t *testing.T) {
	dependent := []uint32{0x91000442, 0x11000442, 0xd65f03c0}
	independent := []uint32{0x91000442, 0x11000463, 0xd65f03c0}
	a := runPipeline(t, setupEmu(dependent, 0x10000, 0xffff0000))
	b := runPipeline(t, setupEmu(independent, 0x10000, 0xffff0000))
	if a.State.Cycles <= b.State.Cycles {
		t.Fatalf("in-order dependency missed: dependent=%d independent=%d", a.State.Cycles, b.State.Cycles)
	}
	for _, n := range a.State.GprPending {
		if n != 0 {
			t.Fatal("pending GPR did not drain")
		}
	}
}
