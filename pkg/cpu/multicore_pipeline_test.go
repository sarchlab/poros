package cpu

import (
	"encoding/binary"
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/timing"

	"github.com/sarchlab/poros/pkg/emu"
)

// TestMultiCore_Basic verifies that 2 cores running simple programs both
// complete and that aggregate stats are correct.
func TestMultiCore_Basic(t *testing.T) {
	// Core 0: MOV X0, #42; RET  (2 instructions)
	// Core 1: MOV X0, #99; MOV X1, #7; ADD X0, X0, X1; RET  (4 instructions)

	makeEmu := func(code []uint32) *emu.Emulator {
		em := emu.NewEmulator()
		em.MaxInst = 1000
		buf := make([]byte, len(code)*4)
		for i, inst := range code {
			binary.LittleEndian.PutUint32(buf[i*4:], inst)
		}
		codeBase := uint64(0x1000)
		haltPC := uint64(0xFFFF0000)
		em.LoadProgram(codeBase, buf)
		em.Regs.PC = codeBase
		em.Regs.WriteX(30, haltPC)
		em.Regs.SP = 0x10000
		em.LoadProgram(0x10000-64, make([]byte, 64))
		em.SetHaltPC(haltPC)
		return em
	}

	code0 := []uint32{
		0xd2800540, // MOV X0, #42
		0xd65f03c0, // RET
	}
	code1 := []uint32{
		0xd2800C60, // MOV X0, #99
		0xd28000E1, // MOV X1, #7
		0x8b010000, // ADD X0, X0, X1
		0xd65f03c0, // RET
	}

	simulation := sim.MakeBuilder().Build()
	mc := NewMultiCorePipeline("Test", simulation, 3490*timing.MHz, []CoreConfig{
		{Emulator: makeEmu(code0)},
		{Emulator: makeEmu(code1)},
	})

	if err := simulation.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := simulation.Terminate(); err != nil {
			t.Error(err)
		}
	}()

	err := mc.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Both cores should have completed.
	if !mc.Cores[0].Resources.Emu.Halted {
		t.Error("core 0 did not halt")
	}
	if !mc.Cores[1].Resources.Emu.Halted {
		t.Error("core 1 did not halt")
	}

	// Check results.
	if got := mc.Cores[0].Resources.Emu.Regs.ReadX(0); got != 42 {
		t.Errorf("core 0: X0 = %d, want 42", got)
	}
	if got := mc.Cores[1].Resources.Emu.Regs.ReadX(0); got != 106 {
		t.Errorf("core 1: X0 = %d, want 106", got)
	}

	// TotalCycles = max of per-core cycles.
	if mc.TotalCycles == 0 {
		t.Error("TotalCycles should be > 0")
	}
	max := mc.Cores[0].State.Cycles
	if mc.Cores[1].State.Cycles > max {
		max = mc.Cores[1].State.Cycles
	}
	if mc.TotalCycles != max {
		t.Errorf("TotalCycles = %d, want max(%d, %d) = %d",
			mc.TotalCycles, mc.Cores[0].State.Cycles, mc.Cores[1].State.Cycles, max)
	}

	// TotalInstructions = sum.
	sum := mc.Cores[0].State.InstDone + mc.Cores[1].State.InstDone
	if mc.TotalInstructions != sum {
		t.Errorf("TotalInstructions = %d, want %d", mc.TotalInstructions, sum)
	}

	t.Logf("Core0: %d insts, %d cycles; Core1: %d insts, %d cycles; Total: %d cycles, %d insts",
		mc.Cores[0].State.InstDone, mc.Cores[0].State.Cycles,
		mc.Cores[1].State.InstDone, mc.Cores[1].State.Cycles,
		mc.TotalCycles, mc.TotalInstructions)
}

// TestMultiCore_SharedFillTransport exercises real OoO cores on one Akita
// engine. Separate cold address ranges avoid cache sharing; only the shared
// DRAM transport should serialize their fills. These are model consistency
// checks, not assertions about measured M2 bandwidth.
func TestMultiCore_SharedFillTransport(t *testing.T) {
	type program struct {
		base           uint64
		loads, padding int
	}
	run := func(programs ...program) []uint64 {
		t.Helper()
		configs := make([]CoreConfig, len(programs))
		for core, p := range programs {
			em := emu.NewEmulator()
			em.MaxInst = 10000
			var code []uint32
			for i := 0; i < p.loads; i++ {
				// Widely separated lines do not train sequential prefetching.
				addr := p.base + uint64(i)*0x100000
				code = append(code,
					0xd2800014|uint32(addr&0xffff)<<5, // MOVZ X20, low16
					0xf2a00014|uint32(addr>>16)<<5,    // MOVK X20, high16, LSL16
					0xf9400280|uint32(i))              // LDR Xi, [X20]
				data := make([]byte, 8)
				binary.LittleEndian.PutUint64(data, uint64(i+1))
				em.LoadProgram(addr, data)
			}
			for i := 0; i < p.padding; i++ {
				code = append(code, 0xd503201f) // NOP; retain client without memory traffic
			}
			buf := make([]byte, 4*len(code))
			for i, inst := range code {
				binary.LittleEndian.PutUint32(buf[i*4:], inst)
			}
			em.LoadProgram(0x1000, buf)
			em.Regs.PC = 0x1000
			em.SetHaltPC(0x1000 + uint64(len(buf)))
			configs[core] = CoreConfig{Emulator: em}
		}
		simulation := sim.MakeBuilder().Build()
		mc := NewMultiCorePipeline("Fills", simulation, 3490*timing.MHz, configs)
		if err := simulation.Initialize(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := simulation.Terminate(); err != nil {
				t.Error(err)
			}
		}()
		if err := mc.Run(); err != nil {
			t.Fatal(err)
		}
		cycles := make([]uint64, len(programs))
		for core, p := range programs {
			c := mc.Cores[core]
			if !c.State.Done || !c.Resources.Emu.Halted {
				t.Fatalf("core %d did not finish", core)
			}
			if got, want := c.State.InstDone, uint64(3*p.loads+p.padding); got != want {
				t.Fatalf("core %d instructions = %d, want %d", core, got, want)
			}
			for i := 0; i < p.loads; i++ {
				if got := c.Resources.Emu.Regs.ReadX(i); got != uint64(i+1) {
					t.Fatalf("core %d X%d = %d, want %d", core, i, got, i+1)
				}
			}
			cycles[core] = c.State.Cycles
		}
		return cycles
	}

	a, b := program{base: 0x10000000, loads: 8}, program{base: 0x40000000, loads: 8}
	independentA, independentB := run(a)[0], run(b)[0]
	shared := run(a, b)
	if got, independent := max(shared[0], shared[1]), max(independentA, independentB); got <= independent {
		t.Fatalf("shared transport makespan %d did not exceed independent transports %d", got, independent)
	}
	short := program{base: a.base, loads: 1}
	early := run(short, b)
	short.padding = 5000
	retained := run(short, b)
	if early[0] >= early[1] || retained[0] <= retained[1] {
		t.Fatalf("client lifecycle control did not bracket sibling: early=%v retained=%v", early, retained)
	}
	if early[1] != retained[1] {
		t.Fatalf("early client exit changed sibling fill completion: early=%v retained=%v", early, retained)
	}
	t.Logf("independent=%d/%d shared=%v early-exit=%v retained-client=%v", independentA, independentB, shared, early, retained)
}
