package evaluation

import (
	"context"
	"crypto/sha256"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/benchmark"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
)

type Native struct {
	Raw         []uint64  `json:"raw_roi_ticks"`
	Timer       string    `json:"timer"`
	Numer       uint32    `json:"timebase_numer"`
	Denom       uint32    `json:"timebase_denom"`
	MinPositive uint64    `json:"observed_min_positive_ticks"`
	N           int       `json:"n"`
	Batch       int       `json:"batch"`
	Overhead    float64   `json:"timer_overhead_ns"`
	Samples     []float64 `json:"samples_ns"`
	X1          []float64 `json:"x1"`
	X2          []float64 `json:"x2"`
}
type PilotCase struct {
	SizeID                  string         `json:"size_id"`
	TimingQuality           []string       `json:"timing_quality_flags"`
	N                       int            `json:"n"`
	WorkingSet              uint64         `json:"working_set_bytes"`
	Status                  string         `json:"status"`
	Native                  Native         `json:"native"`
	MedianNS                float64        `json:"native_batch_mean_median_ns"`
	P10NS                   float64        `json:"native_batch_mean_p10_ns"`
	P90NS                   float64        `json:"native_batch_mean_p90_ns"`
	Instructions            uint64         `json:"instructions"`
	Cycles                  uint64         `json:"cycles"`
	CPI                     float64        `json:"cpi"`
	SimulatedNS             float64        `json:"simulated_ns"`
	AbsolutePercentageError float64        `json:"absolute_percentage_error"`
	Cache                   mem.CacheStats `json:"simulated_cache_counts"`
	BranchPredictions       uint64         `json:"simulated_branch_predictions"`
	BranchHits              uint64         `json:"simulated_branch_direction_hits"`
	OutputSHA               string         `json:"simulated_output_sha256"`
	SimWallSeconds          float64        `json:"simulator_wall_seconds"`
}
type PilotReport struct {
	ErrorFormula  string            `json:"error_formula"`
	Dirty         bool              `json:"working_tree_dirty"`
	HarnessFlags  []string          `json:"harness_link_flags"`
	LoadBefore    string            `json:"host_load_before"`
	LoadAfter     string            `json:"host_load_after"`
	SchemaVersion int               `json:"schema_version"`
	Commit        string            `json:"commit"`
	Created       string            `json:"created_utc"`
	Status        string            `json:"status"`
	Hardware      map[string]string `json:"hardware"`
	Compiler      string            `json:"compiler"`
	Flags         []string          `json:"kernel_flags"`
	Hashes        map[string]string `json:"sha256"`
	KernelHex     string            `json:"kernel_hex"`
	GoVersion     string            `json:"go_version"`
	Profile       ooo.Spec          `json:"cpu_spec"`
	CacheConfig   mem.CacheConfig   `json:"cache_config"`
	AssumedHz     float64           `json:"assumed_hz"`
	Policy        string            `json:"measurement_policy"`
	Limitations   []string          `json:"limitations"`
	Cases         []PilotCase       `json:"cases"`
}

