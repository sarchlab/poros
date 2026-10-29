package evaluation

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"
)

// Capture binds a test log to the source/environment observed before execution.
// It is local evidence, not a cryptographic attestation of a remote machine.
type Capture struct {
	SchemaVersion int               `json:"schema_version"`
	Commit        string            `json:"tested_commit"`
	Dirty         bool              `json:"tested_working_tree_dirty"`
	GoVersion     string            `json:"tested_go_version"`
	Platform      string            `json:"tested_platform"`
	Started       string            `json:"started_utc"`
	Finished      string            `json:"finished_utc"`
	Sources       map[string]string `json:"source_sha256"`
	LogSHA        string            `json:"log_sha256"`
	ExitCode      *int              `json:"test_exit_code"`
}

func sourceHashes(root string) (map[string]string, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	out, e := cmd.Output()
	if e != nil {
		return nil, e
	}
	hashes := map[string]string{}
	for _, name := range strings.Split(string(out), "\x00") {
		if name == "" {
			continue
		}
		b, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			return nil, e
		}
		hashes[name] = hash(b)
	}
	return hashes, nil
}
func BeginCapture(root string) (Capture, error) {
	c := Capture{SchemaVersion: 1, GoVersion: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH, Started: time.Now().UTC().Format(time.RFC3339Nano)}
	b, e := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if e != nil {
		return c, e
	}
	c.Commit = strings.TrimSpace(string(b))
	c.Dirty, e = WorkingTreeDirty(root)
	if e != nil {
		return c, e
	}
	c.Sources, e = sourceHashes(root)
	return c, e
}
func SealCapture(root, path, log string, exit int) (Capture, error) {
	var c Capture
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = Decode(strings.NewReader(string(b)), &c); e != nil {
		return c, e
	}
	now, e := BeginCapture(root)
	if e != nil {
		return c, e
	}
	if c.SchemaVersion != 1 || c.Finished != "" || c.Started == "" || c.Commit != now.Commit || c.Dirty != now.Dirty || c.Platform != now.Platform || c.GoVersion != now.GoVersion || !reflect.DeepEqual(c.Sources, now.Sources) {
		return c, fmt.Errorf("source/environment changed during tests or invalid capture")
	}
	b, e = os.ReadFile(log)
	if e != nil {
		return c, e
	}
	c.LogSHA = hash(b)
	c.Finished = time.Now().UTC().Format(time.RFC3339Nano)
	c.ExitCode = &exit
	return c, nil
}
func ReadCapture(path string, log []byte) (*Capture, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c Capture
	if e = Decode(strings.NewReader(string(b)), &c); e != nil {
		return nil, e
	}
	if c.SchemaVersion != 1 || c.Commit == "" || c.Started == "" || c.Finished == "" || c.Platform == "" || c.GoVersion == "" || len(c.Sources) == 0 || c.ExitCode == nil || c.LogSHA != hash(log) {
		return nil, fmt.Errorf("missing or mismatched tested provenance")
	}
	return &c, nil
}
func SaveCapture(path string, c Capture) error {
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}

func CheckCapturePath(root, path string) error {
	absRoot, e := filepath.Abs(root)
	if e != nil {
		return e
	}
	absPath, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	rel, e := filepath.Rel(absRoot, absPath)
	if e != nil {
		return e
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	if e := exec.Command("git", "-C", root, "check-ignore", "--quiet", "--", rel).Run(); e != nil {
		return fmt.Errorf("capture output must be outside repository or git-ignored: %s", path)
	}
	return nil
}
