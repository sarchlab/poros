#include <stdint.h>
#include <stdio.h>
static uint64_t f32_0_1_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w1,#0,#1":"+r"(d):"r"(n));return d;}
static uint64_t f32_0_1_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w0,#0,#1":"+r"(d):"r"(n));return d;}
static uint64_t f32_0_16_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w1,#0,#16":"+r"(d):"r"(n));return d;}
static uint64_t f32_0_16_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w0,#0,#16":"+r"(d):"r"(n));return d;}
static uint64_t f32_0_32_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w1,#0,#32":"+r"(d):"r"(n));return d;}
static uint64_t f32_0_32_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w0,#0,#32":"+r"(d):"r"(n));return d;}
static uint64_t f32_1_1_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w1,#1,#1":"+r"(d):"r"(n));return d;}
static uint64_t f32_1_1_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w0,#1,#1":"+r"(d):"r"(n));return d;}
static uint64_t f32_1_15_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w1,#1,#15":"+r"(d):"r"(n));return d;}
static uint64_t f32_1_15_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w0,#1,#15":"+r"(d):"r"(n));return d;}
static uint64_t f32_1_31_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w1,#1,#31":"+r"(d):"r"(n));return d;}
static uint64_t f32_1_31_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w0,#1,#31":"+r"(d):"r"(n));return d;}
static uint64_t f32_16_1_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w1,#16,#1":"+r"(d):"r"(n));return d;}
static uint64_t f32_16_1_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w0,#16,#1":"+r"(d):"r"(n));return d;}
static uint64_t f32_16_8_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w1,#16,#8":"+r"(d):"r"(n));return d;}
static uint64_t f32_16_8_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w0,#16,#8":"+r"(d):"r"(n));return d;}
static uint64_t f32_16_16_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w1,#16,#16":"+r"(d):"r"(n));return d;}
static uint64_t f32_16_16_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w0,#16,#16":"+r"(d):"r"(n));return d;}
static uint64_t f32_31_1_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w1,#31,#1":"+r"(d):"r"(n));return d;}
static uint64_t f32_31_1_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %w0,%w0,#31,#1":"+r"(d):"r"(n));return d;}
static uint64_t f64_0_1_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x1,#0,#1":"+r"(d):"r"(n));return d;}
static uint64_t f64_0_1_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x0,#0,#1":"+r"(d):"r"(n));return d;}
static uint64_t f64_0_32_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x1,#0,#32":"+r"(d):"r"(n));return d;}
static uint64_t f64_0_32_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x0,#0,#32":"+r"(d):"r"(n));return d;}
static uint64_t f64_0_64_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x1,#0,#64":"+r"(d):"r"(n));return d;}
static uint64_t f64_0_64_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x0,#0,#64":"+r"(d):"r"(n));return d;}
static uint64_t f64_1_1_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x1,#1,#1":"+r"(d):"r"(n));return d;}
static uint64_t f64_1_1_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x0,#1,#1":"+r"(d):"r"(n));return d;}
static uint64_t f64_1_31_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x1,#1,#31":"+r"(d):"r"(n));return d;}
static uint64_t f64_1_31_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x0,#1,#31":"+r"(d):"r"(n));return d;}
static uint64_t f64_1_63_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x1,#1,#63":"+r"(d):"r"(n));return d;}
static uint64_t f64_1_63_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x0,#1,#63":"+r"(d):"r"(n));return d;}
static uint64_t f64_32_1_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x1,#32,#1":"+r"(d):"r"(n));return d;}
static uint64_t f64_32_1_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x0,#32,#1":"+r"(d):"r"(n));return d;}
static uint64_t f64_32_16_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x1,#32,#16":"+r"(d):"r"(n));return d;}
static uint64_t f64_32_16_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x0,#32,#16":"+r"(d):"r"(n));return d;}
static uint64_t f64_32_32_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x1,#32,#32":"+r"(d):"r"(n));return d;}
static uint64_t f64_32_32_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x0,#32,#32":"+r"(d):"r"(n));return d;}
static uint64_t f64_63_1_0(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x1,#63,#1":"+r"(d):"r"(n));return d;}
static uint64_t f64_63_1_1(uint64_t d,uint64_t n){__asm__ volatile("bfxil %x0,%x0,#63,#1":"+r"(d):"r"(n));return d;}
typedef uint64_t (*fn)(uint64_t,uint64_t);
struct entry{fn f;unsigned bits,lsb,width,alias;};
static struct entry cases[]={{f32_0_1_0,32,0,1,0},{f32_0_1_1,32,0,1,1},{f32_0_16_0,32,0,16,0},{f32_0_16_1,32,0,16,1},{f32_0_32_0,32,0,32,0},{f32_0_32_1,32,0,32,1},{f32_1_1_0,32,1,1,0},{f32_1_1_1,32,1,1,1},{f32_1_15_0,32,1,15,0},{f32_1_15_1,32,1,15,1},{f32_1_31_0,32,1,31,0},{f32_1_31_1,32,1,31,1},{f32_16_1_0,32,16,1,0},{f32_16_1_1,32,16,1,1},{f32_16_8_0,32,16,8,0},{f32_16_8_1,32,16,8,1},{f32_16_16_0,32,16,16,0},{f32_16_16_1,32,16,16,1},{f32_31_1_0,32,31,1,0},{f32_31_1_1,32,31,1,1},{f64_0_1_0,64,0,1,0},{f64_0_1_1,64,0,1,1},{f64_0_32_0,64,0,32,0},{f64_0_32_1,64,0,32,1},{f64_0_64_0,64,0,64,0},{f64_0_64_1,64,0,64,1},{f64_1_1_0,64,1,1,0},{f64_1_1_1,64,1,1,1},{f64_1_31_0,64,1,31,0},{f64_1_31_1,64,1,31,1},{f64_1_63_0,64,1,63,0},{f64_1_63_1,64,1,63,1},{f64_32_1_0,64,32,1,0},{f64_32_1_1,64,32,1,1},{f64_32_16_0,64,32,16,0},{f64_32_16_1,64,32,16,1},{f64_32_32_0,64,32,32,0},{f64_32_32_1,64,32,32,1},{f64_63_1_0,64,63,1,0},{f64_63_1_1,64,63,1,1}};
static uint64_t state=0x78af6cd94e2b1035ULL;
static uint64_t next(void){state^=state<<13;state^=state>>7;state^=state<<17;return state;}
int main(void){unsigned bad=0,first=1;printf("{\"cases\":[");
 for(unsigned c=0;c<sizeof(cases)/sizeof(cases[0]);c++)for(unsigned sample=0;sample<16;sample++){
  struct entry t=cases[c];uint64_t d=next(),n=next();if(sample==0){d=~0ULL;n=0;}if(sample==1){d=0;n=~0ULL;}if(t.alias)n=d;
  uint64_t mask=t.width==64?~0ULL:(1ULL<<t.width)-1;
  uint64_t expected=(d&~mask)|((n>>t.lsb)&mask);if(t.bits==32)expected&=0xffffffff;
  uint64_t out=t.f(d,n);if(out!=expected)bad++;
  if(!first)putchar(',');first=0;
  printf("{\"bits\":%u,\"lsb\":%u,\"width\":%u,\"alias\":%s,\"destination\":\"%016llx\",\"source\":\"%016llx\",\"output\":\"%016llx\"}",t.bits,t.lsb,t.width,t.alias?"true":"false",(unsigned long long)d,(unsigned long long)n,(unsigned long long)out);
 }
 printf("],\"bad\":%u}\n",bad);return bad?1:0;
}