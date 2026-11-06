#include "../../polybench/kernels/kernel_gramschmidt.c"
#include "runtime.h"

static double A[262144];
static double R[262144];
static double Q[262144];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int m;
  if (!argument(argv[1], 2, 512, &m))
    return 2;
  int n;
  if (!argument(argv[2], 1, 512, &n))
    return 2;
  if (n > m)
    return 2;
  /* Diagonally dominant top square block guarantees independent columns. */
  for (int i = 0; i < m; i++)
    for (int j = 0; j < n; j++)
      A[i * n + j] =
          (i == j ? 2.0 : 0.0) + (double)((i * 3 + j * 7) % 11) / (20 * m);
  kernel_gramschmidt(m, n, A, R, Q);
  if (output(R, (n * n) * sizeof(double)))
    return 3;
  if (output(Q, (m * n) * sizeof(double)))
    return 3;
  if (output(A, (m * n) * sizeof(double)))
    return 3;
  return 0;
}
