// Read only documented nonsecret CPU/runtime capabilities, never a raw commpage.
// XNU osfmk/arm/cpu_capabilities.h defines these stable ARM64 commpage offsets.
#include <stdint.h>
#include <stdio.h>
#include <mach/mach_time.h>
int main(void) {
    const volatile uint8_t *p = (const volatile uint8_t *)0xfffffc000ULL;
    mach_timebase_info_data_t tb;
    if (mach_timebase_info(&tb) != 0) return 1;
    printf("{\"cpu_capabilities\":%llu,\"version\":%u,"
           "\"user_page_shift_64_legacy\":%u,\"cache_line_size\":%u,"
           "\"cpu_family\":%u,\"user_timebase\":%u,"
           "\"continuous_hwclock\":%u,\"kdebug\":%u,"
           "\"timebase_numer\":%u,\"timebase_denom\":%u}\n",
           *(const volatile uint64_t *)(p + 0x10),
           *(const volatile uint16_t *)(p + 0x1e), p[0x25],
           *(const volatile uint16_t *)(p + 0x26),
           *(const volatile uint32_t *)(p + 0x80), p[0x90], p[0x91],
           *(const volatile uint32_t *)(p + 0x100), tb.numer, tb.denom);
}
