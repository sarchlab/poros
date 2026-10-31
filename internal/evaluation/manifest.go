// Package evaluation implements the versioned validation contract, not CPU behavior.
package evaluation

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sarchlab/poros/internal/benchmarks"
)

type Size struct {
	ID              string         `json:"id"`
	Dimensions      map[string]int `json:"dimensions"`
	WorkingSetBytes *uint64        `json:"working_set_bytes"`
	Status          string         `json:"status"`
	Constraint      string         `json:"constraint"`
}
type Workload struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Role      string `json:"role"`
	Execution string `json:"execution"`
	Source    string `json:"source"`
	Sizes     []Size `json:"sizes"`
}
type Manifest struct {
	SchemaVersion   int        `json:"schema_version"`
	SplitVersion    string     `json:"split_version"`
	SplitLimitation string     `json:"split_limitation"`
	Workloads       []Workload `json:"workloads"`
}

func Decode(r io.Reader, v any) error {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one JSON value, got %v", err)
	}
	return nil
}
func LoadManifest(root string) (Manifest, error) {
	var m Manifest
	f, e := os.Open(filepath.Join(root, "evaluation/workloads.json"))
	if e != nil {
		return m, e
	}
	defer f.Close()
	if e = Decode(f, &m); e != nil {
		return m, e
	}
	return m, m.Validate(root)
}
func (m Manifest) Validate(root string) error {
	if m.SchemaVersion != 1 || m.SplitVersion == "" || m.SplitLimitation == "" {
		return fmt.Errorf("missing/unsupported manifest contract")
	}
	expected := map[string]bool{"mvt": true}
	for _, name := range benchmarks.ComputeNames() {
		expected[name] = true
	}
	catalog, err := benchmarks.Load(root)
	if err != nil {
		return err
	}
	migrated := map[string]benchmarks.Workload{}
	for _, w := range catalog.Workloads {
		migrated[w.Name] = w
	}
	seen := map[string]bool{}
	for _, w := range m.Workloads {
		if w.ID == "" || seen[w.ID] {
			return fmt.Errorf("empty/duplicate workload %q", w.ID)
		}
		seen[w.ID] = true
		if w.Role != "calibration" && w.Role != "validation" && w.Role != "validation-legacy" {
			return fmt.Errorf("invalid role %q", w.Role)
		}
		if w.Status != "planned" && w.Status != "elf" {
			return fmt.Errorf("invalid status %q", w.Status)
		}
		if w.Status == "elf" && w.ID != "mvt" {
			b, ok := migrated[w.ID]
			if !ok || w.Execution != "linux-elf-user" || w.Source != b.Source || len(w.Sizes) != 6 {
				return fmt.Errorf("invalid ELF workload %s", w.ID)
			}
			for i, size := range w.Sizes {
				c := b.Cases[i]
				if size.ID != c.ID || size.Status != "elf" || len(size.Dimensions) != len(b.Dimensions) {
					return fmt.Errorf("ELF case inventory mismatch %s", w.ID)
				}
				for j, d := range b.Dimensions {
					if fmt.Sprint(size.Dimensions[d.Name]) != c.Args[j] {
						return fmt.Errorf("ELF dimensions mismatch %s", w.ID)
					}
				}
			}
			delete(expected, w.ID)
		}
		if w.Status == "elf" && w.ID == "mvt" {
			if w.ID != "mvt" || w.Role != "calibration" || w.Execution != "linux-elf-user" || w.Source != "tests/usertime/mvt.c" || len(w.Sizes) != 6 {
				return fmt.Errorf("invalid migrated ELF workload")
			}
			if _, err := os.Stat(filepath.Join(root, w.Source)); err != nil {
				return err
			}
			raw, err := os.ReadFile(filepath.Join(root, "tests/usertime/manifest.json"))
			if err != nil {
				return err
			}
			var suite struct {
				Schema int `json:"schema"`
				Cases  []struct {
					ID, Workload string
					Point        int
				} `json:"cases"`
			}
			if err := json.Unmarshal(raw, &suite); err != nil {
				return err
			}
			allowed := map[int]bool{}
			for _, c := range suite.Cases {
				if c.Workload != "mvt" {
					continue
				}
				if allowed[c.Point] || c.ID != fmt.Sprintf("mvt-%d", c.Point) {
					return fmt.Errorf("invalid MVT ELF inventory")
				}
				allowed[c.Point] = true
			}
			if suite.Schema != 2 || len(allowed) != len(w.Sizes) {
				return fmt.Errorf("MVT inventory differs between manifests")
			}
			for _, size := range w.Sizes {
				n := size.Dimensions["n"]
				if size.Status != "elf" || size.ID != fmt.Sprintf("n%d", n) || !allowed[n] {
					return fmt.Errorf("MVT size differs from ELF inventory")
				}
				delete(allowed, n)
			}
			delete(expected, w.ID)
		}
		sizes := map[string]bool{}
		for _, s := range w.Sizes {
			if s.ID == "" || sizes[s.ID] || s.Constraint == "" {
				return fmt.Errorf("invalid size in %s", w.ID)
			}
			sizes[s.ID] = true
			if s.Status != "planned" && s.Status != "elf" {
				return fmt.Errorf("invalid size status")
			}
			if w.Status == "planned" && s.Status != "planned" {
				return fmt.Errorf("planned workload cannot have executable sizes")
			}
			if len(s.Dimensions) == 0 || ((w.ID == "mvt" || s.Status == "planned") && (s.WorkingSetBytes == nil || *s.WorkingSetBytes == 0)) {
				return fmt.Errorf("size needs dimensions and bytes")
			}
			for _, n := range s.Dimensions {
				if n < 1 {
					return fmt.Errorf("nonpositive dimension")
				}
			}
			if s.Status == "elf" && w.ID == "mvt" {
				n, ok := s.Dimensions["n"]
				if w.ID != "mvt" || !ok || len(s.Dimensions) != 1 || n < 1 || n > 1536 || *s.WorkingSetBytes != uint64(8*(n*n+4*n)) {
					return fmt.Errorf("invalid MVT ELF size")
				}
			}
		}

	}
	if len(expected) > 0 {
		return fmt.Errorf("missing required workloads: %v", expected)
	}
	return nil
}
