#include "driver.h"
extern void emit(unsigned long);
int run(unsigned long argc, const char **argv) {
    if (argc != 3) return 2;
    unsigned long index = parse(argv[1]), iterations = parse(argv[2]);
    // One-based case indexes make invalid numeric arguments unambiguous.
    if (!index || index > NUM_PROBES || !iterations || iterations > 100000000) return 2;
    unsigned long buffer[64] __attribute__((aligned(64)));
    initialize(buffer);
    emit(probes[index - 1](iterations, buffer));
    return 0;
}
