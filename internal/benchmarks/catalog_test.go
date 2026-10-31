package benchmarks

import (
	"encoding/binary"
	"math"
	"strconv"
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

func TestTiledFacesimNumericalOutput(t *testing.T) {
	for _, n := range []int{33, 257} {
		in, err := facesimFixture(n)
		if err != nil {
			t.Fatal(err)
		}
		forces, _, err := facesimReference(in)
		if err != nil {
			t.Fatal(err)
		}
		raw := make([]byte, 8*len(forces))
		for i, v := range forces {
			binary.LittleEndian.PutUint64(raw[i*8:], math.Float64bits(v))
		}
		w := Workload{Name: "facesim", Dimensions: make([]Dimension, 1)}
		c := Case{Args: []string{strconv.Itoa(n)}, Outputs: []Output{{Type: "double", Count: len(forces)}}}
		if err := CheckNumerical(w, c, raw); err != nil {
			t.Fatal(err)
		}
		// Corrupt a late mesh block, not only the historical first block.
		binary.LittleEndian.PutUint64(raw[len(raw)-8:], math.Float64bits(1000))
		if CheckNumerical(w, c, raw) == nil {
			t.Fatal("accepted late-block corruption")
		}
	}
}

func TestCannealDeclaredRangeCannotOverflowInt32(t *testing.T) {
	// For fixed m the generated edges repeat every m/gcd(2,m). Bound the
	// remainder by the maximum Manhattan distance, and exploit monotonic cost
	// in n to cover every allowed n without executing millions of edges per m.
	const maxN, maxM = 8388608, 8192
	abs := func(x int64) int64 {
		if x < 0 {
			return -x
		}
		return x
	}
	all, even := make([]int64, maxM), make([]int64, maxM)
	for a := 0; a < maxM-1; a++ {
		cost := abs(int64((a*37)%1000-((a+1)*37)%1000)) + abs(int64((a*73)%1000-((a+1)*73)%1000))
		all[a+1] = all[a] + cost
		even[a+1] = even[a]
		if a%2 == 0 {
			even[a+1] += cost
		}
	}
	for m := 2; m <= maxM; m++ {
		period, total := m, all[m-1]+int64(((m-1)*37)%1000+((m-1)*73)%1000)
		if m%2 == 0 {
			period, total = m/2, even[m-1]
		}
		bound := int64(maxN/period)*total + int64(maxN%period)*1998
		if bound > math.MaxInt32 {
			t.Fatalf("m=%d upper cost=%d exceeds int32", m, bound)
		}
	}
}
