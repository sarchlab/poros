/*
 * measure_overhead.c — Estimate timing overhead in PolyBench measurements.
 *
 * Measures the cost of gettimeofday + an empty function call,
 * which represents the minimum overhead in any PolyBench kernel timing.
 * Reports median of 1001 trials.
 */
#include <stdio.h>
#include <stdlib.h>
#include <sys/time.h>

static double rtclock(void) {
    struct timeval Tp;
    gettimeofday(&Tp, NULL);
    return Tp.tv_sec + Tp.tv_usec * 1.0e-6;
}

/* Empty kernel — mimics the function-call overhead of a real kernel. */
__attribute__((noinline))
static void kernel_empty(int n, double *A) {
    (void)n; (void)A;
    /* no work */
}

static int cmp_double(const void *a, const void *b) {
    double da = *(const double *)a;
    double db = *(const double *)b;
    return (da > db) - (da < db);
}

int main(void) {
    const int TRIALS = 1001;
    double times[TRIALS];
    double dummy[1]; /* argument to prevent optimising away the call */

    for (int i = 0; i < TRIALS; i++) {
        double t_start = rtclock();
        kernel_empty(1, dummy);
        double t_end = rtclock();
        times[i] = t_end - t_start;
    }

    qsort(times, TRIALS, sizeof(double), cmp_double);

    double median = times[TRIALS / 2];
    double p10 = times[TRIALS / 10];
    double p90 = times[TRIALS * 9 / 10];
    double min_val = times[0];
    double max_val = times[TRIALS - 1];

    printf("Timer + empty-call overhead (n=%d trials):\n", TRIALS);
    printf("  min    = %0.6f s  (%0.1f µs)\n", min_val, min_val * 1e6);
    printf("  p10    = %0.6f s  (%0.1f µs)\n", p10, p10 * 1e6);
    printf("  median = %0.6f s  (%0.1f µs)\n", median, median * 1e6);
    printf("  p90    = %0.6f s  (%0.1f µs)\n", p90, p90 * 1e6);
    printf("  max    = %0.6f s  (%0.1f µs)\n", max_val, max_val * 1e6);

    return 0;
}
