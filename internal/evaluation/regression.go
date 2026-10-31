package evaluation

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sarchlab/poros/internal/benchmarks"
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

// RequiredCases is independent of observed output and requires every supported
// name. Full: six points x three modes; short: point3 x three modes.
func RequiredCases(root, tier string) (map[string]bool, error) {
	if tier != "short" && tier != "full" {
		return nil, fmt.Errorf("unknown tier %q", tier)
	}
	if _, err := benchmarks.Load(root); err != nil {
		return nil, err
	}
	required := map[string]bool{}
	for _, name := range benchmarks.ComputeNames() {
		for point := 1; point <= 6; point++ {
			if tier == "short" && point != 3 {
				continue
			}
			for _, mode := range []string{"functional", "inorder", "ooo"} {
				required[fmt.Sprintf("TestBenchmarkELF/%s/%d/%s", name, point, mode)] = true
			}
		}
		required["TestBenchmarkELF/"+name+"/invalid"] = true
	}
	for _, mode := range []string{"functional", "inorder", "ooo"} {
		required["TestMVTCompleteELF/"+mode] = true
		required["TestGuestMath/"+mode] = true
	}
	for _, name := range []string{"integer", "memory", "branch"} {
		required["TestCompleteWorkloadShapesAndResults/"+name] = true
	}
	required["TestELFMultiCoreLifecycle"] = true
	return required, nil
}

func Regression(root, tier, commit string, r io.Reader) (RegressionReport, error) {
	report := RegressionReport{Acceptance: "unverified", Provenance: "unknown", SchemaVersion: 1, Commit: commit, GoVersion: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH, Tier: tier, Status: "fail", Evidence: "complete ELF execution; native compiler oracle; full output checks; independent numerical equations; architectural mode agreement"}
	var err error
	report.Dirty, err = WorkingTreeDirty(root)
	if err != nil {
		return report, err
	}
	report.Profile = "whole ELF user execution;3490MHz;default CPU/cache parameters;cold modeled caches;no host application hooks;not performance calibration"
	report.Hashes = map[string]string{}
	for _, name := range []string{"evaluation/workloads.json", "tests/benchmarks/manifest.json", "tests/benchmarks/provenance.json", "tests/usertime/manifest.json", "pkg/cpu/ooo/pipeline.go", "pkg/cpu/inorder/pipeline.go", "pkg/mem/cache.go"} {
		b, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			return report, e
		}
		report.Hashes[name] = hash(b)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return report, err
	}
	report.Hashes["go-test-json"] = hash(data)
	cmd := exec.Command("go", "list", "-f", "{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}", "./...")
	cmd.Dir = root
	packages, err := cmd.Output()
	if err != nil {
		return report, err
	}
	report.RequiredPackages = strings.Fields(string(packages))
	if len(report.RequiredPackages) == 0 {
		return report, fmt.Errorf("no test-bearing packages")
	}
	m, err := LoadManifest(root)
	if err != nil {
		return report, err
	}
	for _, w := range m.Workloads {
		if w.Status == "planned" {
			report.Planned = append(report.Planned, w.ID)
		}
	}
	required, err := RequiredCases(root, tier)
	if err != nil {
		return report, err
	}
	seen := map[string]string{}
	packagePass := map[string]bool{}
	packageRun := map[string]bool{}
	packageSkip := map[string]bool{}
	failed := false
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Buffer(make([]byte, 4096), 4<<20)
	for scan.Scan() {
		var ev struct{ Action, Package, Test, Output string }
		if err := json.Unmarshal(scan.Bytes(), &ev); err != nil {
			return report, fmt.Errorf("invalid go test JSON: %w", err)
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
		if ev.Package == "github.com/sarchlab/poros/pkg/elfsim" && required[ev.Test] && (ev.Action == "pass" || ev.Action == "skip" || ev.Action == "fail") {
			if seen[ev.Test] != "" {
				return report, fmt.Errorf("duplicate result %s", ev.Test)
			}
			seen[ev.Test] = ev.Action
		}
	}
	if err := scan.Err(); err != nil {
		return report, err
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
		report.Cases = append(report.Cases, Case{id, status, report.Evidence})
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
