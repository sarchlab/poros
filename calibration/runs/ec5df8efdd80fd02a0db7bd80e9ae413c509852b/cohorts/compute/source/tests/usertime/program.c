#include "runtime.h"
#if CASE == 2
static volatile u64 data[32 * 1024 * 1024 / sizeof(u64)];
#endif
int guest_main(u64 *sp) {
    u64 n, value = 1;
#if CASE == 2
    u64 bytes;
    if (sp[0] != 3 || !number((char *)sp[2], 10000000, &n) ||
        !number((char *)sp[3], sizeof(data), &bytes) || bytes < 8 || (bytes & (bytes - 1))) return 2;
    u64 count = bytes / sizeof(u64);
    for (u64 i = 0; i < count; i++) data[i] = i;
    for (u64 i = 0; i < n; i++) { u64 j = i & (count - 1); data[j] += i; value += data[j]; }
#else
    if (sp[0] != 2 || !number((char *)sp[2], 10000000, &n)) return 2;
    for (u64 i = 0; i < n; i++) {
        value = (value * 1664525 + 1013904223) & 0xffffffffUL;
#if CASE == 3
        if (value & 0x10000) value ^= 0x1234; else value += 7;
#endif
    }
#endif
    return output(&value, sizeof(value));
}
