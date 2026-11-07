// Build on Apple silicon: clang -O2 -arch arm64 frontend.c frontend.S -o frontend
#include <pthread.h>
#include <pthread/qos.h>
#include <stdio.h>
#include <time.h>

extern void nop_probe(unsigned long iterations);
extern void add_probe(unsigned long iterations);

static double thread_seconds(void) {
    struct timespec value;
    clock_gettime(CLOCK_THREAD_CPUTIME_ID, &value);
    return value.tv_sec + 1e-9 * value.tv_nsec;
}

int main(void) {
    if (pthread_set_qos_class_self_np(QOS_CLASS_USER_INTERACTIVE, 0)) return 1;
    for (int trial = 0; trial < 9; trial++) {
        for (int kind = 0; kind < 2; kind++) {
            double start = thread_seconds();
            (kind ? add_probe : nop_probe)(2000000);
            printf("%s %d %.9f\n", kind ? "add" : "nop", trial, thread_seconds() - start);
        }
    }
    return 0;
}
