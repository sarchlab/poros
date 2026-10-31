#include "../../parsec/kernels/raytrace.c"
#include "runtime.h"

static double result[32];
static double triangles[36] = {-2, -2, 0, 0,  -2, 0, 0, 0, 0, -2, -2, 0,
                               0,  0,  0, -2, 0,  0, 0, 0, 0, 2,  0,  0,
                               2,  2,  0, 0,  0,  0, 2, 2, 0, 0,  2,  0};
static double bbox[42] = {-2, -2, -.1, 2,   2,   .1,  -2, -2, -.1, 0,   0,
                          .1, 0,  0,   -.1, 2,   2,   .1, -2, -2,  -.1, 0,
                          0,  .1, -2,  -2,  -.1, 0,   0,  .1, 0,   0,   -.1,
                          2,  2,  .1,  0,   0,   -.1, 2,  2,  .1};
static int children[14] = {1, 2, 3, 4, 5, 6, -1, -1, -1, -1, -1, -1, -1, -1};
static int triidx[7] = {-1, -1, -1, 0, 1, 2, 3};

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int nrays;
  if (!argument(argv[1], 1, 32, &nrays))
    return 2;
  for (int i = 0; i < nrays; i++) {
    double ray[6] = {-1, -1, 5, 0, 0, -1};
    if (i % 4 == 1) {
      ray[0] = 1;
      ray[1] = 1;
    }
    if (i % 4 == 2) {
      ray[0] = 3;
      ray[1] = 3;
    }
    if (i % 4 == 3) {
      ray[0] = -1.5;
      ray[1] = -1.25;
      ray[3] = 0.1;
      ray[4] = 0.05;
    }
    result[i] = bvh_trace(ray, bbox, children, triangles, triidx, 7);
  };
  if (output(result, (nrays) * sizeof(double)))
    return 3;
  return 0;
}
