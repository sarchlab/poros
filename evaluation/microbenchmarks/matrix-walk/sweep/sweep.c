#include "runtime.h"

enum { MAX_DIM = 2048, MAX_WIDTH = 4100, MAX_OFFSET = 1024,
       MAX_CELLS = MAX_DIM * (MAX_DIM + 64) + MAX_OFFSET };
static unsigned input[MAX_CELLS] __attribute__((aligned(16384)));
static unsigned output_cells[MAX_CELLS] __attribute__((aligned(16384)));

struct shape { int rows, cols, a_width, b_width, b_offset; };

static int read_shape(char **arg, struct shape *s) {
    return argument(arg[0], 1, MAX_DIM, &s->rows)
        && argument(arg[1], 1, MAX_DIM, &s->cols)
        && argument(arg[2], s->cols, MAX_WIDTH, &s->a_width)
        && argument(arg[3], s->cols, MAX_WIDTH, &s->b_width)
        && argument(arg[4], 0, MAX_OFFSET, &s->b_offset)
        && (s->rows - 1) * s->a_width + s->cols <= MAX_CELLS
        && (s->rows - 1) * s->b_width + s->cols + s->b_offset <= MAX_CELLS;
}

#ifdef __clang__
#define KEEP_WALK __attribute__((noinline))
#else
#define KEEP_WALK __attribute__((noipa))
#endif
static KEEP_WALK void walk(unsigned *dst, int outer, int inner,
                          int a_outer, int a_inner, int b_outer, int b_inner) {
    for (int i = 0; i < outer; ++i) {
        int a = i * a_outer;
        int b = i * b_outer;
        for (int j = 0; j < inner; ++j) {
            dst[b] = input[a] + 1u;
            a += a_inner;
            b += b_inner;
        }
    }
}

static void traverse(struct shape s, int column, int repeats) {
    unsigned *dst = output_cells + s.b_offset;
    int outer = column ? s.cols : s.rows;
    int inner = column ? s.rows : s.cols;
    int a_outer = column ? 1 : s.a_width;
    int a_inner = column ? s.a_width : 1;
    int b_outer = column ? 1 : s.b_width;
    int b_inner = column ? s.b_width : 1;
    for (int k = 0; k < repeats; ++k) {
        walk(dst, outer, inner, a_outer, a_inner, b_outer, b_inner);
        __asm__ volatile("" ::: "memory");
    }
}

static void initialize(struct shape s) {
    for (int r = 0; r < s.rows; ++r)
        for (int c = 0; c < s.cols; ++c)
            input[r * s.a_width + c] = (unsigned)(r * s.cols + c);
}

static int verified(struct shape s) {
    const unsigned *dst = output_cells + s.b_offset;
    for (int r = 0; r < s.rows; ++r)
        for (int c = 0; c < s.cols; ++c)
            if (dst[r * s.b_width + c] != (unsigned)(r * s.cols + c) + 1u)
                return 0;
    return 1;
}

int workload(int argc, char **argv) {
    struct shape s;
    int repeats;
    if (argc != 8 || !read_shape(argv + 2, &s)
        || !argument(argv[7], 1, 256, &repeats)
        || (argv[1][0] != 'r' && argv[1][0] != 'c') || argv[1][1])
        return 1;
    initialize(s);
    traverse(s, argv[1][0] == 'c', repeats);
    if (!verified(s))
        return 2;
    return output("PASS\n", 5) ? 3 : 0;
}
