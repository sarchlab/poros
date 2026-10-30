// Own-process ARMv8.1 atomic value/alias oracle. No host synchronization state.
#include <stdint.h>
#include <inttypes.h>
#include <stdio.h>
int main(void){
 const uint64_t inputs[]={0,7,UINT64_MAX,0x8000000080000000ULL};unsigned index=0;
 puts("{\"vectors\":[");
 for(unsigned width=4;width<=8;width+=4)for(unsigned trial=0;trial<4;trial++)for(unsigned mode=0;mode<4;mode++){
  uint64_t before=inputs[trial],cell=before,compare=before+(mode==1),replacement=before^0x876543215abc9876ULL,old;
  if(width==4){before=(uint32_t)before;cell=before;compare=(uint32_t)compare;replacement=(uint32_t)replacement;}
  if(mode==2)replacement=compare;
  old=compare;
  if(mode==3){
   old=replacement;
   if(width==8)__asm__ volatile("swpal %x[v],%x[v],[%x[p]]" : [v]"+&r"(old) : [p]"r"(&cell) : "memory");
   else __asm__ volatile("swpal %w[v],%w[v],[%x[p]]" : [v]"+&r"(old) : [p]"r"(&cell) : "memory");
  }else if(mode==2){
   if(width==8)__asm__ volatile("casal %x[c],%x[c],[%x[p]]" : [c]"+&r"(old) : [p]"r"(&cell) : "memory");
   else __asm__ volatile("casal %w[c],%w[c],[%x[p]]" : [c]"+&r"(old) : [p]"r"(&cell) : "memory");
  }else{
   if(width==8)__asm__ volatile("casal %x[c],%x[v],[%x[p]]" : [c]"+&r"(old) : [v]"r"(replacement),[p]"r"(&cell) : "memory");
   else __asm__ volatile("casal %w[c],%w[v],[%x[p]]" : [c]"+&r"(old) : [v]"r"(replacement),[p]"r"(&cell) : "memory");
  }
  printf("%s{\"width\":%u,\"mode\":%u,\"before\":%"PRIu64",\"compare\":%"PRIu64",\"replacement\":%"PRIu64",\"old\":%"PRIu64",\"after\":%"PRIu64"}",index++?",":"",width,mode,before,compare,replacement,old,cell);
 }puts("]}");return 0;
}
