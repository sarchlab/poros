#include <stdint.h>
#include <stdio.h>
static unsigned f32_0(uint64_t*p,uint64_t*q,uint64_t add){uint64_t status;__asm__ volatile("mov x9,%1\nmov x10,%2\nmov x11,%3\nmov x12,#13\nldxr w14,[x9]\nldadd w11,w15,[x10]\nstxr w16,w12,[x9]\nmov %0,x16":"=&r"(status):"r"(p),"r"(q),"r"(add):"x9","x10","x11","x12","x14","x15","x16","memory");return(unsigned)status;}
static unsigned f32_1(uint64_t*p,uint64_t*q,uint64_t add){uint64_t status;__asm__ volatile("mov x9,%1\nmov x10,%2\nmov x11,%3\nmov x12,#13\nldxr w14,[x9]\nnop\nstxr w16,w12,[x9]\nmov %0,x16":"=&r"(status):"r"(p),"r"(q),"r"(add):"x9","x10","x11","x12","x14","x15","x16","memory");return(unsigned)status;}
static unsigned f64_0(uint64_t*p,uint64_t*q,uint64_t add){uint64_t status;__asm__ volatile("mov x9,%1\nmov x10,%2\nmov x11,%3\nmov x12,#13\nldxr x14,[x9]\nldadd x11,x15,[x10]\nstxr w16,x12,[x9]\nmov %0,x16":"=&r"(status):"r"(p),"r"(q),"r"(add):"x9","x10","x11","x12","x14","x15","x16","memory");return(unsigned)status;}
static unsigned f64_1(uint64_t*p,uint64_t*q,uint64_t add){uint64_t status;__asm__ volatile("mov x9,%1\nmov x10,%2\nmov x11,%3\nmov x12,#13\nldxr x14,[x9]\nnop\nstxr w16,x12,[x9]\nmov %0,x16":"=&r"(status):"r"(p),"r"(q),"r"(add):"x9","x10","x11","x12","x14","x15","x16","memory");return(unsigned)status;}
typedef unsigned(*fn)(uint64_t*,uint64_t*,uint64_t);
struct entry{fn f;unsigned bits,control;};
static struct entry cases[]={{f32_0,32,0},{f32_1,32,1},{f64_0,64,0},{f64_1,64,1}};
int main(void){_Alignas(64)uint64_t data[64];unsigned bad=0,first=1;printf("{\"cases\":[");for(unsigned c=0;c<4;c++)for(unsigned off=0;off<=128;off+=8)for(unsigned add=0;add<=9;add+=9){
struct entry t=cases[c];unsigned success=0,failure=0;for(unsigned i=0;i<100;i++){uint64_t*p=data,*q=data+off/8;*p=7;*q=7;unsigned st=t.f(p,q,add);if(st==0)success++;else if(st==1)failure++;else bad++;if(*p!=(st==0?13u:off==0&&!t.control?7u+add:7u))bad++;}
if(!first)putchar(',');first=0;printf("{\"bits\":%u,\"control\":%s,\"offset\":%u,\"add\":%u,\"success\":%u,\"failure\":%u}",t.bits,t.control?"true":"false",off,add,success,failure);
}printf("],\"bad\":%u}\n",bad);return bad?1:0;}
