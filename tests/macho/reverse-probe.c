#include <stdint.h>
#include <stdio.h>
extern void run0(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run0:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe0\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe0:\nrev16 w0,w1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run1(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run1:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe1\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe1:\nrev16 w1,w1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run2(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run2:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe2\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe2:\nrev16 x0,x1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run3(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run3:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe3\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe3:\nrev16 x1,x1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run4(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run4:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe4\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe4:\nrev32 x0,x1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run5(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run5:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe5\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe5:\nrev32 x1,x1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
typedef void (*fn)(uint64_t*,uint64_t*);
struct variant {fn f;const char*ins;int width,d,s,c;const char*cond;};
static struct variant variants[]={
{run0,"rev16 w0,w1",32,0,1,2,""},
{run1,"rev16 w1,w1",32,1,1,2,""},
{run2,"rev16 x0,x1",64,0,1,2,""},
{run3,"rev16 x1,x1",64,1,1,2,""},
{run4,"rev32 x0,x1",64,0,1,2,""},
{run5,"rev32 x1,x1",64,1,1,2,""},
};
static uint64_t values[]={0x0ULL,0x1ULL,0x64ULL,0xffffffffffffffffULL,0x80000000ULL,0x8000000000000000ULL,0x7fffffffffffffffULL,0x123456789abcdefULL};
static uint64_t counts[]={0x0ULL,0x1ULL,0xfULL,0x10ULL,0x1fULL,0x20ULL,0x21ULL,0x3fULL,0x40ULL,0x41ULL,0x7fULL,0xffULL,0xffffffffffffffffULL,0x100000020ULL};
int main(void){int first=1;puts("[");
for(unsigned i=0;i<sizeof(variants)/sizeof(variants[0]);i++){struct variant v=variants[i];
for(int j=0;j<1;j++)for(int k=0;k<8;k++){uint64_t flags=(uint64_t)k<<28,source=values[k],count=0x5aa55aa55aa55aa5ULL;
uint64_t in[4]={0xa5a5a5a55a5a5a5aULL,source,count,flags},out[5];v.f(in,out);
printf("%s{\"instruction\":\"%s\",\"encoding\":\"%08x\",\"width\":%d,\"destination\":%d,\"source\":%d,\"count_register\":%d,\"condition\":\"%s\",\"before\":[\"%016llx\",\"%016llx\",\"%016llx\"],\"after\":[\"%016llx\",\"%016llx\",\"%016llx\"],\"nzcv_before\":%llu,\"nzcv_after\":%llu}",first?"":",\n",v.ins,(unsigned)out[4],v.width,v.d,v.s,v.c,v.cond,in[0],in[1],in[2],out[0],out[1],out[2],in[3],out[3]);first=0;
}}puts("\n]");return 0;}