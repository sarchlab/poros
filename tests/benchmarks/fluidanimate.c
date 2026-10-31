#include "../../parsec/kernels/fluidanimate.c"
#include "runtime.h"

static double px[16];
static double py[16];
static double pz[16];
static double vx[16];
static double vy[16];
static double vz[16];
static double density[16];
static double pressure[16];
static double fx[16];
static double fy[16];
static double fz[16];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int n;
  if (!argument(argv[1], 1, 16, &n))
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
