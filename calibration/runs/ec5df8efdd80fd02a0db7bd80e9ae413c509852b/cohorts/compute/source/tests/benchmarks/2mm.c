#include "../../polybench/kernels/kernel_2mm.c"
#include "runtime.h"

static double tmp[2000];
static double A[2800];
static double B[3500];
static double C[4000];
static double D[3200];

int workload(int argc, char **argv) {
  if (argc != 5)
    return 2;
  int ni;
  if (!argument(argv[1], 1, 40, &ni))
    return 2;
  int nj;
  if (!argument(argv[2], 1, 50, &nj))
    return 2;
  int nk;
  if (!argument(argv[3], 1, 70, &nk))
    return 2;
  int nl;
  if (!argument(argv[4], 1, 80, &nl))
    return 2;
  for (int i = 0; i < ni; i++)
    for (int j = 0; j < nk; j++)
      A[i * nk + j] = (double)((i * j + 1) % ni) / ni;
  for (int i = 0; i < nk; i++)
    for (int j = 0; j < nj; j++)
      B[i * nj + j] = (double)(i * (j + 1) % nj) / nj;
  for (int i = 0; i < nj; i++)
    for (int j = 0; j < nl; j++)
      C[i * nl + j] = (double)((i * (j + 3) + 1) % nl) / nl;
  for (int i = 0; i < ni; i++)
    for (int j = 0; j < nl; j++)
      D[i * nl + j] = (double)(i * (j + 2) % nk) / nk;
  kernel_2mm(ni, nj, nk, nl, 1.5, 1.2, tmp, A, B, C, D);
  if (output(D, (ni * nl) * sizeof(double)))
    return 3;
  if (output(tmp, (ni * nj) * sizeof(double)))
    return 3;
  return 0;
}
