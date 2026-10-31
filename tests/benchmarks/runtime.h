#ifndef POROS_BENCHMARK_RUNTIME_H
#define POROS_BENCHMARK_RUNTIME_H
#include <stddef.h>

/* Explicit FMA preserves historical input-generation rounding where used. */
double fma(double, double, double);

/* The same source can run natively as an independent compiler/platform
 * reference. Only argument transport and stdout transport differ. */
int workload(int argc, char **argv);
static int argument(const char *s, int low, int high, int *result) {
  unsigned value = 0;
  if (!*s)
    return 0;
  while (*s) {
    unsigned digit = (unsigned)(*s++ - '0');
    if (digit > 9 || value > ((unsigned)high - digit) / 10)
      return 0;
    value = value * 10 + digit;
    if (value > (unsigned)high)
      return 0;
  }
  if (value < (unsigned)low)
    return 0;
  *result = (int)value;
  return 1;
}
#ifdef NATIVE_REFERENCE
#include <stdio.h>
static int output(const void *data, size_t bytes) {
  return fwrite(data, 1, bytes, stdout) != bytes;
}
int main(int argc, char **argv) { return workload(argc, argv); }
#else
static int output(const void *data, size_t bytes) {
  register unsigned long x0 __asm__("x0") = 1;
  register const void *x1 __asm__("x1") = data;
  register size_t x2 __asm__("x2") = bytes;
  register unsigned long x8 __asm__("x8") = 64;
  __asm__ volatile("svc #0" : "+r"(x0) : "r"(x1), "r"(x2), "r"(x8) : "memory");
  return x0 != bytes;
}
int guest_main(unsigned long *sp) {
  return workload((int)sp[0], (char **)(sp + 1));
}
#endif
#endif
