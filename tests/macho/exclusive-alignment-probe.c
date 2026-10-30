#include <stdint.h>
#include <stdio.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <unistd.h>
#include <string.h>
extern uint64_t probe_4_0_0(void *,uint64_t);
__asm__(".p2align 2\n.globl _probe_4_0_0\n_probe_4_0_0:\nclrex\nstxr w2,w1,[x0]\nmov w0,w2\nret");
extern uint64_t probe_4_0_1(void *,uint64_t);
__asm__(".p2align 2\n.globl _probe_4_0_1\n_probe_4_0_1:\nmov x9,sp\nmov sp,x0\nclrex\nstxr w2,w1,[sp]\nmov w0,w2\nmov sp,x9\nret");
extern uint64_t probe_4_1_0(void *,uint64_t);
__asm__(".p2align 2\n.globl _probe_4_1_0\n_probe_4_1_0:\nldxr w0,[x0]\nclrex\nret");
extern uint64_t probe_4_1_1(void *,uint64_t);
__asm__(".p2align 2\n.globl _probe_4_1_1\n_probe_4_1_1:\nmov x9,sp\nmov sp,x0\nldxr w0,[sp]\nclrex\nmov sp,x9\nret");
extern uint64_t probe_8_0_0(void *,uint64_t);
__asm__(".p2align 2\n.globl _probe_8_0_0\n_probe_8_0_0:\nclrex\nstxr w2,x1,[x0]\nmov w0,w2\nret");
extern uint64_t probe_8_0_1(void *,uint64_t);
__asm__(".p2align 2\n.globl _probe_8_0_1\n_probe_8_0_1:\nmov x9,sp\nmov sp,x0\nclrex\nstxr w2,x1,[sp]\nmov w0,w2\nmov sp,x9\nret");
extern uint64_t probe_8_1_0(void *,uint64_t);
__asm__(".p2align 2\n.globl _probe_8_1_0\n_probe_8_1_0:\nldxr x0,[x0]\nclrex\nret");
extern uint64_t probe_8_1_1(void *,uint64_t);
__asm__(".p2align 2\n.globl _probe_8_1_1\n_probe_8_1_1:\nmov x9,sp\nmov sp,x0\nldxr x0,[sp]\nclrex\nmov sp,x9\nret");
typedef uint64_t(*probe)(void*,uint64_t);
static probe probes[]={probe_4_0_0,probe_4_0_1,probe_4_1_0,probe_4_1_1,probe_8_0_0,probe_8_0_1,probe_8_1_0,probe_8_1_1};
static int run(unsigned index,unsigned off){
unsigned width=index<4?4:8,load=(index/2)%2;unsigned char*p=mmap(0,16384,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(p==MAP_FAILED)return 99;
void*addr=p+8192+off;uint64_t initial=9,actual=0;memcpy(addr,&initial,width);uint64_t r=probes[index](addr,15);memcpy(&actual,addr,width);return actual==9&&r==(load?9:1)?0:96;}
int main(void){puts("{\"cases\":[");unsigned count=0;for(unsigned index=0;index<8;index++)for(unsigned off=0;off<(index%2?16:32);off++){
pid_t p=fork();if(p<0)return 2;if(p==0)_exit(run(index,off));int status;if(waitpid(p,&status,0)!=p)return 3;
printf("%s{\"width\":%u,\"load\":%s,\"sp\":%s,\"offset\":%u,\"exit\":%d,\"signal\":%d}",count++?",":"",index<4?4:8,(index/2)%2?"true":"false",index%2?"true":"false",off,WIFEXITED(status)?WEXITSTATUS(status):-1,WIFSIGNALED(status)?WTERMSIG(status):0);
}puts("]}");return 0;}
