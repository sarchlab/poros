package demo

import (
	"testing"

	"github.com/sarchlab/akita/v4/sim"
)

func TestCounter_ReachesTarget(t *testing.T) {
	engine := sim.NewSerialEngine()

	counter := NewCounter("TestCounter", engine, 1*sim.GHz, 10)
	counter.TickLater()

	err := engine.Run()
	if err != nil {
		t.Fatalf("engine.Run() failed: %v", err)
	}

	if counter.Current != 10 {
		t.Errorf("expected counter to reach 10, got %d", counter.Current)
	}
}

func TestCounter_ZeroTarget(t *testing.T) {
	engine := sim.NewSerialEngine()

	counter := NewCounter("ZeroCounter", engine, 1*sim.GHz, 0)
	counter.TickLater()

	err := engine.Run()
	if err != nil {
		t.Fatalf("engine.Run() failed: %v", err)
	}

	if counter.Current != 0 {
		t.Errorf("expected counter to stay at 0, got %d", counter.Current)
	}
}
