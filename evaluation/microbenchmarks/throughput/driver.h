#ifndef THROUGHPUT_DRIVER_H
#define THROUGHPUT_DRIVER_H
#define PROBE(name, instructions, kind) \
    extern unsigned long name(unsigned long, unsigned long *) __asm__("_" #name);
#include "cases.def"
#undef PROBE
static unsigned long (*const probes[])(unsigned long, unsigned long *) = {
#define PROBE(name, instructions, kind) name,
#include "cases.def"
#undef PROBE
};
#define NUM_PROBES (sizeof(probes) / sizeof(probes[0]))
static unsigned long parse(const char *s) {
    unsigned long n = 0;
    if (!*s) return 0;
    while (*s) {
        if (*s < '0' || *s > '9' || n > 100000000) return 0;
        n = n * 10 + *s++ - '0';
    }
    return n;
}
static void initialize(unsigned long *buffer) {
    for (unsigned long i = 0; i < 64; i++) buffer[i] = i + 1;
}
#endif
