#include <stdint.h>
#include <stdio.h>
extern void run0(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run0:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe0\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe0:\nlsl w0,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run1(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run1:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe1\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe1:\nlsl w1,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run2(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run2:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe2\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe2:\nlsl w2,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run3(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run3:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe3\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe3:\nlsl w1,w1,w1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run4(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run4:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe4\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe4:\nlsl x0,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run5(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run5:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe5\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe5:\nlsl x1,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run6(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run6:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe6\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe6:\nlsl x2,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run7(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run7:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe7\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe7:\nlsl x1,x1,x1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run8(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run8:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe8\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe8:\nlsr w0,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run9(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run9:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe9\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe9:\nlsr w1,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run10(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run10:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe10\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe10:\nlsr w2,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run11(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run11:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe11\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe11:\nlsr w1,w1,w1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run12(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run12:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe12\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe12:\nlsr x0,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run13(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run13:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe13\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe13:\nlsr x1,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run14(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run14:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe14\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe14:\nlsr x2,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run15(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run15:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe15\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe15:\nlsr x1,x1,x1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run16(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run16:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe16\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe16:\nasr w0,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run17(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run17:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe17\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe17:\nasr w1,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run18(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run18:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe18\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe18:\nasr w2,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run19(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run19:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe19\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe19:\nasr w1,w1,w1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run20(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run20:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe20\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe20:\nasr x0,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run21(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run21:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe21\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe21:\nasr x1,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run22(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run22:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe22\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe22:\nasr x2,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run23(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run23:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe23\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe23:\nasr x1,x1,x1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run24(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run24:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe24\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe24:\nror w0,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run25(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run25:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe25\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe25:\nror w1,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run26(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run26:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe26\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe26:\nror w2,w1,w2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run27(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run27:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe27\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe27:\nror w1,w1,w1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run28(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run28:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe28\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe28:\nror x0,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run29(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run29:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe29\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe29:\nror x1,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run30(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run30:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe30\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe30:\nror x2,x1,x2\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run31(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run31:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe31\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe31:\nror x1,x1,x1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
typedef void (*fn)(uint64_t*,uint64_t*);
struct variant {fn f;const char*ins;int width,d,s,c;const char*cond;};
static struct variant variants[]={
{run0,"lsl w0,w1,w2",32,0,1,2,""},
{run1,"lsl w1,w1,w2",32,1,1,2,""},
{run2,"lsl w2,w1,w2",32,2,1,2,""},
{run3,"lsl w1,w1,w1",32,1,1,1,""},
{run4,"lsl x0,x1,x2",64,0,1,2,""},
{run5,"lsl x1,x1,x2",64,1,1,2,""},
{run6,"lsl x2,x1,x2",64,2,1,2,""},
{run7,"lsl x1,x1,x1",64,1,1,1,""},
{run8,"lsr w0,w1,w2",32,0,1,2,""},
{run9,"lsr w1,w1,w2",32,1,1,2,""},
{run10,"lsr w2,w1,w2",32,2,1,2,""},
{run11,"lsr w1,w1,w1",32,1,1,1,""},
{run12,"lsr x0,x1,x2",64,0,1,2,""},
{run13,"lsr x1,x1,x2",64,1,1,2,""},
{run14,"lsr x2,x1,x2",64,2,1,2,""},
{run15,"lsr x1,x1,x1",64,1,1,1,""},
{run16,"asr w0,w1,w2",32,0,1,2,""},
{run17,"asr w1,w1,w2",32,1,1,2,""},
{run18,"asr w2,w1,w2",32,2,1,2,""},
{run19,"asr w1,w1,w1",32,1,1,1,""},
{run20,"asr x0,x1,x2",64,0,1,2,""},
{run21,"asr x1,x1,x2",64,1,1,2,""},
{run22,"asr x2,x1,x2",64,2,1,2,""},
{run23,"asr x1,x1,x1",64,1,1,1,""},
{run24,"ror w0,w1,w2",32,0,1,2,""},
{run25,"ror w1,w1,w2",32,1,1,2,""},
{run26,"ror w2,w1,w2",32,2,1,2,""},
{run27,"ror w1,w1,w1",32,1,1,1,""},
{run28,"ror x0,x1,x2",64,0,1,2,""},
{run29,"ror x1,x1,x2",64,1,1,2,""},
{run30,"ror x2,x1,x2",64,2,1,2,""},
{run31,"ror x1,x1,x1",64,1,1,1,""},
};
static uint64_t values[]={0x0ULL,0x1ULL,0x64ULL,0xffffffffffffffffULL,0x80000000ULL,0x8000000000000000ULL,0x7fffffffffffffffULL,0x123456789abcdefULL};
static uint64_t counts[]={0x0ULL,0x1ULL,0xfULL,0x10ULL,0x1fULL,0x20ULL,0x21ULL,0x3fULL,0x40ULL,0x41ULL,0x7fULL,0xffULL,0xffffffffffffffffULL,0x100000020ULL};
int main(void){int first=1;puts("[");
for(unsigned i=0;i<sizeof(variants)/sizeof(variants[0]);i++){struct variant v=variants[i];
for(int j=0;j<14;j++)for(int k=0;k<8;k++){uint64_t flags=(uint64_t)((j+k)&15)<<28,source=values[k],count=counts[j];
uint64_t in[4]={0xa5a5a5a55a5a5a5aULL,source,count,flags},out[5];v.f(in,out);
printf("%s{\"instruction\":\"%s\",\"encoding\":\"%08x\",\"width\":%d,\"destination\":%d,\"source\":%d,\"count_register\":%d,\"condition\":\"%s\",\"before\":[\"%016llx\",\"%016llx\",\"%016llx\"],\"after\":[\"%016llx\",\"%016llx\",\"%016llx\"],\"nzcv_before\":%llu,\"nzcv_after\":%llu}",first?"":",\n",v.ins,(unsigned)out[4],v.width,v.d,v.s,v.c,v.cond,in[0],in[1],in[2],out[0],out[1],out[2],in[3],out[3]);first=0;
}}puts("\n]");return 0;}