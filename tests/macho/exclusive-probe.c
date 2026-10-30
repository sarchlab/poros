#include <stdint.h>
#include <stdio.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <unistd.h>
#define ARGS [l]"=&r"(loaded),[s]"=&r"(status) : [p]"r"(p),[q]"r"(q),[v]"r"(value),[t]"r"(ordinary) : "memory"
#define L "ldxr %w[l],[%x[p]]\n\t"
#define S "stxr %w[s],%w[v],[%x[p]]"
static int run(unsigned mode){
 uint32_t *p=mmap(0,32768,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(p==MAP_FAILED)return 99;
 uint32_t *q=p+(mode==3?1:mode==4?16:mode==5?32:mode==6?1:4096);
 *p=7;*q=0;if(mode==9||mode==10){if(mprotect(p,16384,PROT_READ))return 98;}if(mode==11){if(mprotect(q,16384,PROT_READ))return 97;}
 uint32_t success=0,failure=0,first_loaded=0,final=0;unsigned reps=mode<9?100:1;
 for(unsigned i=0;i<reps;i++){
 if(mode<9){*p=7;*q=0;}uint32_t loaded=0,status=99,value=9,ordinary=11;
 switch(mode){
 case 0:__asm__ volatile(L S : ARGS);break;
 case 1:__asm__ volatile(L "clrex\n\t" S : ARGS);break;
 case 2:__asm__ volatile(L "str %w[t],[%x[p]]\n\t" S : ARGS);break;
 case 3:case 4:case 5:__asm__ volatile(L "str %w[t],[%x[q]]\n\t" S : ARGS);break;
 case 6:__asm__ volatile(L "ldxr %w[l],[%x[q]]\n\t" S : ARGS);break;
 case 7:__asm__ volatile(L "stxr %w[s],%x[v],[%x[p]]" : ARGS);break;
 case 8:case 9:__asm__ volatile("clrex\n\tmov %w[l],wzr\n\t" S : ARGS);break;
 case 10:__asm__ volatile(L S : ARGS);break;
 case 11:__asm__ volatile(L "stxr %w[s],%w[v],[%x[q]]" : ARGS);break;
 }
 if(status==0)success++;else if(status==1)failure++;else return 96;
 first_loaded=loaded;final=*p;
 }
 printf(",\"successes\":%u,\"failures\":%u,\"loaded\":%u,\"final_original\":%u",success,failure,first_loaded,final);fflush(stdout);return 0;
}
int main(void){const char*names[]={"normal","clrex","same-address-store","adjacent-store","store-plus64","store-plus128","second-load-other-address","width-mismatch","no-monitor","no-monitor-readonly-target","loaded-readonly-target","monitor-other-readonly-target"};puts("{\"cases\":[");
for(unsigned m=0;m<12;m++){printf("%s{\"name\":\"%s\"",m?",":"",names[m]);fflush(stdout);pid_t p=fork();if(p<0)return 2;if(p==0)_exit(run(m));int s;if(waitpid(p,&s,0)!=p)return 3;printf(",\"exit\":%d,\"signal\":%d}",WIFEXITED(s)?WEXITSTATUS(s):-1,WIFSIGNALED(s)?WTERMSIG(s):0);}puts("]}");return 0;}
