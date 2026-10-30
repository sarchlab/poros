#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <mach/mach_time.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
typedef kern_return_t (*timebasetrap)(mach_timebase_info_t);
static int run(unsigned which,timebasetrap f){
 unsigned char *area=mmap(0,3*16384,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(area==MAP_FAILED)return 9;memset(area,0xa5,3*16384);
 unsigned char *out=area+16384;
 switch(which){case 0:break;case 1:out+=1;break;case 2:out=0;break;case 3:out=(void*)1;break;case 4:if(mprotect(out,16384,PROT_READ))return 8;break;case 5:if(mprotect(out,16384,PROT_NONE))return 8;break;case 6:out-=4;if(mprotect(area+16384,16384,PROT_READ))return 8;break;case 7:out-=4;if(munmap(area+16384,16384))return 8;break;case 8:out-=8;if(mprotect(area+16384,16384,PROT_NONE))return 8;break;case 9:out-=7;if(mprotect(area+16384,16384,PROT_READ))return 8;break;}
 kern_return_t kr=f((void*)out);
 unsigned char bytes[8];memset(bytes,0xa5,8);mach_vm_size_t n=0;
 if(which!=2&&which!=3&&which!=5){for(unsigned i=0;i<8;i++){mach_vm_read_overwrite(mach_task_self(),(mach_vm_address_t)(out+i),1,(mach_vm_address_t)(bytes+i),&n);}}
 printf("%s{\"case\":%u,\"return\":%u,\"bytes\":\"",which?",":"",which,(unsigned)kr);for(unsigned i=0;i<8;i++)printf("%02x",bytes[i]);printf("\"}");fflush(stdout);return 0;
}
int main(void){if(getpagesize()!=16384)return 1;timebasetrap f=(timebasetrap)dlsym(RTLD_DEFAULT,"mach_timebase_info_trap");if(!f)return 2;mach_timebase_info_data_t t;mach_timebase_info(&t);printf("{\"numer\":%u,\"denom\":%u,\"cases\":[",t.numer,t.denom);fflush(stdout);for(unsigned i=0;i<10;i++){pid_t p=fork();if(p<0)return 3;if(p==0)_exit(run(i,f));int s;if(waitpid(p,&s,0)!=p||!WIFEXITED(s)||WEXITSTATUS(s))return 4;}puts("]}");return 0;}
