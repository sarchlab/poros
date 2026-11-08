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
)

type Options struct {
	Model         string
	TimingProfile *TimingProfile
	// FrequencyHz is integer CPU Hz in [1 MHz, 1 THz]; zero selects 3.49 GHz.
	FrequencyHz float64
	Limits      linuxuser.RunOptions
}
type Statistics struct {
	ReadFill               *mem.StreamStats `json:"read_fill,omitempty"`
	Model                  string           `json:"model"`
	AssumedHz              float64          `json:"assumed_hz"`
	Cycles                 uint64           `json:"user_cycles"`
	Retired                uint64           `json:"retired_instructions"`
	CPI                    float64          `json:"cpi"`
	PredictedUserSeconds   float64          `json:"predicted_user_seconds"`
	Cache                  mem.CacheStats   `json:"cache"`
	CacheConfig            mem.CacheConfig  `json:"cache_config"`
	BranchMetricsAvailable bool             `json:"branch_metrics_available"`
	BranchPredictions      uint64           `json:"branch_predictions"`
	BranchDirectionHits    uint64           `json:"branch_direction_hits"`
	Configuration          any              `json:"configuration"`
	Policy                 string           `json:"policy"`
}
type Result struct {
	linuxuser.Result
	Timing Statistics `json:"timing"`
}

func Run(ctx context.Context, p *linuxuser.Process, o Options) (r Result, err error) {
	if o.Model == "" {
		o.Model = "ooo"
	}
	if o.FrequencyHz == 0 {
		o.FrequencyHz = 3.49e9
	}
	if (o.Model != "ooo" && o.Model != "inorder") || math.IsNaN(o.FrequencyHz) || math.IsInf(o.FrequencyHz, 0) || o.FrequencyHz < 1e6 || o.FrequencyHz > 1e12 || math.Trunc(o.FrequencyHz) != o.FrequencyHz {
		return r, fmt.Errorf("invalid timing model/frequency")
	}
	profile := DefaultTimingProfile(timing.Freq(o.FrequencyHz))
	if o.TimingProfile != nil {
		if o.Model != "ooo" {
			return r, fmt.Errorf("timing profiles require the OoO model")
		}
		profile = *o.TimingProfile
		if err := profile.Validate(); err != nil {
			return r, err
		}
		if profile.Configuration.Freq != timing.Freq(o.FrequencyHz) {
			return r, fmt.Errorf("timing profile frequency differs from requested CPU frequency")
		}
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
	cache := mem.NewCache(profile.CacheConfig)
	r.Timing = Statistics{Model: o.Model, AssumedHz: o.FrequencyHz, CacheConfig: cache.Config(), Policy: "whole ELF user execution; cold modeled cache; SVC serializes, counts as one user instruction; host syscall work and loader excluded; no kernel cache effects; not a validated hardware profile"}
	var pipelineError string
	if o.Model == "ooo" {
		spec := profile.Configuration
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
		if spec.StreamPrefetch.Enabled {
			stats := c.State.Stream.Stats
			r.Timing.ReadFill = &stats
		}
		r.Timing.BranchMetricsAvailable = true
		r.Timing.BranchPredictions = c.State.Predictor.Predictions
		r.Timing.BranchDirectionHits = c.State.Predictor.Hits
		pipelineError = c.State.Err
	} else {
		spec := inorder.DefaultSpec()
		spec.Freq = timing.Freq(o.FrequencyHz)
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
