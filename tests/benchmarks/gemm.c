#include "../../polybench/kernels/kernel_gemm.c"
#include "runtime.h"

static double C[268800];
static double A[307200];
static double B[358400];

int workload(int argc, char **argv) {
  if (argc != 4)
    return 2;
  int ni;
  if (!argument(argv[1], 1, 480, &ni))
    return 2;
  int nj;
  if (!argument(argv[2], 1, 560, &nj))
    return 2;
  int nk;
  if (!argument(argv[3], 1, 640, &nk))
    return 2;
  for (int i = 0; i < ni; i++)
    for (int j = 0; j < nj; j++)
      C[i * nj + j] = (double)((i * j + 1) % ni) / ni;
  for (int i = 0; i < ni; i++)
    for (int j = 0; j < nk; j++)
      A[i * nk + j] = (double)(i * (j + 1) % nk) / nk;
  for (int i = 0; i < nk; i++)
    for (int j = 0; j < nj; j++)
      B[i * nj + j] = (double)(i * (j + 2) % nj) / nj;
  kernel_gemm(ni, nj, nk, 1.5, 1.2, C, A, B);
  if (output(C, (ni * nj) * sizeof(double)))
    return 3;
  return 0;
}
