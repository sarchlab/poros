package evaluation

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Case struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}
type RegressionReport struct {
	Acceptance        string            `json:"acceptance_status"`
	Provenance        string            `json:"provenance_status"`
	ProvenanceError   string            `json:"provenance_error,omitempty"`
	Tested            *Capture          `json:"tested_provenance"`
	Dirty             bool              `json:"analysis_working_tree_dirty"`
	Hashes            map[string]string `json:"sha256"`
	RequiredPackages  []string          `json:"required_packages"`
	Profile           string            `json:"profile"`
	SchemaVersion     int               `json:"schema_version"`
	Commit            string            `json:"analysis_commit"`
	GoVersion         string            `json:"analysis_go_version"`
	Platform          string            `json:"analysis_platform"`
	Tier              string            `json:"tier"`
	Status            string            `json:"coverage_status"`
	Evidence          string            `json:"evidence"`
	Cases             []Case            `json:"cases"`
	IndependentChecks []Case            `json:"independent_checks"`
	Planned           []string          `json:"planned"`
}
type fixture struct {
	Name, Mode           string
	Cores                int
	Instructions, Cycles []uint64
	Digest               []string
}

func RequiredCases(root, tier string) (map[string]bool, error) {
	if tier != "short" && tier != "full" {
		return nil, fmt.Errorf("unknown tier %q", tier)
	}
	required := map[string]bool{}
	for _, file := range []string{"akita-v4-baseline.jsonl", "pipeline-v4-baseline.jsonl"} {
		f, e := os.Open(filepath.Join(root, "pkg/cpu/testdata", file))
		if e != nil {
			return nil, e
		}
		s := bufio.NewScanner(f)
		count := 0
		for s.Scan() {
			var r fixture
			if e := json.Unmarshal(s.Bytes(), &r); e != nil {
				f.Close()
				return nil, e
			}
			if r.Name == "" || r.Cores < 1 || len(r.Cycles) != r.Cores || len(r.Instructions) != r.Cores || len(r.Digest) != r.Cores {
				f.Close()
				return nil, fmt.Errorf("invalid fixture row")
			}
			id := fmt.Sprintf("TestV4Baseline/%s/%d", r.Name, r.Cores)
			if file == "pipeline-v4-baseline.jsonl" {
				id = "TestV4PipelineVariants/" + r.Name + "/" + r.Mode
			}
			if required[id] {
				f.Close()
				return nil, fmt.Errorf("duplicate case %s", id)
			}
			required[id] = true
			count++
		}
		e = s.Err()
		f.Close()
		if e != nil {
			return nil, e
		}
		want := 82
		if file == "pipeline-v4-baseline.jsonl" {
			want = 8
		}
		if count != want {
			return nil, fmt.Errorf("%s has %d rows, expected %d", file, count, want)
		}
	}
	if tier == "short" {
		for id := range required {
			if !containsCase(id, "raytrace") && !containsCase(id, "blackscholes") {
				delete(required, id)
			}
		}
	}
	return required, nil
}
func containsCase(id, name string) bool {
	return strings.HasPrefix(id, "TestV4Baseline/"+name+"/") || strings.HasPrefix(id, "TestV4PipelineVariants/"+name+"/")
}

