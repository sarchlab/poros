#include <stdint.h>
#include <stdio.h>
#include <sys/mman.h>
int main(void){uint32_t*p=mmap(0,16384,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(p==MAP_FAILED)return 2;puts("{\"cases\":[");unsigned index=0;
for(unsigned kind=0;kind<2;kind++)for(unsigned off=0;off<=128;off+=4){uint32_t*q=p+off/4;unsigned success=0,failure=0;
for(unsigned i=0;i<100;i++){*p=7;if(q!=p)*q=0;uint32_t old,status,value=9,ordinary=11;
if(kind==0)__asm__ volatile("ldxr %w[o],[%x[p]]\n\tstr %w[t],[%x[q]]\n\tstxr %w[s],%w[v],[%x[p]]" : [o]"=&r"(old),[s]"=&r"(status):[p]"r"(p),[q]"r"(q),[v]"r"(value),[t]"r"(ordinary):"memory");
else __asm__ volatile("ldxr %w[o],[%x[p]]\n\tldxr %w[o],[%x[q]]\n\tstxr %w[s],%w[v],[%x[p]]" : [o]"=&r"(old),[s]"=&r"(status):[p]"r"(p),[q]"r"(q),[v]"r"(value):"memory");
if(status==0){success++;if(*p!=9)return 3;}else if(status==1)failure++;else return 4;
}printf("%s{\"kind\":\"%s\",\"offset\":%u,\"successes\":%u,\"failures\":%u}",index++?",":"",kind?"second-exclusive-load":"ordinary-store",off,success,failure);
}puts("]}");return 0;}
