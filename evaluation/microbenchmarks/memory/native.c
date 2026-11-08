#include "driver.h"
#include <pthread.h>
#include <pthread/qos.h>
#include <stdio.h>
#include <time.h>
static double seconds(void) {
    struct timespec t;
    if (clock_gettime(CLOCK_THREAD_CPUTIME_ID, &t)) return -1;
    return t.tv_sec + t.tv_nsec * 1e-9;
}
int main(int argc, char **argv) {
    if (argc != 5) return 2;
    u64 kind = parse(argv[1]), bytes = parse(argv[2]), passes = parse(argv[3]), trials = parse(argv[4]);
    if (!valid(kind, bytes, 2 * passes) || !trials || trials > 21) return 2;
    if (pthread_set_qos_class_self_np(QOS_CLASS_USER_INTERACTIVE, 0)) return 3;
    for (u64 trial = 0; trial < trials; ++trial) {
        double elapsed[2]; u64 results[2];
        for (u64 order = 0; order < 2; ++order) {
            u64 length = (trial + order) % 2;
            initialize(kind, bytes);
            u64 warm = probes[kind - 1](WARMUP_PASSES, buffer, bytes, heads);
            if (!verify(kind, bytes, WARMUP_PASSES, warm)) return 4;
            double start = seconds();
            results[length] = probes[kind - 1](passes * (length + 1), buffer, bytes, heads);
            elapsed[length] = seconds() - start;
            if (start < 0 || elapsed[length] <= 0 ||
                !verify(kind, bytes, passes * (length + 1), results[length])) return 5;
        }
        printf("%lu %.12g %.12g %lu %lu\n", trial, elapsed[0], elapsed[1], results[0], results[1]);
    }
    return 0;
}
