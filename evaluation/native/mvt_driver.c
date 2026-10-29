// Initialization/reset and correctness checks are outside each measured ROI.
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <mach/mach_time.h>

extern void kernel_mvt(int, double *, double *, double *, double *, double *);

static mach_timebase_info_data_t timebase;
static uint64_t now(void) { return mach_absolute_time(); }
static double ns(uint64_t ticks) {
    return (double)ticks * timebase.numer / timebase.denom;
}

static void reset(int n, double *x1, double *x2) {
    for (int i = 0; i < n; i++) {
        x1[i] = (double)(i % n) / n;
        x2[i] = (double)((i + 1) % n) / n;
    }
}

int main(int argc, char **argv) {
    if (argc != 3) return 2;
    if (mach_timebase_info(&timebase) != KERN_SUCCESS || !timebase.denom) return 2;
    char *end;
    long dimension = strtol(argv[1], &end, 10);
    if (*end || dimension < 1 || dimension > 1536) return 2;
    int n = (int)dimension;
    long reps = strtol(argv[2], &end, 10);
    if (*end || reps < 5 || reps > 101) return 2;
    double *x1 = calloc(n, sizeof(double));
    double *x2 = calloc(n, sizeof(double));
    double *y1 = calloc(n, sizeof(double));
    double *y2 = calloc(n, sizeof(double));
    double *a = calloc((size_t)n * n, sizeof(double));
    if (!x1 || !x2 || !y1 || !y2 || !a) return 3;
    for (int i = 0; i < n; i++) {
        y1[i] = (double)((i + 3) % n) / n;
        y2[i] = (double)((i + 4) % n) / n;
        for (int j = 0; j < n; j++) a[(size_t)i * n + j] = (double)((i * j) % n) / n;
    }
    reset(n, x1, x2);
    kernel_mvt(n, x1, x2, y1, y2, a);
    uint64_t overhead = UINT64_MAX;
    uint64_t min_positive = UINT64_MAX;
    for (int i = 0; i < 1000; i++) {
        uint64_t t = now();
        uint64_t dt = now() - t;
        if (dt < overhead) overhead = dt;
        if (dt && dt < min_positive) min_positive = dt;
    }
    if (min_positive == UINT64_MAX) return 4;
    int batch = n < 130 ? 100 : 5;
    uint64_t *raw = calloc((size_t)reps * batch, sizeof(uint64_t));
    if (!raw) return 3;
    for (int r = 0; r < reps; r++) {
        for (int b = 0; b < batch; b++) {
            reset(n, x1, x2);
            uint64_t t = now();
            kernel_mvt(n, x1, x2, y1, y2, a);
            raw[r * batch + b] = now() - t;
        }
    }
    // Emit only after all timings; retain individual durations and batch means.
    printf("{\"n\":%d,\"batch\":%d,\"timer\":\"mach_absolute_time\",\"timebase_numer\":%u,\"timebase_denom\":%u,\"observed_min_positive_ticks\":%llu,\"timer_overhead_ns\":%.17g,\"samples_ns\":[", n, batch, timebase.numer, timebase.denom, (unsigned long long)min_positive, ns(overhead));
    for (int r = 0; r < reps; r++) {
        uint64_t total = 0;
        for (int b = 0; b < batch; b++) total += raw[r * batch + b];
        printf("%s%.17g", r ? "," : "", ns(total) / batch);
    }
    printf("],\"raw_roi_ticks\":[");
    for (int i = 0; i < reps * batch; i++) printf("%s%llu", i ? "," : "", (unsigned long long)raw[i]);
    printf("],\"x1\":[");
    for (int i = 0; i < n; i++) printf("%s%.17g", i ? "," : "", x1[i]);
    printf("],\"x2\":[");
    for (int i = 0; i < n; i++) printf("%s%.17g", i ? "," : "", x2[i]);
    printf("]}\n");
    free(raw); free(x1); free(x2); free(y1); free(y2); free(a);
    return 0;
}
