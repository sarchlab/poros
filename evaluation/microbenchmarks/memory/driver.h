#ifndef MEMORY_DRIVER_H
#define MEMORY_DRIVER_H
/* One shared input recipe and ABI, with platform-specific wrappers. */
typedef unsigned long u64;
typedef unsigned int u32;
#ifndef MAX_BYTES
#define MAX_BYTES (128UL * 1024 * 1024)
#endif
#define NODE_BYTES 128UL
#define WARMUP_PASSES 2UL
#define CHASE_TAIL 17UL
#define WRITE_VALUE 0x5a5a5a5a5a5a5a5aUL
static u64 buffer[MAX_BYTES / 8] __attribute__((aligned(16384)));
static u32 permutation[MAX_BYTES / NODE_BYTES];
static u64 *heads[16];
static const u64 chain_counts[5] = {1, 2, 4, 8, 16};
typedef u64 (*probe_fn)(u64 passes, u64 *data, u64 bytes, u64 **starts);
extern u64 chase1(u64, u64 *, u64, u64 **);
extern u64 chase2(u64, u64 *, u64, u64 **);
extern u64 chase4(u64, u64 *, u64, u64 **);
extern u64 chase8(u64, u64 *, u64, u64 **);
extern u64 chase16(u64, u64 *, u64, u64 **);
extern u64 stream_read(u64, u64 *, u64, u64 **);
extern u64 stream_write(u64, u64 *, u64, u64 **);
extern u64 stream_copy(u64, u64 *, u64, u64 **);
static probe_fn probes[] = {chase1, chase2, chase4, chase8, chase16,
                           stream_read, stream_write, stream_copy};
static u64 parse(const char *s) {
    u64 n = 0;
    if (!*s) return 0;
    for (; *s; ++s) {
        if (*s < '0' || *s > '9' || n > 100000000UL) return 0;
        n = n * 10 + (u64)(*s - '0');
    }
    return n;
}
static int valid(u64 kind, u64 bytes, u64 passes) {
    return kind >= 1 && kind <= 8 && bytes >= 8192 && bytes <= MAX_BYTES &&
           bytes % (NODE_BYTES * 16) == 0 && passes >= 1 && passes <= 33554432;
}
static u64 random_next(u64 *state) {
    u64 x = *state;
    x ^= x << 13; x ^= x >> 7; x ^= x << 17;
    return *state = x;
}
static void initialize(u64 kind, u64 bytes) {
    if (kind <= 5) {
        u64 nodes = bytes / NODE_BYTES, chains = chain_counts[kind - 1];
        u64 length = nodes / chains, seed = 0x6a09e667f3bcc909UL;
        for (u64 i = 0; i < nodes; ++i) permutation[i] = (u32)i;
        for (u64 i = nodes - 1; i; --i) {
            u64 j = random_next(&seed) % (i + 1);
            u32 tmp = permutation[i]; permutation[i] = permutation[j]; permutation[j] = tmp;
        }
        for (u64 c = 0; c < chains; ++c) {
            u64 start = c * length;
            heads[c] = &buffer[(u64)permutation[start] * (NODE_BYTES / 8)];
            for (u64 i = 0; i < length; ++i) {
                u64 next = i + 1 == length ? start : start + i + 1;
                buffer[(u64)permutation[start + i] * (NODE_BYTES / 8)] =
                    (u64)&buffer[(u64)permutation[next] * (NODE_BYTES / 8)];
            }
        }
    } else {
        u64 count = bytes / 8;
        for (u64 i = 0; i < count; ++i)
            buffer[i] = kind == 8 && i >= count / 2 ? 0 : i + 1;
    }
}
/* Full write/copy output validation is deliberately outside the native timer. */
static int verify(u64 kind, u64 bytes, u64 passes, u64 result) {
    if (kind <= 5) {
        u64 nodes = bytes / NODE_BYTES, chains = chain_counts[kind - 1], length = nodes / chains;
        u64 wanted = 0;
        for (u64 c = 0; c < chains; ++c)
            wanted += (u64)permutation[c * length + CHASE_TAIL % length] * NODE_BYTES;
        return result == wanted;
    }
    u64 count = bytes / 8;
    if (kind == 6) return result == passes * (count * (count + 1) / 2);
    if (kind == 7) {
        for (u64 i = 0; i < count; ++i) if (buffer[i] != WRITE_VALUE) return 0;
        return result == WRITE_VALUE;
    }
    for (u64 i = 0; i < count / 2; ++i)
        if (buffer[i] != i + 1 || buffer[i + count / 2] != i + 1) return 0;
    return result == count / 2;
}
#endif
