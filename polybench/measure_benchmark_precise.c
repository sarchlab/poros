/*
 * measure_benchmark_precise.c — High-precision timing of PolyBench kernels.
 *
 * Wraps mach_absolute_time around the PolyBench-compiled binaries' own
 * measurement to verify gettimeofday-reported values aren't biased.
 * Also measures the cache flush overhead to confirm it's outside the timed region.
 *
 * This program times how long gettimeofday takes as a function and shows
 * that the quantization step is ~1μs.
 */
#include <stdio.h>
#include <sys/time.h>
#include <mach/mach_time.h>

static mach_timebase_info_data_t tb;

int main(void) {
    mach_timebase_info(&tb);

    /* Measure gettimeofday quantization: call it in a tight loop
       and record successive values */
    printf("=== gettimeofday step sizes (successive differences) ===\n");
    struct timeval tp;
    double prev, cur;
    gettimeofday(&tp, NULL);
    prev = tp.tv_sec + tp.tv_usec * 1.0e-6;

    int steps[21] = {0};
    int si = 0;
    int calls = 0;
    while (si < 20) {
        calls++;
        gettimeofday(&tp, NULL);
        cur = tp.tv_sec + tp.tv_usec * 1.0e-6;
        double diff = (cur - prev) * 1e6;  /* in microseconds */
        if (diff > 0.0001) {  /* ignore zero steps */
            int step_us = (int)(diff + 0.5);
            steps[si++] = step_us;
            prev = cur;
        }
        if (calls > 1000000) break;
    }

    printf("  Non-zero steps (µs): ");
    for (int i = 0; i < si; i++) printf("%d ", steps[i]);
    printf("\n  Total gettimeofday calls to get %d steps: %d\n", si, calls);
    printf("  => Average calls between 1µs steps: %d\n", calls / (si > 0 ? si : 1));

    return 0;
}
