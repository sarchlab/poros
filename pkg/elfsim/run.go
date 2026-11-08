// Package elfsim connects a Linux user process to the existing CPU timing models.
package elfsim

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/messaging/twowaybuffered"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/cpu"
	"github.com/sarchlab/poros/pkg/cpu/detailed"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"github.com/sarchlab/poros/pkg/mem"
	"github.com/sarchlab/poros/pkg/mem/timed"
)

type Options struct {
	Model string
	// MemoryBackend selects akita (default for OoO) or analytical. In-order
	// and multicore retain their existing analytical memory policy.
	MemoryBackend string
	TimingProfile *TimingProfile
	// FrequencyHz is integer CPU Hz in [1 MHz, 1 THz]; zero selects 3.49 GHz.
	FrequencyHz float64
	Limits      linuxuser.RunOptions
}
type TimingProfile struct {
	Configuration detailed.Spec `json:"configuration"`
	CacheConfig   timed.Config  `json:"cache_config"`
}

func DefaultTimingProfile(freq timing.Freq) TimingProfile {
	c := detailed.DefaultSpec()
	c.Freq = freq
	return TimingProfile{Configuration: c, CacheConfig: timed.DefaultConfig(freq)}
}

type Statistics struct {
	Model                  string                   `json:"model"`
	AssumedHz              float64                  `json:"assumed_hz"`
	Cycles                 uint64                   `json:"user_cycles"`
	Retired                uint64                   `json:"retired_instructions"`
	CPI                    float64                  `json:"cpi"`
	PredictedUserSeconds   float64                  `json:"predicted_user_seconds"`
	Cache                  mem.CacheStats           `json:"cache"`
	CacheConfig            any                      `json:"cache_config"`
	MemoryBackend          string                   `json:"memory_backend"`
	CacheMetricsAvailable  bool                     `json:"cache_metrics_available"`
	MemoryRequests         *detailed.MemoryCounters `json:"memory_requests,omitempty"`
	BranchMetricsAvailable bool                     `json:"branch_metrics_available"`
	BranchPredictions      uint64                   `json:"branch_predictions"`
	BranchDirectionHits    uint64                   `json:"branch_direction_hits"`
	Configuration          any                      `json:"configuration"`
	Policy                 string                   `json:"policy"`
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
	if o.MemoryBackend == "" {
		if o.Model == "ooo" {
			o.MemoryBackend = "akita"
		} else {
			o.MemoryBackend = "analytical"
		}
	}
	if (o.MemoryBackend != "akita" && o.MemoryBackend != "analytical") || (o.Model != "ooo" && o.MemoryBackend != "analytical") || (o.TimingProfile != nil && o.MemoryBackend != "akita") {
		return r, fmt.Errorf("invalid memory backend for timing model")
	}
	if (o.Model != "ooo" && o.Model != "inorder") || math.IsNaN(o.FrequencyHz) || math.IsInf(o.FrequencyHz, 0) || o.FrequencyHz < 1e6 || o.FrequencyHz > 1e12 || math.Trunc(o.FrequencyHz) != o.FrequencyHz {
		return r, fmt.Errorf("invalid timing model/frequency")
	}
	if o.Limits.Timeout == 0 {
		o.Limits.Timeout = 2 * time.Second
	}
	if o.Limits.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Limits.Timeout)
		defer cancel()
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
	var cache *mem.Cache
	if o.MemoryBackend == "analytical" {
		cache = mem.NewCache(mem.DefaultCacheConfig())
	}
	r.Timing = Statistics{Model: o.Model, AssumedHz: o.FrequencyHz, MemoryBackend: o.MemoryBackend, CacheMetricsAvailable: o.MemoryBackend == "analytical", CacheConfig: mem.DefaultCacheConfig(), Policy: "whole ELF user execution; cold modeled cache; SVC serializes, counts as one user instruction; host syscall work and loader excluded; no kernel cache effects; not a validated hardware profile"}
	var pipelineError string
	if o.MemoryBackend == "akita" {
		profile := DefaultTimingProfile(timing.Freq(o.FrequencyHz))
		if o.TimingProfile != nil {
			profile = *o.TimingProfile
		}
		if profile.Configuration.Freq != timing.Freq(o.FrequencyHz) || profile.Configuration.LineBytes != uint64(1)<<profile.CacheConfig.L1.Log2BlockSize || profile.Configuration.Bottom != "L1.Top" {
			return r, fmt.Errorf("inconsistent CPU/memory timing profile")
		}
		if err = profile.Configuration.Validate(); err != nil {
			return r, err
		}
		if err = profile.CacheConfig.Validate(profile.Configuration.Freq); err != nil {
			return r, err
		}
		c := modeling.MakeBuilder[*detailed.Comp](profile.Configuration).WithSimulation(s).WithResources(detailed.Resources{Execution: cpu.NewELFTimingExecution(x), Context: ctx}).Build("CPU")
		c.BindPort("Memory", twowaybuffered.MakeBuilder().WithSimulation(s).WithSpec(twowaybuffered.Spec{IncomingCapacity: profile.CacheConfig.PortCapacity, OutgoingCapacity: profile.CacheConfig.PortCapacity}).Build())
		if _, err = timed.Build(s, c.Ports.Memory, profile.CacheConfig); err != nil {
			return r, err
		}
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
		r.Timing.Configuration = profile.Configuration
		r.Timing.CacheConfig = profile.CacheConfig
		r.Timing.MemoryRequests = &c.State.Memory
		r.Timing.BranchMetricsAvailable = true
		r.Timing.BranchPredictions = c.State.Predictor.Predictions
		r.Timing.BranchDirectionHits = c.State.Predictor.Hits
		pipelineError = c.State.Err
		r.Timing.Policy += "; response-driven Akita writeback caches and simple banked memory; functional values are independent of timing-only payloads; no DRAM command or prefetch model"
	} else if o.Model == "ooo" {
		spec := ooo.DefaultSpec()
		spec.Freq = timing.Freq(o.FrequencyHz)
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
	if cache != nil {
		r.Timing.Cache = cache.Stats()
	}
	r.Timing.PredictedUserSeconds = float64(r.Timing.Cycles) / o.FrequencyHz
	if !x.Stopped() {
		return r, fmt.Errorf("pipeline stopped before process: %s", pipelineError)
	}
	if r.Timing.Retired != r.Instructions && !(o.MemoryBackend == "akita" && ctx.Err() != nil && r.Termination == "time_limit") {
		return r, fmt.Errorf("retirement mismatch: pipeline %d process %d", r.Timing.Retired, r.Instructions)
	}
	return r, nil
}
