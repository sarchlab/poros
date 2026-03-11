#include <stdio.h>
#include <stdlib.h>
#include <time.h>

extern void BlkSchlsEqEuroNoDiv(
    double *prices, double *S, double *X, double *r,
    double *v, double *T, int *otype, int numOptions);

#define NUM_OPTIONS 16
#define NUM_ITERS 1000000

int main() {
    double S[] = {42.0, 100.0, 50.0, 80.0, 120.0, 30.0, 95.0, 70.0,
                  55.0, 110.0, 45.0, 85.0, 60.0, 75.0, 90.0, 65.0};
    double X[] = {40.0, 105.0, 48.0, 82.0, 115.0, 32.0, 100.0, 68.0,
                  52.0, 108.0, 50.0, 80.0, 58.0, 72.0, 88.0, 70.0};
    double r[] = {0.05, 0.04, 0.06, 0.03, 0.05, 0.04, 0.05, 0.06,
                  0.03, 0.05, 0.04, 0.06, 0.05, 0.03, 0.04, 0.05};
    double v[] = {0.20, 0.25, 0.30, 0.15, 0.22, 0.35, 0.18, 0.28,
                  0.25, 0.20, 0.30, 0.22, 0.25, 0.18, 0.20, 0.32};
    double T[] = {0.50, 1.00, 0.25, 0.75, 1.50, 0.30, 0.80, 0.60,
                  1.00, 0.50, 0.75, 1.25, 0.40, 0.90, 1.00, 0.50};
    int otype[] = {0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1};
    double prices[NUM_OPTIONS];

    // Warmup
    for (int i = 0; i < 1000; i++) {
        BlkSchlsEqEuroNoDiv(prices, S, X, r, v, T, otype, NUM_OPTIONS);
    }

    // Timed run
    struct timespec start, end;
    clock_gettime(CLOCK_MONOTONIC, &start);
    for (int i = 0; i < NUM_ITERS; i++) {
        BlkSchlsEqEuroNoDiv(prices, S, X, r, v, T, otype, NUM_OPTIONS);
    }
    clock_gettime(CLOCK_MONOTONIC, &end);

    double elapsed = (end.tv_sec - start.tv_sec) + (end.tv_nsec - start.tv_nsec) / 1e9;
    printf("BlackScholes: %d options x %d iterations\n", NUM_OPTIONS, NUM_ITERS);
    printf("Total time: %.6f s\n", elapsed);
    printf("Per iteration: %.3f ns\n", elapsed / NUM_ITERS * 1e9);
    printf("Per option: %.3f ns\n", elapsed / NUM_ITERS / NUM_OPTIONS * 1e9);

    // Print first price to prevent dead code elimination
    printf("prices[0] = %.15f\n", prices[0]);

    return 0;
}
