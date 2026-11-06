#include "../../polybench/kernels/kernel_trisolv.c"
#include "runtime.h"

static double L[7929856];
static double b[2816];
static double x[2816];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 2816, &n))
    return 2;
  for (int i = 0; i < n; i++) {
    b[i] = i;
    for (int j = 0; j <= i; j++)
      L[i * n + j] = (double)(i + n - j + 1) * 2 / n;
  }
  kernel_trisolv(n, L, x, b);
  if (output(x, (n) * sizeof(double)))
    return 3;
  return 0;
}
