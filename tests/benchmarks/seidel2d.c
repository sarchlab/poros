#include "../../polybench/kernels/kernel_seidel_2d.c"
#include "runtime.h"

static double A[14400];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int tsteps;
  if (!argument(argv[1], 1, 40, &tsteps))
    return 2;
  int n;
  if (!argument(argv[2], 3, 120, &n))
    return 2;
  for (int i = 0; i < n; i++)
    for (int j = 0; j < n; j++) {
      A[i * n + j] = (double)(i * (j + 2) + 2) / n;
    }
  kernel_seidel_2d(tsteps, n, A);
  if (output(A, (n * n) * sizeof(double)))
    return 3;
  return 0;
}
