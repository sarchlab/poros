#include "../../polybench/kernels/kernel_syr2k.c"
#include "runtime.h"

static double C[409600];
static double A[307200];
static double B[307200];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int n;
  if (!argument(argv[1], 1, 640, &n))
    return 2;
  int m;
  if (!argument(argv[2], 1, 480, &m))
    return 2;
  for (int i = 0; i < n; i++)
    for (int j = 0; j < m; j++) {
      A[i * m + j] = (double)((i * j + 1) % n) / n;
      B[i * m + j] = (double)((i * j + 2) % m) / m;
    }
  for (int i = 0; i < n; i++)
    for (int j = 0; j < n; j++)
      C[i * n + j] = (double)((i * j + 3) % n) / m;
  kernel_syr2k(n, m, 1.5, 1.2, C, A, B);
  if (output(C, (n * n) * sizeof(double)))
    return 3;
  return 0;
}
