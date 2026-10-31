#include "../../polybench/kernels/kernel_heat_3d.c"
#include "runtime.h"

static double A[8000];
static double B[8000];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int tsteps;
  if (!argument(argv[1], 1, 40, &tsteps))
    return 2;
  int n;
  if (!argument(argv[2], 3, 20, &n))
    return 2;
  for (int i = 0; i < n; i++)
    for (int j = 0; j < n; j++)
      for (int k = 0; k < n; k++)
        A[(i * n + j) * n + k] = B[(i * n + j) * n + k] =
            (double)(i + j + n - k) * 10 / n;
  kernel_heat_3d(tsteps, n, A, B);
  if (output(A, (n * n * n) * sizeof(double)))
    return 3;
  if (output(B, (n * n * n) * sizeof(double)))
    return 3;
  return 0;
}
