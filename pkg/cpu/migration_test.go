package cpu

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/sarchlab/akita/v5/sim"
	"github.com/sarchlab/akita/v5/sim/modeling"
	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/benchmark"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
)

type baselineRecord struct {
	Mode                 string `json:",omitempty"`
	Name                 string
	Cores                int
	Instructions, Cycles []uint64
	Digest               []string
}

// TestV4Baseline checks all registered kernels against the frozen v4 checkout.
// Long mode tests one and two cores; short mode keeps a small functional/timing
// regression and a repeated run so routine tests still exercise this contract.
func TestV4Baseline(t *testing.T) {
	f, err := os.Open("testdata/akita-v4-baseline.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	seen := map[string]bool{}
	for scanner.Scan() {
		var want baselineRecord
		if err := json.Unmarshal(scanner.Bytes(), &want); err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("%s/%d", want.Name, want.Cores)
		if seen[key] {
			t.Fatalf("duplicate baseline %s", key)
		}
		seen[key] = true
		count++
		b := benchmark.ByName(want.Name)
		if b == nil {
			t.Fatalf("unknown baseline benchmark %s", want.Name)
		}
		if testing.Short() && want.Name != "raytrace" {
			continue
		}
		t.Run(key, func(t *testing.T) {
			got := runBaseline(t, *b, want.Cores)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("v4 regression:\ngot  %+v\nwant %+v", got, want)
			}
			if want.Name == "raytrace" {
				if again := runBaseline(t, *b, want.Cores); !reflect.DeepEqual(got, again) {
					t.Fatalf("nondeterministic result: %+v vs %+v", got, again)
				}
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 2*len(benchmark.All()) {
		t.Fatalf("baseline has %d rows for %d benchmarks", count, len(benchmark.All()))
	}
	for _, b := range benchmark.All() {
		for _, n := range []int{1, 2} {
			if !seen[fmt.Sprintf("%s/%d", b.Name, n)] {
				t.Fatalf("missing baseline %s/%d", b.Name, n)
			}
		}
	}
}
func runBaseline(t *testing.T, b benchmark.Benchmark, n int) baselineRecord {
	t.Helper()
	s := sim.MakeBuilder().Build()
	defer func() {
		if err := s.Terminate(); err != nil {
			t.Error(err)
		}
	}()
	configs := make([]CoreConfig, n)
	for i := range configs {
		configs[i] = CoreConfig{Emulator: b.Setup(), FuncHookCycles: b.FuncHookCycles}
	}
	mc := NewMultiCorePipeline("Snapshot", s, 3490*timing.MHz, configs)
	for _, c := range mc.Cores {
		benchmark.WarmCache(c.Resources.Cache, b.Regions)
	}
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := mc.Run(); err != nil {
		t.Fatal(err)
	}
	got := baselineRecord{Name: b.Name, Cores: n}
	for _, c := range mc.Cores {
		if !c.Resources.Emu.Halted {
			t.Fatal("emulator did not halt")
		}
		got.Instructions = append(got.Instructions, c.State.InstDone)
		got.Cycles = append(got.Cycles, c.State.Cycles)
		got.Digest = append(got.Digest, architectureDigest(c.Resources.Emu, b))
	}
	// v5's public clock uses whole picoseconds; cycle-based reports retain the
	// original 3.49 GHz conversion rather than replacing it with this period.
	if got, want := s.Engine().CurrentTime(), timing.VTimeInPicoSec(mc.TotalCycles)*(3490*timing.MHz).Period(); got != want {
		t.Fatalf("clock: got %d ps, want %d ps", got, want)
	}
	return got
}
func architectureDigest(em *emu.Emulator, b benchmark.Benchmark) string {
	h := sha256.New()
	data, err := json.Marshal(em.Regs)
	if err != nil {
		panic(err)
	}
	h.Write(data)
	// Readback happens after execution. Do not append millions of observer reads
	// to the modeled memory-access log; these reads do not affect execution.
	tracking := em.Mem.TrackAccesses
	em.Mem.TrackAccesses = false
	defer func() { em.Mem.TrackAccesses = tracking }()
	var buf [4096]byte
	for _, r := range b.Regions {
		for off := 0; off < r.Size; {
			n := min(len(buf), r.Size-off)
			for i := 0; i < n; i++ {
				buf[i] = em.Mem.ReadU8(r.Addr + uint64(off+i))
			}
			h.Write(buf[:n])
			off += n
		}
	}
	var x [8]byte
	binary.LittleEndian.PutUint64(x[:], em.InstCount)
	h.Write(x[:])
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Exact counts for both other pipeline variants prevent rounded CLI CPI output
// from masking an instruction or cycle regression in the in-order refactor.
func TestV4PipelineVariants(t *testing.T) {
	f, err := os.Open("testdata/pipeline-v4-baseline.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var want baselineRecord
		if err := json.Unmarshal(scanner.Bytes(), &want); err != nil {
			t.Fatal(err)
		}
		count++
		if testing.Short() && want.Name != "raytrace" {
			continue
		}
		t.Run(want.Name+"/"+want.Mode, func(t *testing.T) {
			b := benchmark.ByName(want.Name)
			if b == nil {
				t.Fatal("unknown kernel")
			}
			s := sim.MakeBuilder().Build()
			em := b.Setup()
			em.Mem.TrackAccesses = true
			cache := mem.NewCache(mem.DefaultCacheConfig())
			benchmark.WarmCache(cache, b.Regions)
			got := baselineRecord{Name: want.Name, Mode: want.Mode, Cores: 1}
			switch want.Mode {
			case "inorder":
				c := modeling.MakeBuilder[*inorder.Comp](inorder.DefaultSpec()).WithSimulation(s).WithResources(inorder.Resources{Emu: em, Cache: cache}).Build("CPU")
				runTestSimulation(t, s, c)
				if c.State.Err != "" {
					t.Fatal(c.State.Err)
				}
				got.Instructions = []uint64{c.State.InstDone}
				got.Cycles = []uint64{c.State.Cycles}
			case "no-bp":
				spec := ooo.DefaultSpec()
				spec.BranchPrediction = false
				spec.FuncHookCycles = HookCycleSpec(b.FuncHookCycles)
				c := modeling.MakeBuilder[*ooo.Comp](spec).WithSimulation(s).WithResources(ooo.Resources{Emu: em, Cache: cache}).Build("CPU")
				runTestSimulation(t, s, c)
				if c.State.Err != "" {
					t.Fatal(c.State.Err)
				}
				got.Instructions = []uint64{c.State.InstDone}
				got.Cycles = []uint64{c.State.Cycles}
			default:
				t.Fatal("unknown pipeline variant")
			}
			if !em.Halted {
				t.Fatal("emulator did not halt")
			}
			got.Digest = []string{architectureDigest(em, *b)}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("pipeline v4 regression:\ngot  %+v\nwant %+v", got, want)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 8 {
		t.Fatalf("got %d fixture rows, want 8", count)
	}
}
