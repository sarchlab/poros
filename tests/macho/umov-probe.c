#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <stddef.h>
struct result { uint64_t value, other, flags; uint32_t encoding, pad; uint64_t vector[2]; };
_Static_assert(offsetof(struct result, vector)==32,"layout");
#define PROBE(name,instruction) \
static __attribute__((noinline)) void name(struct result *out, const uint64_t *input, uint64_t old, uint64_t other, uint64_t flags) { \
 __asm__ volatile("ldr q0,[%1]\nmov x11,%2\nmov x10,%3\nmsr nzcv,%4\nadr x12,1f\nldr w12,[x12]\nstr w12,[%0,#24]\n1: " instruction "\nstr x11,[%0]\nstr x10,[%0,#8]\nmrs x12,nzcv\nstr x12,[%0,#16]\nstr q0,[%0,#32]" :: "r"(out),"r"(input),"r"(old),"r"(other),"r"(flags):"x10","x11","x12","v0","cc","memory"); \
}
PROBE(f8_0_0, "umov w11,v0.b[0]")
PROBE(f8_0_1, "umov wzr,v0.b[0]")
PROBE(f8_1_0, "umov w11,v0.b[1]")
PROBE(f8_1_1, "umov wzr,v0.b[1]")
PROBE(f8_2_0, "umov w11,v0.b[2]")
PROBE(f8_2_1, "umov wzr,v0.b[2]")
PROBE(f8_3_0, "umov w11,v0.b[3]")
PROBE(f8_3_1, "umov wzr,v0.b[3]")
PROBE(f8_4_0, "umov w11,v0.b[4]")
PROBE(f8_4_1, "umov wzr,v0.b[4]")
PROBE(f8_5_0, "umov w11,v0.b[5]")
PROBE(f8_5_1, "umov wzr,v0.b[5]")
PROBE(f8_6_0, "umov w11,v0.b[6]")
PROBE(f8_6_1, "umov wzr,v0.b[6]")
PROBE(f8_7_0, "umov w11,v0.b[7]")
PROBE(f8_7_1, "umov wzr,v0.b[7]")
PROBE(f8_8_0, "umov w11,v0.b[8]")
PROBE(f8_8_1, "umov wzr,v0.b[8]")
PROBE(f8_9_0, "umov w11,v0.b[9]")
PROBE(f8_9_1, "umov wzr,v0.b[9]")
PROBE(f8_10_0, "umov w11,v0.b[10]")
PROBE(f8_10_1, "umov wzr,v0.b[10]")
PROBE(f8_11_0, "umov w11,v0.b[11]")
PROBE(f8_11_1, "umov wzr,v0.b[11]")
PROBE(f8_12_0, "umov w11,v0.b[12]")
PROBE(f8_12_1, "umov wzr,v0.b[12]")
PROBE(f8_13_0, "umov w11,v0.b[13]")
PROBE(f8_13_1, "umov wzr,v0.b[13]")
PROBE(f8_14_0, "umov w11,v0.b[14]")
PROBE(f8_14_1, "umov wzr,v0.b[14]")
PROBE(f8_15_0, "umov w11,v0.b[15]")
PROBE(f8_15_1, "umov wzr,v0.b[15]")
PROBE(f16_0_0, "umov w11,v0.h[0]")
PROBE(f16_0_1, "umov wzr,v0.h[0]")
PROBE(f16_1_0, "umov w11,v0.h[1]")
PROBE(f16_1_1, "umov wzr,v0.h[1]")
PROBE(f16_2_0, "umov w11,v0.h[2]")
PROBE(f16_2_1, "umov wzr,v0.h[2]")
PROBE(f16_3_0, "umov w11,v0.h[3]")
PROBE(f16_3_1, "umov wzr,v0.h[3]")
PROBE(f16_4_0, "umov w11,v0.h[4]")
PROBE(f16_4_1, "umov wzr,v0.h[4]")
PROBE(f16_5_0, "umov w11,v0.h[5]")
PROBE(f16_5_1, "umov wzr,v0.h[5]")
PROBE(f16_6_0, "umov w11,v0.h[6]")
PROBE(f16_6_1, "umov wzr,v0.h[6]")
PROBE(f16_7_0, "umov w11,v0.h[7]")
PROBE(f16_7_1, "umov wzr,v0.h[7]")
PROBE(f32_0_0, "umov w11,v0.s[0]")
PROBE(f32_0_1, "umov wzr,v0.s[0]")
PROBE(f32_1_0, "umov w11,v0.s[1]")
PROBE(f32_1_1, "umov wzr,v0.s[1]")
PROBE(f32_2_0, "umov w11,v0.s[2]")
PROBE(f32_2_1, "umov wzr,v0.s[2]")
PROBE(f32_3_0, "umov w11,v0.s[3]")
PROBE(f32_3_1, "umov wzr,v0.s[3]")
PROBE(f64_0_0, "umov x11,v0.d[0]")
PROBE(f64_0_1, "umov xzr,v0.d[0]")
PROBE(f64_1_0, "umov x11,v0.d[1]")
PROBE(f64_1_1, "umov xzr,v0.d[1]")
struct entry { int bits,lane,zero; void(*run)(struct result*,const uint64_t*,uint64_t,uint64_t,uint64_t); };
static const struct entry cases[]={{8,0,0,f8_0_0},{8,0,1,f8_0_1},{8,1,0,f8_1_0},{8,1,1,f8_1_1},{8,2,0,f8_2_0},{8,2,1,f8_2_1},{8,3,0,f8_3_0},{8,3,1,f8_3_1},{8,4,0,f8_4_0},{8,4,1,f8_4_1},{8,5,0,f8_5_0},{8,5,1,f8_5_1},{8,6,0,f8_6_0},{8,6,1,f8_6_1},{8,7,0,f8_7_0},{8,7,1,f8_7_1},{8,8,0,f8_8_0},{8,8,1,f8_8_1},{8,9,0,f8_9_0},{8,9,1,f8_9_1},{8,10,0,f8_10_0},{8,10,1,f8_10_1},{8,11,0,f8_11_0},{8,11,1,f8_11_1},{8,12,0,f8_12_0},{8,12,1,f8_12_1},{8,13,0,f8_13_0},{8,13,1,f8_13_1},{8,14,0,f8_14_0},{8,14,1,f8_14_1},{8,15,0,f8_15_0},{8,15,1,f8_15_1},{16,0,0,f16_0_0},{16,0,1,f16_0_1},{16,1,0,f16_1_0},{16,1,1,f16_1_1},{16,2,0,f16_2_0},{16,2,1,f16_2_1},{16,3,0,f16_3_0},{16,3,1,f16_3_1},{16,4,0,f16_4_0},{16,4,1,f16_4_1},{16,5,0,f16_5_0},{16,5,1,f16_5_1},{16,6,0,f16_6_0},{16,6,1,f16_6_1},{16,7,0,f16_7_0},{16,7,1,f16_7_1},{32,0,0,f32_0_0},{32,0,1,f32_0_1},{32,1,0,f32_1_0},{32,1,1,f32_1_1},{32,2,0,f32_2_0},{32,2,1,f32_2_1},{32,3,0,f32_3_0},{32,3,1,f32_3_1},{64,0,0,f64_0_0},{64,0,1,f64_0_1},{64,1,0,f64_1_0},{64,1,1,f64_1_1}};
int main(void) {
 const uint64_t inputs[][2]={{0,0},{UINT64_MAX,UINT64_MAX},{0x0706050403020100ull,0x0f0e0d0c0b0a0908ull},{0x80ff7f00aa55fe01ull,0xfeff80017f00aa55ull},{0x8000000080000000ull,0x8000000000000001ull}};
 const uint64_t old=0xfedcba9876543210ull,other=0x0123456789abcdefull;
 int first=1;
 printf("[\n");
 for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++)for(unsigned p=0;p<sizeof(inputs)/sizeof(inputs[0]);p++) {
  struct result out={0};const struct entry *c=&cases[i];uint64_t flags=(p&1)?0xa0000000ull:0x50000000ull;
  c->run(&out,inputs[p],old,other,flags);
  uint64_t wanted=0;memcpy(&wanted,(const unsigned char*)inputs[p]+c->lane*(c->bits/8),c->bits/8);if(c->zero)wanted=old;
  if(out.value!=wanted||out.other!=other||out.flags!=flags||memcmp(out.vector,inputs[p],16))return 3;
  printf("%s{\"bits\":%d,\"lane\":%d,\"zero_destination\":%s,\"encoding\":\"%08x\",\"vector_lo\":\"%016llx\",\"vector_hi\":\"%016llx\",\"initial_x11\":\"%016llx\",\"result_x11\":\"%016llx\",\"preserved_x10\":\"%016llx\",\"nzcv\":\"%08llx\",\"source_unchanged\":true}",first?"":",\n",c->bits,c->lane,c->zero?"true":"false",out.encoding,(unsigned long long)inputs[p][0],(unsigned long long)inputs[p][1],(unsigned long long)old,(unsigned long long)out.value,(unsigned long long)out.other,(unsigned long long)out.flags);first=0;
 }
 puts("\n]");return 0;
}
