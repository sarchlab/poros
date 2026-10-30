#include <stdio.h>
#include <stdint.h>
int main(void){
 const uint64_t values[]={0,1,0x123456789abcULL,0x0000ffffffffffffULL,0x0001000000000000ULL,0x000f123456789abcULL,0x0010123456789abcULL,0x007f123456789abcULL,0x0080123456789abcULL,0x00ff123456789abcULL,0xab00123456789abcULL,0xab7f123456789abcULL,0xab80123456789abcULL,0xabff123456789abcULL,0xffff123456789abcULL,~0ULL};
 puts("{\"vectors\":[");for(unsigned i=0;i<sizeof(values)/sizeof(values[0]);i++){
 uint64_t d=values[i],a=values[i];
 __asm__ volatile(".arch armv8.3-a\n\txpacd %0" : "+r"(d));
 __asm__ volatile(".arch armv8.3-a\n\txpaci %0" : "+r"(a));
 printf("%s{\"input\":\"%016llx\",\"xpacd\":\"%016llx\",\"xpaci\":\"%016llx\"}",i?",":"",values[i],d,a);
 }puts("]}");return 0;}
