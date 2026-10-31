#include "atax_kernel.h"
#include "runtime.h"

static double A[159900];
static double x[410];
static double y[410];
static double tmp[390];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int m;
  if (!argument(argv[1], 1, 390, &m))
    return 2;
  int n;
  if (!argument(argv[2], 1, 410, &n))
    return 2;
  for (int j = 0; j < n; j++)
    x[j] = 1.0 + (double)j / n;
  for (int i = 0; i < m; i++)
    for (int j = 0; j < n; j++)
      A[i * n + j] = (double)((i + j) % n) / (5 * m);
  kernel_atax(m, n, A, x, y, tmp);
  if (output(y, (n) * sizeof(double)))
    return 3;
  if (output(tmp, (m) * sizeof(double)))
    return 3;
  return 0;
}
