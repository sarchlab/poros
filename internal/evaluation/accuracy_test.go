package evaluation

import (
	"math"
	"testing"
)

func TestAccuracyFormula(t *testing.T) {
	for _, pair := range [][2]float64{{2, 1}, {.5, 1}} {
		v, e := AccuracyError(pair[0], pair[1])
		if e != nil || v != 100 {
			t.Fatal(v, e)
		}
	}
	for _, v := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, e := AccuracyError(v, 1); e == nil {
			t.Fatal("invalid accepted")
		}
	}
}
