#include <math.h>

// FEM tetrahedron force computation (Neo-Hookean model)
// For each tet: compute deformation gradient, stress, and forces
//
// ARM64 ABI:
//   X0 = forces output (double[nVerts * 3])
//   X1 = positions (double[nVerts * 3])
//   X2 = tetIndices (int[nTets * 4])
//   X3 = DmInv (double[nTets * 9]) - precomputed inverse reference shape matrix
//   X4 = restVolume (double[nTets])
//   D0 = mu (material parameter)
//   D1 = lambda (material parameter)
//   W5 = nTets
//   W6 = nVerts
void compute_tet_forces(
    double *forces,
    double *positions,
    int *tetIndices,
    double *DmInv,
    double *restVolume,
    double mu,
    double lambda,
    int nTets,
    int nVerts
) {
    // Zero forces (use volatile to prevent bzero optimization)
    for (volatile int i = 0; i < nVerts * 3; i++) {
        forces[i] = 0.0;
    }

    for (int t = 0; t < nTets; t++) {
        int i0 = tetIndices[t * 4 + 0];
        int i1 = tetIndices[t * 4 + 1];
        int i2 = tetIndices[t * 4 + 2];
        int i3 = tetIndices[t * 4 + 3];

        // Get positions
        double x0 = positions[i0*3], y0 = positions[i0*3+1], z0 = positions[i0*3+2];
        double x1 = positions[i1*3], y1 = positions[i1*3+1], z1 = positions[i1*3+2];
        double x2 = positions[i2*3], y2 = positions[i2*3+1], z2 = positions[i2*3+2];
        double x3 = positions[i3*3], y3 = positions[i3*3+1], z3 = positions[i3*3+2];

        // Ds = [x1-x0, x2-x0, x3-x0; y1-y0, y2-y0, y3-y0; z1-z0, z2-z0, z3-z0]
        double ds00 = x1-x0, ds01 = x2-x0, ds02 = x3-x0;
        double ds10 = y1-y0, ds11 = y2-y0, ds12 = y3-y0;
        double ds20 = z1-z0, ds21 = z2-z0, ds22 = z3-z0;

        // DmInv for this tet (row-major 3x3)
        double *dm = &DmInv[t * 9];

        // F = Ds * DmInv (deformation gradient)
        double f00 = ds00*dm[0] + ds01*dm[3] + ds02*dm[6];
        double f01 = ds00*dm[1] + ds01*dm[4] + ds02*dm[7];
        double f02 = ds00*dm[2] + ds01*dm[5] + ds02*dm[8];
        double f10 = ds10*dm[0] + ds11*dm[3] + ds12*dm[6];
        double f11 = ds10*dm[1] + ds11*dm[4] + ds12*dm[7];
        double f12 = ds10*dm[2] + ds11*dm[5] + ds12*dm[8];
        double f20 = ds20*dm[0] + ds21*dm[3] + ds22*dm[6];
        double f21 = ds20*dm[1] + ds21*dm[4] + ds22*dm[7];
        double f22 = ds20*dm[2] + ds21*dm[5] + ds22*dm[8];

        // det(F) = determinant of deformation gradient
        double J = f00*(f11*f22 - f12*f21)
                 - f01*(f10*f22 - f12*f20)
                 + f02*(f10*f21 - f11*f20);

        // Neo-Hookean: P = mu*(F - F^-T) + lambda*log(J)*F^-T
        // F^-T = (1/J) * cofactor(F)
        double logJ = log(J);  // Linked guest libm computes log.

        // Cofactor matrix (adjugate transpose)
        double c00 = f11*f22 - f12*f21;
        double c01 = f12*f20 - f10*f22;
        double c02 = f10*f21 - f11*f20;
        double c10 = f02*f21 - f01*f22;
        double c11 = f00*f22 - f02*f20;
        double c12 = f01*f20 - f00*f21;
        double c20 = f01*f12 - f02*f11;
        double c21 = f02*f10 - f00*f12;
        double c22 = f00*f11 - f01*f10;

        // P = mu*F + (lambda*logJ - mu)/J * cofactor
        double coeff = (lambda * logJ - mu) / J;
        double p00 = mu*f00 + coeff*c00;
        double p01 = mu*f01 + coeff*c01;
        double p02 = mu*f02 + coeff*c02;
        double p10 = mu*f10 + coeff*c10;
        double p11 = mu*f11 + coeff*c11;
        double p12 = mu*f12 + coeff*c12;
        double p20 = mu*f20 + coeff*c20;
        double p21 = mu*f21 + coeff*c21;
        double p22 = mu*f22 + coeff*c22;

        // Force = -V * P * DmInv^T
        double V = restVolume[t];
        double h00 = -V * (p00*dm[0] + p01*dm[1] + p02*dm[2]);
        double h01 = -V * (p00*dm[3] + p01*dm[4] + p02*dm[5]);
        double h02 = -V * (p00*dm[6] + p01*dm[7] + p02*dm[8]);
        double h10 = -V * (p10*dm[0] + p11*dm[1] + p12*dm[2]);
        double h11 = -V * (p10*dm[3] + p11*dm[4] + p12*dm[5]);
        double h12 = -V * (p10*dm[6] + p11*dm[7] + p12*dm[8]);
        double h20 = -V * (p20*dm[0] + p21*dm[1] + p22*dm[2]);
        double h21 = -V * (p20*dm[3] + p21*dm[4] + p22*dm[5]);
        double h22 = -V * (p20*dm[6] + p21*dm[7] + p22*dm[8]);

        // Accumulate forces on vertices
        // f1 += h[0], f2 += h[1], f3 += h[2]
        // f0 -= h[0] + h[1] + h[2]
        forces[i1*3+0] += h00; forces[i1*3+1] += h10; forces[i1*3+2] += h20;
        forces[i2*3+0] += h01; forces[i2*3+1] += h11; forces[i2*3+2] += h21;
        forces[i3*3+0] += h02; forces[i3*3+1] += h12; forces[i3*3+2] += h22;
        forces[i0*3+0] -= h00+h01+h02;
        forces[i0*3+1] -= h10+h11+h12;
        forces[i0*3+2] -= h20+h21+h22;
    }
}
