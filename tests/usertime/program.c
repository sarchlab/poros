// Complete freestanding computation programs; no function extraction or ROI.
typedef unsigned long u64;
static volatile u64 data[8192];
static u64 compute(u64 n) {
 u64 sum=1;
#if CASE == 1
 for(u64 i=0;i<n;i++) {sum=(sum*1664525+1013904223)&0xffffffffUL;}
#elif CASE == 2
 for(u64 i=0;i<8192;i++)data[i]=i;
 for(u64 i=0;i<n;i++){u64 j=i&8191;data[j]+=i;sum+=data[j];}
#elif CASE == 3
 for(u64 i=0;i<n;i++){sum=(sum*1664525+1013904223)&0xffffffffUL;if(sum&0x10000)sum^=0x1234;else sum+=7;}
#endif
 return sum;
}
#ifdef DARWIN
#include <stdlib.h>
#include <unistd.h>
int main(int argc,char**argv){if(argc!=2)return 2;u64 n=strtoul(argv[1],0,10);u64 result=compute(n);return write(1,&result,8)==8?0:3;}
#else
int guest_main(u64 *sp){if(sp[0]!=2)return 2;char*s=(char*)sp[2];u64 n=0;while(*s){if(*s<'0'||*s>'9')return 2;n=n*10+*s++-'0';if(n>10000000)return 2;}u64 result=compute(n);
 register u64 x0 asm("x0")=1,x1 asm("x1")=(u64)&result,x2 asm("x2")=8,x8 asm("x8")=64;
 asm volatile("svc #0":"+r"(x0):"r"(x1),"r"(x2),"r"(x8):"memory");return x0==8?0:3;}
#endif
