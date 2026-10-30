package darwinuser

import (
	"context"
	"github.com/sarchlab/poros/internal/machotest"
	"github.com/sarchlab/poros/pkg/emu"
	"testing"
)

func TestQuiescenceIsExplicitVirtualSchedulerState(t *testing.T) {
	p := process(t, Options{Quiescence: QuiescenceNoScheduling})
	var counter [8]byte
	if p.Memory.Read(0xfffffc180, counter[:]) != nil || counter != [8]byte{} {
		t.Fatal("wrong virtual initial epoch")
	}
	if p.Memory.Write(0xfffffc180, make([]byte, 8)) == nil || p.Memory.Read(0xfffffc188, make([]byte, 1)) == nil {
		t.Fatal("counter mapping expanded or writable")
	}
	// Neither user instructions nor a non-scheduling kernel query fabricate a
	// global quiescence event. Host elapsed time is not consulted either.
	x, e := p.NewExecution(context.Background(), RunOptions{})
	if e != nil {
		t.Fatal(e)
	}
	defer x.Close()
	if _, _, e = x.StepDetailed(); e != nil {
		t.Fatal(e)
	}
	r := emu.Registers{}
	r.GPR[16] = 20
	if !p.kernel.trap(&r, p.Memory) {
		t.Fatal("getpid")
	}
	if p.Memory.Read(0xfffffc180, counter[:]) != nil || counter != [8]byte{} {
		t.Fatal("fabricated quiescence event")
	}
	absent := process(t, Options{})
	if absent.Memory.Read(0xfffffc180, counter[:]) == nil {
		t.Fatal("unconfigured counter guessed")
	}
	if _, e := Parse(machotest.Image(2), machotest.Image(7), Options{Quiescence: "instruction-ticks"}); e == nil {
		t.Fatal("arbitrary counter policy accepted")
	}
}

func TestNativeOnceGenerationProgressAndNoProgress(t *testing.T) {
	var native struct {
		Cases []struct {
			CounterBefore uint64 `json:"counter_before"`
			CounterAfter  uint64 `json:"counter_after"`
			Before        uint64 `json:"predicate_before,string"`
			After         uint64 `json:"predicate_after,string"`
			PreviousCalls uint64 `json:"calls_before"`
			CurrentCalls  uint64 `json:"calls_after"`
		}
	}
	kernelNativeResult(t, "once-quiescence-probe.c", &native)
	if len(native.Cases) != 14 {
		t.Fatal(len(native.Cases))
	}
	stable, advanced := false, false
	for i, c := range native.Cases {
		if c.CurrentCalls != 1 || c.CounterAfter < c.CounterBefore {
			t.Fatal("once repeated or counter reversed", c)
		}
		if i == 0 {
			if c.PreviousCalls != 0 || c.Before != 0 || c.After&3 != 1 || c.After>>2 < c.CounterBefore || c.After>>2 > c.CounterAfter {
				t.Fatal("initial generation", c)
			}
			continue
		}
		if c.PreviousCalls != 1 {
			t.Fatal("callback count", c)
		}
		if c.Before == ^uint64(0) {
			if c.After != c.Before {
				t.Fatal("DONE changed")
			}
			continue
		}
		if c.CounterBefore == c.CounterAfter && c.CounterAfter-(c.Before>>2) < 4 {
			if c.After != c.Before {
				t.Fatal("unquiesced predicate completed")
			}
			stable = true
		}
		if c.After == ^uint64(0) {
			if c.CounterAfter-(c.Before>>2) < 4 {
				t.Fatal("premature completion")
			}
			advanced = true
		}
	}
	if !stable || !advanced {
		t.Fatal("native observation lacks both transitions")
	}
}

func TestNativeOnceCompletedGenerationThreshold(t *testing.T) {
	var native struct {
		Cases []struct {
			Delta, Attempts uint64
			BeforeCounter   uint64 `json:"counter_before"`
			AfterCounter    uint64 `json:"counter_after"`
			Before          uint64 `json:"predicate_before,string"`
			After           uint64 `json:"predicate_after,string"`
			Calls           uint64 `json:"callback_calls"`
		} `json:"synthetic_completed_generation_cases"`
	}
	kernelNativeResult(t, "once-threshold-probe.c", &native)
	if len(native.Cases) != 7 {
		t.Fatal(len(native.Cases))
	}
	for _, c := range native.Cases {
		if c.BeforeCounter != c.AfterCounter || c.Before&3 != 1 || c.BeforeCounter-(c.Before>>2) != c.Delta || c.Calls != 0 || c.Attempts < 1 || c.Attempts > 100 {
			t.Fatal(c)
		}
		want := c.Before
		if c.Delta >= 4 {
			want = ^uint64(0)
		}
		if c.After != want {
			t.Fatal(c)
		}
	}
}
