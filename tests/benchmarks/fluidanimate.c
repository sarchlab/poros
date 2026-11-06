#include "../../parsec/kernels/fluidanimate.c"
#include "runtime.h"

static double px[4096];
static double py[4096];
static double pz[4096];
static double vx[4096];
static double vy[4096];
static double vz[4096];
static double density[4096];
static double pressure[4096];
static double fx[4096];
static double fy[4096];
static double fz[4096];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 4096, &n))
    return 2;
  for (int i = 0; i < n; i++) {
    px[i] = (i % 4) * 0.3;
    py[i] = (i / 4) * 0.3;
    vx[i] = 0.1 * (1 - 2 * (i % 2));
    vy[i] = 0.1 * (1 - 2 * ((i / 2) % 2));
  }
  sph_compute(px, py, pz, vx, vy, vz, density, pressure, fx, fy, fz, n, 1, 1,
              1000, 3, 3.5);
  if (output(density, (n) * sizeof(double)))
    return 3;
  if (output(pressure, (n) * sizeof(double)))
    return 3;
  if (output(fx, (n) * sizeof(double)))
    return 3;
  if (output(fy, (n) * sizeof(double)))
    return 3;
  if (output(fz, (n) * sizeof(double)))
    return 3;
  return 0;
}
