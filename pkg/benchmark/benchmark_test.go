package benchmark

import (
	"math"
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

// TestBySuiteParsec verifies that BySuite("parsec") returns exactly 3 benchmarks.
func TestBySuiteParsec(t *testing.T) {
	parsec := BySuite("parsec")
	if len(parsec) != 3 {
		t.Errorf("BySuite(\"parsec\") returned %d, want 3", len(parsec))
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

// TestStreamclusterCorrectness verifies the streamcluster kernel produces
// correct Euclidean distance-squared values.
func TestStreamclusterCorrectness(t *testing.T) {
	b := ByName("streamcluster")
	if b == nil {
		t.Fatal("streamcluster benchmark not found")
	}

	em := b.Setup()
	err := em.Run()
	if err != nil {
		t.Fatalf("emulator error: %v", err)
	}

	npoints, dim := 32, 16
	totalCoords := npoints * dim

	p1 := make([]float64, totalCoords)
	p2 := make([]float64, totalCoords)
	for i := 0; i < totalCoords; i++ {
		p1[i] = 1.0 + float64(i)*0.01
		p2[i] = 2.0 - float64(i)*0.02
	}

	outBase := uint64(0x300000)
	for i := 0; i < npoints; i++ {
		got := em.Mem.ReadF64(outBase + uint64(i*8))

		expected := 0.0
		for j := 0; j < dim; j++ {
			diff := p1[i*dim+j] - p2[i*dim+j]
			expected += diff * diff
		}

		if math.Abs(got-expected) > 1e-6 {
			t.Errorf("out[%d]: got %f, want %f (diff=%e)", i, got, expected, got-expected)
		}
	}
}
