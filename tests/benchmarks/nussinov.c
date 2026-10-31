#include "../../polybench/kernels/kernel_nussinov.c"
#include "runtime.h"

static unsigned char seq[180];
static int table[32400];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 180, &n))
    return 2;
  for (int i = 0; i < n; i++)
    seq[i] = (i + 1) % 4;
  kernel_nussinov(n, seq, table);
  if (output(table, (n * n) * sizeof(int)))
    return 3;
  return 0;
}
