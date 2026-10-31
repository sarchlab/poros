#include "../../polybench/kernels/kernel_gemver.c"
#include "runtime.h"

static double A[14400];
static double u1[120];
static double u2[120];
static double v1[120];
static double v2[120];
static double w[120];
static double x[120];
static double y[120];
static double z[120];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 120, &n))
    return 2;
  for (int i = 0; i < n; i++) {
    u1[i] = i;
    u2[i] = ((double)(i + 1) / n) / 2;
    v1[i] = ((double)(i + 1) / n) / 4;
    v2[i] = ((double)(i + 1) / n) / 6;
    y[i] = ((double)(i + 1) / n) / 8;
    z[i] = ((double)(i + 1) / n) / 9;
    for (int j = 0; j < n; j++)
      A[i * n + j] = (double)(i * j % n) / n;
  }
  kernel_gemver(n, 1.5, 1.2, A, u1, v1, u2, v2, w, x, y, z);
  if (output(w, (n) * sizeof(double)))
    return 3;
  if (output(x, (n) * sizeof(double)))
    return 3;
  if (output(A, (n * n) * sizeof(double)))
    return 3;
  return 0;
}
