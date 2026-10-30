#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
static unsigned run(uint32_t *p, unsigned svc) {
 uint64_t status;
 if (svc) {
  __asm__ volatile("ldxr w10, [%1]\nmov x16, #20\nsvc #0x80\nmov w11, #9\nstxr w12, w11, [%1]\nmov %0, x12" : "=&r"(status) : "r"(p) : "x0","x1","x10","x11","x12","x16","cc","memory");
 } else {
  __asm__ volatile("ldxr w10, [%1]\nmov w11, #9\nstxr w12, w11, [%1]\nmov %0, x12" : "=&r"(status) : "r"(p) : "x10","x11","x12","cc","memory");
 }
 return (unsigned)status;
}
int main(void) {
 uint32_t *p=aligned_alloc(64,64); if(!p)return 2;
 unsigned ok[2]={0}, failed[2]={0}, bad=0;
 for(unsigned mode=0;mode<2;mode++)for(unsigned i=0;i<100;i++) {
  *p=7; unsigned s=run(p,mode); if(s==0)ok[mode]++;else if(s==1)failed[mode]++;else bad++;
  if(*p!=(s==0?9u:7u))bad++;
 }
 printf("{\"iterations\":100,\"control_success\":%u,\"control_failed\":%u,\"svc_success\":%u,\"svc_failed\":%u,\"bad\":%u}\n",ok[0],failed[0],ok[1],failed[1],bad);
 free(p); return bad?1:0;
}
