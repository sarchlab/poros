// Package benchmarks describes complete, freestanding Linux ELF workloads.
// It contains no guest register setup, application callbacks or simulated code.
package benchmarks

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ComputeNames is deliberately independent of the editable manifest. Removing a
// workload from its manifest cannot remove it from required regression coverage.
const computeInventory = "atax gemm 2mm 3mm bicg gesummv gemver doitgen trisolv durbin jacobi1d syrk syr2k trmm symm cholesky lu floydwarshall covariance correlation gramschmidt ludcmp nussinov adi fdtd2d heat3d jacobi2d seidel2d deriche swaptions blackscholes streamcluster canneal fluidanimate raytrace vips bodytrack facesim ferret x264"

func ComputeNames() []string { return strings.Fields(computeInventory) }

type Dimension struct {
	Name     string
	Min, Max int
}
type Output struct {
	Type  string
	Count int
}
type Case struct {
	ID      string `json:"id"`
	Args    []string
	Outputs []Output
}
type Workload struct {
	Name, Suite, Source, Kernel, Binary string
	Dimensions                          []Dimension
	Cases                               []Case
}
type Manifest struct {
	SchemaVersion int `json:"schema_version"`
	Workloads     []Workload
}

func Load(root string) (Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(filepath.Join(root, "tests/benchmarks/manifest.json"))
	if err != nil {
		return m, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&m); err != nil {
		return m, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return m, fmt.Errorf("expected one JSON value")
	}
	if err := CheckProvenance(root, m); err != nil {
		return m, err
	}
	if m.SchemaVersion != 1 || len(m.Workloads) != len(ComputeNames()) {
		return m, fmt.Errorf("incomplete benchmark manifest")
	}
	expected := map[string]bool{}
	for _, n := range ComputeNames() {
		expected[n] = true
	}
	for _, w := range m.Workloads {
		if !expected[w.Name] {
			return m, fmt.Errorf("unexpected or duplicate workload %q", w.Name)
		}
		delete(expected, w.Name)
		if len(w.Cases) != 6 || len(w.Dimensions) == 0 {
			return m, fmt.Errorf("%s requires six cases and dimensions", w.Name)
		}
		if w.Source != "tests/benchmarks/"+w.Name+".c" || w.Binary != "tests/benchmarks/"+w.Name+".elf" {
			return m, fmt.Errorf("invalid paths for %s", w.Name)
		}
		for _, p := range []string{w.Source, w.Binary} {
			if _, err := os.Stat(filepath.Join(root, p)); err != nil {
				return m, err
			}
		}
		dimensions := map[string]bool{}
		for _, d := range w.Dimensions {
			if d.Name == "" || dimensions[d.Name] {
				return m, fmt.Errorf("empty/duplicate dimension in %s", w.Name)
			}
			dimensions[d.Name] = true
		}
		inputs := map[string]bool{}
		for i, c := range w.Cases {
			key := strings.Join(c.Args, " ")
			if c.ID != strconv.Itoa(i+1) || len(c.Args) != len(w.Dimensions) || inputs[key] || len(c.Outputs) == 0 {
				return m, fmt.Errorf("invalid case %s/%s", w.Name, c.ID)
			}
			inputs[key] = true
			for j, s := range c.Args {
				n, err := strconv.Atoi(s)
				d := w.Dimensions[j]
				if err != nil || n < d.Min || n > d.Max || d.Min < 1 || d.Min > d.Max {
					return m, fmt.Errorf("invalid argument %s/%s/%s", w.Name, c.ID, s)
				}
			}
			for _, o := range c.Outputs {
				if (o.Type != "double" && o.Type != "int") || o.Count < 1 || o.Count > 1<<20 {
					return m, fmt.Errorf("invalid output %s/%s", w.Name, c.ID)
				}
			}
		}
	}
	return m, nil
}

