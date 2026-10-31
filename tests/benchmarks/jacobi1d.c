#include "../../polybench/kernels/kernel_jacobi_1d.c"
#include "runtime.h"

static double A[400];
static double B[400];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int tsteps;
  if (!argument(argv[1], 1, 100, &tsteps))
    return 2;
  int n;
  if (!argument(argv[2], 3, 400, &n))
    return 2;
  for (int i = 0; i < n; i++) {
    A[i] = (i + 2.0) / n;
    B[i] = (i + 3.0) / n;
  }
  kernel_jacobi_1d(tsteps, n, A, B);
  if (output(A, (n) * sizeof(double)))
    return 3;
  if (output(B, (n) * sizeof(double)))
    return 3;
  return 0;
}
