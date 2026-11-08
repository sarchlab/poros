#include "driver.h"
extern void emit(u64);
int run(u64 argc, const char **argv) {
    if (argc != 4) return 2;
    u64 kind = parse(argv[1]), bytes = parse(argv[2]), passes = parse(argv[3]);
    if (!valid(kind, bytes, passes)) return 2;
    initialize(kind, bytes);
    u64 warm = probes[kind - 1](WARMUP_PASSES, buffer, bytes, heads);
    if (!verify(kind, bytes, WARMUP_PASSES, warm)) return 3;
    u64 result = probes[kind - 1](passes, buffer, bytes, heads);
    if (!verify(kind, bytes, passes, result)) return 4;
    emit(result);
    return 0;
}
