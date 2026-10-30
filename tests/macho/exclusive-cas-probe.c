#include <stdint.h>
#include <stdio.h>
int main(void){_Alignas(64) uint32_t data[64];unsigned bad=0,first=1;printf("{\"cases\":[");
for(unsigned kind=0;kind<3;kind++)for(unsigned off=0;off<=128;off+=4)for(unsigned match=0;match<2;match++){
 unsigned successes=0,failures=0;for(unsigned repeat=0;repeat<100;repeat++){
  uint32_t*p=data,*q=data+off/4;*p=7;*q=7;uint32_t cmp=match?7:0,status,old,repl=9,value=13;
  if(kind==0)__asm__ volatile("ldxr %w[o],[%x[p]]\nnop\nstxr %w[s],%w[v],[%x[p]]": [o]"=&r"(old),[c]"+r"(cmp),[s]"=&r"(status):[p]"r"(p),[q]"r"(q),[r]"r"(repl),[v]"r"(value):"memory");
  else if(kind==1)__asm__ volatile("ldxr %w[o],[%x[p]]\ncas %w[c],%w[r],[%x[q]]\nstxr %w[s],%w[v],[%x[p]]": [o]"=&r"(old),[c]"+r"(cmp),[s]"=&r"(status):[p]"r"(p),[q]"r"(q),[r]"r"(repl),[v]"r"(value):"memory");
  else __asm__ volatile("ldxr %w[o],[%x[p]]\nswp %w[r],%w[c],[%x[q]]\nstxr %w[s],%w[v],[%x[p]]": [o]"=&r"(old),[c]"+r"(cmp),[s]"=&r"(status):[p]"r"(p),[q]"r"(q),[r]"r"(repl),[v]"r"(value):"memory");
  if(status==0)successes++;else if(status==1)failures++;else bad++;
  if((kind!=0&&cmp!=7)||*p!=(status==0?13u:(off==0&&(kind==2||(kind==1&&match))?9u:7u)))bad++;
 }
 if(!first)putchar(',');first=0;printf("{\"kind\":%u,\"offset\":%u,\"match\":%s,\"success\":%u,\"failure\":%u}",kind,off,match?"true":"false",successes,failures);
}printf("],\"bad\":%u}\n",bad);return bad?1:0;}
