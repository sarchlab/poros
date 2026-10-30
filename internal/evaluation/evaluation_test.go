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
	enc.Encode(map[string]string{"Action": "pass", "Package": "github.com/sarchlab/poros/pkg/cpu", "Test": "TestRegistryFacesimNumericalReference"})
	return b.Bytes()
}
func TestRegressionCoverage(t *testing.T) {
	root := repo(t)
	req, e := RequiredCases(root, "full")
	if e != nil || len(req) != 88 {
		t.Fatalf("%d %v", len(req), e)
	}
	good := logFor(t, root, req, "", "pass")
	r, e := Regression(root, "full", "test", bytes.NewReader(good))
	if e != nil || r.Status != "pass" || len(r.Cases) != 88 || len(r.Planned) != 6 {
		t.Fatalf("%+v %v", r, e)
	}
	for _, bad := range [][]byte{logFor(t, root, req, "TestV4Baseline/atax/1", "pass"), logFor(t, root, req, "", "skip"), logFor(t, root, req, "", "fail"), good[:len(good)/2], nil} {
		if _, e := Regression(root, "full", "test", bytes.NewReader(bad)); e == nil {
			t.Fatal("accepted incomplete/failed report")
		}
	}
	if _, e := Regression(root, "full", "test", bytes.NewReader(append(good, []byte("garbage")...))); e == nil {
		t.Fatal("accepted invalid JSON")
	}
}
func TestPortableHookRequiresNumericalReference(t *testing.T) {
	root := repo(t)
	req, err := RequiredCases(root, "full")
	if err != nil {
		t.Fatal(err)
	}
	good := logFor(t, root, req, "", "pass")
	for _, workload := range []string{"blackscholes", "bodytrack"} {
		for _, action := range []string{"missing", "skip", "fail", "pass"} {
			t.Run(workload+"/"+action, func(t *testing.T) {
				var log bytes.Buffer
				log.Write(good)
				enc := json.NewEncoder(&log)
				enc.Encode(map[string]string{"Action": "output", "Package": "github.com/sarchlab/poros/pkg/cpu", "Test": "TestV4Baseline/" + workload + "/1", "Output": "nonportable host-math hook reference:"})
				if action != "missing" {
					enc.Encode(map[string]string{"Action": action, "Package": "github.com/sarchlab/poros/pkg/cpu", "Test": map[string]string{"blackscholes": "TestRegistryBlackScholesNumericalReference", "bodytrack": "TestRegistryBodytrackNumericalReference"}[workload]})
				}
				report, err := Regression(root, "full", "test", &log)
				if (err == nil) != (action == "pass") {
					t.Fatalf("status %s: %v", action, err)
				}
				if len(report.Cases) != 88 || len(report.IndependentChecks) != 2 || report.IndependentChecks[0].Status != action {
					t.Fatalf("incorrect coverage: %+v", report)
				}
			})
		}
	}
}

func TestFacesimReferenceRequired(t *testing.T) {
	root := repo(t)
	for _, tier := range []string{"short", "full"} {
		req, err := RequiredCases(root, tier)
		if err != nil {
			t.Fatal(err)
		}
		good := logFor(t, root, req, "", "pass")
		lines := bytes.Split(good, []byte("\n"))
		var without bytes.Buffer
		for _, line := range lines {
			if len(line) > 0 && !bytes.Contains(line, []byte("TestRegistryFacesimNumericalReference")) {
				without.Write(line)
				without.WriteByte('\n')
			}
		}
		for _, action := range []string{"missing", "skip", "fail", "pass"} {
			t.Run(tier+"/"+action, func(t *testing.T) {
				var log bytes.Buffer
				log.Write(without.Bytes())
				if action != "missing" {
					json.NewEncoder(&log).Encode(map[string]string{"Action": action, "Package": "github.com/sarchlab/poros/pkg/cpu", "Test": "TestRegistryFacesimNumericalReference"})
				}
				_, err := Regression(root, tier, "test", &log)
				if (err == nil) != (action == "pass") {
					t.Fatalf("action%s error%v", action, err)
				}
			})
		}
	}
}

