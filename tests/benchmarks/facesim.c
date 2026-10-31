#include "../../parsec/kernels/facesim.c"
#include "runtime.h"

static double position[983040];
static double rest[983040];
static double inverse[4718592];
static double volume[524288];
static double forces[983040];
/* The 32 nondegenerate M0 tetrahedra, in their original order. */
static int tets[2097152];
static const int block_tets[128] = {
    0,  1,  3,  9,  1,  2,  4,  10, 1,  2,  5,  11, 3,  4,  6,  12, 3,  4,  7,
    13, 4,  5,  8,  14, 3,  6,  7,  15, 4,  6,  7,  16, 5,  7,  8,  17, 9,  10,
    12, 18, 9,  10, 13, 19, 0,  1,  4,  10, 0,  3,  4,  12, 0,  4,  10, 12, 0,
    9,  10, 12, 1,  3,  4,  13, 1,  3,  9,  13, 1,  4,  5,  13, 1,  5,  11, 13,
    1,  9,  10, 13, 1,  10, 11, 13, 2,  4,  5,  14, 2,  4,  10, 14, 2,  10, 11,
    14, 3,  7,  13, 15, 3,  9,  12, 13, 3,  12, 13, 15, 4,  5,  7,  13, 4,  6,
    12, 16, 4,  7,  8,  16, 4,  8,  14, 16, 4,  10, 12, 13};

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int ntets;
  if (!argument(argv[1], 1, 524288, &ntets))
    return 2;
  /* Independent translated mesh blocks add nodes and tetrahedra, not timed
   * repetitions. The first block is exactly the original correctness fixture. */
  int nodes = ((ntets + 31) / 32) * 20;
  for (int i = 0; i < nodes; i++) {
    int local = i % 20, block = i / 20;
    double x = (local % 3) * 0.5 + block * 2.0;
    double y = ((local / 3) % 3) * 0.5, z = (local / 9) * 0.5;
    double deformation = 0.01 * (1.0 + (block % 7) * 0.01);
    rest[i * 3] = x;
    rest[i * 3 + 1] = y;
    rest[i * 3 + 2] = z;
    position[i * 3] = fma((local * 7) % 5, deformation, x);
    position[i * 3 + 1] = fma((local * 11) % 5, deformation, y);
    position[i * 3 + 2] = fma((local * 13) % 5, deformation, z);
  }
  for (int t = 0; t < ntets; t++) {
    for (int c = 0; c < 4; c++)
      tets[t * 4 + c] = block_tets[(t % 32) * 4 + c] + (t / 32) * 20;
    double a[9];
    for (int r = 0; r < 3; r++)
      for (int c = 0; c < 3; c++)
        a[r * 3 + c] =
            rest[tets[t * 4 + c + 1] * 3 + r] - rest[tets[t * 4] * 3 + r];
    double cof[9] = {a[4] * a[8] - a[5] * a[7], a[5] * a[6] - a[3] * a[8],
                     a[3] * a[7] - a[4] * a[6], a[2] * a[7] - a[1] * a[8],
                     a[0] * a[8] - a[2] * a[6], a[1] * a[6] - a[0] * a[7],
                     a[1] * a[5] - a[2] * a[4], a[2] * a[3] - a[0] * a[5],
                     a[0] * a[4] - a[1] * a[3]};
    double det = a[0] * cof[0] + a[1] * cof[1] + a[2] * cof[2];
    if (fabs(det) < 1e-12)
      return 4;
    volume[t] = fabs(det) / 6;
    for (int r = 0; r < 3; r++)
      for (int c = 0; c < 3; c++)
        inverse[t * 9 + r * 3 + c] = cof[c * 3 + r] / det;
  }
  compute_tet_forces(forces, position, tets, inverse, volume, 1000, 5000, ntets,
                     nodes);
  if (output(forces, (nodes * 3) * sizeof(double)))
    return 3;
  return 0;
}
