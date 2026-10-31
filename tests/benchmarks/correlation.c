#include "../../polybench/kernels/kernel_correlation.c"
#include "runtime.h"

static double data[8000];
static double result[6400];
static double mean[80];
static double stddev[80];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int m;
  if (!argument(argv[1], 1, 80, &m))
    return 2;
  int n;
  if (!argument(argv[2], 2, 100, &n))
    return 2;
  for (int i = 0; i < n; i++)
    for (int j = 0; j < m; j++)
      data[i * m + j] = (double)(i * j) / m + i;
  kernel_correlation(m, n, n, data, result, mean, stddev);
  if (output(result, (m * m) * sizeof(double)))
    return 3;
  if (output(mean, (m) * sizeof(double)))
    return 3;
  if (output(stddev, (m) * sizeof(double)))
    return 3;
  return 0;
}
