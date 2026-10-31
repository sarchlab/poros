#include "../../polybench/kernels/kernel_gesummv.c"
#include "runtime.h"

static double A[5308416];
static double B[5308416];
static double x[2304];
static double y[2304];
static double tmp[2304];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 2304, &n))
    return 2;
  for (int i = 0; i < n; i++) {
    x[i] = (double)(i % n) / n;
    for (int j = 0; j < n; j++) {
      A[i * n + j] = (double)((i * j + 1) % n) / n;
      B[i * n + j] = (double)((i * j + 2) % n) / n;
    }
  }
  kernel_gesummv(n, 1.5, 1.2, A, B, tmp, x, y);
  if (output(y, (n) * sizeof(double)))
    return 3;
  if (output(tmp, (n) * sizeof(double)))
    return 3;
  return 0;
}
