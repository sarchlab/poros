#include "../../parsec/kernels/x264.c"
#include "runtime.h"

static unsigned char current[640 * 640];
static unsigned char reference[640 * 640];
static int result[3];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int range;
  if (!argument(argv[1], 1, 256, &range))
    return 2;
  /* Preserve the small fixture; larger ranges require a padded frame. */
  int width = range <= 8 ? 128 : 2 * range + 128;
  int origin = range <= 8 ? 48 : range + 48;
  for (int i = 0; i < width * width; i++)
    current[i] = reference[i] = (i * 73 + 17) % 256;
  for (int y = 0; y < 16; y++)
    for (int x = 0; x < 16; x++)
      reference[(origin - 2 + y) * width + origin + 3 + x] =
          current[(origin + y) * width + origin + x] + (y * 3 + x * 7) % 5;
  x264_motion_search(current + origin * width + origin,
                     reference + origin * width + origin, width,
                     range, &result[0], &result[1], &result[2]);
  if (output(result, (3) * sizeof(int)))
    return 3;
  return 0;
}
