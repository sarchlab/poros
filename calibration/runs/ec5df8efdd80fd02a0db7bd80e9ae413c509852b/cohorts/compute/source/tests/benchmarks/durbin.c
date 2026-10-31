#include "../../polybench/kernels/kernel_durbin.c"
#include "runtime.h"

static double r[400];
static double y[400];
static double z[400];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 400, &n))
    return 2;
  for (int i = 0; i < n; i++)
    r[i] = n + 1 - i;
  kernel_durbin(n, r, y, z);
  if (output(y, (n) * sizeof(double)))
    return 3;
  return 0;
}
