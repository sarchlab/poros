package cpu

import (
	"fmt"
	"math"
	"testing"

	"github.com/sarchlab/poros/pkg/benchmark"
)

type facesimInput struct {
	positions  []float64
	tets       [][4]int
	inverse    [][3][3]float64
	volumes    []float64
	mu, lambda float64
}

func cofactor3(a [3][3]float64) (c [3][3]float64, det float64) {
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			c[i][j] = a[(i+1)%3][(j+1)%3]*a[(i+2)%3][(j+2)%3] - a[(i+1)%3][(j+2)%3]*a[(i+2)%3][(j+1)%3]
		}
	}
	for j := 0; j < 3; j++ {
		det += a[0][j] * c[0][j]
	}
	return
}

// Direct matrix equations for compressible Neo-Hookean energy and its forces.
// This reference does not decode or execute guest instructions.
func facesimReference(in facesimInput) ([]float64, float64, error) {
	forces := make([]float64, len(in.positions))
	energy := 0.0
	for k, ids := range in.tets {
		var f [3][3]float64
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				for j := 0; j < 3; j++ {
					f[r][c] += (in.positions[ids[j+1]*3+r] - in.positions[ids[0]*3+r]) * in.inverse[k][j][c]
				}
			}
		}
		cof, jac := cofactor3(f)
		if math.IsNaN(jac) || math.IsInf(jac, 0) || jac <= 0 {
			return nil, 0, fmt.Errorf("tet%d invalid deformation determinant %g", k, jac)
		}
		logJ := math.Log(jac)
		norm := 0.0
		var stress [3][3]float64
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				norm += f[r][c] * f[r][c]
				stress[r][c] = in.mu*f[r][c] + (in.lambda*logJ-in.mu)*cof[r][c]/jac
			}
		}
		energy += in.volumes[k] * (in.mu*(norm-3)/2 - in.mu*logJ + in.lambda*logJ*logJ/2)
		for r := 0; r < 3; r++ {
			for j := 0; j < 3; j++ {
				v := 0.0
				for c := 0; c < 3; c++ {
					v -= in.volumes[k] * stress[r][c] * in.inverse[k][j][c]
				}
				forces[ids[j+1]*3+r] += v
				forces[ids[0]*3+r] -= v
			}
		}
	}
	for _, v := range forces {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, 0, fmt.Errorf("nonfinite force")
		}
	}
	if math.IsNaN(energy) || math.IsInf(energy, 0) {
		return nil, 0, fmt.Errorf("nonfinite energy")
	}
	return forces, energy, nil
}

func TestRegistryFacesimNumericalReference(t *testing.T) {
	b := benchmark.ByName("facesim")
	em := b.Setup()
	if em.Regs.ReadX(5) != 32 || em.Regs.ReadX(6) != 20 {
		t.Fatal("unexpected Facesim dimensions")
	}
	in := facesimInput{positions: make([]float64, 60), tets: make([][4]int, 32), inverse: make([][3][3]float64, 32), volumes: make([]float64, 32), mu: em.Regs.ReadD(0), lambda: em.Regs.ReadD(1)}
	for i := range in.positions {
		in.positions[i] = em.Mem.ReadF64(em.Regs.ReadX(1) + uint64(i*8))
	}
	used := map[int]bool{}
	for k := range in.tets {
		for j := 0; j < 4; j++ {
			id := int(em.Mem.ReadU32(em.Regs.ReadX(2) + uint64((k*4+j)*4)))
			if id < 0 || id >= 20 {
				t.Fatal("invalid vertex")
			}
			in.tets[k][j] = id
			used[id] = true
		}
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				in.inverse[k][r][c] = em.Mem.ReadF64(em.Regs.ReadX(3) + uint64((k*9+r*3+c)*8))
			}
		}
		in.volumes[k] = em.Mem.ReadF64(em.Regs.ReadX(4) + uint64(k*8))
		// Independently reconstruct rest geometry and validate the supplied inverse.
		rest := func(id, r int) float64 {
			return []float64{float64(id%3) / 2, float64((id/3)%3) / 2, float64(id/9) / 2}[r]
		}
		var dm [3][3]float64
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				dm[r][c] = rest(in.tets[k][c+1], r) - rest(in.tets[k][0], r)
			}
		}
		_, det := cofactor3(dm)
		if det == 0 || math.Abs(in.volumes[k]-math.Abs(det)/6) > 1e-14 {
			t.Fatalf("tet%d invalid rest geometry", k)
		}
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				v := 0.0
				for j := 0; j < 3; j++ {
					v += dm[r][j] * in.inverse[k][j][c]
				}
				want := 0.0
				if r == c {
					want = 1
				}
				if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v-want) > 1e-12 {
					t.Fatalf("tet%d invalid inverse", k)
				}
			}
		}
	}
	if len(used) != 20 {
		t.Fatal("unused vertices")
	}
	want, _, err := facesimReference(in)
	if err != nil {
		t.Fatal(err)
	}
	// The energy gradient is an independent check on stress/cofactor/sign indexing.
	for i := range in.positions {
		x := in.positions[i]
		h := 1e-6
		in.positions[i] = x + h
		_, plus, e := facesimReference(in)
		if e != nil {
			t.Fatal(e)
		}
		in.positions[i] = x - h
		_, minus, e := facesimReference(in)
		if e != nil {
			t.Fatal(e)
		}
		in.positions[i] = x
		gradient := -(plus - minus) / (2 * h)
		if math.Abs(gradient-want[i]) > 1e-5*(1+math.Abs(want[i])) {
			t.Fatalf("energy gradient%d got%g force%g", i, gradient, want[i])
		}
	}
	output := em.Regs.ReadX(0)
	if err := em.Run(); err != nil {
		t.Fatal(err)
	}
	maxError := 0.0
	var balance [3]float64
	for i, v := range want {
		got := em.Mem.ReadF64(output + uint64(i*8))
		diff := math.Abs(got - v)
		maxError = math.Max(maxError, diff)
		if math.IsNaN(got) || math.IsInf(got, 0) || diff > 1e-10*(1+math.Abs(v)) {
			t.Fatalf("force%d got%.17g want%.17g", i, got, v)
		}
		balance[i%3] += got
	}
	for _, v := range balance {
		if math.Abs(v) > 1e-9 {
			t.Fatalf("unbalanced force %g", v)
		}
	}
	t.Logf("32 positive finite deformation determinants; 60 finite forces; max_abs_error=%.17g; force_balance=%v", maxError, balance)
	// Domain faults must not become successful NaN comparisons.
	in.positions[0] = math.NaN()
	if _, _, err := facesimReference(in); err == nil {
		t.Fatal("accepted NaN input")
	}
}
