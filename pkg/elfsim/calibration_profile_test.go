package elfsim

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"github.com/sarchlab/poros/pkg/mem"
)

func TestTimingProfileStrictValidation(t *testing.T) {
	good := DefaultTimingProfile(3490 * timing.MHz)
	for name, change := range map[string]func(*TimingProfile){
		"negative stores":   func(p *TimingProfile) { p.Configuration.NumStores = -1 },
		"zero MSHR":         func(p *TimingProfile) { p.Configuration.NumMSHREntries = 0 },
		"huge ROB":          func(p *TimingProfile) { p.Configuration.ROBCapacity = 1 << 30 },
		"bad geometry":      func(p *TimingProfile) { p.CacheConfig.L1LineSize = 0 },
		"negative latency":  func(p *TimingProfile) { p.CacheConfig.DRAMLatency = -1 },
		"invalid predictor": func(p *TimingProfile) { p.Configuration.Predictor.BTBWays = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			p := good
			change(&p)
			b, _ := json.Marshal(p)
			if _, err := ReadTimingProfile(strings.NewReader(string(b))); err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
	b, _ := json.Marshal(good)
	if _, err := ReadTimingProfile(strings.NewReader(string(b))); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`null`, `{}`, string(b) + ` {}`, strings.Replace(string(b), `"num_stores":`, `"num_storse":`, 1)} {
		if _, err := ReadTimingProfile(strings.NewReader(bad)); err == nil {
			t.Fatal("malformed profile accepted", bad)
		}
	}
}

func TestTimingProfileAPIContract(t *testing.T) {
	image, err := os.ReadFile("../../tests/elf/hello.elf")
	if err != nil {
		t.Fatal(err)
	}
	fresh := func() *linuxuser.Process {
		p, err := linuxuser.Parse(image, linuxuser.Options{Args: []string{"program"}})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	profile := DefaultTimingProfile(timing.GHz)
	profile.Configuration.NumStores = 2
	profile.CacheConfig.L1Latency = 7
	got, err := Run(context.Background(), fresh(), Options{Model: "ooo", FrequencyHz: 1e9, TimingProfile: &profile})
	if err != nil {
		t.Fatal(err)
	}
	if got.Termination != "exit" || string(got.Stdout) != "hello, ELF\n" || got.Instructions != 51 || !reflect.DeepEqual(got.Timing.Configuration, profile.Configuration) || got.Timing.CacheConfig != profile.CacheConfig {
		t.Fatalf("profile API did not use requested values: %+v", got)
	}
	for _, tc := range []struct {
		name       string
		options    Options
		diagnostic string
	}{
		{"inorder", Options{Model: "inorder", FrequencyHz: 1e9, TimingProfile: &profile}, "require the OoO model"},
		{"frequency", Options{Model: "ooo", FrequencyHz: 2e9, TimingProfile: &profile}, "frequency differs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fresh()
			before := p.Registers
			result, err := Run(context.Background(), p, tc.options)
			if err == nil || !strings.Contains(err.Error(), tc.diagnostic) || result.Instructions != 0 || p.Registers != before {
				t.Fatalf("invalid profile executed or wrong error: result%+v err%v", result, err)
			}
		})
	}
	p := fresh()
	before := p.Registers
	result, err := RunMultiCore(context.Background(), []*linuxuser.Process{p}, Options{Model: "ooo", FrequencyHz: 1e9, TimingProfile: &profile})
	if err == nil || !strings.Contains(err.Error(), "custom timing profiles") || len(result.Cores) != 0 || p.Registers != before {
		t.Fatalf("multicore silently accepted profile: result%+v err%v", result, err)
	}
}

// Calibration belongs to the single-core ELF runner; it must not leak into
// the generic component defaults, the in-order runner, or multicore experiments.
func TestELFCalibrationScope(t *testing.T) {
	image, err := os.ReadFile("../../tests/elf/hello.elf")
	if err != nil {
		t.Fatal(err)
	}
	fresh := func() *linuxuser.Process {
		p, err := linuxuser.Parse(image, linuxuser.Options{Args: []string{"program"}})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	freq := timing.GHz
	genericCPU := ooo.DefaultSpec()
	genericCPU.Freq = freq
	genericCache := mem.DefaultCacheConfig()
	if genericCPU.NumStores != 0 || genericCache.L2Latency != 14 {
		t.Fatal("generic defaults changed")
	}
	expected := TimingProfile{Configuration: genericCPU, CacheConfig: genericCache}
	expected.Configuration.NumStores = 2
	expected.CacheConfig.L2Latency = 20
	if !reflect.DeepEqual(DefaultTimingProfile(freq), expected) {
		t.Fatal("ELF calibration changed more than store throughput and L2 latency")
	}
	for _, mode := range []string{"ooo", "inorder"} {
		r, err := Run(context.Background(), fresh(), Options{Model: mode, FrequencyHz: float64(freq)})
		if err != nil {
			t.Fatal(err)
		}
		if r.Termination != "exit" || string(r.Stdout) != "hello, ELF\n" || r.Instructions != 51 {
			t.Fatalf("%s functional result changed: %+v", mode, r)
		}
		var want any = expected.Configuration
		wantCache := expected.CacheConfig
		if mode == "inorder" {
			c := inorder.DefaultSpec()
			c.Freq = freq
			want = c
			wantCache = genericCache
		}
		if !reflect.DeepEqual(r.Timing.Configuration, want) || r.Timing.CacheConfig != wantCache {
			t.Fatalf("%s effective configuration: %+v", mode, r.Timing)
		}
	}
	multi, err := RunMultiCore(context.Background(), []*linuxuser.Process{fresh(), fresh()}, Options{Model: "ooo", FrequencyHz: float64(freq)})
	if err != nil {
		t.Fatal(err)
	}
	if len(multi.Cores) != 2 {
		t.Fatal("missing multicore results")
	}
	for i, r := range multi.Cores {
		actual := r.Timing.Configuration.(ooo.Spec)
		// The multicore builder emits an empty hook slice instead of nil.
		if len(actual.FuncHookCycles) == 0 {
			actual.FuncHookCycles = nil
		}
		if r.Termination != "exit" || string(r.Stdout) != "hello, ELF\n" || r.Instructions != 51 || !reflect.DeepEqual(actual, genericCPU) || r.Timing.CacheConfig != genericCache {
			t.Fatalf("multicore%d changed calibration scope: %+v", i, r)
		}
	}
}

func TestTimingProfileRequiresCompleteNonNullShape(t *testing.T) {
	canonical, err := json.Marshal(DefaultTimingProfile(timing.GHz))
	if err != nil {
		t.Fatal(err)
	}
	// Enumerate every key in the four canonical objects, including fields whose
	// legal zero values cannot distinguish omission after ordinary Go decoding.
	for _, path := range [][]string{{}, {"configuration"}, {"cache_config"}, {"configuration", "predictor"}} {
		var source map[string]any
		if err := json.Unmarshal(canonical, &source); err != nil {
			t.Fatal(err)
		}
		object := source
		for _, key := range path {
			object = object[key].(map[string]any)
		}
		for key := range object {
			field := strings.Join(append(append([]string{}, path...), key), ".")
			for _, mutation := range []string{"omitted", "null"} {
				if field == "configuration.func_hook_cycles" && mutation == "null" {
					continue
				}
				t.Run(field+"/"+mutation, func(t *testing.T) {
					var changed map[string]any
					if err := json.Unmarshal(canonical, &changed); err != nil {
						t.Fatal(err)
					}
					parent := changed
					for _, part := range path {
						parent = parent[part].(map[string]any)
					}
					if mutation == "omitted" {
						delete(parent, key)
					} else {
						parent[key] = nil
					}
					data, err := json.Marshal(changed)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := ReadTimingProfile(strings.NewReader(string(data))); err == nil || !strings.Contains(err.Error(), field) {
						t.Fatalf("%s field accepted or incorrect diagnostic: %v", mutation, err)
					}
				})
			}
		}
	}
	// Zero is allowed when explicitly supplied; null is allowed only for hooks.
	zero := DefaultTimingProfile(timing.GHz)
	zero.Configuration.NumStores = 0
	zero.Configuration.BranchPrediction = false
	zero.Configuration.BranchPenalty = 0
	zero.Configuration.FuncHookCycles = nil
	data, err := json.Marshal(zero)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadTimingProfile(strings.NewReader(string(data)))
	if err != nil || !reflect.DeepEqual(got, &zero) {
		t.Fatalf("explicit zero/null-hook profile rejected: %+v %v", got, err)
	}
}
