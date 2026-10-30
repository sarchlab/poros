#include <stdint.h>
#include <stdio.h>
int main(void){puts("{\"path\":\"/AppleInternal/XBS/.isChrooted\",\"cases\":[");for(unsigned i=0;i<12;i++){const char*path="/AppleInternal/XBS/.isChrooted";uint64_t mode=i<9?i:0;if(i==9)path=0;if(i==10)path=(void*)1;if(i==11){path=0;mode=8;}
 register uint64_t x0 __asm__("x0")=(uint64_t)path,x1 __asm__("x1")=mode,n __asm__("x16")=33;uint64_t c;__asm__ volatile("svc #0x80\n\tcset %x[c],cs":"+r"(x0),"+r"(x1),[c]"=r"(c):"r"(n):"cc","memory");printf("%s{\"case\":%u,\"mode\":%llu,\"path_kind\":%u,\"x0\":%llu,\"x1\":%llu,\"carry\":%llu}",i?",":"",i,(unsigned long long)mode,i<9?0:i-8,(unsigned long long)x0,(unsigned long long)x1,(unsigned long long)c);
 }puts("]}");return 0;}
