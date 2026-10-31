#include "../../polybench/kernels/kernel_trmm.c"
#include "runtime.h"

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
    for (int j = 0; j < i; j++)
      A[i * m + j] = (double)((i + j) % m) / m;
    A[i * m + i] = 1;
    for (int j = 0; j < n; j++)
      B[i * n + j] = (double)((n + i - j) % n) / n;
  }
  kernel_trmm(m, n, 1.5, A, B);
  if (output(B, (m * n) * sizeof(double)))
    return 3;
  return 0;
}
