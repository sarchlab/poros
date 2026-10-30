#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
typedef kern_return_t (*maptrap)(mach_port_name_t,mach_vm_address_t*,mach_vm_size_t,mach_vm_offset_t,int,vm_prot_t);
static int run(unsigned which,maptrap f){
 const char*names[]={"invalid-task","null-address-pointer","bad-address-pointer","zero-size","one-byte-size","unaligned-fixed-hint","invalid-protection","noncontiguous-mask","maximum-mask","readonly-address-pointer","noncontiguous-large-anywhere-mask"};
 unsigned char *arena=mmap(0,2<<20,PROT_NONE,MAP_PRIVATE|MAP_ANON,-1,0);
 mach_vm_address_t *slot=mmap(0,16384,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);
 if(arena==MAP_FAILED||slot==MAP_FAILED)return 99;
 mach_vm_address_t gap=(mach_vm_address_t)arena+65536;
 if(munmap((void*)gap,131072))return 98;
 *slot=gap;mach_vm_address_t *arg=slot;mach_vm_size_t size=16384;mach_vm_offset_t mask=0;int flags=0;vm_prot_t prot=VM_PROT_READ|VM_PROT_WRITE;mach_port_t target=mach_task_self();
 switch(which){case 0:target=0;break;case 1:arg=0;break;case 2:arg=(void*)1;break;case 3:size=0;break;case 4:size=1;break;case 5:*slot=gap+1;break;case 6:prot=8;break;case 7:mask=5;break;case 8:mask=~(uint64_t)0;break;case 9:if(mprotect(slot,16384,PROT_READ))return 97;break;case 10:mask=0x14000;flags=VM_FLAGS_ANYWHERE;*slot=gap|0x4000;break;}
 kern_return_t kr=f(target,arg,size,mask,flags,prot);
 unsigned char byte=255;mach_vm_size_t got=0;
 int mapped=mach_vm_read_overwrite(mach_task_self(),gap,1,(mach_vm_address_t)&byte,&got)==KERN_SUCCESS&&got==1;
 printf("%s{\"name\":\"%s\",\"return\":%u,\"gap_readable\":%s,\"zero_fill\":%s,\"address_equals_gap\":%s,\"returned_mask_match\":%s}",which?",":"",names[which],(unsigned)kr,mapped?"true":"false",mapped&&byte==0?"true":"false",*slot==gap?"true":"false",(*slot&mask)==0?"true":"false");fflush(stdout);return 0;
}
int main(void){maptrap f=(maptrap)dlsym(RTLD_DEFAULT,"_kernelrpc_mach_vm_map_trap");if(!f){fputs("trap symbol unavailable\n",stderr);return 2;}puts("{\"cases\":[");fflush(stdout);
 for(unsigned i=0;i<11;i++){pid_t p=fork();if(p<0)return 3;if(p==0)_exit(run(i,f));int s;if(waitpid(p,&s,0)!=p||!WIFEXITED(s)||WEXITSTATUS(s))return 4;}puts("]}");return 0;}
