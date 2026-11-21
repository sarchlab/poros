#include "runtime.h"

enum { MAX_N = 512, MAX_LD = 528, MAX_CELLS = MAX_N * MAX_LD };

static unsigned input[MAX_CELLS] __attribute__((aligned(64)));
static unsigned result[MAX_CELLS] __attribute__((aligned(64)));

/* Swap the address strides without changing the compiled loop body. */
#ifdef __clang__
#define KEEP_WALK __attribute__((noinline))
#else
#define KEEP_WALK __attribute__((noipa))
#endif
static KEEP_WALK void walk(int n, int outer_step, int inner_step) {
    for (int outer = 0; outer < n; ++outer) {
        int i = outer * outer_step;
        for (int inner = 0; inner < n; ++inner) {
            result[i] = input[i] + 1u;
            i += inner_step;
        }
    }
}

int workload(int argc, char **argv) {
    int n;
    int ld;
    int repeats = 1;
    if (argc < 3 || argc > 5 || !argument(argv[2], 1, MAX_N, &n))
        return 1;

    ld = n;
    if (argc >= 4 && !argument(argv[3], n, MAX_LD, &ld))
        return 1;

    if (argc == 5 && !argument(argv[4], 1, 256, &repeats))
        return 1;

    char order = argv[1][0];
    if ((order != 'r' && order != 'c') || argv[1][1] != '\0')
        return 1;

    for (int row = 0; row < n; ++row)
        for (int col = 0; col < n; ++col)
            input[row * ld + col] = (unsigned)(row * n + col);

    const int outer_step = order == 'r' ? ld : 1;
    const int inner_step = order == 'r' ? 1 : ld;
    for (int repeat = 0; repeat < repeats; ++repeat) {
        walk(n, outer_step, inner_step);
        __asm__ volatile("" ::: "memory");
    }

    for (int row = 0; row < n; ++row)
        for (int col = 0; col < n; ++col)
            if (result[row * ld + col] != (unsigned)(row * n + col) + 1u)
                return 2;

    const char pass[] = "PASS\n";
    return output(pass, sizeof(pass) - 1) ? 3 : 0;
}
