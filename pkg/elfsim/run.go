// Package elfsim connects a Linux user process to the existing CPU timing models.
package elfsim

import (
	"context"
	"fmt"
	"math"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/cpu"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"github.com/sarchlab/poros/pkg/mem"
	"github.com/sarchlab/poros/pkg/target"
)

type Options struct {
	Model string
	// Target names the modeled device; empty selects target.Default.
	Target string
	// FrequencyHz is integer CPU Hz in [1 MHz, 1 THz]; zero selects the
	// target's own frequency.
	FrequencyHz float64
	Limits      linuxuser.RunOptions
}
type Statistics struct {
	Model                  string          `json:"model"`
	Target                 target.Identity `json:"target"`
	AssumedHz              float64         `json:"assumed_hz"`
	Cycles                 uint64          `json:"user_cycles"`
	Retired                uint64          `json:"retired_instructions"`
	CPI                    float64         `json:"cpi"`
	PredictedUserSeconds   float64         `json:"predicted_user_seconds"`
	Cache                  mem.CacheStats  `json:"cache"`
	CacheConfig            mem.CacheConfig `json:"cache_config"`
	BranchMetricsAvailable bool            `json:"branch_metrics_available"`
	BranchPredictions      uint64          `json:"branch_predictions"`
	BranchDirectionHits    uint64          `json:"branch_direction_hits"`
	Configuration          any             `json:"configuration"`
	Policy                 string          `json:"policy"`
}
type Result struct {
	linuxuser.Result
	Timing Statistics `json:"timing"`
}

// configure resolves the requested device and clock. An unknown target is an
// error; a zero frequency keeps the clock of the device's selected model.
func configure(o *Options) (target.Target, error) {
	if o.Model == "" {
		o.Model = "ooo"
	}
	if o.Model != "ooo" && o.Model != "inorder" {
		return target.Target{}, fmt.Errorf("invalid timing model/frequency")
	}
	if o.Target == "" {
		o.Target = target.Default
	}
	t, err := target.Lookup(o.Target)
	if err != nil {
		return t, err
	}
	if o.FrequencyHz == 0 {
		o.FrequencyHz = float64(t.OoO.Freq)
		if o.Model == "inorder" {
			o.FrequencyHz = float64(t.InOrder.Freq)
		}
	}
	if math.IsNaN(o.FrequencyHz) || math.IsInf(o.FrequencyHz, 0) || o.FrequencyHz < 1e6 || o.FrequencyHz > 1e12 || math.Trunc(o.FrequencyHz) != o.FrequencyHz {
		return t, fmt.Errorf("invalid timing model/frequency")
	}
	return t.WithFrequency(timing.Freq(o.FrequencyHz)), nil
}

func Run(ctx context.Context, p *linuxuser.Process, o Options) (r Result, err error) {
	device, err := configure(&o)
	if err != nil {
		return r, err
	}
	x, err := p.NewExecution(ctx, o.Limits)
	if err != nil {
		return r, err
	}
	defer x.Close()
	s := sim.MakeBuilder().Build()
	defer func() {
		if e := s.Terminate(); err == nil {
			err = e
		}
	}()
	cache := mem.NewCache(device.Cache)
	r.Timing = Statistics{Model: o.Model, Target: device.Identity, AssumedHz: o.FrequencyHz, CacheConfig: cache.Config(), Policy: "whole ELF user execution; cold modeled cache; SVC serializes, counts as one user instruction; host syscall work and loader excluded; no kernel cache effects; not a validated hardware profile"}
	var pipelineError string
	if o.Model == "ooo" {
		spec := device.OoO
		c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Execution: cpu.NewELFTimingExecution(x), Cache: cache}).Build("CPU")
		if err = s.Initialize(); err != nil {
			return r, err
		}
		c.TickLater()
		if err = s.Run(); err != nil {
			return r, err
		}
		r.Timing.Cycles = c.State.Cycles
		r.Timing.Retired = c.State.InstDone
		r.Timing.CPI = c.State.CPI()
		r.Timing.Configuration = spec
		r.Timing.BranchMetricsAvailable = true
		r.Timing.BranchPredictions = c.State.Predictor.Predictions
		r.Timing.BranchDirectionHits = c.State.Predictor.Hits
		pipelineError = c.State.Err
	} else {
		spec := device.InOrder
		c := modeling.MakeBuilder[*inorder.Comp](spec).WithSimulation(s).WithResources(inorder.Resources{Execution: x, Cache: cache}).Build("CPU")
		if err = s.Initialize(); err != nil {
			return r, err
		}
		c.TickLater()
		if err = s.Run(); err != nil {
			return r, err
		}
		r.Timing.Cycles = c.State.Cycles
		r.Timing.Retired = c.State.InstDone
		r.Timing.CPI = c.State.CPI()
		r.Timing.Configuration = spec
		pipelineError = c.State.Err
	}
	r.Result = x.Result()
	r.Timing.Cache = cache.Stats()
	r.Timing.PredictedUserSeconds = float64(r.Timing.Cycles) / o.FrequencyHz
	if !x.Stopped() {
		return r, fmt.Errorf("pipeline stopped before process: %s", pipelineError)
	}
	if r.Timing.Retired != r.Instructions {
		return r, fmt.Errorf("retirement mismatch: pipeline %d process %d", r.Timing.Retired, r.Instructions)
	}
	return r, nil
}
