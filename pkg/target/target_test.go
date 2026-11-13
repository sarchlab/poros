package target

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sarchlab/akita/v5/sim/timing"
	"github.com/sarchlab/poros/pkg/cpu/inorder"
	"github.com/sarchlab/poros/pkg/cpu/ooo"
	"github.com/sarchlab/poros/pkg/mem"
)

func TestLookupNamesEveryTargetAndRejectsOthers(t *testing.T) {
	if want := []string{"m2", "m1", "grace", "m5-max"}; !reflect.DeepEqual(Names(), want) {
		t.Fatalf("targets %v, want %v", Names(), want)
	}
	for _, name := range Names() {
		got, err := Lookup(name)
		if err != nil || got.Name != name || got.Hardware == "" {
			t.Fatalf("Lookup(%q) = %+v, %v", name, got.Identity, err)
		}
	}
	if device, err := Lookup(Default); err != nil || device.Name != "m2" {
		t.Fatalf("default target %+v, %v", device.Identity, err)
	}
	// Near misses are errors that list the valid names, never another device.
	for _, name := range []string{"", "M2", "m3", "m5", "m5max", "apple-m2", " m2", "grace "} {
		got, err := Lookup(name)
		if err == nil || !reflect.DeepEqual(got, Target{}) {
			t.Fatalf("Lookup(%q) accepted as %+v", name, got.Identity)
		}
		for _, valid := range Names() {
			if !strings.Contains(err.Error(), valid) {
				t.Fatalf("error %q does not list %s", err, valid)
			}
		}
	}
}

// M2 is the calibrated configuration: the package defaults, unchanged.
func TestM2IsThePackageDefaults(t *testing.T) {
	m2 := M2()
	if m2.ParameterSource != "m2" || !reflect.DeepEqual(m2.OoO, ooo.DefaultSpec()) ||
		!reflect.DeepEqual(m2.InOrder, inorder.DefaultSpec()) || !reflect.DeepEqual(m2.Cache, mem.DefaultCacheConfig()) {
		t.Fatalf("M2 differs from the pipeline and cache defaults: %+v", m2)
	}
}

// A target that names another as its parameter source must carry exactly that
// target's core, memory and clock, under its own name and hardware.
func TestParameterSourceIsTheConfigurationCarried(t *testing.T) {
	hardware := map[string]bool{}
	for _, device := range All() {
		source, err := Lookup(device.ParameterSource)
		if err != nil || source.ParameterSource != source.Name {
			t.Fatalf("%s: parameter source %q has no parameters of its own", device.Name, device.ParameterSource)
		}
		if !reflect.DeepEqual(device.OoO, source.OoO) || !reflect.DeepEqual(device.InOrder, source.InOrder) || !reflect.DeepEqual(device.Cache, source.Cache) {
			t.Fatalf("%s does not carry the %s configuration it declares", device.Name, source.Name)
		}
		if hardware[device.Hardware] {
			t.Fatalf("hardware %q names two targets", device.Hardware)
		}
		hardware[device.Hardware] = true
	}
}

func TestWithFrequencyReplacesBothClocksOnly(t *testing.T) {
	base := M1()
	changed := base.WithFrequency(2 * timing.GHz)
	if changed.OoO.Freq != 2*timing.GHz || changed.InOrder.Freq != 2*timing.GHz || changed.Identity != base.Identity {
		t.Fatalf("frequency override: %+v", changed)
	}
	changed.OoO.Freq, changed.InOrder.Freq = base.OoO.Freq, base.InOrder.Freq
	if !reflect.DeepEqual(changed, base) || M1().OoO.Freq != base.OoO.Freq {
		t.Fatal("frequency override changed other parameters or the registry")
	}
}

// The evaluation scripts read evaluation/devices.json; it must name exactly
// the targets the simulator builds, in the same order.
func TestDeviceDeclarationMatchesTargets(t *testing.T) {
	data, err := os.ReadFile("../../evaluation/devices.json")
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		Default string
		Devices []struct {
			ID              string
			Hardware        string
			ParameterSource string `json:"parameter_source"`
		}
	}
	if err = json.Unmarshal(data, &declared); err != nil {
		t.Fatal(err)
	}
	var got []Identity
	for _, d := range declared.Devices {
		got = append(got, Identity{Name: d.ID, Hardware: d.Hardware, ParameterSource: d.ParameterSource})
	}
	var want []Identity
	for _, device := range All() {
		want = append(want, device.Identity)
	}
	if declared.Default != Default || !reflect.DeepEqual(got, want) {
		t.Fatalf("devices.json declares %+v (default %q), simulator builds %+v (default %q)", got, declared.Default, want, Default)
	}
}

// evaluation/model-profile.json is the declared M2 profile. Every target whose
// parameters come from M2 must be validated against exactly that declaration;
// a target with its own parameters against evaluation/model-profile-<name>.json.
func TestDeclaredModelProfileCoversTargetsCarryingIt(t *testing.T) {
	for _, device := range All() {
		path := "../../evaluation/model-profile.json"
		if device.ParameterSource != Default {
			path = "../../evaluation/model-profile-" + device.ParameterSource + ".json"
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", device.Name, err)
		}
		var declared any
		if err = json.Unmarshal(data, &declared); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(map[string]any{"configuration": device.OoO, "cache_config": device.Cache})
		if err != nil {
			t.Fatal(err)
		}
		var actual any
		if err = json.Unmarshal(raw, &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(declared, actual) {
			t.Fatalf("%s differs from %s\ndeclared=%s\nactual=%s", device.Name, path, data, raw)
		}
	}
}

// M1 is the M2 configuration except for the parameters its own measurements
// establish; everything else stays M2's.
func TestM1DiffersFromM2OnlyInMeasuredParameters(t *testing.T) {
	m1, m2 := M1(), M2()
	if m1.ParameterSource != "m1" || m1.OoO.Freq != M1Frequency || m1.InOrder.Freq != M1Frequency {
		t.Fatalf("M1 identity or clock: %+v", m1)
	}
	if m1.OoO.FetchTakenBranches != 1 || !m1.OoO.StoreDependencePrediction || m1.Cache.L2Latency != 18 {
		t.Fatalf("M1 front end, dependence prediction or L2 latency: %+v %+v", m1.OoO, m1.Cache)
	}
	if m2.OoO.FetchTakenBranches != 0 || m2.OoO.StoreDependencePrediction {
		t.Fatalf("M2 must keep the frozen front end and dependence timing: %+v", m2.OoO)
	}
	core := m1.OoO
	core.Freq, core.FetchTakenBranches, core.StoreDependencePrediction = m2.OoO.Freq, 0, false
	inorder := m1.InOrder
	inorder.Freq = m2.InOrder.Freq
	cache := m1.Cache
	cache.L2Latency = m2.Cache.L2Latency
	if !reflect.DeepEqual(core, m2.OoO) || !reflect.DeepEqual(inorder, m2.InOrder) || !reflect.DeepEqual(cache, m2.Cache) {
		t.Fatal("M1 differs from M2 in an undeclared parameter")
	}
}
