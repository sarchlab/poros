#include "../../polybench/kernels/kernel_3mm.c"
#include "runtime.h"

static double A[153600];
static double B[192000];
static double C[256000];
static double D[358400];
static double E[128000];
static double F[224000];
static double G[179200];

int workload(int argc, char **argv) {
  if (argc != 6)
    return 2;
  int ni;
  if (!argument(argv[1], 1, 320, &ni))
    return 2;
  int nj;
  if (!argument(argv[2], 1, 400, &nj))
    return 2;
  int nk;
  if (!argument(argv[3], 1, 480, &nk))
    return 2;
  int nl;
  if (!argument(argv[4], 1, 560, &nl))
    return 2;
  int nm;
  if (!argument(argv[5], 1, 640, &nm))
    return 2;
  for (int i = 0; i < ni; i++)
    for (int j = 0; j < nk; j++)
      A[i * nk + j] = (double)((i * j + 1) % ni) / (5 * ni);
  for (int i = 0; i < nk; i++)
    for (int j = 0; j < nj; j++)
      B[i * nj + j] = (double)((i * (j + 1) + 2) % nj) / (5 * nj);
  for (int i = 0; i < nj; i++)
    for (int j = 0; j < nm; j++)
      C[i * nm + j] = (double)(i * (j + 3) % nl) / (5 * nl);
  for (int i = 0; i < nm; i++)
    for (int j = 0; j < nl; j++)
      D[i * nl + j] = (double)((i * (j + 2) + 2) % nk) / (5 * nk);
  kernel_3mm(ni, nj, nk, nl, nm, E, A, B, F, C, D, G);
  if (output(G, (ni * nl) * sizeof(double)))
    return 3;
  if (output(E, (ni * nj) * sizeof(double)))
    return 3;
  if (output(F, (nj * nl) * sizeof(double)))
    return 3;
  return 0;
}