func hash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func command(ctx context.Context, name string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, name, args...)
	b, e := c.CombinedOutput()
	if e != nil {
		return nil, fmt.Errorf("%s failed: %w: %s", name, e, b)
	}
	return b, nil
}
func FunctionBytes(path string) ([]byte, error) {
	f, e := macho.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	if f.Cpu != macho.CpuArm64 || f.Symtab == nil {
		return nil, fmt.Errorf("need arm64 Mach-O symbol table")
	}
	sec := f.Section("__text")
	if sec == nil {
		return nil, fmt.Errorf("no text")
	}
	raw, e := sec.Data()
	if e != nil {
		return nil, e
	}
	start, end := uint64(0), sec.Addr+sec.Size
	found := false
	for _, s := range f.Symtab.Syms {
		if s.Name == "_kernel_mvt" {
			start = s.Value
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("missing kernel symbol")
	}
	for _, s := range f.Symtab.Syms {
		if s.Value > start && s.Value < end && s.Sect > 0 && int(s.Sect) <= len(f.Sections) && f.Sections[s.Sect-1] == sec {
			end = s.Value
		}
	}
	if start < sec.Addr || end > sec.Addr+uint64(len(raw)) || end <= start {
		return nil, fmt.Errorf("invalid function bounds")
	}
	// Stop at the last RET: alignment padding can differ after linking.
	b := raw[start-sec.Addr : end-sec.Addr]
	last := -1
	for i := 0; i+4 <= len(b); i += 4 {
		if b[i] == 0xc0 && b[i+1] == 0x03 && b[i+2] == 0x5f && b[i+3] == 0xd6 {
			last = i + 4
		}
	}
	if last < 0 {
		return nil, fmt.Errorf("no return in kernel")
	}
	return append([]byte(nil), b[:last]...), nil
}
func Pilot(ctx context.Context, root, commit string, sizes []int, reps int) (PilotReport, error) {
	r := PilotReport{ErrorFormula: AccuracyFormula, SchemaVersion: 1, Commit: commit, Created: time.Now().UTC().Format(time.RFC3339), Status: "fail", GoVersion: runtime.Version(), Hardware: map[string]string{"core_type": "unknown", "core_affinity": "not pinned"}, Hashes: map[string]string{}, Profile: ooo.DefaultSpec(), CacheConfig: mem.DefaultCacheConfig(), AssumedHz: float64(ooo.DefaultSpec().Freq),
		Policy:      "Function-level MVT, initialized deterministic data; native warm-up traversal then reset outside each individually timed ROI, batches sum ROI times; simulator initializes data and warms tags in allocation order, then clears counters. Times are cycles / assumed frequency. No frequency or P/E correction.",
		Limitations: []string{"Provisional evaluation pipeline evidence, not a calibrated/validated hardware profile.", "Native core identity and instantaneous frequency are unknown; hardware counters unavailable.", "Native and simulated cache-start states are not proven equivalent, especially above cache capacity.", "Function bytes match, but native and simulated virtual addresses/alignment need not match.", "Legacy MVT informed earlier model development; these are future-calibration cases, not independent holdouts.", "Timer overhead is reported, not subtracted; very short ROIs have limited timing precision."}}
	dirty, e := WorkingTreeDirty(root)
	if e != nil {
		return r, e
	}
	r.Dirty = dirty
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return r, fmt.Errorf("native pilot requires darwin/arm64")
	}
	if reps < 5 || reps > 101 || len(sizes) == 0 {
		return r, fmt.Errorf("invalid repetitions/sizes")
	}
	seen := map[int]bool{}
	for _, n := range sizes {
		if n < 1 || n > 1536 || seen[n] {
			return r, fmt.Errorf("invalid or duplicate size %d", n)
		}
		seen[n] = true
	}
	for k, args := range map[string][]string{"cpu": {"sysctl", "-n", "machdep.cpu.brand_string"}, "model": {"sysctl", "-n", "hw.model"}, "memory_bytes": {"sysctl", "-n", "hw.memsize"}, "os": {"sw_vers", "-productVersion"}} {
		b, e := command(ctx, args[0], args[1:]...)
		if e != nil {
			return r, e
		}
		r.Hardware[k] = strings.TrimSpace(string(b))
	}
	b, e := command(ctx, "clang", "--version")
	if e != nil {
		return r, e
	}
	r.Compiler = strings.TrimSpace(string(b))
	temp, e := os.MkdirTemp("", "poros-mvt-")
	if e != nil {
		return r, e
	}
	defer os.RemoveAll(temp)
	src := filepath.Join(root, "polybench/kernels/kernel_mvt.c")
	driver := filepath.Join(root, "evaluation/native/mvt_driver.c")
	obj := filepath.Join(temp, "kernel.o")
	binary := filepath.Join(temp, "mvt")
	r.HarnessFlags = []string{"-O2"}
	r.Flags = []string{"-O1", "-fno-vectorize", "-fno-slp-vectorize", "-fno-unroll-loops", "-fno-stack-protector"}
	args := append(append([]string{}, r.Flags...), "-c", src, "-o", obj)
	if _, e = command(ctx, "clang", args...); e != nil {
		return r, e
	}
	if _, e = command(ctx, "clang", "-O2", driver, obj, "-o", binary); e != nil {
		return r, e
	}
	code, e := matchingFunctionBytes(obj, binary)
	if e != nil {
		return r, e
	}
	r.KernelHex = hex.EncodeToString(code)
	r.Hashes["executed_kernel"] = hash(code)
	for name, path := range map[string]string{"kernel_source": src, "harness_source": driver, "manifest": filepath.Join(root, "evaluation/workloads.json"), "evaluation_source": filepath.Join(root, "internal/evaluation/pilot.go"), "object": obj, "native_executable": binary} {
		data, e := os.ReadFile(path)
		if e != nil {
			return r, e
		}
		r.Hashes[name] = hash(data)
	}
	// Collect all native samples before starting expensive simulation.
	load, e := command(ctx, "sysctl", "-n", "vm.loadavg")
	if e != nil {
		return r, e
	}
	r.LoadBefore = strings.TrimSpace(string(load))
	for _, n := range sizes {
		b, e := command(ctx, binary, fmt.Sprint(n), fmt.Sprint(reps))
		if e != nil {
			return r, e
		}
		var native Native
		if e = Decode(strings.NewReader(string(b)), &native); e != nil {
			return r, e
		}
		if native.N != n || len(native.Samples) != reps || native.Batch < 1 || len(native.Raw) != reps*native.Batch || native.Timer != "mach_absolute_time" || native.Denom == 0 || native.Numer == 0 || native.MinPositive == 0 {
			return r, fmt.Errorf("incomplete native samples")
		}
		if e = CheckMVT(n, native.X1, native.X2); e != nil {
			return r, e
		}
		for j, v := range native.Samples {
			var total uint64
			for _, tick := range native.Raw[j*native.Batch : (j+1)*native.Batch] {
				total += tick
			}
			mean := float64(total) * float64(native.Numer) / float64(native.Denom) / float64(native.Batch)
			if math.Abs(mean-v) > 1e-9*math.Max(1, v) {
				return r, fmt.Errorf("raw sample/mean mismatch")
			}
			if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
				return r, fmt.Errorf("invalid timing")
			}
		}
		sorted := append([]float64(nil), native.Samples...)
		sort.Float64s(sorted)
		r.Cases = append(r.Cases, PilotCase{SizeID: fmt.Sprintf("n%d", n), N: n, WorkingSet: uint64(8 * (n*n + 4*n)), Native: native, MedianNS: quantile(sorted, .5), P10NS: quantile(sorted, .1), P90NS: quantile(sorted, .9), Status: "pending"})
	}
	load, e = command(ctx, "sysctl", "-n", "vm.loadavg")
	if e != nil {
		return r, e
	}
	r.LoadAfter = strings.TrimSpace(string(load))
	for i := range r.Cases {
		c := &r.Cases[i]
		quantum := float64(c.Native.MinPositive) * float64(c.Native.Numer) / float64(c.Native.Denom)
		if c.MedianNS < 10*math.Max(quantum, c.Native.Overhead) {
			c.TimingQuality = append(c.TimingQuality, "roi-too-short-relative-to-clock")
		}
		if (c.P90NS-c.P10NS)/c.MedianNS > 0.2 {
			c.TimingQuality = append(c.TimingQuality, "batch-mean-spread-over-20-percent")
		}
		c.TimingQuality = append(c.TimingQuality, "core-and-frequency-unconfirmed", "cache-state-not-equivalent")
		if e = simulate(ctx, code, &r.Cases[i]); e != nil {
			return r, e
		}
	}
	r.Status = "pass"
	return r, nil
}
func quantile(sorted []float64, q float64) float64 {
	index := q * float64(len(sorted)-1)
	lo := int(index)
	hi := int(math.Ceil(index))
	return sorted[lo] + (sorted[hi]-sorted[lo])*(index-float64(lo))
}
func InitialMVT(n int) (x1, x2, y1, y2, a []float64) {
	x1 = make([]float64, n)
	x2 = make([]float64, n)
	y1 = make([]float64, n)
	y2 = make([]float64, n)
	a = make([]float64, n*n)
	for i := 0; i < n; i++ {
		x1[i] = float64(i%n) / float64(n)
		x2[i] = float64((i+1)%n) / float64(n)
		y1[i] = float64((i+3)%n) / float64(n)
		y2[i] = float64((i+4)%n) / float64(n)
		for j := 0; j < n; j++ {
			a[i*n+j] = float64((i*j)%n) / float64(n)
		}
	}
	return
}
func CheckMVT(n int, x1, x2 []float64) error {
	if n < 1 || n > 1536 || len(x1) != n || len(x2) != n {
		return fmt.Errorf("invalid output dimensions")
	}
	want1, want2, y1, y2, a := InitialMVT(n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			want1[i] = math.FMA(a[i*n+j], y1[j], want1[i])
			want2[i] = math.FMA(a[j*n+i], y2[j], want2[i])
		}
	}
	for k, pair := range [][2][]float64{{x1, want1}, {x2, want2}} {
		for i, v := range pair[0] {
			want := pair[1][i]
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v-want) > 1e-12+1e-12*math.Abs(want) {
				return fmt.Errorf("output x%d[%d]=%.17g, expected %.17g", k+1, i, v, want)
			}
		}
	}
	return nil
}
func simulate(ctx context.Context, code []byte, c *PilotCase) (err error) {
	start := time.Now()
	n := c.N
	if n < 1 || n > 1536 || len(code) == 0 || len(code) > 65536 {
		return fmt.Errorf("invalid pilot bounds")
	}
	em := emu.NewEmulator()
	em.LoadProgram(0x10000, code)
	x1, x2, y1, y2, a := InitialMVT(n)
	data := [][]float64{x1, x2, y1, y2, a}
	regions := []benchmark.DataRegion{}
	addr := uint64(0x100000)
	addresses := []uint64{}
	for _, values := range data {
		addr = (addr + 63) &^ 63
		addresses = append(addresses, addr)
		for i, v := range values {
			em.Mem.WriteF64(addr+uint64(i*8), v)
		}
		regions = append(regions, benchmark.DataRegion{Addr: addr, Size: len(values) * 8})
		addr += uint64(len(values)) * 8
	}
	stack := (addr + 4095) &^ 4095
	em.LoadProgram(stack, make([]byte, 8192))
	em.Regs.SP = stack + 8192
	em.Regs.PC = 0x10000
	em.Regs.WriteX(0, uint64(n))
	for i, v := range addresses {
		em.Regs.WriteX(i+1, v)
	}
	em.Regs.WriteX(30, 0xffff0000)
	em.SetHaltPC(0xffff0000)
	em.Mem.TrackAccesses = true
	cache := mem.NewCache(mem.DefaultCacheConfig())
	benchmark.WarmCache(cache, regions)
	cache.ResetStats()
	s := sim.MakeBuilder().Build()
	defer func() {
		if e := s.Terminate(); err == nil {
			err = e
		}
	}()
	core := modeling.MakeBuilder[*ooo.Comp](ooo.DefaultSpec()).WithSimulation(s).WithResources(ooo.Resources{Emu: em, Cache: cache}).Build("EvaluationMVT")
	if err = s.Initialize(); err != nil {
		return err
	}
	core.TickLater()
	// Run the initialized serial engine in bounded chunks. This also bounds malformed
	// kernel execution without modifying the shared timing component.
	engine, ok := s.Engine().(*timing.SerialEngine)
	if !ok {
		return fmt.Errorf("pilot requires serial engine")
	}
	limit := uint64(100*n*n + 1000000)
	for cycle := uint64(100000); ; cycle += 100000 {
		if err = ctx.Err(); err != nil {
			return err
		}
		if cycle > limit {
			cycle = limit
		}
		if err = engine.RunUntil(timing.VTimeInPicoSec(cycle) * core.Spec.Freq.Period()); err != nil {
			return err
		}
		if core.State.Done {
			break
		}
		if cycle == limit {
			return fmt.Errorf("simulation cycle budget exhausted")
		}
	}
	if core.State.Err != "" || !em.Halted {
		return fmt.Errorf("simulation failed: %s", core.State.Err)
	}
	for i := 0; i < n; i++ {
		x1[i] = em.Mem.ReadF64(addresses[0] + uint64(i*8))
		x2[i] = em.Mem.ReadF64(addresses[1] + uint64(i*8))
	}
	if err = CheckMVT(n, x1, x2); err != nil {
		return err
	}
	c.Instructions = core.State.InstDone
	c.Cycles = core.State.Cycles
	c.CPI = core.State.CPI()
	c.SimulatedNS = float64(c.Cycles) * 1e9 / float64(core.Spec.Freq)
	c.AbsolutePercentageError, err = AccuracyError(c.SimulatedNS, c.MedianNS)
	if err != nil {
		return err
	}
	c.Cache = cache.Stats()
	c.BranchPredictions = core.State.Predictor.Predictions
	c.BranchHits = core.State.Predictor.Hits
	b, _ := json.Marshal([][]float64{x1, x2})
	c.OutputSHA = hash(b)
	c.SimWallSeconds = time.Since(start).Seconds()
	c.Status = "pass"
	return nil
}

func matchingFunctionBytes(object, binary string) ([]byte, error) {
	code, e := FunctionBytes(object)
	if e != nil {
		return nil, e
	}
	linked, e := FunctionBytes(binary)
	if e != nil {
		return nil, e
	}
	if string(code) != string(linked) {
		return nil, fmt.Errorf("linked/object kernel bytes differ")
	}
	return code, nil
}
