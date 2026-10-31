#include "../../parsec/kernels/blackscholes.c"
#include "runtime.h"

static double prices[1024];
static double S[1024];
static double X[1024];
static double r[1024];
static double v[1024];
static double T[1024];
static int otype[1024];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 1024, &n))
    return 2;
  for (int i = 0; i < n; i++) {
    S[i] = fma(i % 80, 1.25, 30);
    X[i] = fma(i % 80, 1.25, 28);
    r[i] = fma(i % 10, 0.005, 0.03);
    v[i] = fma(i % 20, 0.02, 0.15);
    T[i] = fma(i % 8, 0.25, 0.25);
    otype[i] = i % 2;
  }
  BlkSchlsEqEuroNoDiv(prices, S, X, r, v, T, otype, n);
  if (output(prices, (n) * sizeof(double)))
    return 3;
  return 0;
}
