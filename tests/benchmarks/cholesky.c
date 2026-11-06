#include "../../polybench/kernels/kernel_cholesky.c"
#include "runtime.h"

static double A0[262144];
static double A[262144];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 512, &n))
    return 2;
  for (int i = 0; i < n; i++) {
    for (int j = 0; j <= i; j++)
      A0[i * n + j] = (double)((-j) % n) / n + 1;
    A0[i * n + i] = 1;
  }
  for (int t = 0; t < n; t++)
    for (int r = 0; r < n; r++)
      for (int s = 0; s < n; s++)
        A[r * n + s] = fma(A0[r * n + t], A0[s * n + t], A[r * n + s]);
  kernel_cholesky(n, A);
  if (output(A, (n * n) * sizeof(double)))
    return 3;
  return 0;
}
