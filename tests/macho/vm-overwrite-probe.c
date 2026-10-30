#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
typedef kern_return_t (*maptrap)(mach_port_name_t,mach_vm_address_t*,mach_vm_size_t,mach_vm_offset_t,int,vm_prot_t);
static int run(unsigned which,maptrap f){
 const size_t page=16384;
 unsigned char *arena=mmap(0,8*page,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);
 mach_vm_address_t *slot=mmap(0,page,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);
 if(arena==MAP_FAILED||slot==MAP_FAILED)return 99;
 for(unsigned i=0;i<8;i++)memset(arena+i*page,0x40+i,page);
 mach_vm_address_t base=(mach_vm_address_t)arena;
 *slot=base+2*page;mach_vm_address_t *arg=slot;mach_vm_size_t size=2*page;mach_vm_offset_t mask=0;int flags=0x07004000;vm_prot_t prot=3;mach_port_t target=mach_task_self();
 switch(which){
 case 0:break;
 case 1:if(munmap(arena+2*page,2*page))return 98;break;
 case 2:if(munmap(arena+3*page,page))return 98;break;
 case 3:*slot+=1;size=1;break;
 case 4:prot=0;break;
 case 5:prot=1;break;
 case 6:prot=8;break;
 case 7:size=0;break;
 case 8:target=0;break;
 case 9:arg=0;break;
 case 10:if(mprotect(slot,page,PROT_READ))return 97;break;
 case 11:flags=0x07004001;break;
 case 12:mask=~(uint64_t)0;break;
 case 13:arg=(void*)(arena+2*page);*arg=base+2*page;break;
 case 14:if(mprotect(arena+2*page,2*page,PROT_NONE))return 97;break;
 case 15:if(mprotect(arena+2*page,page,PROT_READ))return 97;break;
 case 16:flags=0x07000000;break;
 case 17:*slot+=page-1;size=page+1;break;
 case 18:mask=5;break;
 case 19:if(munmap(arena+2*page,page))return 98;break;
 default:return 96;
 }
 kern_return_t kr=f(target,arg,size,mask,flags,prot);
 printf("%s{\"case\":%u,\"return\":%u,\"slot_delta\":%lld,\"page_size\":%zu,\"pages\":[",which?",":"",which,(unsigned)kr,(long long)(*slot-base),page);
 for(unsigned i=0;i<8;i++){
  unsigned char byte=255;mach_vm_size_t got=0;
  int readable=mach_vm_read_overwrite(mach_task_self(),base+i*page,1,(mach_vm_address_t)&byte,&got)==KERN_SUCCESS&&got==1;
  mach_vm_address_t addr=base+i*page;mach_vm_size_t length=0;vm_region_basic_info_data_64_t info={0};mach_msg_type_number_t count=VM_REGION_BASIC_INFO_COUNT_64;mach_port_t object=MACH_PORT_NULL;
  kern_return_t region=mach_vm_region(mach_task_self(),&addr,&length,VM_REGION_BASIC_INFO_64,(vm_region_info_t)&info,&count,&object);
  int mapped=region==0&&addr<=base+i*page&&base+i*page<addr+length;
  if(object)mach_port_deallocate(mach_task_self(),object);
  printf("%s{\"readable\":%s,\"byte\":%u,\"mapped\":%s,\"protection\":%u,\"max_protection\":%u}",i?",":"",readable?"true":"false",byte,mapped?"true":"false",mapped?(unsigned)info.protection:0,mapped?(unsigned)info.max_protection:0);
 }
 if(which==13){uint64_t copied=0;mach_vm_size_t got=0;mach_vm_read_overwrite(mach_task_self(),base+2*page,8,(mach_vm_address_t)&copied,&got);printf("],\"in_mapping_slot_delta\":%lld}",(long long)(copied-base));}
 else printf("]}");
 fflush(stdout);return 0;
}
int main(void){if(getpagesize()!=16384)return 1;maptrap f=(maptrap)dlsym(RTLD_DEFAULT,"_kernelrpc_mach_vm_map_trap");if(!f)return 2;puts("{\"cases\":[");fflush(stdout);
 for(unsigned i=0;i<20;i++){pid_t p=fork();if(p<0)return 3;if(p==0)_exit(run(i,f));int s;if(waitpid(p,&s,0)!=p||!WIFEXITED(s)||WEXITSTATUS(s))return 4;}puts("]}");return 0;}
