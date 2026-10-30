#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>
int main(void){unsigned flags[]={0,1,2};puts("{\"cases\":[");for(unsigned i=0;i<3;i++){
 register uint64_t x0 __asm__("x0")=(uint64_t)"/dev/dtracehelper";register uint64_t x1 __asm__("x1")=flags[i];register uint64_t x2 __asm__("x2")=0;register uint64_t x16 __asm__("x16")=5;uint64_t carry;
 __asm__ volatile("svc #0x80\n\tcset %x[c],cs" : "+r"(x0),"+r"(x1),"+r"(x2),[c]"=r"(carry):"r"(x16):"cc","memory");
 uint64_t result=x0,second=x1;if(!carry)close((int)x0);
 printf("%s{\"flags\":%u,\"x0\":%llu,\"x1\":%llu,\"carry\":%llu}",i?",":"",flags[i],(unsigned long long)result,(unsigned long long)second,(unsigned long long)carry);
 }puts("]}");return 0;}
