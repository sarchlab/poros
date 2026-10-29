package ooo

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/modeling/modelingtest"
	"github.com/sarchlab/poros/pkg/emu"
)

func TestModel(t *testing.T) { modelingtest.CheckTicking(t, (*Comp)(nil), DefaultSpec()) }
func TestConfigurationIsolationAndStateRoundTrip(t *testing.T) {
	s := sim.MakeBuilder().Build()
	defer s.Terminate()
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
	a.State.IntUnits[0] = 3
	a.State.Predictor.Bimodal[0] = 3
	if b.State.Rob[0] != 0 || b.State.IntUnits[0] != 0 || b.State.Predictor.Bimodal[0] != 2 {
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
			defer s.Terminate()
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
