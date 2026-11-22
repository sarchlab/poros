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

func TestM2AddsApplePipelineAndCachePolicy(t *testing.T) {
	m2 := M2()
	wantCore := ooo.DefaultSpec()
	if wantCore.MultiplyIssueQueue != 0 {
		t.Fatal("generic OoO configuration must allow unlimited waiting multiplies")
	}
	wantCore.MultiplyIssueQueue = 38
	wantCore.NumFlagPipes, wantCore.NumBranchPipes = 3, 2
	wantCore.FlagBranchDependency = true
	wantCore.Predictor.TaggedHistLen = 64
	wantCache := mem.DefaultCacheConfig()
	wantCache.L2ColorPageBytes, wantCache.L2RandomVictim = 16<<10, true
	if m2.ParameterSource != "m2" || !reflect.DeepEqual(m2.OoO, wantCore) ||
		!reflect.DeepEqual(m2.InOrder, inorder.DefaultSpec()) || !reflect.DeepEqual(m2.Cache, wantCache) {
		t.Fatalf("unexpected M2 configuration: %+v", m2)
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
	if !m1.OoO.StoreQueueInOrder || m1.OoO.StoreBufferSize != 140 || !m1.Cache.InterleavedStoreRunsWait {
		t.Fatalf("M1 store queue or store-run rule: %+v %+v", m1.OoO, m1.Cache)
	}
	if m2.OoO.FetchTakenBranches != 0 || m2.OoO.StoreDependencePrediction {
		t.Fatalf("M2 must keep the frozen front end and dependence timing: %+v", m2.OoO)
	}
	if m2.OoO.StoreQueueInOrder || m2.OoO.StoreBufferSize != 56 || m2.Cache.InterleavedStoreRunsWait {
		t.Fatalf("M2 must keep the frozen store buffer and store-miss timing: %+v %+v", m2.OoO, m2.Cache)
	}
	if m1.OoO.MultiplyIssueQueue != 43 {
		t.Fatalf("M1 multiply queue = %d, want 43", m1.OoO.MultiplyIssueQueue)
	}
	core := m1.OoO
	core.MultiplyIssueQueue = m2.OoO.MultiplyIssueQueue
	core.Freq, core.FetchTakenBranches, core.StoreDependencePrediction = m2.OoO.Freq, 0, false
	core.StoreQueueInOrder, core.StoreBufferSize = false, m2.OoO.StoreBufferSize
	inorder := m1.InOrder
	inorder.Freq = m2.InOrder.Freq
	cache := m1.Cache
	if cache.L2Size != 12<<20 || cache.L2Ways != 12 {
		t.Fatal("M1 must use the 12 MiB performance cluster L2")
	}
	cache.L2Size, cache.L2Ways = m2.Cache.L2Size, m2.Cache.L2Ways
	cache.L2Latency, cache.InterleavedStoreRunsWait = m2.Cache.L2Latency, false
	if !reflect.DeepEqual(core, m2.OoO) || !reflect.DeepEqual(inorder, m2.InOrder) || !reflect.DeepEqual(cache, m2.Cache) {
		t.Fatal("M1 differs from M2 in an undeclared parameter")
	}
}

// Grace is the M2 configuration except for the parameters its profile
// declares; everything else stays M2's. M2 and M1 use none of the mechanisms
// that exist for Grace, so their timing cannot have changed with them.
func TestGraceDiffersFromM2OnlyInDeclaredParameters(t *testing.T) {
	grace, m2 := Grace(), M2()
	if grace.ParameterSource != "grace" || grace.OoO.Freq != GraceFrequency || grace.InOrder.Freq != GraceFrequency {
		t.Fatalf("Grace identity or clock: %+v", grace)
	}
	for _, device := range []Target{m2, M1()} {
		core := device.OoO
		if core.Latencies != (ooo.Latencies{}) || core.NumIntDiv != 0 || core.StoreForwardLatency != 0 || core.FPIssueQueue != 0 {
			t.Fatalf("%s must keep the default latencies, dividers, forwarding and issue queue: %+v", device.Name, core)
		}
		if device.Cache.PrefetchStrideBytes != 0 {
			t.Fatalf("%s must keep the sequential-only stream prefetcher", device.Name)
		}
	}
	if grace.OoO.NumSelectFusion != m2.OoO.NumSelectFusion || grace.OoO.StoreDependencePrediction || grace.OoO.ROBCapacity != m2.OoO.ROBCapacity {
		t.Fatalf("Grace must keep M2's compare/select fusion, dependence timing and window: %+v", grace.OoO)
	}
	if grace.OoO.MultiplyIssueQueue != 0 {
		t.Fatal("Grace inherited an Apple multiply queue bound")
	}
	if grace.OoO.NumFlagPipes != 0 || grace.OoO.NumBranchPipes != 0 || grace.OoO.FlagBranchDependency || grace.OoO.Predictor.TaggedHistLen != 32 || grace.Cache.L2RandomVictim || grace.Cache.L2ColorPageBytes != 0 {
		t.Fatal("Grace inherited Apple scheduling or cache geometry")
	}
	core := grace.OoO
	core.NumFlagPipes, core.NumBranchPipes = m2.OoO.NumFlagPipes, m2.OoO.NumBranchPipes
	core.FlagBranchDependency, core.Predictor = m2.OoO.FlagBranchDependency, m2.OoO.Predictor
	core.MultiplyIssueQueue = m2.OoO.MultiplyIssueQueue
	core.Freq, core.Latencies = m2.OoO.Freq, ooo.Latencies{}
	core.NumIntDiv, core.NumFPDiv = 0, m2.OoO.NumFPDiv
	core.NumLDST, core.NumLoad, core.NumStore = m2.OoO.NumLDST, m2.OoO.NumLoad, m2.OoO.NumStore
	core.StoreForwardLatency, core.FPIssueQueue, core.BranchPenalty = 0, 0, m2.OoO.BranchPenalty
	inorder := grace.InOrder
	inorder.Freq = m2.InOrder.Freq
	cache := grace.Cache
	cache.L2ColorPageBytes, cache.L2RandomVictim = m2.Cache.L2ColorPageBytes, m2.Cache.L2RandomVictim
	cache.L1Size, cache.L1Ways, cache.L1Latency = m2.Cache.L1Size, m2.Cache.L1Ways, m2.Cache.L1Latency
	cache.L2Size, cache.L2Ways, cache.L2LineSize = m2.Cache.L2Size, m2.Cache.L2Ways, m2.Cache.L2LineSize
	cache.SLCSize, cache.PrefetchStrideBytes = m2.Cache.SLCSize, 0
	cache.TLBPageBytes, cache.TLBSets, cache.TLBWays = m2.Cache.TLBPageBytes, m2.Cache.TLBSets, m2.Cache.TLBWays
	if !reflect.DeepEqual(core, m2.OoO) || !reflect.DeepEqual(inorder, m2.InOrder) || !reflect.DeepEqual(cache, m2.Cache) {
		t.Fatal("Grace differs from M2 in an undeclared parameter")
	}
	// The published geometry, as the profile documents it.
	if c := grace.Cache; c.L1Size != 64<<10 || c.L1Ways != 4 || c.L2Size != 1<<20 || c.L2Ways != 8 || c.L2LineSize != 64 ||
		c.TLBPageBytes != 64<<10 || c.TLBSets*c.TLBWays != 2048 {
		t.Fatalf("Grace cache or translation geometry: %+v", c)
	}
}
