#include <stdint.h>
#include <stdio.h>
static uint64_t f32_plain_0(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadd w10,w11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_plain_1(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadd w10,w10,[x9]\nmov %0,x10":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_plain_2(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadd w10,wzr,[x9]\nmov %0,xzr":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_plain_3(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadd wzr,w11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_a_0(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadda w10,w11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_a_1(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadda w10,w10,[x9]\nmov %0,x10":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_a_2(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadda w10,wzr,[x9]\nmov %0,xzr":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_a_3(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadda wzr,w11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_l_0(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddl w10,w11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_l_1(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddl w10,w10,[x9]\nmov %0,x10":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_l_2(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddl w10,wzr,[x9]\nmov %0,xzr":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_l_3(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddl wzr,w11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_al_0(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddal w10,w11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_al_1(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddal w10,w10,[x9]\nmov %0,x10":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_al_2(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddal w10,wzr,[x9]\nmov %0,xzr":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f32_al_3(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddal wzr,w11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_plain_0(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadd x10,x11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_plain_1(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadd x10,x10,[x9]\nmov %0,x10":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_plain_2(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadd x10,xzr,[x9]\nmov %0,xzr":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_plain_3(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadd xzr,x11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_a_0(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadda x10,x11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_a_1(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadda x10,x10,[x9]\nmov %0,x10":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_a_2(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadda x10,xzr,[x9]\nmov %0,xzr":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_a_3(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldadda xzr,x11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_l_0(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddl x10,x11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_l_1(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddl x10,x10,[x9]\nmov %0,x10":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_l_2(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddl x10,xzr,[x9]\nmov %0,xzr":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_l_3(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddl xzr,x11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_al_0(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddal x10,x11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_al_1(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddal x10,x10,[x9]\nmov %0,x10":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_al_2(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddal x10,xzr,[x9]\nmov %0,xzr":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
static uint64_t f64_al_3(uint64_t*p,uint64_t value){uint64_t out;__asm__ volatile("mov x9,%1\nmov x10,%2\nldaddal xzr,x11,[x9]\nmov %0,x11":"=&r"(out):"r"(p),"r"(value):"x9","x10","x11","memory");return out;}
typedef uint64_t(*fn)(uint64_t*,uint64_t);
struct entry{fn f;unsigned bits,alias;const char*order;};
static struct entry cases[]={{f32_plain_0,32,0,"plain"},{f32_plain_1,32,1,"plain"},{f32_plain_2,32,2,"plain"},{f32_plain_3,32,3,"plain"},{f32_a_0,32,0,"a"},{f32_a_1,32,1,"a"},{f32_a_2,32,2,"a"},{f32_a_3,32,3,"a"},{f32_l_0,32,0,"l"},{f32_l_1,32,1,"l"},{f32_l_2,32,2,"l"},{f32_l_3,32,3,"l"},{f32_al_0,32,0,"al"},{f32_al_1,32,1,"al"},{f32_al_2,32,2,"al"},{f32_al_3,32,3,"al"},{f64_plain_0,64,0,"plain"},{f64_plain_1,64,1,"plain"},{f64_plain_2,64,2,"plain"},{f64_plain_3,64,3,"plain"},{f64_a_0,64,0,"a"},{f64_a_1,64,1,"a"},{f64_a_2,64,2,"a"},{f64_a_3,64,3,"a"},{f64_l_0,64,0,"l"},{f64_l_1,64,1,"l"},{f64_l_2,64,2,"l"},{f64_l_3,64,3,"l"},{f64_al_0,64,0,"al"},{f64_al_1,64,1,"al"},{f64_al_2,64,2,"al"},{f64_al_3,64,3,"al"}};
int main(void){unsigned bad=0,first=1;uint64_t init[]={0,~0ULL,0xffffffffULL,0xaabbccdd80000000ULL,0x123456789abcdef0ULL},adds[]={0,1,2,0x80000000ULL,0xfedcba9876543210ULL};printf("{\"cases\":[");
for(unsigned c=0;c<sizeof(cases)/sizeof(cases[0]);c++)for(unsigned sample=0;sample<5;sample++){
 struct entry t=cases[c];_Alignas(16)uint64_t memory=init[sample];uint64_t add=adds[sample],mask=t.bits==32?0xffffffffULL:~0ULL,old=memory&mask;
 uint64_t expected=(memory&~mask)|((old+(t.alias==3?0:add))&mask);uint64_t out=t.f(&memory,add);
 if(memory!=expected||out!=(t.alias==2?0:old))bad++;
 if(!first)putchar(',');first=0;printf("{\"bits\":%u,\"order\":\"%s\",\"alias\":%u,\"initial\":\"%016llx\",\"add\":\"%016llx\",\"old\":\"%016llx\",\"memory\":\"%016llx\"}",t.bits,t.order,t.alias,(unsigned long long)init[sample],(unsigned long long)add,(unsigned long long)out,(unsigned long long)memory);
}printf("],\"bad\":%u}\n",bad);return bad?1:0;}
