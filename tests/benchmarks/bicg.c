#include "../../polybench/kernels/kernel_bicg.c"
#include "runtime.h"

static double A[14384];
static double s[124];
static double q[116];
static double p[124];
static double r[116];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int m;
  if (!argument(argv[1], 1, 116, &m))
    return 2;
  int n;
  if (!argument(argv[2], 1, 124, &n))
    return 2;
  for (int i = 0; i < m; i++) {
    r[i] = (double)(i % m) / m;
    for (int j = 0; j < n; j++)
      A[i * n + j] = (double)(i * (j + 1) % n) / n;
  }
  for (int i = 0; i < n; i++)
    p[i] = (double)(i % n) / n;
  kernel_bicg(m, n, A, s, q, p, r);
  if (output(s, (n) * sizeof(double)))
    return 3;
  if (output(q, (m) * sizeof(double)))
    return 3;
  return 0;
}
