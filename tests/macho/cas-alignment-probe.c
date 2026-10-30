#include <stdint.h>
#include <stdio.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <unistd.h>
#include <string.h>
static void run(unsigned off,unsigned width) {
 unsigned char *p=mmap(0,16384,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);
 if(p==MAP_FAILED)_exit(99);
 uint64_t cmp=0,replacement=9,got=0;void *addr=p+off;
 if(width==8)__asm__ volatile("casal %x[c],%x[v],[%x[p]]" : [c]"+&r"(cmp) : [v]"r"(replacement),[p]"r"(addr) : "memory");
 else __asm__ volatile("casal %w[c],%w[v],[%x[p]]" : [c]"+&r"(cmp) : [v]"r"(replacement),[p]"r"(addr) : "memory");
 memcpy(&got,addr,width);
 _exit(cmp==0&&got==9?0:96);
}
int main(void){puts("{\"cases\":[");unsigned index=0;
 for(unsigned width=4;width<=8;width+=4)for(unsigned off=0;off<32;off++){
  pid_t pid=fork();if(pid<0)return 2;if(pid==0)run(off,width);
  int status;if(waitpid(pid,&status,0)!=pid)return 3;
  printf("%s{\"width\":%u,\"offset\":%u,\"exit\":%d,\"signal\":%d}",index++?",":"",width,off,WIFEXITED(status)?WEXITSTATUS(status):-1,WIFSIGNALED(status)?WTERMSIG(status):0);
 }puts("]}");return 0;
}
