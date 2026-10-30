// Common freestanding process interface. All initialization and output are guest work.
typedef unsigned long u64;
static int number(const char *s, u64 limit, u64 *value) {
    u64 n = 0;
    if (!*s) return 0;
    do {
        if (*s < '0' || *s > '9' || n > (limit - (*s - '0')) / 10) return 0;
        n = n * 10 + (*s++ - '0');
    } while (*s);
    if (!n || n > limit) return 0;
    *value = n;
    return 1;
}
static int output(const void *data, u64 bytes) {
    register u64 x0 asm("x0") = 1, x1 asm("x1") = (u64)data;
    register u64 x2 asm("x2") = bytes, x8 asm("x8") = 64;
    asm volatile("svc #0" : "+r"(x0) : "r"(x1), "r"(x2), "r"(x8) : "memory");
    return x0 == bytes ? 0 : 3;
}
