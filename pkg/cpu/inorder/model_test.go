package inorder

import (
	"encoding/binary"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/modeling/modelingtest"
	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

func TestModel(t *testing.T) { modelingtest.CheckTicking(t, (*Comp)(nil), DefaultSpec()) }
func TestLifecycleAndStateRoundTrip(t *testing.T) {
	s := sim.MakeBuilder().Build()
	em := emu.NewEmulator()
	em.Mem.TrackAccesses = true
	em.Regs.PC = 0x1000
	c := modeling.MakeBuilder[*Comp](DefaultSpec()).WithSimulation(s).WithResources(Resources{Emu: em}).Build("CPU")
	if err := s.Run(); err == nil {
		t.Fatal("Run before Initialize must fail")
	}
	if c.State.FetchPC != 0 {
		t.Fatal("state initialized during Build")
	}
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Terminate(); err != nil {
			t.Error(err)
		}
	}()
	if c.State.FetchPC != 0x1000 {
		t.Fatal("initializer did not seed fetch PC")
	}
	// Fetch ADD X1, X0, X0 once, including its operand metadata.
	em.Mem.WriteU32(0x1000, 0x8b000001)
	c.Middlewares.Pipeline.stageFetch()
	if c.State.Decode.Op != arm64asm.ADD || c.State.Decode.DestGPR != 1 || !reflect.DeepEqual(c.State.Decode.SourceGPRs, []int{0, 0}) {
		t.Fatalf("wrong fetched operands: %+v", c.State.Decode)
	}
	c.State.Decode.CyclesLeft = 3
	c.State.GprPending[0] = 1
	c.State.Cycles = 7
	data, err := json.Marshal(c.State)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.State, restored) {
		t.Fatal("pipeline state did not round-trip")
	}
	em.Mem.WriteU32(0x1000, 0xd65f03c0)
	if got := restored.Decode.Word; got != 0x8b000001 {
		t.Fatalf("stored %#x, want fetched word", got)
	}
}
func TestIndependentPipelineState(t *testing.T) {
	s := sim.MakeBuilder().Build()
	defer func() {
		if err := s.Terminate(); err != nil {
			t.Error(err)
		}
	}()
	var comps []*Comp
	for _, name := range []string{"A", "B"} {
		em := emu.NewEmulator()
		em.Mem.TrackAccesses = true
		var code [8]byte
		binary.LittleEndian.PutUint32(code[:], 0xd2800540)
		binary.LittleEndian.PutUint32(code[4:], 0xd65f03c0)
		em.LoadProgram(0x1000, code[:])
		em.Regs.PC = 0x1000
		em.Regs.WriteX(30, 0xffff)
		em.SetHaltPC(0xffff)
		comps = append(comps, modeling.MakeBuilder[*Comp](DefaultSpec()).WithSimulation(s).WithResources(Resources{Emu: em}).Build(name))
	}
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	for _, c := range comps {
		c.TickLater()
	}
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	for _, c := range comps {
		if c.State.Err != "" || !c.Resources.Emu.Halted || c.Resources.Emu.Regs.ReadX(0) != 42 || c.State.InstDone != 2 {
			t.Fatalf("unexpected result: %+v", c.State)
		}
	}
}
