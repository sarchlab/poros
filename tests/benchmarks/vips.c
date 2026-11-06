#include "../../parsec/kernels/vips.c"
#include "runtime.h"

static double input[5308416];
static double result[5308416];
static double weights[9] = {1.0 / 16, 2.0 / 16, 1.0 / 16, 2.0 / 16, 4.0 / 16,
                            2.0 / 16, 1.0 / 16, 2.0 / 16, 1.0 / 16};

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int w;
  if (!argument(argv[1], 3, 2304, &w))
    return 2;
  int h;
  if (!argument(argv[2], 3, 2304, &h))
    return 2;
  for (int i = 0; i < w * h; i++)
    input[i] = (double)(i % 256) / 255;
  convolve2d(result, input, weights, w, h, 3);
  if (output(result, (w * h) * sizeof(double)))
    return 3;
  return 0;
}
