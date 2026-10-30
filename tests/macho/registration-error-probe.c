#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
extern int __bsdthread_register(void*,void*,int,void*,uintptr_t,uintptr_t);
struct result {uint64_t x0,x1,nzcv;};
static struct result raw(uint64_t aa,uint64_t bb,uint64_t cc,uint64_t dd,uint64_t ee,uint64_t ff){
register uint64_t a __asm__("x0")=aa,b __asm__("x1")=bb,c __asm__("x2")=cc,d __asm__("x3")=dd,e __asm__("x4")=ee,f __asm__("x5")=ff,n __asm__("x16")=366;uint64_t flags,in=0x50000000;
__asm__ volatile("msr nzcv,%9\nsvc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(d),"r"(e),"r"(f),"r"(n),"r"(dd),"r"(in):"memory","cc");return(struct result){a,b,flags};}
static struct result results[8];static int actual_result;static volatile unsigned char untouched[8][60];
static int hook(void*a,void*b,int c,void*d,uintptr_t e,uintptr_t f){
for(unsigned k=0;k<7;k++){volatile unsigned char copy[60];for(unsigned i=0;i<60;i++)copy[i]=((volatile unsigned char*)d)[i];uint64_t size=e,pth=(uint32_t)c,ptr=(uintptr_t)copy;
if(k==0)pth=0xffffffff;if(k==1)pth=0x7fffffff;if(k==2)size=0;if(k==3)size=7;if(k==4)ptr=1;if(k==5)copy[0]=59;if(k==6){size=61;copy[0]=60;}
results[k]=raw((uintptr_t)a,(uintptr_t)b,pth,ptr,size,f);
for(unsigned i=0;i<60;i++)untouched[k][i]=copy[i];
}
actual_result=__bsdthread_register(a,b,c,d,e,f);
results[7]=raw((uintptr_t)a,(uintptr_t)b,(uint32_t)c,(uintptr_t)d,e,f);for(unsigned i=0;i<60;i++)untouched[7][i]=((volatile unsigned char*)d)[i];return actual_result;}
__attribute__((used,section("__DATA,__interpose"))) static struct{const void*r;const void*o;}pair={hook,__bsdthread_register};
__attribute__((constructor)) static void report(void){const char*s=getenv("POROS_QUERY_FD");if(!s)return;int fd=atoi(s);const char*names[]={"negative-pthsize","oversize-pthsize","zero-data-size","seven-data-size","invalid-data-pointer","version59-size60","version60-size61","duplicate-registration"};dprintf(fd,"{\"actual_result\":%d,\"cases\":[",actual_result);for(unsigned k=0;k<8;k++){dprintf(fd,"%s{\"name\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"input_nzcv\":1342177280,\"output_nzcv\":%llu,\"after_hex\":\"",k?",":"",names[k],results[k].x0,results[k].x1,results[k].nzcv);for(unsigned i=0;i<60;i++)dprintf(fd,"%02x",untouched[k][i]);dprintf(fd,"\"}");}dprintf(fd,"]}\n");}
