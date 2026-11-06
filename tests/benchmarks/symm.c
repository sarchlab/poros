#include "../../polybench/kernels/kernel_symm.c"
#include "runtime.h"

static double C[307200];
static double A[230400];
static double B[307200];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int m;
  if (!argument(argv[1], 1, 480, &m))
    return 2;
  int n;
  if (!argument(argv[2], 1, 640, &n))
    return 2;
  for (int i = 0; i < m; i++) {
    for (int j = 0; j < n; j++) {
      C[i * n + j] = (double)((i + j) % 100) / m;
      B[i * n + j] = (double)((n + i - j) % 100) / m;
    }
    for (int j = 0; j < m; j++)
      A[i * m + j] = j <= i ? (double)((i + j) % 100) / m : -999;
  }
  kernel_symm(m, n, 1.5, 1.2, C, A, B);
  if (output(C, (m * n) * sizeof(double)))
    return 3;
  return 0;
}
