#include "../../parsec/kernels/ferret.c"
#include "runtime.h"

static double queries[256];
static double database[2048];
static double result[64];

int workload(int argc, char **argv) {
  if (argc != 5)
    return 2;
  int nq;
  if (!argument(argv[1], 1, 16, &nq))
    return 2;
  int nd;
  if (!argument(argv[2], 1, 128, &nd))
    return 2;
  int dim;
  if (!argument(argv[3], 1, 16, &dim))
    return 2;
  int k;
  if (!argument(argv[4], 1, 4, &k))
    return 2;
  if (k > nd)
    return 2;
  for (int i = 0; i < nq * dim; i++)
    queries[i] = fma(i, 0.1, 0.5);
  for (int i = 0; i < nd * dim; i++)
    database[i] = fma(i % 17, 0.2, -1);
  knn_search(result, queries, database, nq, nd, dim, k);
  if (output(result, (nq * k) * sizeof(double)))
    return 3;
  return 0;
}
