#include "../../polybench/kernels/kernel_ludcmp.c"
#include "runtime.h"

static double A0[14400];
static double A[14400];
static double b[120];
static double x[120];
static double y[120];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 120, &n))
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
  for (int i = 0; i < n; i++)
    b[i] = fma((double)(i + 1) / n, 0.5, 4);
  kernel_ludcmp(n, A, b, x, y);
  if (output(A, (n * n) * sizeof(double)))
    return 3;
  if (output(x, (n) * sizeof(double)))
    return 3;
  if (output(y, (n) * sizeof(double)))
    return 3;
  return 0;
}
