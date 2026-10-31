#include "../runtime.h"
#include <math.h>

/* IEEE-754 edge inputs, independent of application inputs: signed zeros,
 * subnormal/normal boundaries, rounding neighbors, overflow and domain errors.
 */
static const unsigned long inputs[] = {0xbff0000000000000UL,
                                       0x8000000000000000UL,
                                       0,
                                       1,
                                       0x0010000000000000UL,
                                       0x3fe0000000000000UL,
                                       0x3ff0000000000000UL,
                                       0x3ff0000000000001UL,
                                       0x4000000000000000UL,
                                       0x4085e00000000000UL,
                                       0xc087480000000000UL,
                                       0x40862e42fefa39efUL,
                                       0x4086300000000000UL,
                                       0x7ff0000000000000UL,
                                       0x7ff8000000001234UL};
static double result[sizeof(inputs) / sizeof(inputs[0]) * 3];
int workload(int argc, char **argv) {
  (void)argv;
  if (argc != 1)
    return 2;
  for (unsigned i = 0; i < sizeof(inputs) / sizeof(inputs[0]); i++) {
    union {
      unsigned long bits;
      double value;
    } x = {.bits = inputs[i]};
    result[3 * i] = log(x.value);
    result[3 * i + 1] = exp(x.value);
    result[3 * i + 2] = sqrt(x.value);
  }
  return output(result, sizeof(result));
}