func Regression(root, tier, commit string, r io.Reader) (RegressionReport, error) {
	report := RegressionReport{Acceptance: "unverified", Provenance: "unknown", SchemaVersion: 1, Commit: commit, GoVersion: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH, Tier: tier, Status: "fail", Evidence: "legacy-behavior-only"}
	dirty, e := WorkingTreeDirty(root)
	if e != nil {
		return report, e
	}
	report.Dirty = dirty
	report.Profile = "legacy-default;3490MHz;registry-warm-regions;registry-hook-costs"
	report.Hashes = map[string]string{}
	for _, name := range []string{"evaluation/workloads.json", "pkg/cpu/testdata/akita-v4-baseline.jsonl", "pkg/cpu/testdata/pipeline-v4-baseline.jsonl", "pkg/benchmark/registry.go", "pkg/cpu/ooo/pipeline.go", "pkg/cpu/inorder/pipeline.go", "pkg/mem/cache.go"} {
		b, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			return report, e
		}
		report.Hashes[name] = hash(b)
	}
	data, e := io.ReadAll(r)
	if e != nil {
		return report, e
	}
	report.Hashes["go-test-json"] = hash(data)
	cmd := exec.Command("go", "list", "-f", "{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}", "./...")
	cmd.Dir = root
	packages, e := cmd.Output()
	if e != nil {
		return report, e
	}
	report.RequiredPackages = strings.Fields(string(packages))
	if len(report.RequiredPackages) == 0 {
		return report, fmt.Errorf("no test-bearing packages")
	}
	m, e := LoadManifest(root)
	if e != nil {
		return report, e
	}
	for _, w := range m.Workloads {
		if w.Status == "planned" {
			report.Planned = append(report.Planned, w.ID)
		}
	}
	required, e := RequiredCases(root, tier)
	if e != nil {
		return report, e
	}
	seen := map[string]string{}
	modes := map[string]string{}
	packagePass := map[string]bool{}
	packageRun := map[string]bool{}
	packageSkip := map[string]bool{}
	numericalTests := map[string]string{
		"blackscholes": "TestRegistryBlackScholesNumericalReference",
		"bodytrack":    "TestRegistryBodytrackNumericalReference",
	}
	numericalStatus := map[string]string{}
	for _, name := range numericalTests {
		numericalStatus[name] = "missing"
	}
	failed := false
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Buffer(make([]byte, 4096), 4<<20)
	for scan.Scan() {
		var ev struct{ Action, Package, Test, Output string }
		if e := json.Unmarshal(scan.Bytes(), &ev); e != nil {
			return report, fmt.Errorf("invalid go test JSON: %w", e)
		}
		// This marker is emitted by portableHookReference in pkg/cpu/migration_test.go.
		if ev.Package == "github.com/sarchlab/poros/pkg/cpu" && required[ev.Test] && strings.Contains(ev.Output, "nonportable host-math hook reference:") {
			modes[ev.Test] = "same-host functional architectural digest; independent numerical test required"
		}
		if ev.Package == "github.com/sarchlab/poros/pkg/cpu" && numericalStatus[ev.Test] != "" && (ev.Action == "pass" || ev.Action == "skip" || ev.Action == "fail") {
			if numericalStatus[ev.Test] != "missing" {
				return report, fmt.Errorf("duplicate result %s", ev.Test)
			}
			numericalStatus[ev.Test] = ev.Action
		}
		if ev.Action == "fail" {
			failed = true
		}
		if ev.Action == "start" {
			packageRun[ev.Package] = true
		}
		if ev.Test == "" && ev.Action == "skip" {
			packageSkip[ev.Package] = true
		}
		if ev.Test == "" && ev.Action == "pass" {
			packagePass[ev.Package] = true
		}
		if ev.Package == "github.com/sarchlab/poros/pkg/cpu" && required[ev.Test] && (ev.Action == "pass" || ev.Action == "skip" || ev.Action == "fail") {
			if seen[ev.Test] != "" {
				return report, fmt.Errorf("duplicate result %s", ev.Test)
			}
			seen[ev.Test] = ev.Action
		}
	}
	if e := scan.Err(); e != nil {
		return report, e
	}
	ids := []string{}
	for id := range required {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		status := seen[id]
		if status == "" {
			status = "missing"
		}
		if status != "pass" {
			failed = true
		}
		mode := modes[id]
		if mode == "" {
			mode = "frozen ARM64 architectural digest"
		}
		report.Cases = append(report.Cases, Case{id, status, "exact historical instructions/cycles; " + mode})
	}
	for _, workload := range []string{"blackscholes", "bodytrack"} {
		needed := false
		for id := range modes {
			if containsCase(id, workload) {
				needed = true
			}
		}
		if needed {
			name := numericalTests[workload]
			status := numericalStatus[name]
			report.IndependentChecks = append(report.IndependentChecks, Case{name, status, "all registry outputs; reference independent of simulated instructions, shares host math"})
			if status != "pass" {
				failed = true
			}
		}
	}

	for pkg := range packageRun {
		if !packagePass[pkg] && !packageSkip[pkg] {
			failed = true
		}
	}
	for _, pkg := range report.RequiredPackages {
		if !packagePass[pkg] {
			failed = true
		}
	}
	if failed {
		return report, fmt.Errorf("test failure, incomplete package, missing or skipped required case")
	}
	report.Status = "pass"
	return report, nil
}

func WorkingTreeDirty(root string) (bool, error) {
	b, e := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	return len(b) > 0, e
}
