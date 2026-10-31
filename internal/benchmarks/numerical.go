package benchmarks

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

// CheckNumerical supplements compiler/ISA differential checks with equations
// independent of the C implementation for sensitive and integer workloads.
func CheckNumerical(w Workload, c Case, raw []byte) error {
	if err := CheckOutput(c, raw, raw); err != nil {
		return err
	}
	if len(c.Args) != len(w.Dimensions) {
		return fmt.Errorf("argument shape mismatch")
	}
	args := make([]int, len(c.Args))
	for i, s := range c.Args {
		args[i], _ = strconv.Atoi(s)
	}
	value := func(i int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(raw[8*i:])) }
	close := func(i int, want, atol, rtol float64) error {
		got := value(i)
		if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > atol+rtol*math.Abs(want) {
			return fmt.Errorf("%s numerical output[%d]=%.17g want %.17g", w.Name, i, got, want)
		}
		return nil
	}
	switch w.Name {
	case "gramschmidt":
		m, n := args[0], args[1]
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				dot := 0.0
				for r := 0; r < m; r++ {
					dot += value(n*n+r*n+i) * value(n*n+r*n+j)
				}
				want := 0.0
				if i == j {
					want = 1
				}
				if math.Abs(dot-want) > 1e-11 {
					return fmt.Errorf("QR columns %d,%d inner product %g", i, j, dot)
				}
			}
		}
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				sum := 0.0
				for k := 0; k < n; k++ {
					sum += value(n*n+i*n+k) * value(k*n+j)
				}
				want := float64((i*3+j*7)%11) / float64(20*m)
				if i == j {
					want += 2
				}
				if math.Abs(sum-want) > 1e-11 {
					return fmt.Errorf("QR reconstruction %d,%d=%g want%g", i, j, sum, want)
				}
			}
		}
	case "blackscholes":
		for i := 0; i < args[0]; i++ {
			s := math.FMA(float64(i%80), 1.25, 30)
			x := math.FMA(float64(i%80), 1.25, 28)
			r := math.FMA(float64(i%10), .005, .03)
			v := math.FMA(float64(i%20), .02, .15)
			t := math.FMA(float64(i%8), .25, .25)
			d1 := (math.Log(s/x) + (r+v*v/2)*t) / (v * math.Sqrt(t))
			d2 := d1 - v*math.Sqrt(t)
			cdf := func(x float64) float64 { return .5 * math.Erfc(-x/math.Sqrt2) }
			want := s*cdf(d1) - x*math.Exp(-r*t)*cdf(d2)
			if i%2 == 1 {
				want = x*math.Exp(-r*t)*cdf(-d2) - s*cdf(-d1)
			}
			// The source kernel intentionally uses the A&S normal-CDF approximation;
			// its error bound is 7.5e-8 per CDF, amplified by stock/discounted strike.
			if err := close(i, want, 8e-8*(s+x*math.Exp(-r*t)), 0); err != nil {
				return err
			}
		}
	case "bodytrack":
		n, sd, od := args[0], args[1], args[2]
		for p := 0; p < n; p++ {
			sum := 0.0
			for i := 0; i < od; i++ {
				pred := 0.0
				for j := 0; j < sd; j++ {
					state := math.FMA(.05, float64(j), .1*float64(p+1))
					pred += float64(((i+j)%5 + 1)) * .1 * state
				}
				diff := pred - math.FMA(.5, float64(i), 1)
				sum += diff * diff
			}
			want := math.Exp(-.5 * sum)
			if err := close(p, want, 4*math.SmallestNonzeroFloat64, 1e-10); err != nil {
				return err
			}
		}
	case "canneal":
		n, m := args[0], args[1]
		want := 0
		abs := func(x int) int {
			if x < 0 {
				return -x
			}
			return x
		}
		for i := 0; i < n; i++ {
			a, b := (2*i)%m, (2*i+1)%m
			want += abs((a*37)%1000-(b*37)%1000) + abs((a*73)%1000-(b*73)%1000)
		}
		if int(int32(binary.LittleEndian.Uint32(raw))) != want {
			return fmt.Errorf("incorrect Manhattan cost")
		}
	case "raytrace":
		for i := 0; i < args[0]; i++ {
			want := 5.0
			if i%4 == 2 {
				want = -1
			}
			if err := close(i, want, 1e-12, 0); err != nil {
				return err
			}
		}
	case "facesim":
		in, err := facesimFixture(args[0])
		if err != nil {
			return err
		}
		forces, _, err := facesimReference(in)
		if err != nil {
			return err
		}
		var balance [3]float64
		for i, want := range forces {
			if err := close(i, want, 1e-10, 1e-10); err != nil {
				return err
			}
			balance[i%3] += value(i)
			x, h := in.positions[i], 1e-6
			in.positions[i] = x + h
			_, plus, e := facesimReference(in)
			if e != nil {
				return e
			}
			in.positions[i] = x - h
			_, minus, e := facesimReference(in)
			if e != nil {
				return e
			}
			in.positions[i] = x
			if math.Abs(-(plus-minus)/(2*h)-want) > 1e-5*(1+math.Abs(want)) {
				return fmt.Errorf("Facesim energy-gradient mismatch at%d", i)
			}
		}
		for _, b := range balance {
			if math.Abs(b) > 1e-9 {
				return fmt.Errorf("Facesim unbalanced force %g", b)
			}
		}
	}
	return nil
}

func facesimFixture(n int) (facesimInput, error) {
	tets := [][4]int{{0, 1, 3, 9}, {1, 2, 4, 10}, {1, 2, 5, 11}, {3, 4, 6, 12}, {3, 4, 7, 13}, {4, 5, 8, 14}, {3, 6, 7, 15}, {4, 6, 7, 16}, {5, 7, 8, 17}, {9, 10, 12, 18}, {9, 10, 13, 19}, {0, 1, 4, 10}, {0, 3, 4, 12}, {0, 4, 10, 12}, {0, 9, 10, 12}, {1, 3, 4, 13}, {1, 3, 9, 13}, {1, 4, 5, 13}, {1, 5, 11, 13}, {1, 9, 10, 13}, {1, 10, 11, 13}, {2, 4, 5, 14}, {2, 4, 10, 14}, {2, 10, 11, 14}, {3, 7, 13, 15}, {3, 9, 12, 13}, {3, 12, 13, 15}, {4, 5, 7, 13}, {4, 6, 12, 16}, {4, 7, 8, 16}, {4, 8, 14, 16}, {4, 10, 12, 13}}
	in := facesimInput{positions: make([]float64, 60), tets: tets[:n], inverse: make([][3][3]float64, n), volumes: make([]float64, n), mu: 1000, lambda: 5000}
	rest := func(i, r int) float64 { return []float64{float64(i%3) / 2, float64((i/3)%3) / 2, float64(i/9) / 2}[r] }
	for i := 0; i < 20; i++ {
		for r, mult := range []int{7, 11, 13} {
			in.positions[i*3+r] = math.FMA(float64(i*mult%5), .01, rest(i, r))
		}
	}
	for k, ids := range in.tets {
		var dm [3][3]float64
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				dm[r][c] = rest(ids[c+1], r) - rest(ids[0], r)
			}
		}
		cof, det := cofactor3(dm)
		if det == 0 {
			return in, fmt.Errorf("degenerate rest tetrahedron")
		}
		in.volumes[k] = math.Abs(det) / 6
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				in.inverse[k][r][c] = cof[c][r] / det
			}
		}
	}
	return in, nil
}
