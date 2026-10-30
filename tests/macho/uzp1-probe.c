#include <stdint.h>
#include <stdio.h>
#include <string.h>
__attribute__((noinline)) static uint64_t run0(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v2.8b,v0.8b,v1.8b\nmrs %0,nzcv\nstr q2,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run1(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.8b,v0.8b,v1.8b\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run2(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v1.8b,v0.8b,v1.8b\nmrs %0,nzcv\nstr q1,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run3(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.8b,v0.8b,v0.8b\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run4(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v2.16b,v0.16b,v1.16b\nmrs %0,nzcv\nstr q2,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run5(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.16b,v0.16b,v1.16b\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run6(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v1.16b,v0.16b,v1.16b\nmrs %0,nzcv\nstr q1,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run7(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.16b,v0.16b,v0.16b\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run8(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v2.4h,v0.4h,v1.4h\nmrs %0,nzcv\nstr q2,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run9(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.4h,v0.4h,v1.4h\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run10(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v1.4h,v0.4h,v1.4h\nmrs %0,nzcv\nstr q1,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run11(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.4h,v0.4h,v0.4h\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run12(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v2.8h,v0.8h,v1.8h\nmrs %0,nzcv\nstr q2,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run13(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.8h,v0.8h,v1.8h\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run14(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v1.8h,v0.8h,v1.8h\nmrs %0,nzcv\nstr q1,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run15(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.8h,v0.8h,v0.8h\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run16(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v2.2s,v0.2s,v1.2s\nmrs %0,nzcv\nstr q2,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run17(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.2s,v0.2s,v1.2s\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run18(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v1.2s,v0.2s,v1.2s\nmrs %0,nzcv\nstr q1,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run19(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.2s,v0.2s,v0.2s\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run20(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v2.4s,v0.4s,v1.4s\nmrs %0,nzcv\nstr q2,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run21(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.4s,v0.4s,v1.4s\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run22(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v1.4s,v0.4s,v1.4s\nmrs %0,nzcv\nstr q1,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run23(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.4s,v0.4s,v0.4s\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run24(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v2.2d,v0.2d,v1.2d\nmrs %0,nzcv\nstr q2,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run25(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.2d,v0.2d,v1.2d\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run26(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v1.2d,v0.2d,v1.2d\nmrs %0,nzcv\nstr q1,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run27(const unsigned char*a,const unsigned char*b,unsigned char*out){uint64_t flags,initial=0x90000000;
 __asm__ volatile("ldr q0,[%1]\nldr q1,[%2]\nmovi v2.16b,#0xa5\nmsr nzcv,%4\nuzp1 v0.2d,v0.2d,v0.2d\nmrs %0,nzcv\nstr q0,[%3]":"=&r"(flags):"r"(a),"r"(b),"r"(out),"r"(initial):"v0","v1","v2","memory","cc");return flags;}
struct entry{const char*name;int width,n,variant;uint64_t(*fn)(const unsigned char*,const unsigned char*,unsigned char*);};
static struct entry entries[]={
{"8b",1,8,0,run0},
{"8b",1,8,1,run1},
{"8b",1,8,2,run2},
{"8b",1,8,3,run3},
{"16b",1,16,0,run4},
{"16b",1,16,1,run5},
{"16b",1,16,2,run6},
{"16b",1,16,3,run7},
{"4h",2,8,0,run8},
{"4h",2,8,1,run9},
{"4h",2,8,2,run10},
{"4h",2,8,3,run11},
{"8h",2,16,0,run12},
{"8h",2,16,1,run13},
{"8h",2,16,2,run14},
{"8h",2,16,3,run15},
{"2s",4,8,0,run16},
{"2s",4,8,1,run17},
{"2s",4,8,2,run18},
{"2s",4,8,3,run19},
{"4s",4,16,0,run20},
{"4s",4,16,1,run21},
{"4s",4,16,2,run22},
{"4s",4,16,3,run23},
{"2d",8,16,0,run24},
{"2d",8,16,1,run25},
{"2d",8,16,2,run26},
{"2d",8,16,3,run27}};
int main(void){unsigned emitted=0;puts("[");int shifts[]={-128,-65,-64,-33,-32,-17,-16,-9,-8,-1,0,1,7,8,15,16,31,32,63,64,127,3};
for(unsigned f=0;f<sizeof(entries)/sizeof(entries[0]);f++){struct entry e=entries[f];for(unsigned p=0;p<4;p++){unsigned char a[16],b[16],out[16];for(int k=0;k<16;k++){a[k]=p%4==0?255:p%4==1?(unsigned char)(k*17):p%4==2?(unsigned char)(0x80^(k*29)):(unsigned char)(1+k);b[k]=(unsigned char)(0xd3^(k*11));}
(void)shifts;
uint64_t flags=e.fn(a,b,out);printf("%s{\"function\":\"run%u\",\"arrangement\":\"%s\",\"variant\":%d,\"nzcv\":%llu,\"a_hex\":\"",emitted++?",":"",f,e.name,e.variant,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",a[k]);printf("\",\"b_hex\":\"");for(int k=0;k<16;k++)printf("%02x",b[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}}puts("]");return 0;}