// CheckOutput checks the entire output, including tails and untouched boundary
// cells. Floating point comparisons reject NaN/Inf in both actual and oracle.
// Integer outputs are exact. The oracle is a separate native compiler/runtime
// or execution of the identical ELF in QEMU/Linux, never simulator output.
func CheckOutput(c Case, actual, reference []byte) error {
	size := 0
	for _, o := range c.Outputs {
		if o.Type == "int" {
			size += 4 * o.Count
		} else {
			size += 8 * o.Count
		}
	}
	if len(actual) != size || len(reference) != size {
		return fmt.Errorf("output size got %d reference %d want %d", len(actual), len(reference), size)
	}
	offset := 0
	for _, o := range c.Outputs {
		for i := 0; i < o.Count; i++ {
			if o.Type == "int" {
				a, b := binary.LittleEndian.Uint32(actual[offset:]), binary.LittleEndian.Uint32(reference[offset:])
				if a != b {
					return fmt.Errorf("integer at byte %d: got %d want %d", offset, int32(a), int32(b))
				}
				offset += 4
				continue
			}
			a, b := math.Float64frombits(binary.LittleEndian.Uint64(actual[offset:])), math.Float64frombits(binary.LittleEndian.Uint64(reference[offset:]))
			if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) || math.Abs(a-b) > 1e-10+1e-9*math.Max(math.Abs(a), math.Abs(b)) {
				return fmt.Errorf("double at byte %d: got %.17g want %.17g", offset, a, b)
			}
			offset += 8
		}
	}
	return nil
}

// CheckProvenance fails before execution if any included source or checked-in
// executable differs from the last successful recorded cross-compiler build.
func CheckProvenance(root string, m Manifest) error {
	var p struct {
		Schema           int `json:"schema"`
		Compiler, Linker string
		Files            map[string]string
		Libraries        map[string]string
	}
	b, err := os.ReadFile(filepath.Join(root, "tests/benchmarks/provenance.json"))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &p); err != nil {
		return err
	}
	if p.Schema != 1 || p.Compiler == "" || p.Linker == "" || len(p.Libraries) != 2 || len(p.Libraries["libc.a"]) != 64 || len(p.Libraries["libgcc.a"]) != 64 {
		return fmt.Errorf("invalid ELF build provenance")
	}
	required := []string{"tests/benchmarks/runtime.h", "tests/benchmarks/build.sh", "tests/benchmarks/provenance.py", "tests/benchmarks/atax_kernel.h", "tests/usertime/start.S", "tests/usertime/layout.ld", "tests/benchmarks/manifest.json", "tests/benchmarks/support/math.c", "tests/benchmarks/support/math.elf"}
	for _, w := range m.Workloads {
		required = append(required, w.Source, w.Binary, w.Kernel)
	}
	for _, name := range required {
		if p.Files[name] == "" {
			return fmt.Errorf("missing build provenance: %s", name)
		}
	}
	for name, want := range p.Files {
		if filepath.IsAbs(name) || strings.Contains(name, "..") {
			return fmt.Errorf("invalid provenance path")
		}
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != want {
			return fmt.Errorf("stale ELF build: %s changed; rebuild tests/benchmarks", name)
		}
	}
	return nil
}

// Inventory adapts the existing accuracy suite into the same ELF-only catalog.
// Its source, binaries, six points and reference measurements stay unchanged.
func Inventory(root string) ([]Workload, error) {
	m, err := Load(root)
	if err != nil {
		return nil, err
	}
	var old struct {
		Cases []struct {
			ID, Workload, File string
			Args               []string
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "tests/usertime/manifest.json"))
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &old); err != nil {
		return nil, err
	}
	for _, name := range []string{"mvt", "integer", "memory", "branch"} {
		w := Workload{Name: name, Suite: "micro", Binary: "tests/usertime/" + name + ".elf", Source: "tests/usertime/program.c"}
		if name == "mvt" {
			w.Suite = "polybench"
			w.Source = "tests/usertime/mvt.c"
		}
		for _, c := range old.Cases {
			if c.Workload == name {
				if len(c.Args) < 2 || c.File != name+".elf" {
					return nil, fmt.Errorf("invalid existing ELF case")
				}
				w.Cases = append(w.Cases, Case{ID: strconv.Itoa(len(w.Cases) + 1), Args: c.Args[1:]})
			}
		}
		if len(w.Cases) != 6 {
			return nil, fmt.Errorf("missing existing six-point workload %s", name)
		}
		m.Workloads = append(m.Workloads, w)
	}
	return m.Workloads, nil
}
