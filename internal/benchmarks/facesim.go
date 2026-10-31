package benchmarks

import (
	"fmt"
	"math"
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
