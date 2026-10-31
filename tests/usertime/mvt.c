#include "runtime.h"
#define MAX_N 4096
static double a[MAX_N * MAX_N];
static double x1[MAX_N], x2[MAX_N], y1[MAX_N], y2[MAX_N];
int guest_main(u64 *sp) {
    u64 n;
    if (sp[0] != 2 || !number((char *)sp[2], MAX_N, &n)) return 2;
    for (u64 i = 0; i < n; i++) {
        x1[i] = (double)i / n;
        x2[i] = (double)((i + 1) % n) / n;
        y1[i] = (double)((i + 3) % n) / n;
        y2[i] = (double)((i + 4) % n) / n;
        for (u64 j = 0; j < n; j++) a[i*n+j] = (double)((i*j) % n) / n;
    }
    for (u64 i = 0; i < n; i++)
        for (u64 j = 0; j < n; j++) x1[i] += a[i*n+j] * y1[j];
    for (u64 i = 0; i < n; i++)
        for (u64 j = 0; j < n; j++) x2[i] += a[j*n+i] * y2[j];
    if (output(x1, n * sizeof(double))) return 3;
    return output(x2, n * sizeof(double));
}
