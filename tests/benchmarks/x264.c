#include "../../parsec/kernels/x264.c"
#include "runtime.h"

static unsigned char current[16384];
static unsigned char reference[16384];
static int result[3];

int workload(int argc, char **argv) {
  if (argc != 2)
    return 2;
  int range;
  if (!argument(argv[1], 1, 8, &range))
    return 2;
  for (int i = 0; i < 128 * 128; i++)
    current[i] = reference[i] = (i * 73 + 17) % 256;
  for (int y = 0; y < 16; y++)
    for (int x = 0; x < 16; x++)
      reference[(48 - 2 + y) * 128 + 48 + 3 + x] =
          current[(48 + y) * 128 + 48 + x] + (y * 3 + x * 7) % 5;
  x264_motion_search(current + 48 * 128 + 48, reference + 48 * 128 + 48, 128,
                     range, &result[0], &result[1], &result[2]);
  if (output(result, (3) * sizeof(int)))
    return 3;
  return 0;
}
