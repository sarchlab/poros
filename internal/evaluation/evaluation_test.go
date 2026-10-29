package evaluation

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repo(t *testing.T) string {
	t.Helper()
	p, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestManifest(t *testing.T) {
	root := repo(t)
	m, e := LoadManifest(root)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Workloads) != 47 {
		t.Fatal("unexpected inventory")
	}
	t.Run("missing workload", func(t *testing.T) {
		n := m
		n.Workloads = n.Workloads[1:]
		if n.Validate(root) == nil {
			t.Fatal("accepted missing workload")
		}
	})
	t.Run("planned cannot pass", func(t *testing.T) {
		n := m
		n.Workloads = append([]Workload(nil), m.Workloads...)
		n.Workloads[len(n.Workloads)-1].Status = "pass"
		if n.Validate(root) == nil {
			t.Fatal("accepted pass status")
		}
	})
	t.Run("missing source", func(t *testing.T) {
		n := m
		n.Workloads = append([]Workload(nil), m.Workloads...)
		n.Workloads[0].Source = "missing.c"
		if n.Validate(root) == nil {
			t.Fatal("accepted missing source")
		}
	})
	t.Run("unknown field", func(t *testing.T) {
		var m Manifest
		if Decode(strings.NewReader(`{"schema_version":1,"surprise":true}`), &m) == nil {
			t.Fatal("accepted unknown field")
		}
	})
	t.Run("extra value", func(t *testing.T) {
		var m Manifest
		if Decode(strings.NewReader(`{} {}`), &m) == nil {
			t.Fatal("accepted extra value")
		}
	})
}
func logFor(t *testing.T, root string, required map[string]bool, omit string, action string) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	cmd := exec.Command("go", "list", "-f", "{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}", "./...")
	cmd.Dir = root
	out, e := cmd.Output()
	if e != nil {
		t.Fatal(e)
	}
	for _, pkg := range strings.Fields(string(out)) {
		enc.Encode(map[string]string{"Action": "start", "Package": pkg})
		enc.Encode(map[string]string{"Action": "pass", "Package": pkg})
	}
	enc.Encode(map[string]string{"Action": "start", "Package": "no-tests"})
	enc.Encode(map[string]string{"Action": "skip", "Package": "no-tests"})
	for id := range required {
		if id == omit {
			continue
		}
		enc.Encode(map[string]string{"Action": action, "Package": "github.com/sarchlab/poros/pkg/cpu", "Test": id})
	}
	return b.Bytes()
}
func TestRegressionCoverage(t *testing.T) {
	root := repo(t)
	req, e := RequiredCases(root, "full")
	if e != nil || len(req) != 90 {
		t.Fatalf("%d %v", len(req), e)
	}
	good := logFor(t, root, req, "", "pass")
	r, e := Regression(root, "full", "test", bytes.NewReader(good))
	if e != nil || r.Status != "pass" || len(r.Cases) != 90 || len(r.Planned) != 6 {
		t.Fatalf("%+v %v", r, e)
	}
	for _, bad := range [][]byte{logFor(t, root, req, "TestV4Baseline/mvt/1", "pass"), logFor(t, root, req, "", "skip"), logFor(t, root, req, "", "fail"), good[:len(good)/2], nil} {
		if _, e := Regression(root, "full", "test", bytes.NewReader(bad)); e == nil {
			t.Fatal("accepted incomplete/failed report")
		}
	}
	if _, e := Regression(root, "full", "test", bytes.NewReader(append(good, []byte("garbage")...))); e == nil {
		t.Fatal("accepted invalid JSON")
	}
}
func TestReferenceRejectsWrongOutput(t *testing.T) {
	n := 8
	x1, x2, y1, y2, a := InitialMVT(n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			x1[i] += a[i*n+j] * y1[j]
			x2[i] += a[j*n+i] * y2[j]
		}
	}
	if e := CheckMVT(n, x1, x2); e != nil {
		t.Fatal(e)
	}
	x1[3] += 1
	if CheckMVT(n, x1, x2) == nil {
		t.Fatal("accepted wrong result")
	}
}
func TestMissingFixture(t *testing.T) {
	if _, e := RequiredCases(t.TempDir(), "full"); e == nil {
		t.Fatal("accepted missing fixture")
	}
}
func TestPilotBounds(t *testing.T) {
	for _, n := range []int{0, 1537} {
		if CheckMVT(n, nil, nil) == nil {
			t.Fatal("accepted bad dimension")
		}
	}
}
func TestFixtureIdentity(t *testing.T) {
	root := repo(t)
	for path, want := range map[string]string{
		"akita-v4-baseline.jsonl": "80f257d33ddeb9e2a10ccf3f310e2b3aaa09b0f2417e2c3d6950214b88ad344f", "pipeline-v4-baseline.jsonl": "2834a614b5147b0500cb000e7437e879e594af59339a8ee487bda13476d9820a",
	} {
		b, e := os.ReadFile(filepath.Join(root, "pkg/cpu/testdata", path))
		if e != nil {
			t.Fatal(e)
		}
		if hash(b) != want {
			t.Fatalf("fixture changed: %s; investigate, do not regenerate", path)
		}
	}
}

func TestManifestPilotSelection(t *testing.T) {
	m, e := LoadManifest(repo(t))
	if e != nil {
		t.Fatal(e)
	}
	ns, e := m.PilotSizes(nil)
	if e != nil || len(ns) != 6 || ns[2] != 120 {
		t.Fatalf("%v %v", ns, e)
	}
	if _, e = m.PilotSizes([]int{9}); e == nil {
		t.Fatal("accepted undeclared size")
	}
	if _, e = m.PilotSizes([]int{8, 8}); e == nil {
		t.Fatal("accepted duplicate size")
	}
}
func TestCaptureLogBinding(t *testing.T) {
	dir := t.TempDir()
	code := 0
	c := Capture{SchemaVersion: 1, Commit: "abc", Started: "before", Finished: "after", Platform: "test", GoVersion: "test", Sources: map[string]string{"a": "b"}, LogSHA: hash([]byte("original")), ExitCode: &code}
	path := filepath.Join(dir, "capture.json")
	if e := SaveCapture(path, c); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadCapture(path, []byte("original")); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadCapture(path, []byte("tampered")); e == nil {
		t.Fatal("accepted wrong log")
	}
}
