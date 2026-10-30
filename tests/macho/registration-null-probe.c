#include <stdint.h>
extern int __bsdthread_register(void*,void*,int,void*,uintptr_t,uintptr_t);
#ifndef PROBE_CASE
#define PROBE_CASE 0
#endif
struct result {uint64_t x0,x1,nzcv;};
static struct result trap(uint64_t no,uint64_t aa,uint64_t bb,uint64_t cc,uint64_t dd,uint64_t ee,uint64_t ff){register uint64_t a __asm__("x0")=aa,b __asm__("x1")=bb,c __asm__("x2")=cc,d __asm__("x3")=dd,e __asm__("x4")=ee,f __asm__("x5")=ff,n __asm__("x16")=no;uint64_t flags,in=0x50000000;__asm__ volatile("msr nzcv,%8\nsvc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(d),"r"(e),"r"(f),"r"(n),"r"(in):"memory","cc");return(struct result){a,b,flags};}
static void put32(volatile unsigned char*p,uint32_t v){for(unsigned i=0;i<4;i++)p[i]=(unsigned char)(v>>(8*i));}
static int hook(void*a,void*b,int c,void*d,uintptr_t e,uintptr_t f){
 struct {struct result first,duplicate;uint64_t size,offset,pthsize;} out={0};
 out.size=PROBE_CASE==1?UINT64_MAX:0;out.offset=PROBE_CASE==2?UINT64_MAX:f;out.pthsize=PROBE_CASE==3?0:(uint32_t)c;
 out.first=trap(366,(uintptr_t)a,(uintptr_t)b,out.pthsize,0,out.size,out.offset);
 out.duplicate=trap(366,(uintptr_t)a,(uintptr_t)b,(uint32_t)c,(uintptr_t)d,e,f);
 trap(4,1,(uintptr_t)&out,sizeof(out),0,0,0);trap(1,0,0,0,0,0,0);__builtin_unreachable();}
__attribute__((used,section("__DATA,__interpose"))) static struct{const void*r;const void*o;}pair={hook,__bsdthread_register};
