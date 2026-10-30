#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <unistd.h>
extern void run0(void*,void*);
__asm__(".text\n.p2align 2\n_run0:\nldr x8,[x0]\nldr x9,[x0,#8]\nldr x10,[x0,#16]\nmsr nzcv,x10\n.inst 0xb8bfc108\nstp x8,x9,[x1]\nmrs x10,nzcv\nstr x10,[x1,#16]\nret\n");
extern void run1(void*,void*);
__asm__(".text\n.p2align 2\n_run1:\nldr x8,[x0]\nldr x9,[x0,#8]\nldr x10,[x0,#16]\nmsr nzcv,x10\n.inst 0xb8bfc109\nstp x8,x9,[x1]\nmrs x10,nzcv\nstr x10,[x1,#16]\nret\n");
extern void run2(void*,void*);
__asm__(".text\n.p2align 2\n_run2:\nldr x8,[x0]\nldr x9,[x0,#8]\nldr x10,[x0,#16]\nmsr nzcv,x10\n.inst 0xb8bfc11f\nstp x8,x9,[x1]\nmrs x10,nzcv\nstr x10,[x1,#16]\nret\n");
extern void run3(void*,void*);
__asm__(".text\n.p2align 2\n_run3:\nldr x8,[x0]\nldr x9,[x0,#8]\nldr x10,[x0,#16]\nmsr nzcv,x10\n.inst 0xf8bfc108\nstp x8,x9,[x1]\nmrs x10,nzcv\nstr x10,[x1,#16]\nret\n");
extern void run4(void*,void*);
__asm__(".text\n.p2align 2\n_run4:\nldr x8,[x0]\nldr x9,[x0,#8]\nldr x10,[x0,#16]\nmsr nzcv,x10\n.inst 0xf8bfc109\nstp x8,x9,[x1]\nmrs x10,nzcv\nstr x10,[x1,#16]\nret\n");
extern void run5(void*,void*);
__asm__(".text\n.p2align 2\n_run5:\nldr x8,[x0]\nldr x9,[x0,#8]\nldr x10,[x0,#16]\nmsr nzcv,x10\n.inst 0xf8bfc11f\nstp x8,x9,[x1]\nmrs x10,nzcv\nstr x10,[x1,#16]\nret\n");
struct entry {void(*fn)(void*,void*);unsigned width,dest,word;};
static struct entry entries[]={{run0,4,8,3099574536u},{run1,4,9,3099574537u},{run2,4,31,3099574559u},{run3,8,8,4173316360u},{run4,8,9,4173316361u},{run5,8,31,4173316383u}};
int main(void){
 uint64_t patterns[]={0,1,0xffffffffffffffffULL,0x8000000080000000ULL,0x0123456789abcdefULL,0xffffffff76543210ULL};
 unsigned ix=0;puts("{\"cases\":[");
 for(unsigned e=0;e<6;e++)for(unsigned p=0;p<6;p++)for(unsigned f=0;f<16;f++){
  _Alignas(16) uint64_t memory=patterns[p]; uint64_t in[]={ (uint64_t)&memory,0xdeadbeefcafebabeULL,(uint64_t)f<<28},after[3]={0};
  entries[e].fn(in,after);
  printf("%s{\"width\":%u,\"dest\":%u,\"encoding\":%u,\"memory\":\"%016llx\",\"before\":[\"%016llx\",\"%016llx\"],\"after\":[\"%016llx\",\"%016llx\"],\"nzcv_before\":%llu,\"nzcv_after\":%llu}",ix++?",":"",entries[e].width,entries[e].dest,entries[e].word,(unsigned long long)memory,(unsigned long long)in[0],(unsigned long long)in[1],(unsigned long long)after[0],(unsigned long long)after[1],(unsigned long long)in[2],(unsigned long long)after[2]);
 }puts("]}");return 0;}
