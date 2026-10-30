#include <stdint.h>
#include <stdio.h>
#include <string.h>
static void f_4_0_0(void *d,const void *s){ __asm__ volatile("ldp w2,w3,[%1,#0]\nstnp w2,w3,[%0,#-4]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_4_0_1(void *d,const void *s){ __asm__ volatile("ldnp w2,w3,[%1,#-4]\nstp w2,w3,[%0,#0]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_4_1_0(void *d,const void *s){ __asm__ volatile("ldp w2,w3,[%1,#0]\nstnp w2,w3,[%0,#0]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_4_1_1(void *d,const void *s){ __asm__ volatile("ldnp w2,w3,[%1,#0]\nstp w2,w3,[%0,#0]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_4_2_0(void *d,const void *s){ __asm__ volatile("ldp w2,w3,[%1,#0]\nstnp w2,w3,[%0,#4]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_4_2_1(void *d,const void *s){ __asm__ volatile("ldnp w2,w3,[%1,#4]\nstp w2,w3,[%0,#0]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_8_0_0(void *d,const void *s){ __asm__ volatile("ldp x2,x3,[%1,#0]\nstnp x2,x3,[%0,#-8]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_8_0_1(void *d,const void *s){ __asm__ volatile("ldnp x2,x3,[%1,#-8]\nstp x2,x3,[%0,#0]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_8_1_0(void *d,const void *s){ __asm__ volatile("ldp x2,x3,[%1,#0]\nstnp x2,x3,[%0,#0]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_8_1_1(void *d,const void *s){ __asm__ volatile("ldnp x2,x3,[%1,#0]\nstp x2,x3,[%0,#0]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_8_2_0(void *d,const void *s){ __asm__ volatile("ldp x2,x3,[%1,#0]\nstnp x2,x3,[%0,#8]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_8_2_1(void *d,const void *s){ __asm__ volatile("ldnp x2,x3,[%1,#8]\nstp x2,x3,[%0,#0]"::"r"(d),"r"(s):"x2","x3","memory");}
static void f_16_0_0(void *d,const void *s){ __asm__ volatile("ldp q2,q3,[%1,#0]\nstnp q2,q3,[%0,#-16]"::"r"(d),"r"(s):"v2","v3","memory");}
static void f_16_0_1(void *d,const void *s){ __asm__ volatile("ldnp q2,q3,[%1,#-16]\nstp q2,q3,[%0,#0]"::"r"(d),"r"(s):"v2","v3","memory");}
static void f_16_1_0(void *d,const void *s){ __asm__ volatile("ldp q2,q3,[%1,#0]\nstnp q2,q3,[%0,#0]"::"r"(d),"r"(s):"v2","v3","memory");}
static void f_16_1_1(void *d,const void *s){ __asm__ volatile("ldnp q2,q3,[%1,#0]\nstp q2,q3,[%0,#0]"::"r"(d),"r"(s):"v2","v3","memory");}
static void f_16_2_0(void *d,const void *s){ __asm__ volatile("ldp q2,q3,[%1,#0]\nstnp q2,q3,[%0,#16]"::"r"(d),"r"(s):"v2","v3","memory");}
static void f_16_2_1(void *d,const void *s){ __asm__ volatile("ldnp q2,q3,[%1,#16]\nstp q2,q3,[%0,#0]"::"r"(d),"r"(s):"v2","v3","memory");}
typedef void (*fn)(void*,const void*);
struct entry{fn f;int width,offset,load;};
static struct entry cases[]={{f_4_0_0,4,-4,0},{f_4_0_1,4,-4,1},{f_4_1_0,4,0,0},{f_4_1_1,4,0,1},{f_4_2_0,4,4,0},{f_4_2_1,4,4,1},{f_8_0_0,8,-8,0},{f_8_0_1,8,-8,1},{f_8_1_0,8,0,0},{f_8_1_1,8,0,1},{f_8_2_0,8,8,0},{f_8_2_1,8,8,1},{f_16_0_0,16,-16,0},{f_16_0_1,16,-16,1},{f_16_1_0,16,0,0},{f_16_1_1,16,0,1},{f_16_2_0,16,16,0},{f_16_2_1,16,16,1}};
int main(void){
 _Alignas(64) unsigned char src[512],dst[512]; unsigned bad=0;
 for(unsigned i=0;i<sizeof(src);i++)src[i]=(unsigned char)(i*17+23);
 printf("{\"cases\":[");unsigned first=1;
 for(unsigned c=0;c<sizeof(cases)/sizeof(cases[0]);c++)for(int mis=0;mis<16;mis++){
  struct entry t=cases[c];memset(dst,0,sizeof(dst));t.f(dst+128+mis,src+128+mis);
  int di=128+mis+(t.load?0:t.offset),si=128+mis+(t.load?t.offset:0);unsigned ok=1;
  for(int i=0;i<512;i++){unsigned char want=i>=di&&i<di+t.width*2?src[si+i-di]:0;if(dst[i]!=want)ok=0;}
  if(!ok)bad++; if(!first)putchar(',');first=0;
  printf("{\"width\":%d,\"offset\":%d,\"load\":%s,\"misalignment\":%d,\"ok\":%s,\"bytes\":\"",t.width,t.offset,t.load?"true":"false",mis,ok?"true":"false");
  for(int i=0;i<t.width*2;i++)printf("%02x",dst[di+i]);printf("\"}");
 }
 printf("],\"bad\":%u}\n",bad);return bad?1:0;
}