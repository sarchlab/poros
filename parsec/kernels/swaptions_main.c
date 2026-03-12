#include <stdio.h>
#include <stdlib.h>
#include <math.h>
#include <time.h>

extern void cholesky(double *A, int n);
extern void hjm_step(double *dF, double *L, double *Z, double r, double dt, int n);

#define N 50
#define NUM_ITERS 100000

int main() {
    double *A = (double *)malloc(N * N * sizeof(double));
    double *L = (double *)malloc(N * N * sizeof(double));
    double *Z = (double *)malloc(N * sizeof(double));
    double *dF = (double *)malloc(N * sizeof(double));

    // Initialize symmetric positive-definite matrix
    for (int i = 0; i < N; i++) {
        for (int j = 0; j < N; j++) {
            if (i == j) {
                A[i * N + j] = N + 1.0;
            } else {
                A[i * N + j] = 1.0 / (1.0 + abs(i - j));
            }
        }
    }

    // Initialize random normals (deterministic)
    for (int i = 0; i < N; i++) {
        Z[i] = 0.1 * (i + 1) - 0.05 * N;
    }

    // Warmup
    for (int i = 0; i < 100; i++) {
        // Copy A to L for Cholesky (destructive)
        for (int j = 0; j < N * N; j++) L[j] = A[j];
        cholesky(L, N);
        hjm_step(dF, L, Z, 0.05, 0.01, N);
    }

    // Timed run
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    for (int i = 0; i < NUM_ITERS; i++) {
        for (int j = 0; j < N * N; j++) L[j] = A[j];
        cholesky(L, N);
        hjm_step(dF, L, Z, 0.05, 0.01, N);
    }
    clock_gettime(CLOCK_MONOTONIC, &end);

    double elapsed = (end.tv_sec - start.tv_sec) + (end.tv_nsec - start.tv_nsec) / 1e9;
    printf("Swaptions: n=%d, %d iterations\n", N, NUM_ITERS);
    printf("Total time: %.6f s\n", elapsed);
    printf("Per iteration: %.3f ns\n", elapsed / NUM_ITERS * 1e9);
    printf("dF[0] = %.15f\n", dF[0]);

    free(A);
    free(L);
    free(Z);
    free(dF);
    return 0;
}
