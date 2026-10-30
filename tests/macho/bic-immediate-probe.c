#include <stdint.h>
#include <stdio.h>
#include <string.h>
__attribute__((noinline)) static uint64_t run0(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4h,#0,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run1(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4h,#1,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run2(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4h,#90,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run3(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4h,#128,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run4(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4h,#255,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run5(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4h,#0,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run6(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4h,#1,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run7(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4h,#90,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run8(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4h,#128,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run9(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4h,#255,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run10(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.8h,#0,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run11(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.8h,#1,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run12(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.8h,#90,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run13(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.8h,#128,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run14(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.8h,#255,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run15(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.8h,#0,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run16(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.8h,#1,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run17(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.8h,#90,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run18(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.8h,#128,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run19(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.8h,#255,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run20(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#0,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run21(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#1,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run22(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#90,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run23(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#128,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run24(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#255,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run25(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#0,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run26(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#1,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run27(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#90,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run28(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#128,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run29(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#255,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run30(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#0,lsl #16\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run31(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#1,lsl #16\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run32(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#90,lsl #16\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run33(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#128,lsl #16\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run34(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#255,lsl #16\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run35(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#0,lsl #24\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run36(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#1,lsl #24\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run37(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#90,lsl #24\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run38(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#128,lsl #24\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run39(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.2s,#255,lsl #24\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run40(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#0,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run41(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#1,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run42(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#90,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run43(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#128,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run44(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#255,lsl #0\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run45(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#0,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run46(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#1,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run47(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#90,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run48(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#128,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run49(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#255,lsl #8\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run50(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#0,lsl #16\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run51(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#1,lsl #16\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run52(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#90,lsl #16\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run53(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#128,lsl #16\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run54(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#255,lsl #16\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run55(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#0,lsl #24\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run56(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#1,lsl #24\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run57(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#90,lsl #24\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run58(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#128,lsl #24\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
__attribute__((noinline)) static uint64_t run59(const unsigned char*in,unsigned char*out){uint64_t flags,initial=0x60000000;
 __asm__ volatile("ldr q0,[%1]\nmsr nzcv,%3\nbic v0.4s,#255,lsl #24\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(flags):"r"(in),"r"(out),"r"(initial):"v0","memory","cc");return flags;}
int main(void){unsigned emitted=0;puts("[");
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run0(in,out);
 printf("%s{\"function\":\"run0\",\"arrangement\":\"4h\",\"shift\":0,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run1(in,out);
 printf("%s{\"function\":\"run1\",\"arrangement\":\"4h\",\"shift\":0,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run2(in,out);
 printf("%s{\"function\":\"run2\",\"arrangement\":\"4h\",\"shift\":0,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run3(in,out);
 printf("%s{\"function\":\"run3\",\"arrangement\":\"4h\",\"shift\":0,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run4(in,out);
 printf("%s{\"function\":\"run4\",\"arrangement\":\"4h\",\"shift\":0,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run5(in,out);
 printf("%s{\"function\":\"run5\",\"arrangement\":\"4h\",\"shift\":8,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run6(in,out);
 printf("%s{\"function\":\"run6\",\"arrangement\":\"4h\",\"shift\":8,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run7(in,out);
 printf("%s{\"function\":\"run7\",\"arrangement\":\"4h\",\"shift\":8,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run8(in,out);
 printf("%s{\"function\":\"run8\",\"arrangement\":\"4h\",\"shift\":8,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run9(in,out);
 printf("%s{\"function\":\"run9\",\"arrangement\":\"4h\",\"shift\":8,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run10(in,out);
 printf("%s{\"function\":\"run10\",\"arrangement\":\"8h\",\"shift\":0,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run11(in,out);
 printf("%s{\"function\":\"run11\",\"arrangement\":\"8h\",\"shift\":0,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run12(in,out);
 printf("%s{\"function\":\"run12\",\"arrangement\":\"8h\",\"shift\":0,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run13(in,out);
 printf("%s{\"function\":\"run13\",\"arrangement\":\"8h\",\"shift\":0,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run14(in,out);
 printf("%s{\"function\":\"run14\",\"arrangement\":\"8h\",\"shift\":0,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run15(in,out);
 printf("%s{\"function\":\"run15\",\"arrangement\":\"8h\",\"shift\":8,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run16(in,out);
 printf("%s{\"function\":\"run16\",\"arrangement\":\"8h\",\"shift\":8,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run17(in,out);
 printf("%s{\"function\":\"run17\",\"arrangement\":\"8h\",\"shift\":8,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run18(in,out);
 printf("%s{\"function\":\"run18\",\"arrangement\":\"8h\",\"shift\":8,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run19(in,out);
 printf("%s{\"function\":\"run19\",\"arrangement\":\"8h\",\"shift\":8,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run20(in,out);
 printf("%s{\"function\":\"run20\",\"arrangement\":\"2s\",\"shift\":0,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run21(in,out);
 printf("%s{\"function\":\"run21\",\"arrangement\":\"2s\",\"shift\":0,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run22(in,out);
 printf("%s{\"function\":\"run22\",\"arrangement\":\"2s\",\"shift\":0,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run23(in,out);
 printf("%s{\"function\":\"run23\",\"arrangement\":\"2s\",\"shift\":0,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run24(in,out);
 printf("%s{\"function\":\"run24\",\"arrangement\":\"2s\",\"shift\":0,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run25(in,out);
 printf("%s{\"function\":\"run25\",\"arrangement\":\"2s\",\"shift\":8,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run26(in,out);
 printf("%s{\"function\":\"run26\",\"arrangement\":\"2s\",\"shift\":8,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run27(in,out);
 printf("%s{\"function\":\"run27\",\"arrangement\":\"2s\",\"shift\":8,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run28(in,out);
 printf("%s{\"function\":\"run28\",\"arrangement\":\"2s\",\"shift\":8,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run29(in,out);
 printf("%s{\"function\":\"run29\",\"arrangement\":\"2s\",\"shift\":8,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run30(in,out);
 printf("%s{\"function\":\"run30\",\"arrangement\":\"2s\",\"shift\":16,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run31(in,out);
 printf("%s{\"function\":\"run31\",\"arrangement\":\"2s\",\"shift\":16,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run32(in,out);
 printf("%s{\"function\":\"run32\",\"arrangement\":\"2s\",\"shift\":16,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run33(in,out);
 printf("%s{\"function\":\"run33\",\"arrangement\":\"2s\",\"shift\":16,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run34(in,out);
 printf("%s{\"function\":\"run34\",\"arrangement\":\"2s\",\"shift\":16,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run35(in,out);
 printf("%s{\"function\":\"run35\",\"arrangement\":\"2s\",\"shift\":24,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run36(in,out);
 printf("%s{\"function\":\"run36\",\"arrangement\":\"2s\",\"shift\":24,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run37(in,out);
 printf("%s{\"function\":\"run37\",\"arrangement\":\"2s\",\"shift\":24,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run38(in,out);
 printf("%s{\"function\":\"run38\",\"arrangement\":\"2s\",\"shift\":24,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run39(in,out);
 printf("%s{\"function\":\"run39\",\"arrangement\":\"2s\",\"shift\":24,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run40(in,out);
 printf("%s{\"function\":\"run40\",\"arrangement\":\"4s\",\"shift\":0,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run41(in,out);
 printf("%s{\"function\":\"run41\",\"arrangement\":\"4s\",\"shift\":0,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run42(in,out);
 printf("%s{\"function\":\"run42\",\"arrangement\":\"4s\",\"shift\":0,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run43(in,out);
 printf("%s{\"function\":\"run43\",\"arrangement\":\"4s\",\"shift\":0,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run44(in,out);
 printf("%s{\"function\":\"run44\",\"arrangement\":\"4s\",\"shift\":0,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run45(in,out);
 printf("%s{\"function\":\"run45\",\"arrangement\":\"4s\",\"shift\":8,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run46(in,out);
 printf("%s{\"function\":\"run46\",\"arrangement\":\"4s\",\"shift\":8,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run47(in,out);
 printf("%s{\"function\":\"run47\",\"arrangement\":\"4s\",\"shift\":8,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run48(in,out);
 printf("%s{\"function\":\"run48\",\"arrangement\":\"4s\",\"shift\":8,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run49(in,out);
 printf("%s{\"function\":\"run49\",\"arrangement\":\"4s\",\"shift\":8,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run50(in,out);
 printf("%s{\"function\":\"run50\",\"arrangement\":\"4s\",\"shift\":16,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run51(in,out);
 printf("%s{\"function\":\"run51\",\"arrangement\":\"4s\",\"shift\":16,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run52(in,out);
 printf("%s{\"function\":\"run52\",\"arrangement\":\"4s\",\"shift\":16,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run53(in,out);
 printf("%s{\"function\":\"run53\",\"arrangement\":\"4s\",\"shift\":16,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run54(in,out);
 printf("%s{\"function\":\"run54\",\"arrangement\":\"4s\",\"shift\":16,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run55(in,out);
 printf("%s{\"function\":\"run55\",\"arrangement\":\"4s\",\"shift\":24,\"immediate\":0,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run56(in,out);
 printf("%s{\"function\":\"run56\",\"arrangement\":\"4s\",\"shift\":24,\"immediate\":1,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run57(in,out);
 printf("%s{\"function\":\"run57\",\"arrangement\":\"4s\",\"shift\":24,\"immediate\":90,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run58(in,out);
 printf("%s{\"function\":\"run58\",\"arrangement\":\"4s\",\"shift\":24,\"immediate\":128,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned pattern=0;pattern<4;pattern++){unsigned char in[16],out[16];for(int k=0;k<16;k++)in[k]=pattern==0?0:pattern==1?255:pattern==2?(unsigned char)(k*17):(unsigned char)(0xa5^k);uint64_t flags=run59(in,out);
 printf("%s{\"function\":\"run59\",\"arrangement\":\"4s\",\"shift\":24,\"immediate\":255,\"nzcv\":%llu,\"input_hex\":\"",emitted++?",":"",(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",in[k]);printf("\",\"output_hex\":\"");for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
puts("]");return 0;}
