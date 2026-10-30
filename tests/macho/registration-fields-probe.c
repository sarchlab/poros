#include <stdint.h>
extern int __bsdthread_register(void*,void*,int,void*,uintptr_t,uintptr_t);
#ifndef PROBE_CASE
#define PROBE_CASE 0
#endif
struct result {uint64_t x0,x1,nzcv;};
static struct result trap(uint64_t no,uint64_t aa,uint64_t bb,uint64_t cc,uint64_t dd,uint64_t ee,uint64_t ff){register uint64_t a __asm__("x0")=aa,b __asm__("x1")=bb,c __asm__("x2")=cc,d __asm__("x3")=dd,e __asm__("x4")=ee,f __asm__("x5")=ff,n __asm__("x16")=no;uint64_t flags,in=0x50000000;__asm__ volatile("msr nzcv,%8\nsvc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(d),"r"(e),"r"(f),"r"(n),"r"(in):"memory","cc");return(struct result){a,b,flags};}
static void put32(volatile unsigned char*p,uint32_t v){for(unsigned i=0;i<4;i++)p[i]=(unsigned char)(v>>(8*i));}
static int hook(void*a,void*b,int c,void*d,uintptr_t e,uintptr_t f){
struct {struct result result;uint64_t size,pthsize;unsigned char before[128],after[128];} out={0};volatile unsigned char data[128];for(unsigned i=0;i<128;i++)data[i]=i<e?((volatile unsigned char*)d)[i]:0xaa;
unsigned offset=56;uint32_t value=1920;uint64_t size=e,pth=(uint32_t)c;
switch(PROBE_CASE){case 0:break;case 1:value=0;break;case 2:value=1;break;case 3:value=16383;break;case 4:value=16384;break;case 5:value=0xffffffff;break;case 6:offset=24;value=16376;break;case 7:offset=24;value=16377;break;case 8:offset=8;value=16384;break;case 9:offset=28;value=16384;break;case 10:offset=32;value=16384;break;case 11:offset=52;value=16384;break;case 12:offset=48;value=0xffffffff;break;case 13:size=56;put32(data,56);break;case 14:size=64;put32(data,64);break;case 15:pth=0;break;case 16:pth=65536;break;case 17:pth=65537;break;case 18:value=16152;break;case 19:value=16153;break;case 20:put32(data+24,0);value=16376;break;case 21:put32(data+24,0);value=16377;break;}
put32(data+offset,value);for(unsigned i=0;i<128;i++)out.before[i]=data[i];out.size=size;out.pthsize=pth;
out.result=trap(366,(uintptr_t)a,(uintptr_t)b,pth,(uintptr_t)data,size,f);for(unsigned i=0;i<128;i++)out.after[i]=data[i];trap(4,1,(uintptr_t)&out,sizeof(out),0,0,0);trap(1,0,0,0,0,0,0);__builtin_unreachable();}
__attribute__((used,section("__DATA,__interpose"))) static struct{const void*r;const void*o;}pair={hook,__bsdthread_register};
