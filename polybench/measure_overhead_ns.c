/*
 * measure_overhead_ns.c — Nanosecond-precision overhead measurement.
 *
 * Uses mach_absolute_time (macOS) for sub-microsecond precision.
 * Measures:
 *   1. Pure gettimeofday pair (the PolyBench timer mechanism)
 *   2. gettimeofday pair + empty function call
 *   3. Full PolyBench-style: cache flush + gettimeofday pair + empty call
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/time.h>
#include <mach/mach_time.h>

static mach_timebase_info_data_t timebase;

static double ns_elapsed(uint64_t start, uint64_t end) {
    return (double)(end - start) * timebase.numer / timebase.denom;
}

static double rtclock(void) {
    struct timeval Tp;
    gettimeofday(&Tp, NULL);
    return Tp.tv_sec + Tp.tv_usec * 1.0e-6;
}

__attribute__((noinline))
static void kernel_empty(int n, double *A) {
    (void)n; (void)A;
}

/* Simulates polybench_flush_cache — 33MB default cache size */
static void flush_cache(void) {
    int cs = 33554432 / sizeof(double); /* POLYBENCH_CACHE_SIZE_KB=32768 default */
    double *flush = (double *)calloc(cs, sizeof(double));
    double tmp = 0.0;
    for (int i = 0; i < cs; i++)
        tmp += flush[i];
    if (tmp > 10.0) printf("!"); /* prevent DCE */
    free(flush);
}

static int cmp_double(const void *a, const void *b) {
    double da = *(const double *)a, db = *(const double *)b;
    return (da > db) - (da < db);
}

static void report(const char *label, double *ns_arr, int n) {
    qsort(ns_arr, n, sizeof(double), cmp_double);
    printf("%-45s min=%6.0f ns  median=%6.0f ns  p90=%6.0f ns  max=%6.0f ns\n",
           label, ns_arr[0], ns_arr[n/2], ns_arr[n*9/10], ns_arr[n-1]);
}

int main(void) {
    mach_timebase_info(&timebase);
    const int TRIALS = 1001;
    double times[1001];
    double dummy[1];

    /* Test 1: Pure gettimeofday pair */
    for (int i = 0; i < TRIALS; i++) {
        uint64_t t0 = mach_absolute_time();
        double s = rtclock();
        double e = rtclock();
        uint64_t t1 = mach_absolute_time();
        (void)s; (void)e;
        times[i] = ns_elapsed(t0, t1);
    }
    report("gettimeofday pair:", times, TRIALS);

    /* Test 2: gettimeofday pair + empty call */
    for (int i = 0; i < TRIALS; i++) {
        uint64_t t0 = mach_absolute_time();
        double s = rtclock();
        kernel_empty(1, dummy);
        double e = rtclock();
        uint64_t t1 = mach_absolute_time();
        (void)s; (void)e;
        times[i] = ns_elapsed(t0, t1);
    }
    report("gettimeofday pair + empty call:", times, TRIALS);

    /* Test 3: cache flush + gettimeofday pair + empty call */
    for (int i = 0; i < TRIALS; i++) {
        uint64_t t0 = mach_absolute_time();
        flush_cache();
        double s = rtclock();
        kernel_empty(1, dummy);
        double e = rtclock();
        uint64_t t1 = mach_absolute_time();
        (void)s; (void)e;
        times[i] = ns_elapsed(t0, t1);
    }
    report("cache flush + gettimeofday pair + empty call:", times, TRIALS);

    /* Test 4: What gettimeofday actually reports for the empty call */
    for (int i = 0; i < TRIALS; i++) {
        double s = rtclock();
        kernel_empty(1, dummy);
        double e = rtclock();
        times[i] = (e - s) * 1e9;
    }
    report("gettimeofday-reported empty call (ns):", times, TRIALS);

    /* Test 5: What gettimeofday reports after cache flush + empty call */
    for (int i = 0; i < TRIALS; i++) {
        flush_cache();
        double s = rtclock();
        kernel_empty(1, dummy);
        double e = rtclock();
        times[i] = (e - s) * 1e9;
    }
    report("cache flush then gettimeofday-reported (ns):", times, TRIALS);

    /* Test 6: Measure just how long cache flush takes */
    for (int i = 0; i < TRIALS; i++) {
        uint64_t t0 = mach_absolute_time();
        flush_cache();
        uint64_t t1 = mach_absolute_time();
        times[i] = ns_elapsed(t0, t1);
    }
    report("cache flush alone:", times, TRIALS);

    return 0;
}