func TestMissingFixture(t *testing.T) {
	if _, e := RequiredCases(t.TempDir(), "full"); e == nil {
		t.Fatal("accepted missing fixture")
	}
}
func TestFixtureIdentity(t *testing.T) {
	root := repo(t)
	for path, want := range map[string]string{
		"akita-v4-baseline.jsonl": "4e7f6639bade643ca713b37378b14d67345b31377f41a10fd1f1b6b5790c077f", "pipeline-v4-baseline.jsonl": "2834a614b5147b0500cb000e7437e879e594af59339a8ee487bda13476d9820a",
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

func TestCaptureLifecycle(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "repo")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %s: %v", args, out, e)
		}
	}
	git("init", "-q")
	source := filepath.Join(root, "source.txt")
	if e := os.WriteFile(source, []byte("original"), 0600); e != nil {
		t.Fatal(e)
	}
	git("add", "source.txt")
	git("-c", "user.name=Evaluation Test", "-c", "user.email=evaluation@example.invalid", "commit", "-qm", "fixture")
	path := filepath.Join(dir, "capture.json")
	log := filepath.Join(dir, "log.jsonl")
	if e := os.WriteFile(log, []byte("log"), 0600); e != nil {
		t.Fatal(e)
	}
	c, e := BeginCapture(root)
	if e != nil {
		t.Fatal(e)
	}
	if e = SaveCapture(path, c); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCapture(path, []byte("log")); e == nil {
		t.Fatal("unsealed capture accepted")
	}
	sealed, e := SealCapture(root, path, log, 0)
	if e != nil {
		t.Fatal(e)
	}
	if e = SaveCapture(path, sealed); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCapture(path, []byte("log")); e != nil {
		t.Fatal(e)
	}
	if e = MatchCapture(root, &sealed); e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []string{"source", "commit", "dirty"} {
		other := sealed
		switch mutation {
		case "source":
			other.Sources = map[string]string{"source.txt": "different"}
		case "commit":
			other.Commit = "different"
		case "dirty":
			other.Dirty = true
		}
		if e = MatchCapture(root, &other); e == nil {
			t.Fatalf("accepted %s mismatch", mutation)
		}
	}
	if e = SaveCapture(path, c); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(source, []byte("edited during test"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = SealCapture(root, path, log, 0); e == nil {
		t.Fatal("source mutation accepted")
	}
	if e = MatchCapture(root, &sealed); e == nil {
		t.Fatal("accepted dirty analysis tree")
	}
	git("add", "source.txt")
	git("-c", "user.name=Evaluation Test", "-c", "user.email=evaluation@example.invalid", "commit", "-qm", "changed")
	if e = MatchCapture(root, &sealed); e == nil {
		t.Fatal("accepted changed analysis commit")
	}
	if _, e = SealCapture(root, path, log, 0); e == nil {
		t.Fatal("commit mutation accepted")
	}
	if _, e = ReadCapture(filepath.Join(dir, "missing"), []byte("log")); e == nil {
		t.Fatal("missing capture accepted")
	}
}

func TestMigratedMVTInventoryRequired(t *testing.T) {
	root := repo(t)
	original, err := LoadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"missing", "size", "status", "role"} {
		t.Run(mutation, func(t *testing.T) {
			data, _ := json.Marshal(original)
			var m Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			for i := range m.Workloads {
				w := &m.Workloads[i]
				if w.ID != "mvt" {
					continue
				}
				switch mutation {
				case "missing":
					m.Workloads = append(m.Workloads[:i], m.Workloads[i+1:]...)
				case "size":
					w.Sizes[0].ID = "n12"
					w.Sizes[0].Dimensions["n"] = 12
					*w.Sizes[0].WorkingSetBytes = 8 * (12*12 + 4*12)
				case "status":
					w.Sizes[0].Status = "planned"
				case "role":
					w.Role = "validation"
				}
				break
			}
			if m.Validate(root) == nil {
				t.Fatal("accepted incomplete or divergent MVT support")
			}
		})
	}
}
