#define NATIVE_REFERENCE
#define main guest_reference_main
#include "sweep.c"
#undef main

#include <inttypes.h>
#include <stdint.h>
#include <time.h>
#ifdef __APPLE__
#include <pthread.h>
#endif

static uint64_t now(void) {
#ifdef __APPLE__
    return clock_gettime_nsec_np(CLOCK_UPTIME_RAW);
#else
    struct timespec ts;
    if (clock_gettime(CLOCK_MONOTONIC_RAW, &ts))
        __builtin_trap();
    return (uint64_t)ts.tv_sec * 1000000000u + (uint64_t)ts.tv_nsec;
#endif
}

int main(int argc, char **argv) {
    struct shape s;
    int pairs, repeats;
    if (argc != 8 || !read_shape(argv + 1, &s)
        || !argument(argv[6], 1, 101, &pairs)
        || !argument(argv[7], 1, 256, &repeats))
        return 1;
#ifdef __APPLE__
    if (pthread_set_qos_class_self_np(QOS_CLASS_USER_INITIATED, 0) != 0)
        return 3;
#endif
    initialize(s);
    for (int i = 0; i < pairs; ++i) {
        uint64_t elapsed[2];
        for (int j = 0; j < 2; ++j) {
            int c = (i + j) % 2;
            traverse(s, c, 1);
            uint64_t start = now();
            traverse(s, c, repeats);
            elapsed[c] = now() - start;
            if (!verified(s) || elapsed[c] == 0)
                return 2;
        }
        printf("%d,%d,%" PRIu64 ",%" PRIu64 "\n", i + 1, repeats, elapsed[0], elapsed[1]);
    }
    return 0;
}
