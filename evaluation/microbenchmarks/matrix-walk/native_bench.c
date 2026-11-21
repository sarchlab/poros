#define NATIVE_REFERENCE
#define main guest_reference_main
#include "matrix_walk.c"
#undef main

#include <inttypes.h>
#include <pthread.h>
#include <stdint.h>
#include <time.h>

#ifdef __APPLE__
static uint64_t raw_clock_ns(void) {
    return clock_gettime_nsec_np(CLOCK_UPTIME_RAW);
}
#else
static uint64_t raw_clock_ns(void) {
    struct timespec ts;
    if (clock_gettime(CLOCK_MONOTONIC_RAW, &ts) != 0)
        __builtin_trap();
    return (uint64_t)ts.tv_sec * 1000000000 + (uint64_t)ts.tv_nsec;
}
#endif

static uint64_t time_batch(int n, int outer_step, int inner_step, int repeats) {
    /* Warm both streams and fault in their pages before this sample. */
    walk(n, outer_step, inner_step);
    uint64_t start = raw_clock_ns();
    for (int repeat = 0; repeat < repeats; ++repeat) {
        walk(n, outer_step, inner_step);
        __asm__ volatile("" ::: "memory");
    }
    return raw_clock_ns() - start;
}

static int verify(int n, int ld) {
    for (int row = 0; row < n; ++row)
        for (int col = 0; col < n; ++col)
            if (result[row * ld + col] != (unsigned)(row * n + col) + 1u)
                return 0;
    return 1;
}

int main(int argc, char **argv) {
    int n, ld, repeats;
    if (argc != 4 || !argument(argv[1], 1, MAX_N, &n) ||
        !argument(argv[2], n, MAX_LD, &ld) ||
        !argument(argv[3], 1, 10000, &repeats))
        return 1;

#ifdef __APPLE__
    /* Request interactive benchmark scheduling; macOS retains core placement. */
    if (pthread_set_qos_class_self_np(QOS_CLASS_USER_INITIATED, 0) != 0)
        return 3;
#endif

    for (int row = 0; row < n; ++row)
        for (int col = 0; col < n; ++col)
            input[row * ld + col] = (unsigned)(row * n + col);

    for (int trial = 0; trial < 41; ++trial) {
        for (int position = 0; position < 2; ++position) {
            int column_first = (trial + position) % 2;
            uint64_t elapsed = time_batch(
                n, column_first ? 1 : ld, column_first ? ld : 1, repeats);
            if (!verify(n, ld))
                return 2;
            printf("%d,%d,%d,%c,%d,%" PRIu64 "\n",
                   n, ld, trial + 1, column_first ? 'c' : 'r', repeats, elapsed);
        }
    }
    return 0;
}
