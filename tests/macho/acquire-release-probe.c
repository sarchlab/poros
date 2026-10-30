#include <stdint.h>
#include <stdio.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <unistd.h>
#include <string.h>
static void run(unsigned off,unsigned width,unsigned load,unsigned sp){
 unsigned char *p=mmap(0,16384,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(p==MAP_FAILED)_exit(99);
 void *address=p+8192+off;uint64_t v=15,initial=9,got=0;memcpy(address,&initial,width);
 if(width==8){
 if(load){if(sp)__asm__ volatile("mov x15,sp\n\tmov sp,%x[p]\n\tldar %x[v],[sp]\n\tmov sp,x15" : [v]"=&r"(v) : [p]"r"(address) : "x15","memory");else __asm__ volatile("ldar %x[v],[%x[p]]" : [v]"=r"(v) : [p]"r"(address) : "memory");}
 else{if(sp)__asm__ volatile("mov x15,sp\n\tmov sp,%x[p]\n\tstlr %x[v],[sp]\n\tmov sp,x15" :: [v]"r"(v),[p]"r"(address) : "x15","memory");else __asm__ volatile("stlr %x[v],[%x[p]]" :: [v]"r"(v),[p]"r"(address) : "memory");}
 }else{
 if(load){if(sp)__asm__ volatile("mov x15,sp\n\tmov sp,%x[p]\n\tldar %w[v],[sp]\n\tmov sp,x15" : [v]"=&r"(v) : [p]"r"(address) : "x15","memory");else __asm__ volatile("ldar %w[v],[%x[p]]" : [v]"=r"(v) : [p]"r"(address) : "memory");}
 else{if(sp)__asm__ volatile("mov x15,sp\n\tmov sp,%x[p]\n\tstlr %w[v],[sp]\n\tmov sp,x15" :: [v]"r"(v),[p]"r"(address) : "x15","memory");else __asm__ volatile("stlr %w[v],[%x[p]]" :: [v]"r"(v),[p]"r"(address) : "memory");}
 }
 memcpy(&got,address,width);_exit(load?(v==9&&got==9?0:96):(got==15?0:96));
}
int main(void){puts("{\"cases\":[");unsigned index=0;for(unsigned width=4;width<=8;width+=4)for(unsigned load=0;load<2;load++)for(unsigned sp=0;sp<2;sp++)for(unsigned off=0;off<(sp?16:32);off++){
 pid_t pid=fork();if(pid<0)return 2;if(pid==0)run(off,width,load,sp);int status;if(waitpid(pid,&status,0)!=pid)return 3;
 printf("%s{\"width\":%u,\"load\":%s,\"sp\":%s,\"offset\":%u,\"exit\":%d,\"signal\":%d}",index++?",":"",width,load?"true":"false",sp?"true":"false",off,WIFEXITED(status)?WEXITSTATUS(status):-1,WIFSIGNALED(status)?WTERMSIG(status):0);
 }puts("]}");return 0;}
