package demo

import (
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/modeling/modelingtest"
)

func TestCounter_ReachesTarget(t *testing.T) { checkTarget(t, 10) }
func TestCounter_ZeroTarget(t *testing.T)    { checkTarget(t, 0) }
func checkTarget(t *testing.T, target int) {
	t.Helper()
	s := sim.MakeBuilder().Build()
	spec := DefaultSpec()
	spec.Target = target
	c := modeling.MakeBuilder[*Comp](spec).WithSimulation(s).Build("Counter")
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Terminate(); err != nil {
			t.Error(err)
		}
	})
	c.TickLater()
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	if c.State.Current != target {
		t.Fatalf("got %d, want %d", c.State.Current, target)
	}
}
func TestModel(t *testing.T) { modelingtest.CheckTicking(t, (*Comp)(nil), DefaultSpec()) }
