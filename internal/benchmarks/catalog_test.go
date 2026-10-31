package benchmarks

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestCatalogInventory(t *testing.T) {
	m, err := Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Workloads) != 40 {
		t.Fatal("wrong migrated inventory")
	}
	all, err := Inventory("../..")
	if err != nil || len(all) != 44 {
		t.Fatalf("all inventory %d %v", len(all), err)
	}
}
func TestCompleteOutputRejectsCorruption(t *testing.T) {
	c := Case{Outputs: []Output{{Type: "double", Count: 3}, {Type: "int", Count: 1}}}
	good := make([]byte, 28)
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint64(good[i*8:], math.Float64bits(float64(i+1)))
	}
	binary.LittleEndian.PutUint32(good[24:], 7)
	if err := CheckOutput(c, good, good); err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int{0, 16, 24} {
		bad := append([]byte(nil), good...)
		bad[offset+2] ^= 0x80
		if offset < 24 {
			binary.LittleEndian.PutUint64(bad[offset:], math.Float64bits(100))
		}
		if CheckOutput(c, bad, good) == nil {
			t.Fatal("accepted output corruption", offset)
		}
	}
	for _, f := range []float64{math.NaN(), math.Inf(1)} {
		bad := append([]byte(nil), good...)
		binary.LittleEndian.PutUint64(bad[16:], math.Float64bits(f))
		if CheckOutput(c, bad, good) == nil || CheckOutput(c, good, bad) == nil {
			t.Fatal("accepted nonfinite output/oracle")
		}
	}
	if CheckOutput(c, good[:27], good) == nil || CheckOutput(c, append(good, 0), good) == nil {
		t.Fatal("accepted truncated/extra output")
	}
	if CheckNumerical(Workload{Name: "facesim"}, c, nil) == nil {
		t.Fatal("numerical checker accepted missing output")
	}
}
func TestBodytrackTinyWeightsAreNotZero(t *testing.T) {
	w := Workload{Name: "bodytrack", Dimensions: make([]Dimension, 3)}
	c := Case{Args: []string{"64", "6", "8"}, Outputs: []Output{{Type: "double", Count: 64}}}
	if CheckNumerical(w, c, make([]byte, 64*8)) == nil {
		t.Fatal("all-zero likelihoods accepted")
	}
}
