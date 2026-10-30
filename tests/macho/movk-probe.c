#include <stdint.h>
#include <stdio.h>
static uint64_t f32_0(uint64_t d){__asm__ volatile("movk %w0,#0xa5c3,lsl #0":"+r"(d));return d;}
static uint64_t f32_16(uint64_t d){__asm__ volatile("movk %w0,#0xa5c3,lsl #16":"+r"(d));return d;}
static uint64_t f64_0(uint64_t d){__asm__ volatile("movk %x0,#0xa5c3,lsl #0":"+r"(d));return d;}
static uint64_t f64_16(uint64_t d){__asm__ volatile("movk %x0,#0xa5c3,lsl #16":"+r"(d));return d;}
static uint64_t f64_32(uint64_t d){__asm__ volatile("movk %x0,#0xa5c3,lsl #32":"+r"(d));return d;}
static uint64_t f64_48(uint64_t d){__asm__ volatile("movk %x0,#0xa5c3,lsl #48":"+r"(d));return d;}
typedef uint64_t(*fn)(uint64_t);
struct entry{fn f;unsigned bits,shift;};
static struct entry cases[]={{f32_0,32,0},{f32_16,32,16},{f64_0,64,0},{f64_16,64,16},{f64_32,64,32},{f64_48,64,48}};
int main(void){unsigned bad=0,first=1;uint64_t state=0x1e28563c945fa0b7ULL;printf("{\"cases\":[");
for(unsigned c=0;c<sizeof(cases)/sizeof(cases[0]);c++)for(unsigned sample=0;sample<16;sample++){
 struct entry t=cases[c];state^=state<<13;state^=state>>7;state^=state<<17;uint64_t d=sample==0?~0ULL:sample==1?0:state;
 uint64_t expected=(d&~(0xffffULL<<t.shift))|(0xa5c3ULL<<t.shift);if(t.bits==32)expected&=0xffffffff;
 uint64_t out=t.f(d);if(out!=expected)bad++;if(!first)putchar(',');first=0;
 printf("{\"bits\":%u,\"shift\":%u,\"immediate\":42435,\"destination\":\"%016llx\",\"output\":\"%016llx\"}",t.bits,t.shift,(unsigned long long)d,(unsigned long long)out);
}printf("],\"bad\":%u}\n",bad);return bad?1:0;}
