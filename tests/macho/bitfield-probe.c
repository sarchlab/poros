#include <stdio.h>
#include <stdint.h>
int main(void) {
 uint64_t in=0x200000000a8d42ULL,u,s;uint32_t in32=0x80000001,w;
 __asm__("ubfx %0,%1,#51,#11" : "=r"(u) : "r"(in));
 __asm__("sbfx %0,%1,#51,#3" : "=r"(s) : "r"(in));
 __asm__("sbfx %w0,%w1,#28,#4" : "=r"(w) : "r"(in32));
 printf("{\"ubfx51_11\":%llu,\"sbfx51_3\":%lld,\"sbfx_w28_4\":%d}\n",u,(int64_t)s,(int32_t)w);
 return !(u==4 && (int64_t)s==-4 && (int32_t)w==-8);
}
