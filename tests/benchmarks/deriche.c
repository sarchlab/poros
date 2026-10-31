#include "../../polybench/kernels/kernel_deriche.c"
#include "runtime.h"

static double input[1572864];
static double result[1572864];
static double y1[1572864];
static double y2[1572864];

int workload(int argc, char **argv) {
  if (argc != 3)
    return 2;
  int w;
  if (!argument(argv[1], 1, 1536, &w))
    return 2;
  int h;
  if (!argument(argv[2], 1, 1024, &h))
    return 2;
  for (int i = 0; i < w; i++)
    for (int j = 0; j < h; j++)
      input[i * h + j] = (double)((313 * i + 991 * j) % 65536) / 65535;
  kernel_deriche(w, h, 0.25f, input, result, y1, y2);
  if (output(result, (w * h) * sizeof(double)))
    return 3;
  return 0;
}
