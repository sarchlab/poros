#include "../../polybench/kernels/kernel_floyd_warshall.c"
#include "runtime.h"

static double path[409600];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 640, &n))
    return 2;
  for (int i = 0; i < n; i++)
    for (int j = 0; j < n; j++)
      path[i * n + j] =
          ((i + j) % 13 == 0 || (i + j) % 7 == 0 || (i + j) % 11 == 0)
              ? 999
              : i * j % 7 + 1;
  kernel_floyd_warshall(n, path);
  if (output(path, (n * n) * sizeof(double)))
    return 3;
  return 0;
}
