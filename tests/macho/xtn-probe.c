#include <stdint.h>
#include <stdio.h>
#include <string.h>
static void probe(unsigned char *out,const unsigned char *in,int bits,int alias){
#define OP(A,B) if(alias)__asm__ volatile("ldr q0,[%1]\nxtn v0." A ",v0." B "\nstr q0,[%0]"::"r"(out),"r"(in):"v0","memory");else __asm__ volatile("movi v0.16b,#255\nldr q1,[%1]\nxtn v0." A ",v1." B "\nstr q0,[%0]"::"r"(out),"r"(in):"v0","v1","memory")
 if(bits==8){OP("8b","8h");}else if(bits==16){OP("4h","4s");}else{OP("2s","2d");}
#undef OP
}
static uint64_t state=0x2ac56173d9b47ef0ULL;
static uint64_t next(void){state^=state<<13;state^=state>>7;state^=state<<17;return state;}
static void hex(const unsigned char *p){for(int i=0;i<16;i++)printf("%02x",p[i]);}
int main(void){unsigned first=1,bad=0;printf("{\"cases\":[");
 for(int bits=8;bits<=32;bits*=2)for(int alias=0;alias<2;alias++)for(int sample=0;sample<64;sample++){
  unsigned char in[16],out[16],expected[16]={0};
  for(int i=0;i<16;i++)in[i]=(unsigned char)next();
  if(sample==0)memset(in,255,16);if(sample==1)memset(in,0,16);if(sample==2)for(int i=0;i<16;i++)in[i]=(unsigned char)(i*17);
  for(int lane=0;lane<64/bits;lane++)memcpy(expected+lane*(bits/8),in+lane*(bits/4),(size_t)(bits/8));
  probe(out,in,bits,alias);unsigned ok=memcmp(out,expected,16)==0;if(!ok)bad++;
  if(!first)putchar(',');first=0;printf("{\"bits\":%d,\"alias\":%s,\"input\":\"",bits,alias?"true":"false");hex(in);printf("\",\"output\":\"");hex(out);printf("\",\"ok\":%s}",ok?"true":"false");
 }
 printf("],\"bad\":%u}\n",bad);return bad?1:0;
}
