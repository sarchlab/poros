package cpu

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"github.com/sarchlab/poros/pkg/mem"
)

// Complete guest branch execution must drain with prediction both enabled and
// disabled. This replaces the old whole-kernel variant golden while preserving
// the specific component configuration and accounting checks it exercised.
func TestELFBranchPredictionVariants(t *testing.T) {
	image, err := os.ReadFile("../../tests/usertime/branch.elf")
	if err != nil {
		t.Fatal(err)
	}
	var want []byte
	var instructions uint64
	for _, enabled := range []bool{true, false} {
		simulation := sim.MakeBuilder().Build()
		p, err := linuxuser.Parse(image, linuxuser.Options{Args: []string{"branch", "1024"}})
		if err != nil {
			t.Fatal(err)
		}
		execution, err := p.NewExecution(context.Background(), linuxuser.RunOptions{MaxInstructions: 20000, Timeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		spec := ooo.DefaultSpec()
		spec.BranchPrediction = enabled
		core := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(simulation).WithResources(ooo.Resources{Execution: execution, Cache: mem.NewCache(mem.DefaultCacheConfig())}).Build("CPU")
		if err := simulation.Initialize(); err != nil {
			t.Fatal(err)
		}
		core.TickLater()
		if err := simulation.Run(); err != nil {
			t.Fatal(err)
		}
		r := execution.Result()
		execution.Close()
		if err := simulation.Terminate(); err != nil {
			t.Fatal(err)
		}
		if r.Termination != "exit" || r.ExitStatus == nil || *r.ExitStatus != 0 || core.State.Err != "" || core.State.InstDone != r.Instructions {
			t.Fatalf("incomplete branch variant %+v", r)
		}
		if enabled {
			want = r.Stdout
			instructions = r.Instructions
			if core.State.Predictor.Predictions == 0 {
				t.Fatal("prediction enabled but no predictions")
			}
		} else {
			if core.State.Predictor.Predictions != 0 {
				t.Fatal("prediction disabled but counts increased")
			}
			if !bytes.Equal(r.Stdout, want) || r.Instructions != instructions {
				t.Fatal("prediction changed architecture")
			}
		}
		for _, ready := range core.State.GprReady {
			if ready > core.State.Cycles {
				t.Fatal("GPR scoreboard did not drain")
			}
		}
	}
}
func TestMultiCoreRejectsInvalidResources(t *testing.T) {
	image, err := os.ReadFile("../../tests/elf/hello.elf")
	if err != nil {
		t.Fatal(err)
	}
	p, err := linuxuser.Parse(image, linuxuser.Options{})
	if err != nil {
		t.Fatal(err)
	}
	x, err := p.NewExecution(context.Background(), linuxuser.RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	for _, configs := range [][]CoreConfig{nil, {{}}, {{Execution: x, FuncHookCycles: map[uint64]int{1: 2}}}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("accepted invalid multicore resources")
				}
			}()
			NewMultiCorePipeline("bad", nil, 1, configs)
		}()
	}
}
