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
    if (argc != 3) return 2;
    unsigned long index = parse(argv[1]), iterations = parse(argv[2]);
    if (!index || index > NUM_PROBES || !iterations || iterations > 100000000) return 2;
    if (pthread_set_qos_class_self_np(QOS_CLASS_USER_INTERACTIVE, 0)) return 3;
    unsigned long buffer[64] __attribute__((aligned(64)));
    initialize(buffer);
    (void)probes[index - 1](iterations, buffer);
    for (int trial = 0; trial < 9; trial++) {
        double elapsed[2];
        unsigned long result[2];
        for (int order = 0; order < 2; order++) {
            int length = (trial + order) % 2;
            initialize(buffer);
            double start = seconds();
            result[length] = probes[index - 1](iterations * (length + 1), buffer);
            elapsed[length] = seconds() - start;
            if (start < 0 || elapsed[length] <= 0) return 4;
        }
        printf("%d %.12g %lu %lu\n", trial, elapsed[1] - elapsed[0], result[0], result[1]);
    }
    return 0;
}
