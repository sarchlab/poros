package elfsim

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sarchlab/poros/internal/benchmarks"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"github.com/sarchlab/poros/pkg/target"
)

func mvtProcess(t *testing.T) *linuxuser.Process {
	t.Helper()
	image, err := os.ReadFile("../../tests/usertime/mvt.elf")
	if err != nil {
		t.Fatal(err)
	}
	p, err := linuxuser.Parse(image, linuxuser.Options{Args: []string{"program", "24"}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Every target runs a complete program in both timing models and reports its
// own identity with the configuration actually simulated. A target that names
// another as its parameter source must reproduce that target's cycles.
func TestEveryTargetRunsBothModels(t *testing.T) {
	cycles := map[string]uint64{}
	for _, device := range target.All() {
		for _, model := range []string{"ooo", "inorder"} {
			r, err := Run(context.Background(), mvtProcess(t), Options{Model: model, Target: device.Name})
			if err != nil {
				t.Fatalf("%s/%s: %v", device.Name, model, err)
			}
			if r.Termination != "exit" || r.ExitStatus == nil || *r.ExitStatus != 0 || len(r.Stdout) != 16*24 || r.Timing.Cycles == 0 {
				t.Fatalf("%s/%s: incomplete execution %+v", device.Name, model, r.Result)
			}
			if r.Timing.Target != device.Identity || !reflect.DeepEqual(r.Timing.CacheConfig, device.Cache) {
				t.Fatalf("%s/%s: reported target %+v", device.Name, model, r.Timing.Target)
			}
			var configured any = device.OoO
			hz := float64(device.OoO.Freq)
			if model == "inorder" {
				configured, hz = device.InOrder, float64(device.InOrder.Freq)
			}
			if !reflect.DeepEqual(r.Timing.Configuration, configured) || r.Timing.AssumedHz != hz {
				t.Fatalf("%s/%s: reported configuration differs from the target's", device.Name, model)
			}
			cycles[device.Name+"/"+model] = r.Timing.Cycles
		}
	}
	for _, device := range target.All() {
		for _, model := range []string{"ooo", "inorder"} {
			if got, want := cycles[device.Name+"/"+model], cycles[device.ParameterSource+"/"+model]; got != want {
				t.Fatalf("%s/%s: %d cycles, its parameter source %s gives %d", device.Name, model, got, device.ParameterSource, want)
			}
		}
	}
}

// A target changes timing and nothing else. Every kernel, at the small catalog
// case short CI runs, retires the same instructions and prints the same bytes
// on a target with parameters of its own as on M2, with a timing its own
// configuration produced: every mechanism a target enables runs on every kernel.
func TestTargetsWithOwnParametersChangeTimingOnly(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := benchmarks.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	limits := linuxuser.RunOptions{MaxInstructions: 200_000_000, Timeout: 5 * time.Minute, MaxOutput: 16 << 20}
	for _, w := range manifest.Workloads {
		t.Run(w.Name, func(t *testing.T) {
			image, err := os.ReadFile(filepath.Join(root, w.Binary))
			if err != nil {
				t.Fatal(err)
			}
			var m2 Result
			for _, device := range target.All() {
				if device.ParameterSource != device.Name {
					continue // carries another target's parameters, covered by that target
				}
				p, err := linuxuser.Parse(image, linuxuser.Options{Args: append([]string{w.Name}, w.Cases[2].Args...)})
				if err != nil {
					t.Fatal(err)
				}
				r, err := Run(context.Background(), p, Options{Target: device.Name, Limits: limits})
				if err != nil {
					t.Fatalf("%s: %v", device.Name, err)
				}
				if r.Termination != "exit" || r.ExitStatus == nil || *r.ExitStatus != 0 || r.Timing.Cycles == 0 || r.Timing.Retired != r.Instructions {
					t.Fatalf("%s: incomplete execution %+v", device.Name, r.Result)
				}
				if device.Name == target.Default {
					m2 = r
				} else if !reflect.DeepEqual(r.Result, m2.Result) {
					t.Fatalf("%s: architectural result differs from %s", device.Name, target.Default)
				}
			}
		})
	}
}

// Before targets existed every run was M2. Leaving the target empty must give
// exactly the result of asking for M2, including the reported configuration.
func TestEmptyTargetIsM2(t *testing.T) {
	for _, model := range []string{"ooo", "inorder"} {
		implicit, err := Run(context.Background(), mvtProcess(t), Options{Model: model})
		if err != nil {
			t.Fatal(err)
		}
		explicit, err := Run(context.Background(), mvtProcess(t), Options{Model: model, Target: "m2"})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(implicit, explicit) || implicit.Timing.Target.Name != "m2" {
			t.Fatalf("%s: default target differs from m2", model)
		}
		var defaults any = ooo.DefaultSpec()
		if model == "inorder" {
			defaults = inorder.DefaultSpec()
		}
		if !reflect.DeepEqual(implicit.Timing.Configuration, defaults) {
			t.Fatalf("%s: default target no longer uses the package defaults", model)
		}
	}
}

func TestUnknownTargetIsAnErrorNotAFallback(t *testing.T) {
	for _, name := range []string{"m3", "M2", "m5"} {
		r, err := Run(context.Background(), mvtProcess(t), Options{Target: name})
		if err == nil || !strings.Contains(err.Error(), "unknown target") || r.Instructions != 0 || r.Timing.Cycles != 0 {
			t.Fatalf("Run accepted target %q: %+v, %v", name, r.Timing.Target, err)
		}
		m, err := RunMultiCore(context.Background(), []*linuxuser.Process{mvtProcess(t)}, Options{Target: name})
		if err == nil || !strings.Contains(err.Error(), "unknown target") || len(m.Cores) != 0 {
			t.Fatalf("RunMultiCore accepted target %q: %v", name, err)
		}
	}
}

// An explicit frequency replaces only the clock: the device, its other
// parameters and the cycle count stay those of the requested target.
func TestTargetFrequencyOverride(t *testing.T) {
	plain, err := Run(context.Background(), mvtProcess(t), Options{Target: "grace"})
	if err != nil {
		t.Fatal(err)
	}
	slow, err := Run(context.Background(), mvtProcess(t), Options{Target: "grace", FrequencyHz: 1e9})
	if err != nil {
		t.Fatal(err)
	}
	want := target.Grace().OoO
	want.Freq = 1e9
	if slow.Timing.Target != target.Grace().Identity || slow.Timing.AssumedHz != 1e9 || !reflect.DeepEqual(slow.Timing.Configuration, want) {
		t.Fatalf("override changed more than the clock: %+v", slow.Timing)
	}
	if slow.Timing.Cycles != plain.Timing.Cycles || slow.Timing.PredictedUserSeconds != float64(slow.Timing.Cycles)/1e9 {
		t.Fatalf("override changed cycles or seconds: %d vs %d", slow.Timing.Cycles, plain.Timing.Cycles)
	}
}

func TestMultiCoreReportsTarget(t *testing.T) {
	r, err := RunMultiCore(context.Background(), []*linuxuser.Process{mvtProcess(t), mvtProcess(t)}, Options{Target: "m5-max"})
	if err != nil {
		t.Fatal(err)
	}
	device := target.M5Max()
	// Multicore assembly always attaches a hook list, empty for processes.
	device.OoO.FuncHookCycles = []ooo.HookCycles{}
	if len(r.Cores) != 2 {
		t.Fatalf("%d cores", len(r.Cores))
	}
	for _, core := range r.Cores {
		if core.Timing.Target != device.Identity || !reflect.DeepEqual(core.Timing.Configuration, device.OoO) || !reflect.DeepEqual(core.Timing.CacheConfig, device.Cache) {
			t.Fatalf("core reported %+v", core.Timing.Target)
		}
	}
}
