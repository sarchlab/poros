#include <stdint.h>
#include <stdio.h>
extern void run0(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run0:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\n.inst 0xd503241f\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run1(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run1:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\n.inst 0xd503245f\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run2(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run2:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\n.inst 0xd503249f\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run3(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run3:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\n.inst 0xd50324df\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
typedef void (*fn)(uint64_t*,uint64_t*);
int main(void){fn funcs[]={run0,run1,run2,run3};uint32_t words[]={0xd503241f,0xd503245f,0xd503249f,0xd50324df};int first=1;puts("[");for(int i=0;i<4;i++)for(int j=0;j<16;j++){uint64_t in[4]={0xa5a5a5a55a5a5a5aULL^(uint64_t)j,0xfedcba9876543210ULL,0x123456789abcdefULL,(uint64_t)j<<28},out[4];funcs[i](in,out);
printf("%s{\"encoding\":\"%08x\",\"before\":[\"%016llx\",\"%016llx\",\"%016llx\"],\"after\":[\"%016llx\",\"%016llx\",\"%016llx\"],\"nzcv_before\":%llu,\"nzcv_after\":%llu}",first?"":",\n",words[i],in[0],in[1],in[2],out[0],out[1],out[2],in[3],out[3]);first=0;
}puts("\n]");return 0;}