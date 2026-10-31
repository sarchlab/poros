#include "../../parsec/kernels/swaptions.c"
#include "runtime.h"

static double B[2500];
static double A[2500];
static double Z[50];
static double dF[50];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 50, &n))
    return 2;
  for (int i = 0; i < n; i++)
    for (int j = 0; j < n; j++)
      B[i * n + j] = (double)((i * 3 + j * 7 + 1) % (n + 1)) / (n + 1);
  for (int i = 0; i < n; i++) {
    for (int j = 0; j < n; j++)
      for (int k = 0; k < n; k++)
        A[i * n + j] = fma(B[i * n + k], B[j * n + k], A[i * n + j]);
    A[i * n + i] += n;
    Z[i] = (i * 3 - n) * 0.2;
  }
  cholesky(A, n);
  hjm_step(dF, A, Z, 0.05, 0.25, n);
  if (output(dF, (n) * sizeof(double)))
    return 3;
  if (output(A, (n * n) * sizeof(double)))
    return 3;
  return 0;
}
