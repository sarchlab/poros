#include <stdint.h>
#include <stdio.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <unistd.h>
#include <string.h>
static void run(unsigned off){
 unsigned char *p=mmap(0,16384,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(p==MAP_FAILED)_exit(99);
 void *address=p+8192+off;uint64_t compare=0,replacement=9,got=0;
 __asm__ volatile("mov x15,sp\n\tmov sp,%x[p]\n\tcasal %x[c],%x[v],[sp]\n\tmov sp,x15" : [c]"+&r"(compare) : [p]"r"(address),[v]"r"(replacement) : "x15","memory");
 memcpy(&got,address,8);_exit(compare==0&&got==9?0:96);
}
int main(void){puts("{\"cases\":[");for(unsigned off=0;off<16;off++){
 pid_t pid=fork();if(pid<0)return 2;if(pid==0)run(off);int status;if(waitpid(pid,&status,0)!=pid)return 3;
 printf("%s{\"sp_offset\":%u,\"exit\":%d,\"signal\":%d}",off?",":"",off,WIFEXITED(status)?WEXITSTATUS(status):-1,WIFSIGNALED(status)?WTERMSIG(status):0);
 }puts("]}");return 0;}
