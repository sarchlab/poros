#include "../../parsec/kernels/streamcluster.c"
#include "runtime.h"

static double p1[16384];
static double p2[16384];
static double result[512];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int n;
  if (!argument(argv[1], 1, 512, &n))
    return 2;
  int dim;
  if (!argument(argv[2], 1, 32, &dim))
    return 2;
  for (int i = 0; i < n * dim; i++) {
    p1[i] = fma(i, 0.01, 1);
    p2[i] = fma(-i, 0.02, 2);
  }
  compute_distances(p1, p2, result, n, dim);
  if (output(result, (n) * sizeof(double)))
    return 3;
  return 0;
}
