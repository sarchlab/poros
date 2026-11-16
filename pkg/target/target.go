// Package target names the hardware devices the simulator can be asked to model
// and builds the complete timing configuration of each.
package target

import (
	"fmt"
	"strings"

	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/mem"
)

// Default is the target modeled when none is requested.
const Default = "m2"

// Identity is the part of a Target reported with every timing result.
type Identity struct {
	Name     string `json:"name"`
	Hardware string `json:"hardware"`
	// ParameterSource names the target whose parameters the configuration
	// carries. A target whose own parameters have not been established names
	// another target here: its predictions are then that target's, and say
	// nothing about the named hardware.
	ParameterSource string `json:"parameter_source"`
}

// Target is the timing configuration of one device: both pipeline models and
// the cache hierarchy, including their clock.
type Target struct {
	Identity
	OoO     ooo.Spec
	InOrder inorder.Spec
	Cache   mem.CacheConfig
}

// builders lists every target in display order. Each entry composes its own
// configuration; it may start from another target's and replace any part.
var builders = []func() Target{M2, M1, Grace, M5Max}

// M2 is the calibrated Apple M2 performance-core configuration, the defaults of
// the pipeline and cache packages.
func M2() Target {
	return Target{
		Identity: Identity{Name: "m2", Hardware: "Apple M2", ParameterSource: "m2"},
		OoO:      ooo.DefaultSpec(),
		InOrder:  inorder.DefaultSpec(),
		Cache:    mem.DefaultCacheConfig(),
	}
}

// M1 is the Apple M1 performance core: the M2 configuration with the
// differences M1 measurements establish (evaluation/model-profile.md):
//   - its clock, inferred from one-cycle dependency chains on the M1 host and
//     consistent with counted user cycles;
//   - fetch groups that continue across one correctly predicted taken branch,
//     which the M1 NOP and mixed-loop probes sustain at N/8 cycles per trip;
//   - store dependence prediction by static store, which serializes
//     consecutive memory-carried accumulations as M1 doitgen and 2mm show;
//   - an 18-cycle L2 hit, from the M1 pointer chases over 256 KiB and 1 MiB;
//   - a program-ordered store queue of 140 entries in which a store of an
//     interleaved run waits for its line (mem.CacheConfig
//     InterleavedStoreRunsWait): two interleaved store streams, or one stream
//     and another store per trip, cost M1 about 13 cycles per line once the
//     lines come from memory, while one, three or four streams do not. This
//     rule and its 140 are fitted to those timings; they are the model's
//     interpretation, not a measured structure of the core.
//
// Compare/select fusion is kept although the M1 register-only select probes
// do not fuse: M1 runs the memory-carried Nussinov recurrence in the same two
// cycles as its register form, which the model reproduces only with the fused
// pair and its one-cycle forwarding path.
func M1() Target {
	t := M2().as("m1", "Apple M1")
	t.ParameterSource = "m1"
	t.OoO.FetchTakenBranches = 1
	t.OoO.StoreDependencePrediction = true
	t.OoO.StoreQueueInOrder = true
	t.OoO.StoreBufferSize = 140
	t.Cache.L2Latency = 18
	t.Cache.InterleavedStoreRunsWait = true
	return t.WithFrequency(M1Frequency)
}

// M1Frequency is the M1 performance-core clock inferred on the M1 Linux host
// (3.2028 to 3.2037 GHz over nine one-cycle chain probes).
const M1Frequency = 3_203_500_000 * timing.Hz

