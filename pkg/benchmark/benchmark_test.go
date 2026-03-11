package benchmark

import (
	"testing"
)

// TestAllBenchmarkCount verifies that All() returns at least 32 benchmarks.
func TestAllBenchmarkCount(t *testing.T) {
	all := All()
	if len(all) < 32 {
		t.Errorf("All() returned %d benchmarks, want >= 32", len(all))
	}
}

// TestAllNamesUnique verifies that all benchmark names are unique.
func TestAllNamesUnique(t *testing.T) {
	all := All()
	seen := make(map[string]bool, len(all))
	for _, b := range all {
		if seen[b.Name] {
			t.Errorf("duplicate benchmark name: %q", b.Name)
		}
		seen[b.Name] = true
	}
}

// TestAllSetupNonNil verifies that each benchmark's Setup() returns a non-nil
// *emu.Emulator.
func TestAllSetupNonNil(t *testing.T) {
	all := All()
	for _, b := range all {
		em := b.Setup()
		if em == nil {
			t.Errorf("benchmark %q: Setup() returned nil", b.Name)
		}
	}
}

// TestBySuitePolybench verifies that BySuite("polybench") returns exactly 30
// benchmarks.
func TestBySuitePolybench(t *testing.T) {
	pb := BySuite("polybench")
	if len(pb) != 30 {
		t.Errorf("BySuite(\"polybench\") returned %d, want 30", len(pb))
	}
}

// TestBySuiteParsec verifies that BySuite("parsec") returns exactly 2 benchmarks.
func TestBySuiteParsec(t *testing.T) {
	parsec := BySuite("parsec")
	if len(parsec) != 2 {
		t.Errorf("BySuite(\"parsec\") returned %d, want 2", len(parsec))
	}
}

// TestQuickEval verifies that QuickEval() returns exactly 6 benchmarks.
func TestQuickEval(t *testing.T) {
	qe := QuickEval()
	if len(qe) != 6 {
		t.Errorf("QuickEval() returned %d, want 6", len(qe))
	}
}

// TestByName verifies that ByName returns a non-nil result for a known
// benchmark and nil for a nonexistent one.
func TestByName(t *testing.T) {
	b := ByName("atax")
	if b == nil {
		t.Error("ByName(\"atax\") returned nil, want non-nil")
	} else if b.Name != "atax" {
		t.Errorf("ByName(\"atax\").Name = %q, want \"atax\"", b.Name)
	}

	b = ByName("nonexistent")
	if b != nil {
		t.Errorf("ByName(\"nonexistent\") returned non-nil: %+v", b)
	}
}
