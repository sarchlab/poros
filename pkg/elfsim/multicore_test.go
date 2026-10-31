package elfsim

import (
	"context"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"os"
	"testing"
	"time"
)

func TestELFMultiCoreLifecycle(t *testing.T) {
	image, err := os.ReadFile("../../tests/elf/hello.elf")
	if err != nil {
		t.Fatal(err)
	}
	makeProcesses := func(n int) []*linuxuser.Process {
		p := make([]*linuxuser.Process, n)
		for i := range p {
			p[i], err = linuxuser.Parse(image, linuxuser.Options{Args: []string{"hello"}})
			if err != nil {
				t.Fatal(err)
			}
		}
		return p
	}
	for _, limit := range []uint64{10000, 10} {
		r, err := RunMultiCore(context.Background(), makeProcesses(2), Options{Limits: linuxuser.RunOptions{MaxInstructions: limit, Timeout: time.Second}})
		if limit == 10000 && err != nil {
			t.Fatal(err)
		}
		if limit == 10 && err == nil {
			t.Fatal("multicore instruction-limit termination must return an error")
		}
		var sum, max uint64
		for _, c := range r.Cores {
			sum += c.Instructions
			if c.Timing.Cycles > max {
				max = c.Timing.Cycles
			}
			if c.Instructions != c.Timing.Retired {
				t.Fatal("incorrect retire boundary")
			}
			if limit == 10000 && (c.Termination != "exit" || string(c.Stdout) != "hello, ELF\n") {
				t.Fatalf("bad process result %+v", c)
			}
			if limit == 10 && c.Termination != "instruction_limit" {
				t.Fatal("missing limit termination", c.Termination)
			}
		}
		if len(r.Cores) != 2 || sum != r.TotalInstructions || max != r.TotalCycles {
			t.Fatalf("incorrect aggregates %+v", r)
		}
	}
	processes := makeProcesses(1)
	if _, err := RunMultiCore(context.Background(), []*linuxuser.Process{processes[0], processes[0]}, Options{}); err == nil {
		t.Fatal("accepted shared mutable process")
	}
	if _, err := RunMultiCore(context.Background(), nil, Options{}); err == nil {
		t.Fatal("accepted no cores")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, _ := RunMultiCore(ctx, makeProcesses(2), Options{})
	if len(r.Cores) != 2 || r.TotalInstructions != 0 {
		t.Fatalf("canceled process retired instructions %+v", r)
	}
}
