#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/wait.h>
#include <sys/resource.h>
__attribute__((noinline,used)) int target(void) {return 99;}
static int test(int bad) {
 uint64_t result, changed=(uint64_t)bad;
 __asm__ volatile("adrp x9,_target@PAGE\n\tadd x9,x9,_target@PAGEOFF\n\tmov x10,#0x1234\n\tpacia x9,x10\n\tadd x10,x10,%1\n\tblraa x9,x10\n\tmov %0,x0" : "=r"(result) : "r"(changed) : "x0","x9","x10","x30","cc","memory");
 return result==99;
}
int main(void) {
 puts("[");
 for(int i=0;i<2;i++) {
   pid_t p=fork(); if(p<0) return 2;
   if(p==0) {struct rlimit limit={0,0};setrlimit(RLIMIT_CORE,&limit);_exit(test(i)?0:1);}
   int s;if(waitpid(p,&s,0)!=p)return 3;
   printf("{\"modifier\":\"%s\",\"observed\":\"%s\",\"signal\":%d}%s\n",i?"incorrect":"correct",WIFSIGNALED(s)?"signal":WEXITSTATUS(s)==0?"returned_99":"wrong_result",WIFSIGNALED(s)?WTERMSIG(s):0,i?"":",");
 }
 puts("]");
}
