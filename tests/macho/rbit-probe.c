#include <stdint.h>
#include <stdio.h>
static uint64_t probe_rbit(uint64_t value,unsigned bits,unsigned alias){uint64_t out=value;
 if(bits==32){if(alias)__asm__ volatile("rbit %w0,%w0":"+r"(out));else __asm__ volatile("rbit %w0,%w1":"=&r"(out):"r"(value));}
 else{if(alias)__asm__ volatile("rbit %x0,%x0":"+r"(out));else __asm__ volatile("rbit %x0,%x1":"=&r"(out):"r"(value));}return out;}
static uint64_t probe_rev(uint64_t value,unsigned bits,unsigned alias){uint64_t out=value;
 if(bits==32){if(alias)__asm__ volatile("rev %w0,%w0":"+r"(out));else __asm__ volatile("rev %w0,%w1":"=&r"(out):"r"(value));}
 else{if(alias)__asm__ volatile("rev %x0,%x0":"+r"(out));else __asm__ volatile("rev %x0,%x1":"=&r"(out):"r"(value));}return out;}
int main(void){uint64_t state=0x134ad278c9b5e60fULL;unsigned first=1,bad=0;printf("{\"cases\":[");
for(unsigned kind=0;kind<2;kind++)for(unsigned bits=32;bits<=64;bits*=2)for(unsigned alias=0;alias<2;alias++)for(unsigned sample=0;sample<68;sample++){
 state^=state<<13;state^=state>>7;state^=state<<17;uint64_t value=sample<64?(1ULL<<sample):sample==64?0:sample==65?~0ULL:state;uint64_t expected=0;
 if(kind==0){for(unsigned i=0;i<bits;i++)expected|=((value>>i)&1)<<(bits-1-i);}else{for(unsigned i=0;i<bits/8;i++)expected|=((value>>(8*i))&255)<<(bits-8-8*i);}
 uint64_t out=kind==0?probe_rbit(value,bits,alias):probe_rev(value,bits,alias);if(out!=expected)bad++;if(!first)putchar(',');first=0;
 printf("{\"operation\":\"%s\",\"bits\":%u,\"alias\":%s,\"input\":\"%016llx\",\"output\":\"%016llx\"}",kind==0?"rbit":"rev",bits,alias?"true":"false",(unsigned long long)value,(unsigned long long)out);
}printf("],\"bad\":%u}\n",bad);return bad?1:0;}
