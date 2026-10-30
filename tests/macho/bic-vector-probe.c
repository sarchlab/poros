#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <stddef.h>
struct result {uint64_t vector[2],rn[2],rm[2],flags;uint32_t encoding;};
_Static_assert(offsetof(struct result,encoding)==56,"layout");
#define PROBE(name,instruction) \
static __attribute__((noinline)) void name(struct result*out,const uint64_t*initial,const uint64_t*rn,const uint64_t*rm,uint64_t flags){ \
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nldr q2,[%3]\nmsr nzcv,%4\nadr x12,1f\nldr w12,[x12]\nstr w12,[%0,#56]\n1: " instruction "\nstr q0,[%0]\nstr q1,[%0,#16]\nstr q2,[%0,#32]\nmrs x12,nzcv\nstr x12,[%0,#48]"::"r"(out),"r"(initial),"r"(rn),"r"(rm),"r"(flags):"x12","v0","v1","v2","cc","memory"); \
}
PROBE(f8_0,"bic v0.8b,v1.8b,v2.8b")
PROBE(f8_1,"bic v0.8b,v0.8b,v2.8b")
PROBE(f8_2,"bic v0.8b,v1.8b,v0.8b")
PROBE(f8_3,"bic v0.8b,v0.8b,v0.8b")
PROBE(f16_0,"bic v0.16b,v1.16b,v2.16b")
PROBE(f16_1,"bic v0.16b,v0.16b,v2.16b")
PROBE(f16_2,"bic v0.16b,v1.16b,v0.16b")
PROBE(f16_3,"bic v0.16b,v0.16b,v0.16b")
struct entry{int size,alias;void(*run)(struct result*,const uint64_t*,const uint64_t*,const uint64_t*,uint64_t);};
static const struct entry cases[]={{8,0,f8_0},{8,1,f8_1},{8,2,f8_2},{8,3,f8_3},{16,0,f16_0},{16,1,f16_1},{16,2,f16_2},{16,3,f16_3}};
int main(void){
 const uint64_t patterns[][2]={{0,0},{UINT64_MAX,UINT64_MAX},{0x0706050403020100ull,0x0f0e0d0c0b0a0908ull},{0x80ff7f00aa55fe01ull,0xfeff80017f00aa55ull},{0x8000000080000000ull,0x8000000000000001ull}};
 const uint64_t sentinel[2]={0x1234567890abcdefull,0xfedcba0987654321ull};int first=1;puts("[");
 for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++)for(unsigned a=0;a<5;a++)for(unsigned b=0;b<5;b++){
  const struct entry*c=&cases[i];const uint64_t*rn=patterns[a],*rm=patterns[b],*initial=c->alias==0?sentinel:c->alias==2?rm:rn;
  uint64_t want[2]={0,0};const unsigned char*l=(const unsigned char*)rn,*r=(const unsigned char*)(c->alias==3?rn:rm);
  for(int j=0;j<c->size;j++)((unsigned char*)want)[j]=l[j]&(unsigned char)~r[j];
  uint64_t flags=((a+b)&1)?0xa0000000ull:0x50000000ull;struct result out={0};c->run(&out,initial,rn,rm,flags);
  if(memcmp(out.vector,want,16)||memcmp(out.rn,rn,16)||memcmp(out.rm,rm,16)||out.flags!=flags)return 3;
  printf("%s{\"bytes\":%d,\"alias\":%d,\"encoding\":\"%08x\",\"initial_lo\":\"%016llx\",\"initial_hi\":\"%016llx\",\"rn_lo\":\"%016llx\",\"rn_hi\":\"%016llx\",\"rm_lo\":\"%016llx\",\"rm_hi\":\"%016llx\",\"output_lo\":\"%016llx\",\"output_hi\":\"%016llx\",\"nzcv\":\"%08llx\",\"v1_v2_unchanged\":true}",first?"":",\n",c->size,c->alias,out.encoding,(unsigned long long)initial[0],(unsigned long long)initial[1],(unsigned long long)rn[0],(unsigned long long)rn[1],(unsigned long long)rm[0],(unsigned long long)rm[1],(unsigned long long)out.vector[0],(unsigned long long)out.vector[1],(unsigned long long)out.flags);first=0;
 }
 puts("\n]");return 0;
}
