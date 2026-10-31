#include "../../polybench/kernels/kernel_adi.c"
#include "runtime.h"

static double u[262144];
static double v[262144];
static double p[262144];
static double q[262144];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int tsteps;
  if (!argument(argv[1], 1, 40, &tsteps))
    return 2;
  int n;
  if (!argument(argv[2], 3, 512, &n))
    return 2;
  for (int i = 0; i < n; i++)
    for (int j = 0; j < n; j++)
      u[i * n + j] = (double)(i + n - j) / n;
  kernel_adi(tsteps, n, u, v, p, q);
  if (output(u, (n * n) * sizeof(double)))
    return 3;
  if (output(v, (n * n) * sizeof(double)))
    return 3;
  return 0;
}
