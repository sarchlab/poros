#include "../../polybench/kernels/kernel_doitgen.c"
#include "runtime.h"

static double A[960000];
static double C4[14400];
static double sum[120];

int workload(int argc, char **argv) {
  if (argc != 4)
    return 2;
  int nr;
  if (!argument(argv[1], 1, 100, &nr))
    return 2;
  int nq;
  if (!argument(argv[2], 1, 80, &nq))
    return 2;
  int np;
  if (!argument(argv[3], 1, 120, &np))
    return 2;
  for (int i = 0; i < nr; i++)
    for (int j = 0; j < nq; j++)
      for (int k = 0; k < np; k++)
        A[(i * nq + j) * np + k] = (double)((i * j + k) % np) / np;
  for (int i = 0; i < np; i++)
    for (int j = 0; j < np; j++)
      C4[i * np + j] = (double)(i * j % np) / np;
  kernel_doitgen(nr, nq, np, A, C4, sum);
  if (output(A, (nr * nq * np) * sizeof(double)))
    return 3;
  if (output(sum, (np) * sizeof(double)))
    return 3;
  return 0;
}
