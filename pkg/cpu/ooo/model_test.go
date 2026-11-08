package ooo

import (
	"encoding/json"
	"errors"
	"golang.org/x/arch/arm64/arm64asm"
	"reflect"
	"slices"
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/modeling/modelingtest"
	"github.com/sarchlab/poros/pkg/emu"
)

func TestModel(t *testing.T) { modelingtest.CheckTicking(t, (*Comp)(nil), DefaultSpec()) }
func TestConfigurationIsolationAndStateRoundTrip(t *testing.T) {
	s := sim.MakeBuilder().Build()
	defer func() {
		if err := s.Terminate(); err != nil {
			t.Error(err)
		}
	}()
	spec := DefaultSpec()
	spec.FuncHookCycles = []HookCycles{{Address: 0x1000, Cycles: 25}}
	builder := modeling.MakeBuilder[*Comp](spec).WithSimulation(s)
	e1, e2 := emu.NewEmulator(), emu.NewEmulator()
	e1.Mem.TrackAccesses = true
	e2.Mem.TrackAccesses = true
	a := builder.WithResources(Resources{Emu: e1}).Build("A")
	b := builder.WithResources(Resources{Emu: e2}).Build("B")
	spec.FuncHookCycles[0].Cycles = 99
	if a.Spec.FuncHookCycles[0].Cycles != 25 || b.Spec.FuncHookCycles[0].Cycles != 25 {
		t.Fatal("Build must copy configuration slices")
	}
	if len(a.State.Rob) != 0 {
		t.Fatal("runtime arrays allocated before Initialize")
	}
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	a.State.Rob[0] = 123
	a.State.RobCount = 1
	a.State.IntUnits[0] = []UnitReservation{{Start: 10, End: 11}, {Start: 100, End: 101}}
	a.State.Predictor.Bimodal[0] = 3
	if b.State.Rob[0] != 0 || len(b.State.IntUnits[0]) != 0 || b.State.Predictor.Bimodal[0] != 2 {
		t.Fatal("runtime arrays shared across components")
	}
	data, err := json.Marshal(a.State)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.State, restored) {
		t.Fatal("timing/predictor state did not round-trip")
	}
	if a.Middlewares.Pipeline.hookCycles[0x1000] != 25 {
		t.Fatal("hook lookup differs from immutable spec")
	}
}
func TestInvalidConfigurationRejectedAtInitialize(t *testing.T) {
	tests := map[string]func(*Spec){
		"empty ROB":         func(s *Spec) { s.ROBCapacity = 0 },
		"empty units":       func(s *Spec) { s.NumINT = 0 },
		"empty MSHRs":       func(s *Spec) { s.NumMSHREntries = 0 },
		"invalid predictor": func(s *Spec) { s.Predictor.TaggedHistLen = 0 },
		"duplicate hooks":   func(s *Spec) { s.FuncHookCycles = []HookCycles{{1, 2}, {1, 3}} },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			s := sim.MakeBuilder().Build()
			defer func() {
				if err := s.Terminate(); err != nil {
					t.Error(err)
				}
			}()
			spec := DefaultSpec()
			change(&spec)
			em := emu.NewEmulator()
			em.Mem.TrackAccesses = true
			modeling.MakeBuilder[*Comp](spec).WithSimulation(s).WithResources(Resources{Emu: em}).Build("CPU")
			defer func() {
				if recover() == nil {
					t.Fatal("invalid configuration accepted")
				}
			}()
			s.Initialize()
		})
	}
}

// A decoded but unsupported legacy instruction must terminate, not retry forever.
func TestLegacyExecutionErrorStops(t *testing.T) {
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	em := emu.NewEmulator()
	em.Mem.TrackAccesses = true
	em.Regs.PC = 0x1000
	em.Mem.WriteU32(0x1000, 0xd4000001) // SVC, unsupported by the legacy executor.
	c := modeling.MakeBuilder[*Comp](DefaultSpec()).WithSimulation(s).WithResources(Resources{Emu: em}).Build("CPU")
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if !c.Middlewares.Pipeline.Handle(nil) {
			break
		}
	}
	if !c.State.Done || c.State.Err == "" || c.State.InstDone != 0 {
		t.Fatalf("fault did not stop without retirement: %+v", c.State)
	}
}
func TestAmbiguousExecutionResourcesRejected(t *testing.T) {
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
	em := emu.NewEmulator()
	em.Mem.TrackAccesses = true
	modeling.MakeBuilder[*Comp](DefaultSpec()).WithSimulation(s).WithResources(Resources{Emu: em, Execution: em}).Build("CPU")
	defer func() {
		if recover() == nil {
			t.Fatal("ambiguous backend accepted")
		}
	}()
	s.Initialize()
}

type nonStoppingExecution struct {
	*emu.Emulator
	attempts   int
	emptyError bool
}

func (x *nonStoppingExecution) StepDetailed() (arm64asm.Inst, bool, error) {
	x.attempts++
	inst, branched, err := x.Emulator.StepDetailed()
	if err != nil && x.emptyError {
		err = errors.New("")
	}
	return inst, branched, err
}
func (x *nonStoppingExecution) Stopped() bool { return false }
func TestExecutionErrorDrainsWithoutBackendStop(t *testing.T) {
	for _, emptyError := range []bool{false, true} {
		s := sim.MakeBuilder().Build()
		defer s.Terminate()
		x := &nonStoppingExecution{Emulator: emu.NewEmulator(), emptyError: emptyError}
		x.Mem.TrackAccesses = true
		x.Regs.PC = 0x1000
		x.Mem.WriteU32(0x1000, 0xd2800540) // MOV X0,#42, must retire despite the later fault.
		x.Mem.WriteU32(0x1004, 0xc85f7c01) // LDXR X1,[X0], decoded but unsupported.
		c := modeling.MakeBuilder[*Comp](DefaultSpec()).WithSimulation(s).WithResources(Resources{Execution: x}).Build("CPU")
		if err := s.Initialize(); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 100; i++ {
			if !c.Middlewares.Pipeline.Handle(nil) {
				break
			}
		}
		if !c.State.Done || c.State.Err == "" || c.State.InstDone != 1 || x.attempts != 2 || x.Regs.ReadX(0) != 42 {
			t.Fatalf("fault retry or lost older retirement: attempts=%d state=%+v", x.attempts, c.State)
		}
	}
}

type syntheticPeek struct {
	emu.Execution
	inst arm64asm.Inst
}

func (x *syntheticPeek) Peek(uint64) (arm64asm.Inst, error) { return x.inst, nil }

func TestSyntheticPeekCanChangeOperandsAtSamePC(t *testing.T) {
	x := &syntheticPeek{inst: arm64asm.Inst{Op: arm64asm.ADD}}
	r := Resources{Execution: x}
	for _, reg := range []arm64asm.Reg{arm64asm.X1, arm64asm.X2, arm64asm.X8} {
		x.inst.Args = arm64asm.Args{arm64asm.X0, reg, arm64asm.X3}
		_, gprs, fps, err := r.peekSources(0x1000)
		if err != nil || !slices.Equal(gprs, []int{int(reg - arm64asm.X0), 3}) || len(fps) != 0 {
			t.Fatalf("wrong dependencies for %v: GPR %v, FP %v, err %v", x.inst, gprs, fps, err)
		}
	}
}
