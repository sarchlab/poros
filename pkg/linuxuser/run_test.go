package linuxuser

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type fixtureCase struct {
	Name, File, SHA256, Termination string
	Args, Env                       []string
	ExitStatus                      *int   `json:"exit_status"`
	Stdout                          string `json:"stdout_hex"`
	Stderr                          string `json:"stderr_hex"`
	MaxInstructions                 uint64 `json:"max_instructions"`
	MaxOutput                       int    `json:"max_output"`
}

func fixtureCases(t *testing.T) []fixtureCase {
	t.Helper()
	root := "../../tests/elf"
	b, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Sources map[string]string
		Cases   []fixtureCase
	}
	if err = json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	names := []string{"hello", "startup", "memory", "compute", "syscall-errors", "bad-load", "bad-store", "nx", "unsupported-syscall", "unsupported-instruction", "loop", "output-limit", "exit-status", "arithmetic-loop"}
	if len(m.Cases) != len(names) {
		t.Fatal("required fixture inventory changed")
	}
	for i, c := range m.Cases {
		if c.Name != names[i] || c.File != c.Name+".elf" {
			t.Fatal("missing or reordered required fixture", names[i])
		}
	}
	for _, name := range []string{"start.S", "guest.c", "layout.ld", "build.sh"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != m.Sources[name] {
			t.Fatal("fixture source provenance mismatch", name)
		}
	}
	return m.Cases
}
func TestRequiredELFFunctionalFixtures(t *testing.T) {
	for _, c := range fixtureCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			b, err := os.ReadFile("../../tests/elf/" + c.File)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(b)
			if hex.EncodeToString(sum[:]) != c.SHA256 {
				t.Fatal("fixture binary hash mismatch")
			}
			p, err := Parse(b, Options{Args: c.Args, Env: c.Env, ExecName: c.Args[0]})
			if err != nil {
				t.Fatal(err)
			}
			r, err := p.Run(context.Background(), RunOptions{MaxInstructions: c.MaxInstructions, MaxOutput: c.MaxOutput, Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			if r.Termination != c.Termination || !reflect.DeepEqual(r.ExitStatus, c.ExitStatus) || (c.Name != "compute" && hex.EncodeToString(r.Stdout) != c.Stdout) || hex.EncodeToString(r.Stderr) != c.Stderr {
				t.Fatalf("unexpected result: %+v", r)
			}
			if c.Name == "compute" {
				if err := checkCompute(r.Stdout); err != nil {
					t.Fatal(err)
				}
			}
			if r.Instructions == 0 || r.Instructions > c.MaxInstructions {
				t.Fatal("fixture did not execute")
			}
			if _, err = p.Run(context.Background(), RunOptions{}); err == nil {
				t.Fatal("second Run accepted")
			}
		})
	}
}
func TestRunBudgets(t *testing.T) {
	b, err := os.ReadFile("../../tests/elf/loop.elf")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Run(context.Background(), RunOptions{MaxOutput: MaxOutputBytes + 1}); err == nil {
		t.Fatal("unbounded output accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := p.Run(ctx, RunOptions{})
	if err != nil || r.Termination != "time_limit" || r.Instructions != 0 {
		t.Fatalf("canceled run: %+v %v", r, err)
	}
	p, err = Parse(b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r, err = p.Run(context.Background(), RunOptions{MaxInstructions: 7})
	if err != nil || r.Termination != "instruction_limit" || r.Instructions != 7 {
		t.Fatalf("instruction bound: %+v %v", r, err)
	}
}

// Independent numerical oracle: integer/FMA edge exact; remaining binary64
// values finite and within absolute 1e-15 or relative 1e-14 tolerance.
func checkCompute(b []byte) error {
	if len(b) != 40 {
		return fmt.Errorf("wrong numerical output length")
	}
	if binary.LittleEndian.Uint64(b) != 96 || binary.LittleEndian.Uint64(b[24:]) != 0xbc90000000000000 {
		return fmt.Errorf("integer or FMA rounding edge differs")
	}
	for i, want := range map[int]float64{8: 3.25, 16: .875, 32: .1} {
		got := math.Float64frombits(binary.LittleEndian.Uint64(b[i:]))
		delta := math.Abs(got - want)
		if math.IsNaN(got) || math.IsInf(got, 0) || delta > math.Max(1e-15, 1e-14*math.Max(math.Abs(got), math.Abs(want))) {
			return fmt.Errorf("invalid numeric result %g, expected %g", got, want)
		}
	}
	return nil
}
func TestNumericalOracle(t *testing.T) {
	b := make([]byte, 40)
	binary.LittleEndian.PutUint64(b, 96)
	for i, v := range map[int]float64{8: 3.25, 16: .875, 24: -math.Ldexp(1, -54), 32: .1} {
		binary.LittleEndian.PutUint64(b[i:], math.Float64bits(v))
	}
	if err := checkCompute(b); err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint64(b[32:], math.Float64bits(math.Nextafter(.1, 1)))
	if err := checkCompute(b); err != nil {
		t.Fatal("in-tolerance result rejected", err)
	}
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), .11} {
		binary.LittleEndian.PutUint64(b[32:], math.Float64bits(v))
		if checkCompute(b) == nil {
			t.Fatal("invalid numerical result accepted", v)
		}
	}
}
