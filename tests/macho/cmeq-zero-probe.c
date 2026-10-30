#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <stddef.h>
struct result { uint64_t vector[2], source[2], other[2], flags, gpr; uint32_t encoding; };
_Static_assert(offsetof(struct result,encoding)==64,"layout");
#define PROBE(name,instruction) \
static __attribute__((noinline)) void name(struct result *out,const uint64_t *input,const uint64_t *other,uint64_t flags,uint64_t gpr) { \
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%1]\nldr q2,[%2]\nmsr nzcv,%3\nmov x10,%4\nadr x12,1f\nldr w12,[x12]\nstr w12,[%0,#64]\n1: " instruction "\nstr q0,[%0]\nstr q1,[%0,#16]\nstr q2,[%0,#32]\nmrs x12,nzcv\nstr x12,[%0,#48]\nstr x10,[%0,#56]"::"r"(out),"r"(input),"r"(other),"r"(flags),"r"(gpr):"x10","x12","v0","v1","v2","cc","memory"); \
}
PROBE(f8_8_0, "cmeq v0.8b,v1.8b,#0")
PROBE(f8_8_1, "cmeq v0.8b,v0.8b,#0")
PROBE(f8_16_0, "cmeq v0.16b,v1.16b,#0")
PROBE(f8_16_1, "cmeq v0.16b,v0.16b,#0")
PROBE(f16_4_0, "cmeq v0.4h,v1.4h,#0")
PROBE(f16_4_1, "cmeq v0.4h,v0.4h,#0")
PROBE(f16_8_0, "cmeq v0.8h,v1.8h,#0")
PROBE(f16_8_1, "cmeq v0.8h,v0.8h,#0")
PROBE(f32_2_0, "cmeq v0.2s,v1.2s,#0")
PROBE(f32_2_1, "cmeq v0.2s,v0.2s,#0")
PROBE(f32_4_0, "cmeq v0.4s,v1.4s,#0")
PROBE(f32_4_1, "cmeq v0.4s,v0.4s,#0")
PROBE(f64_2_0, "cmeq v0.2d,v1.2d,#0")
PROBE(f64_2_1, "cmeq v0.2d,v0.2d,#0")
struct entry {int bits,lanes,alias;void(*run)(struct result*,const uint64_t*,const uint64_t*,uint64_t,uint64_t);};
static const struct entry cases[]={{8,8,0,f8_8_0},{8,8,1,f8_8_1},{8,16,0,f8_16_0},{8,16,1,f8_16_1},{16,4,0,f16_4_0},{16,4,1,f16_4_1},{16,8,0,f16_8_0},{16,8,1,f16_8_1},{32,2,0,f32_2_0},{32,2,1,f32_2_1},{32,4,0,f32_4_0},{32,4,1,f32_4_1},{64,2,0,f64_2_0},{64,2,1,f64_2_1}};
int main(void){
 int first=1;const uint64_t other[2]={0x0123456789abcdefull,0xfedcba9876543210ull},gpr=0x8877665544332211ull;
 puts("[");
 for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++)for(unsigned p=0;p<6;p++){
  const struct entry*c=&cases[i];uint64_t input[2]={UINT64_MAX,UINT64_MAX},want[2]={0,0};int width=c->bits/8;
  for(int lane=0;lane<c->lanes;lane++){
   uint64_t value=0;
   switch(p){case 0:value=0;break;case 1:value=UINT64_MAX;break;case 2:value=(lane&1)?UINT64_MAX:0;break;case 3:value=(lane==0)?0:1;break;case 4:value=(lane==c->lanes-1)?0:1;break;case 5:value=1ull<<(c->bits-1);break;}
   memcpy((unsigned char*)input+lane*width,&value,width);uint64_t expected=value==0?UINT64_MAX:0;memcpy((unsigned char*)want+lane*width,&expected,width);
  }
  struct result out={0};uint64_t flags=(p&1)?0xa0000000ull:0x50000000ull;c->run(&out,input,other,flags,gpr);
  if(memcmp(out.vector,want,16)||memcmp(out.source,input,16)||memcmp(out.other,other,16)||out.flags!=flags||out.gpr!=gpr)return 3;
  printf("%s{\"bits\":%d,\"lanes\":%d,\"alias\":%s,\"encoding\":\"%08x\",\"input_lo\":\"%016llx\",\"input_hi\":\"%016llx\",\"output_lo\":\"%016llx\",\"output_hi\":\"%016llx\",\"nzcv\":\"%08llx\",\"source_v1_unchanged\":true,\"other_v2_unchanged\":true,\"gpr_x10_unchanged\":true}",first?"":",\n",c->bits,c->lanes,c->alias?"true":"false",out.encoding,(unsigned long long)input[0],(unsigned long long)input[1],(unsigned long long)out.vector[0],(unsigned long long)out.vector[1],(unsigned long long)out.flags);first=0;
 }
 puts("\n]");return 0;
}
