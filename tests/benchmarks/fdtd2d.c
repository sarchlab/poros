#include "../../polybench/kernels/kernel_fdtd_2d.c"
#include "runtime.h"

static double ex[4800];
static double ey[4800];
static double hz[4800];
static double fict[40];

int workload(int argc, char **argv) {
  if (argc != 4)
    return 2;
  int tmax;
  if (!argument(argv[1], 1, 40, &tmax))
    return 2;
  int nx;
  if (!argument(argv[2], 1, 60, &nx))
    return 2;
  int ny;
  if (!argument(argv[3], 1, 80, &ny))
    return 2;
  for (int i = 0; i < tmax; i++)
    fict[i] = i;
  for (int i = 0; i < nx; i++)
    for (int j = 0; j < ny; j++) {
      ex[i * ny + j] = (double)(i * (j + 1)) / nx;
      ey[i * ny + j] = (double)(i * (j + 2)) / ny;
      hz[i * ny + j] = (double)(i * (j + 3)) / nx;
    }
  kernel_fdtd_2d(tmax, nx, ny, ex, ey, hz, fict);
  if (output(ex, (nx * ny) * sizeof(double)))
    return 3;
  if (output(ey, (nx * ny) * sizeof(double)))
    return 3;
  if (output(hz, (nx * ny) * sizeof(double)))
    return 3;
  return 0;
}
