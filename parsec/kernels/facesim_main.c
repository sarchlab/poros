#include <stdio.h>
#include <stdlib.h>
#include <math.h>
#include <time.h>

extern void compute_tet_forces(
    double *forces,
    double *positions,
    int *tetIndices,
    double *DmInv,
    double *restVolume,
    double mu,
    double lambda,
    int nTets,
    int nVerts);

#define N_VERTS 8
#define N_TETS  6
#define NUM_ITERS 1000000

int main() {
    double positions[] = {
        0.0, 0.0, 0.0,   // v0
        1.0, 0.0, 0.0,   // v1
        0.0, 1.0, 0.0,   // v2
        0.0, 0.0, 1.0,   // v3
        1.1, 1.0, 0.0,   // v4
        1.0, 0.1, 1.0,   // v5
        0.0, 1.0, 1.1,   // v6
        1.0, 1.1, 1.0,   // v7
    };

    int tetIndices[] = {
        0, 1, 2, 3,
        1, 4, 2, 7,
        1, 5, 3, 7,
        2, 6, 3, 7,
        1, 2, 3, 7,
        0, 1, 3, 2,
    };

    // Reference positions for DmInv computation
    double refPos[] = {
        0.0, 0.0, 0.0,
        1.0, 0.0, 0.0,
        0.0, 1.0, 0.0,
        0.0, 0.0, 1.0,
        1.0, 1.0, 0.0,
        1.0, 0.0, 1.0,
        0.0, 1.0, 1.0,
        1.0, 1.0, 1.0,
    };

    double DmInv[N_TETS * 9];
    double restVolume[N_TETS];
    double forces[N_VERTS * 3];

    for (int t = 0; t < N_TETS; t++) {
        int i0 = tetIndices[t*4+0], i1 = tetIndices[t*4+1];
        int i2 = tetIndices[t*4+2], i3 = tetIndices[t*4+3];

        double dm00 = refPos[i1*3]-refPos[i0*3], dm01 = refPos[i2*3]-refPos[i0*3], dm02 = refPos[i3*3]-refPos[i0*3];
        double dm10 = refPos[i1*3+1]-refPos[i0*3+1], dm11 = refPos[i2*3+1]-refPos[i0*3+1], dm12 = refPos[i3*3+1]-refPos[i0*3+1];
        double dm20 = refPos[i1*3+2]-refPos[i0*3+2], dm21 = refPos[i2*3+2]-refPos[i0*3+2], dm22 = refPos[i3*3+2]-refPos[i0*3+2];

        double det = dm00*(dm11*dm22-dm12*dm21) - dm01*(dm10*dm22-dm12*dm20) + dm02*(dm10*dm21-dm11*dm20);
        restVolume[t] = fabs(det) / 6.0;

        double inv = 1.0 / det;
        DmInv[t*9+0] = (dm11*dm22-dm12*dm21)*inv;
        DmInv[t*9+1] = (dm02*dm21-dm01*dm22)*inv;
        DmInv[t*9+2] = (dm01*dm12-dm02*dm11)*inv;
        DmInv[t*9+3] = (dm12*dm20-dm10*dm22)*inv;
        DmInv[t*9+4] = (dm00*dm22-dm02*dm20)*inv;
        DmInv[t*9+5] = (dm02*dm10-dm00*dm12)*inv;
        DmInv[t*9+6] = (dm10*dm21-dm11*dm20)*inv;
        DmInv[t*9+7] = (dm01*dm20-dm00*dm21)*inv;
        DmInv[t*9+8] = (dm00*dm11-dm01*dm10)*inv;
    }

    double mu = 1000.0, lambda = 5000.0;

    // Warmup
    for (int i = 0; i < 1000; i++) {
        compute_tet_forces(forces, positions, tetIndices, DmInv, restVolume,
                          mu, lambda, N_TETS, N_VERTS);
    }

    // Timed run
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    for (int i = 0; i < NUM_ITERS; i++) {
        compute_tet_forces(forces, positions, tetIndices, DmInv, restVolume,
                          mu, lambda, N_TETS, N_VERTS);
    }
    clock_gettime(CLOCK_MONOTONIC, &end);

    double elapsed = (end.tv_sec - start.tv_sec) + (end.tv_nsec - start.tv_nsec) / 1e9;
    printf("Facesim FEM: %d tets, %d verts x %d iterations\n", N_TETS, N_VERTS, NUM_ITERS);
    printf("Total time: %.6f s\n", elapsed);
    printf("Per iteration: %.3f ns\n", elapsed / NUM_ITERS * 1e9);

    // Print first force to prevent dead code elimination
    printf("forces[0] = %.15f\n", forces[0]);

    return 0;
}
