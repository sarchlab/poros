package cpu

import (
	"encoding/binary"
	"testing"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/m2sim3/pkg/benchmark"
	"github.com/sarchlab/m2sim3/pkg/emu"
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

	engine := sim.NewSerialEngine()
	mc := NewMultiCorePipeline("Test", engine, 3490*sim.MHz, []CoreConfig{
		{Emulator: makeEmu(code0)},
		{Emulator: makeEmu(code1)},
	})

	err := mc.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Both cores should have completed.
	if !mc.Cores[0].Emu.Halted {
		t.Error("core 0 did not halt")
	}
	if !mc.Cores[1].Emu.Halted {
		t.Error("core 1 did not halt")
	}

	// Check results.
	if got := mc.Cores[0].Emu.Regs.ReadX(0); got != 42 {
		t.Errorf("core 0: X0 = %d, want 42", got)
	}
	if got := mc.Cores[1].Emu.Regs.ReadX(0); got != 106 {
		t.Errorf("core 1: X0 = %d, want 106", got)
	}

	// TotalCycles = max of per-core cycles.
	if mc.TotalCycles == 0 {
		t.Error("TotalCycles should be > 0")
	}
	max := mc.Cores[0].Cycles
	if mc.Cores[1].Cycles > max {
		max = mc.Cores[1].Cycles
	}
	if mc.TotalCycles != max {
		t.Errorf("TotalCycles = %d, want max(%d, %d) = %d",
			mc.TotalCycles, mc.Cores[0].Cycles, mc.Cores[1].Cycles, max)
	}

	// TotalInstructions = sum.
	sum := mc.Cores[0].InstDone + mc.Cores[1].InstDone
	if mc.TotalInstructions != sum {
		t.Errorf("TotalInstructions = %d, want %d", mc.TotalInstructions, sum)
	}

	t.Logf("Core0: %d insts, %d cycles; Core1: %d insts, %d cycles; Total: %d cycles, %d insts",
		mc.Cores[0].InstDone, mc.Cores[0].Cycles,
		mc.Cores[1].InstDone, mc.Cores[1].Cycles,
		mc.TotalCycles, mc.TotalInstructions)
}

// runMultiCoreParsec runs a PARSEC benchmark on nCores cores in parallel,
// each core running an independent Setup() instance. Returns the
// MultiCorePipeline for inspection.
func runMultiCoreParsec(t *testing.T, name string, nCores int) *MultiCorePipeline {
	t.Helper()
	b := benchmark.ByName(name)
	if b == nil {
		t.Fatalf("benchmark %q not found", name)
	}

	configs := make([]CoreConfig, nCores)
	for i := 0; i < nCores; i++ {
		configs[i] = CoreConfig{
			Emulator:       b.Setup(),
			FuncHookCycles: b.FuncHookCycles,
		}
	}

	engine := sim.NewSerialEngine()
	mc := NewMultiCorePipeline("M2sim", engine, 3490*sim.MHz, configs)

	// Warm caches for each core.
	for _, core := range mc.Cores {
		benchmark.WarmCache(core.Cache, b.Regions)
	}

	err := mc.Run()
	if err != nil {
		t.Fatalf("%s multi-core Run() error: %v", name, err)
	}

	// Verify all cores halted.
	for i, core := range mc.Cores {
		if !core.Emu.Halted {
			t.Errorf("%s core %d did not halt", name, i)
		}
		if core.Err != nil {
			t.Errorf("%s core %d error: %v", name, i, core.Err)
		}
	}

	t.Logf("%s %d-core: %d total insts, %d max cycles (per-core: %d, %d, %d, %d)",
		name, nCores, mc.TotalInstructions, mc.TotalCycles,
		mc.Cores[0].Cycles, mc.Cores[1].Cycles,
		mc.Cores[2].Cycles, mc.Cores[3].Cycles)

	return mc
}

func TestMultiCore_Blackscholes_4(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-core test in short mode")
	}
	mc := runMultiCoreParsec(t, "blackscholes", 4)
	if mc.TotalCycles == 0 {
		t.Error("TotalCycles should be > 0")
	}
}

func TestMultiCore_Swaptions_4(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-core test in short mode")
	}
	mc := runMultiCoreParsec(t, "swaptions", 4)
	if mc.TotalCycles == 0 {
		t.Error("TotalCycles should be > 0")
	}
}

func TestMultiCore_Streamcluster_4(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-core test in short mode")
	}
	mc := runMultiCoreParsec(t, "streamcluster", 4)
	if mc.TotalCycles == 0 {
		t.Error("TotalCycles should be > 0")
	}
}

func TestMultiCore_Canneal_4(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-core test in short mode")
	}
	mc := runMultiCoreParsec(t, "canneal", 4)
	if mc.TotalCycles == 0 {
		t.Error("TotalCycles should be > 0")
	}
}
