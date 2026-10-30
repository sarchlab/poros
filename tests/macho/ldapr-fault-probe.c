#include <stdint.h>
#include <stdio.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <unistd.h>
static void probe(unsigned width,unsigned zr,unsigned mode){
 unsigned char *p=mmap(0,32768,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(p==MAP_FAILED)_exit(99);
 *(uint64_t*)p=9;void* addr=p;
 if(mode==1)mprotect(p,16384,PROT_READ);
 if(mode==2)mprotect(p,16384,PROT_NONE);
 if(mode==3){munmap(p,16384);}
 if(mode==4)addr=0;
 if(mode==5){mprotect(p+16384,16384,PROT_NONE);addr=p+16384-width;*(uint64_t*)(p+16384-8)=0x0909090909090909ULL;}
 uint64_t v=0;
 if(width==8){if(zr)__asm__ volatile("ldapr xzr,[%x[p]]"::[p]"r"(addr):"memory");else __asm__ volatile("ldapr %x[v],[%x[p]]":[v]"=r"(v):[p]"r"(addr):"memory");}
 else {if(zr)__asm__ volatile("ldapr wzr,[%x[p]]"::[p]"r"(addr):"memory");else __asm__ volatile("ldapr %w[v],[%x[p]]":[v]"=r"(v):[p]"r"(addr):"memory");}
 if(!zr && mode<2 && v!=9)_exit(98);_exit(0);
}
int main(void){unsigned ix=0;puts("{\"cases\":[");for(unsigned w=4;w<=8;w+=4)for(unsigned zr=0;zr<2;zr++)for(unsigned mode=0;mode<6;mode++){
 pid_t pid=fork();if(pid<0)return 2;if(pid==0)probe(w,zr,mode);int status;if(waitpid(pid,&status,0)!=pid)return 3;
 printf("%s{\"width\":%u,\"zr\":%s,\"mode\":%u,\"exit\":%d,\"signal\":%d}",ix++?",":"",w,zr?"true":"false",mode,WIFEXITED(status)?WEXITSTATUS(status):-1,WIFSIGNALED(status)?WTERMSIG(status):0);
 }puts("]}");return 0;}
