// Package evaluation implements the versioned validation contract, not CPU behavior.
package evaluation

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sarchlab/poros/pkg/benchmark"
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
	expected := map[string]bool{}
	for _, b := range benchmark.All() {
		expected[b.Name] = true
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
		if w.Status != "legacy" && w.Status != "planned" {
			return fmt.Errorf("invalid status %q", w.Status)
		}
		if w.Status == "legacy" {
			if !expected[w.ID] || w.Execution != "embedded-function" {
				return fmt.Errorf("unknown legacy workload %s", w.ID)
			}
			if w.Source == "" || filepath.IsAbs(w.Source) || strings.Contains(w.Source, "..") {
				return fmt.Errorf("invalid source path")
			}
			if _, err := os.Stat(filepath.Join(root, w.Source)); err != nil {
				return err
			}
			delete(expected, w.ID)
		}
		sizes := map[string]bool{}
		for _, s := range w.Sizes {
			if s.ID == "" || sizes[s.ID] || s.Constraint == "" {
				return fmt.Errorf("invalid size in %s", w.ID)
			}
			sizes[s.ID] = true
			if s.Status != "legacy" && s.Status != "planned" && s.Status != "pilot" {
				return fmt.Errorf("invalid size status")
			}
			if w.Status == "planned" && s.Status != "planned" {
				return fmt.Errorf("planned workload cannot have executable sizes")
			}
			if s.Status != "legacy" {
				if len(s.Dimensions) == 0 || s.WorkingSetBytes == nil || *s.WorkingSetBytes == 0 {
					return fmt.Errorf("size needs dimensions and bytes")
				}
				for _, n := range s.Dimensions {
					if n < 1 {
						return fmt.Errorf("nonpositive dimension")
					}
				}
			}
			if s.Status == "pilot" {
				n, ok := s.Dimensions["n"]
				if w.ID != "mvt" || !ok || len(s.Dimensions) != 1 || n < 1 || n > 1536 || *s.WorkingSetBytes != uint64(8*(n*n+4*n)) {
					return fmt.Errorf("invalid MVT pilot size")
				}
			}
		}
		if w.Status == "legacy" && !sizes["legacy-default"] {
			return fmt.Errorf("missing legacy default")
		}
	}
	if len(expected) > 0 {
		return fmt.Errorf("missing legacy workloads: %v", expected)
	}
	return nil
}

func (m Manifest) PilotSizes(requested []int) ([]int, error) {
	allowed := map[int]bool{}
	defaults := []int{}
	for _, w := range m.Workloads {
		if w.ID == "mvt" {
			for _, s := range w.Sizes {
				if s.Status == "pilot" {
					n := s.Dimensions["n"]
					if allowed[n] || s.ID != fmt.Sprintf("n%d", n) {
						return nil, fmt.Errorf("duplicate dimension or mismatched pilot ID")
					}
					allowed[n] = true
					defaults = append(defaults, n)
				}
			}
		}
	}
	if len(defaults) == 0 {
		return nil, fmt.Errorf("manifest has no MVT pilot sizes")
	}
	if len(requested) == 0 {
		return defaults, nil
	}
	seen := map[int]bool{}
	for _, n := range requested {
		if !allowed[n] || seen[n] {
			return nil, fmt.Errorf("undeclared or duplicate pilot size %d", n)
		}
		seen[n] = true
	}
	return requested, nil
}
