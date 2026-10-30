#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/wait.h>
#include <sys/resource.h>
static int test(int mode) {
  uint64_t value=0x1234567800ULL, modifier=0x1234, other=0x1235;
  if (mode==0) __asm__ volatile("pacib %0,%1\n\tautib %0,%2" : "+&r"(value) : "r"(modifier), "r"(other));
  if (mode==1) __asm__ volatile("pacib %0,%1\n\tautia %0,%1" : "+&r"(value) : "r"(modifier));
  if (mode==2) __asm__ volatile("pacib %0,%1\n\teor %0,%0,#1\n\tautib %0,%1" : "+&r"(value) : "r"(modifier));
  return value != (mode==2 ? 0x1234567801ULL : 0x1234567800ULL);
}
int main(void) {
 const char *names[]={"wrong_modifier","wrong_key","changed_pointer"};
 puts("[");
 for(int i=0;i<3;i++) {
   pid_t p=fork();
   if(p<0) return 2;
   if(p==0) {struct rlimit limit={0,0}; setrlimit(RLIMIT_CORE,&limit); _exit(test(i)?0:1);}
   int s; if(waitpid(p,&s,0)!=p) return 3;
   printf("{\"test\":\"%s\",\"observed\":\"%s\",\"signal\":%d}%s\n",names[i], WIFSIGNALED(s)?"authentication_trap":WEXITSTATUS(s)==0?"noncanonical_result":"unexpected_valid_result",WIFSIGNALED(s)?WTERMSIG(s):0,i==2?"":",");
 }
 puts("]");
}
