#include "../../parsec/kernels/bodytrack.c"
#include "runtime.h"

static double states[384];
static double observations[8];
static double projection[48];
static double result[64];

int workload(int argc, char **argv) {
  if (argc != 4)
    return 2;
  int n;
  if (!argument(argv[1], 1, 64, &n))
    return 2;
  int state_dim;
  if (!argument(argv[2], 1, 6, &state_dim))
    return 2;
  int obs_dim;
  if (!argument(argv[3], 1, 8, &obs_dim))
    return 2;
  for (int p = 0; p < n; p++)
    for (int j = 0; j < state_dim; j++)
      states[p * state_dim + j] = fma(0.05, j, 0.1 * (p + 1));
  for (int i = 0; i < obs_dim; i++) {
    observations[i] = fma(0.5, i, 1);
    for (int j = 0; j < state_dim; j++)
      projection[i * state_dim + j] = ((i + j) % 5 + 1) * 0.1;
  }
  bodytrack_likelihood(result, states, observations, projection, n, state_dim,
                       obs_dim);
  if (output(result, (n) * sizeof(double)))
    return 3;
  return 0;
}
