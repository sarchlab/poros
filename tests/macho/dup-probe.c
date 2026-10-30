#include <stdint.h>
#include <stdio.h>
#include <string.h>
__attribute__((noinline)) static void run_0_0_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.8b,w1\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_0_0_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.8b,wzr\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_0_1_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.8b,w1\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_0_1_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.8b,wzr\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_0_31_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.8b,w1\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_0_31_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.8b,wzr\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_1_0_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.16b,w1\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_1_0_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.16b,wzr\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_1_1_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.16b,w1\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_1_1_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.16b,wzr\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_1_31_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.16b,w1\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_1_31_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.16b,wzr\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_2_0_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.4h,w1\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_2_0_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.4h,wzr\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_2_1_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.4h,w1\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_2_1_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.4h,wzr\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_2_31_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.4h,w1\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_2_31_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.4h,wzr\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_3_0_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.8h,w1\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_3_0_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.8h,wzr\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_3_1_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.8h,w1\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_3_1_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.8h,wzr\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_3_31_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.8h,w1\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_3_31_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.8h,wzr\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_4_0_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.2s,w1\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_4_0_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.2s,wzr\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_4_1_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.2s,w1\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_4_1_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.2s,wzr\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_4_31_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.2s,w1\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_4_31_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.2s,wzr\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_5_0_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.4s,w1\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_5_0_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.4s,wzr\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_5_1_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.4s,w1\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_5_1_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.4s,wzr\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_5_31_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.4s,w1\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_5_31_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.4s,wzr\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_6_0_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.2d,x1\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_6_0_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v0.16b,#0xa5\nmsr nzcv,%3\ndup v0.2d,xzr\nmrs %0,nzcv\nstr q0,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v0","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_6_1_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.2d,x1\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_6_1_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v1.16b,#0xa5\nmsr nzcv,%3\ndup v1.2d,xzr\nmrs %0,nzcv\nstr q1,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v1","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_6_31_0(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.2d,x1\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
__attribute__((noinline)) static void run_6_31_1(uint64_t value,unsigned char*out,uint64_t*flags,uint64_t*after){
 register uint64_t input __asm__("x1")=value;uint64_t initial=0xa0000000,got;
 __asm__ volatile("movi v31.16b,#0xa5\nmsr nzcv,%3\ndup v31.2d,xzr\nmrs %0,nzcv\nstr q31,[%2]":"=&r"(got),"+r"(input):"r"(out),"r"(initial):"v31","memory","cc");*flags=got;*after=input;
}
int main(void){uint64_t values[]={0,1,0xffffffffffffffffULL,0x0123456789abcdefULL,0x8000000080000080ULL,0xff00aa55ff008001ULL};unsigned emitted=0;puts("[");
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_0_0_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8b\",\"destination\":0,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_0_0_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8b\",\"destination\":0,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_0_1_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8b\",\"destination\":1,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_0_1_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8b\",\"destination\":1,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_0_31_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8b\",\"destination\":31,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_0_31_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8b\",\"destination\":31,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_1_0_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"16b\",\"destination\":0,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_1_0_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"16b\",\"destination\":0,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_1_1_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"16b\",\"destination\":1,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_1_1_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"16b\",\"destination\":1,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_1_31_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"16b\",\"destination\":31,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_1_31_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"16b\",\"destination\":31,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_2_0_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4h\",\"destination\":0,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_2_0_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4h\",\"destination\":0,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_2_1_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4h\",\"destination\":1,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_2_1_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4h\",\"destination\":1,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_2_31_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4h\",\"destination\":31,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_2_31_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4h\",\"destination\":31,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_3_0_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8h\",\"destination\":0,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_3_0_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8h\",\"destination\":0,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_3_1_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8h\",\"destination\":1,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_3_1_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8h\",\"destination\":1,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_3_31_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8h\",\"destination\":31,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_3_31_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"8h\",\"destination\":31,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_4_0_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2s\",\"destination\":0,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_4_0_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2s\",\"destination\":0,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_4_1_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2s\",\"destination\":1,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_4_1_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2s\",\"destination\":1,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_4_31_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2s\",\"destination\":31,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_4_31_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2s\",\"destination\":31,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_5_0_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4s\",\"destination\":0,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_5_0_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4s\",\"destination\":0,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_5_1_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4s\",\"destination\":1,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_5_1_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4s\",\"destination\":1,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_5_31_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4s\",\"destination\":31,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_5_31_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"4s\",\"destination\":31,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_6_0_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2d\",\"destination\":0,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_6_0_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2d\",\"destination\":0,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_6_1_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2d\",\"destination\":1,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_6_1_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2d\",\"destination\":1,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_6_31_0(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2d\",\"destination\":31,\"source_zero\":false,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
for(unsigned j=0;j<sizeof(values)/sizeof(values[0]);j++){unsigned char out[16];uint64_t flags,after;run_6_31_1(values[j],out,&flags,&after);
 printf("%s{\"arrangement\":\"2d\",\"destination\":31,\"source_zero\":true,\"input\":\"%016llx\",\"source_after\":\"%016llx\",\"nzcv\":%llu,\"output_hex\":\"",emitted++?",":"",(unsigned long long)values[j],(unsigned long long)after,(unsigned long long)flags);for(int k=0;k<16;k++)printf("%02x",out[k]);printf("\"}");}
puts("]");return 0;}
