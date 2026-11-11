#ifndef RECURRENCE_DRIVER_H
#define RECURRENCE_DRIVER_H
#define PROBE(name, instructions, updates, kind) \
    extern unsigned long name(unsigned long, unsigned long *) __asm__("_" #name);
#include "cases.def"
#undef PROBE
static unsigned long (*const probes[])(unsigned long, unsigned long *) = {
#define PROBE(name, instructions, updates, kind) name,
#include "cases.def"
#undef PROBE
};
#define NUM_PROBES (sizeof(probes) / sizeof(probes[0]))
/* Every 32-bit recurrence stays below 2^31 at 64 updates per outer trip. */
#define MAX_ITERATIONS 8388608UL
/* Dense and tight loops use the first line of cells. Kernel replicas read
   row/column candidates at byte offsets 4096 and 8192 from cells. */
static struct {
    unsigned long cells[512];
    unsigned int row[1024];
    unsigned int column[1024];
} data __attribute__((aligned(16384)));
static unsigned long parse(const char *s) {
    unsigned long n = 0;
    if (!*s) return 0;
    while (*s) {
        if (*s < '0' || *s > '9' || n > MAX_ITERATIONS) return 0;
        n = n * 10 + *s++ - '0';
    }
    return n > MAX_ITERATIONS ? 0 : n;
}
static void initialize(void) {
    for (unsigned long i = 0; i < 512; i++) data.cells[i] = i + 1;
    for (unsigned int i = 0; i < 1024; i++) {
        data.row[i] = i + 1;
        data.column[i] = 0;
    }
}
#endif
