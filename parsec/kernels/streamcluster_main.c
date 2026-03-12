#include <stdio.h>
#include <stdlib.h>
#include <time.h>

extern void compute_distances(
    double *p1, double *p2, double *out,
    int npoints, int dim);

#define NPOINTS 512
#define DIM 32
#define NUM_ITERS 1000

int main() {
    double *p1 = (double *)malloc(NPOINTS * DIM * sizeof(double));
    double *p2 = (double *)malloc(NPOINTS * DIM * sizeof(double));
    double *out = (double *)malloc(NPOINTS * sizeof(double));

    for (int i = 0; i < NPOINTS * DIM; i++) {
        p1[i] = 1.0 + i * 0.01;
        p2[i] = 2.0 - i * 0.02;
    }

    // Warmup
    for (int i = 0; i < 100; i++) {
        compute_distances(p1, p2, out, NPOINTS, DIM);
    }

    // Timed run
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    for (int i = 0; i < NUM_ITERS; i++) {
        compute_distances(p1, p2, out, NPOINTS, DIM);
    }
    clock_gettime(CLOCK_MONOTONIC, &end);

    double elapsed = (end.tv_sec - start.tv_sec) + (end.tv_nsec - start.tv_nsec) / 1e9;
    printf("Streamcluster: %d points x %d dims x %d iterations\n", NPOINTS, DIM, NUM_ITERS);
    printf("Total time: %.6f s\n", elapsed);
    printf("Per iteration: %.3f ns\n", elapsed / NUM_ITERS * 1e9);
    printf("out[0] = %.15f\n", out[0]);

    free(p1);
    free(p2);
    free(out);
    return 0;
}