// Grace is the NVIDIA Grace core, an Arm Neoverse V2: the M2 configuration
// with the differences listed here. evaluation/model-profile.md gives the
// source of each and separates what Arm and NVIDIA publish from what is
// inferred from the GH200 user times; nothing was measured on a Grace core for
// this model.
//
// Published (Neoverse V2 Software Optimization Guide and Technical Reference
// Manual, NVIDIA Grace datasheet):
//   - two-cycle integer multiply; one iterative integer divider that ends
//     early on a short quotient (5 to 12 cycles for 32 bits, 5 to 20 for 64);
//   - two-cycle FP add, three-cycle FP multiply, a fused multiply-add whose
//     addend is forwarded late, and two FP dividers of up to 15 cycles that
//     together start two divides in seven cycles;
//   - four-cycle integer and six-cycle FP loads on three load pipes, two of
//     which also take stores;
//   - a 64 KiB four-way L1D, a 1 MiB eight-way L2 with 64-byte lines, 114 MB
//     of L3, and a 2,048-entry eight-way second-level TLB.
//
// Observed on the measured host: 64 KiB pages and the clock the performance
// governor asks for.
//
// Inferred from application times and effective only: the store-to-load
// forwarding latency, the FP issue-queue capacity, the branch penalty, a full
// 15 cycles for every FP divide, and a prefetcher that follows constant strides
// up to a page, as many accesses ahead as the sequential one runs lines ahead
// and no farther than the walk's lines have room in L1.
//
// Everything else is M2's and is not established for Grace: the window, the
// predictor, compare/select fusion as measured on M2, the front end, the L2,
// memory-side and DRAM latencies, both fill transports, the stream prefetcher
// and the page-walk timing.
func Grace() Target {
	t := M2().as("grace", "NVIDIA Grace (Arm Neoverse V2)")
	t.ParameterSource = "grace"
	t.OoO.Latencies = ooo.Latencies{
		Multiply: 2, DivideBase: 4, DivideBitsPerCycle: 4,
		FPArithmetic: 2, FPMultiply: 3, FPAccumulate: 2, FPDivide: 15, FPDivideOccupancy: 7, FPLoadExtra: 2,
	}
	t.OoO.NumIntDiv, t.OoO.NumFPDiv = 1, 2
	t.OoO.NumLDST, t.OoO.NumLoad, t.OoO.NumStore = 3, 3, 2
	t.OoO.StoreForwardLatency = 6
	t.OoO.FPIssueQueue = 36
	t.OoO.BranchPenalty = 11
	t.Cache.L1Size, t.Cache.L1Ways, t.Cache.L1Latency = 64<<10, 4, 4
	t.Cache.L2Size, t.Cache.L2Ways, t.Cache.L2LineSize = 1<<20, 8, 64
	t.Cache.SLCSize = 114 << 20
	t.Cache.PrefetchStrideBytes = 64 << 10
	t.Cache.TLBPageBytes, t.Cache.TLBSets, t.Cache.TLBWays = 64<<10, 256, 8
	return t.WithFrequency(GraceFrequency)
}

// GraceFrequency is the maximum the measured GH200 host reports for the
// pinned core under the performance governor. The frequency the core actually
// ran at was not observed.
const GraceFrequency = 3_429_000_000 * timing.Hz

// M5Max carries the M2 core, memory and clock until its own are established.
func M5Max() Target { return M2().as("m5-max", "Apple M5 Max") }

// as names a configuration for another device. The parameter source is kept,
// so the result says whose parameters it carries.
func (t Target) as(name, hardware string) Target {
	t.Name, t.Hardware = name, hardware
	return t
}

// WithFrequency replaces the clock of both pipeline models.
func (t Target) WithFrequency(f timing.Freq) Target {
	t.OoO.Freq, t.InOrder.Freq = f, f
	return t
}

// All returns every target in display order.
func All() []Target {
	targets := make([]Target, len(builders))
	for i, build := range builders {
		targets[i] = build()
	}
	return targets
}

// Names returns every target name in display order.
func Names() []string {
	names := make([]string, len(builders))
	for i, t := range All() {
		names[i] = t.Name
	}
	return names
}

// Lookup builds the named target. Any other name, including the empty one, is
// an error and never another target; callers that have a default say Default.
func Lookup(name string) (Target, error) {
	for _, t := range All() {
		if t.Name == name {
			return t, nil
		}
	}
	return Target{}, fmt.Errorf("unknown target %q (valid: %s)", name, strings.Join(Names(), ", "))
}
