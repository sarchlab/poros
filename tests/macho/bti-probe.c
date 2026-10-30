#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/wait.h>
#include <sys/resource.h>
#include <sys/sysctl.h>
#include <mach-o/dyld.h>
extern const void* _dyld_get_shared_cache_range(size_t*);
extern uint64_t bti_plain(void),bti_c(void),bti_j(void),bti_jc(void);
__asm__(".text\n.p2align 2\n_bti_plain:\nmov x0,#99\nret\n_bti_c:\n.inst 0xd503245f\nmov x0,#99\nret\n_bti_j:\n.inst 0xd503249f\nmov x0,#99\nret\n_bti_jc:\n.inst 0xd50324df\nmov x0,#99\nret\n");
static uint64_t branch(void*target,int jump){uint64_t result; if(jump)__asm__ volatile("adr x30,1f\nmov x12,%1\nbr x12\n1:mov %0,x0":"=r"(result):"r"(target):"x0","x12","x30","cc","memory");else __asm__ volatile("mov x12,%1\nblr x12\nmov %0,x0":"=r"(result):"r"(target):"x0","x12","x30","cc","memory");return result;}
static int stackprobe(void*target){uint64_t result;__asm__ volatile("mov x9,#0\nmov x0,#99\nblr %1\nmov %0,x0":"=r"(result):"r"(target):"x0","x9","x10","x16","x17","x30","cc","memory");return result==99;}
int main(void){int feature=-1;size_t n=sizeof(feature);if(sysctlbyname("hw.optional.arm.FEAT_BTI",&feature,&n,NULL,0))return 2;size_t cacheSize=0;const uint8_t*cache=_dyld_get_shared_cache_range(&cacheSize);uint64_t offset=0x528cc0;if(!cache||offset+8>=cacheSize||*(const uint32_t*)(cache+offset)!=0xd503245f||*(const uint32_t*)(cache+offset+4)!=0x90000010)return 3;
 void*targets[]={(void*)bti_plain,(void*)bti_c,(void*)bti_j,(void*)bti_jc};const char*names[]={"plain","bti_c","bti_j","bti_jc"};printf("{\"feat_bti\":%d,\"cases\":[\n",feature);fflush(stdout);
 for(int i=0;i<10;i++){pid_t p=fork();if(p<0)return 4;if(p==0){struct rlimit limit={0,0};setrlimit(RLIMIT_CORE,&limit);int ok=i<8?branch(targets[i%4],i/4)==99:stackprobe((void*)(cache+offset+(i==9?4:0)));_exit(ok?0:1);}int s;if(waitpid(p,&s,0)!=p)return 5;printf("%s{\"target\":\"%s\",\"branch\":\"%s\",\"returned_99\":%s,\"signal\":%d}",i?",\n":"",i<8?names[i%4]:i==8?"dyld_chkstk_bti":"dyld_chkstk_skip_bti",i<4||i>=8?"blr":"br_x12",WIFEXITED(s)&&WEXITSTATUS(s)==0?"true":"false",WIFSIGNALED(s)?WTERMSIG(s):0);fflush(stdout);}
 puts("\n]}");return 0;}
