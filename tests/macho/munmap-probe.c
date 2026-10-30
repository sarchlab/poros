#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>
typedef struct { uint64_t x0,x1,nzcv; } raw_result;
static raw_result raw_munmap(uint64_t address,uint64_t size){register uint64_t x0 __asm__("x0")=address;register uint64_t x1 __asm__("x1")=size;register uint64_t x16 __asm__("x16")=73;uint64_t flags;__asm__ volatile("svc #0x80\n mrs %2,nzcv":"+r"(x0),"+r"(x1),"=r"(flags):"r"(x16):"cc","memory");return(raw_result){x0,x1,flags};}
static unsigned readable(mach_vm_address_t base,mach_vm_size_t page){unsigned mask=0;for(unsigned i=0;i<5;i++){unsigned char b=0;mach_vm_size_t got=0;if(mach_vm_read_overwrite(mach_task_self(),base+i*page,1,(mach_vm_address_t)&b,&got)==KERN_SUCCESS&&got==1)mask|=1u<<i;}return mask;}
static unsigned mapped(mach_vm_address_t base,mach_vm_size_t page){unsigned mask=0;for(unsigned i=0;i<5;i++){mach_vm_address_t wanted=base+i*page,address=wanted;mach_vm_size_t size=0;vm_region_basic_info_data_64_t info;mach_msg_type_number_t count=VM_REGION_BASIC_INFO_COUNT_64;mach_port_t object=0;kern_return_t kr=mach_vm_region(mach_task_self(),&address,&size,VM_REGION_BASIC_INFO_64,(vm_region_info_t)&info,&count,&object);if(kr==KERN_SUCCESS&&address<=wanted&&wanted-address<size)mask|=1u<<i;if(object)mach_port_deallocate(mach_task_self(),object);}return mask;}
static int probe(unsigned which){mach_vm_address_t base=0;mach_vm_size_t page=(mach_vm_size_t)getpagesize();if(mach_vm_allocate(mach_task_self(),&base,7*page,VM_FLAGS_ANYWHERE)!=KERN_SUCCESS)return 3;memset((void*)base,7,(size_t)(7*page));
 mach_port_name_t task=mach_task_self();mach_vm_address_t addr=base;mach_vm_size_t size=5*page;
 switch(which){
 case 0:break;
 case 1:addr=base+page;size=0;break;
 case 2:addr=base+page;size=1;break;
 case 3:addr=base+page+1;size=1;break;
 case 4:addr=base+page+1;size=page;break;
 case 5:if(mach_vm_deallocate(task,base+2*page,page)!=KERN_SUCCESS)return 4;addr=base+2*page;size=page;break;
 case 6:if(mach_vm_deallocate(task,base+2*page,page)!=KERN_SUCCESS)return 4;break;
 case 7:addr=0;size=0;break;
 case 8:addr=0;size=1;break;
 case 9:addr=UINT64_MAX;size=1;break;
 case 10:addr=1ULL<<47;size=page;break;
 case 11:addr=UINT64_MAX-page+1;size=2*page;break;
 case 12:addr=base+page;size=page;if(mach_vm_protect(task,addr,size,0,VM_PROT_NONE)!=KERN_SUCCESS)return 5;break;
 case 13:addr=base+page;size=3*page;if(mach_vm_protect(task,base+2*page,page,0,VM_PROT_READ)!=KERN_SUCCESS)return 5;break;
 }
 unsigned mapped_before=mapped(base,page),before=readable(base,page);raw_result result=raw_munmap(addr,size);unsigned after=readable(base,page),mapped_after=mapped(base,page);
 printf("{\"case\":%u,\"x0\":%llu,\"x1\":%llu,\"carry\":%llu,\"before_mask\":%u,\"after_mask\":%u,\"mapped_before\":%u,\"mapped_after\":%u,\"page_size\":%llu}\n",which,(unsigned long long)result.x0,(unsigned long long)result.x1,(unsigned long long)((result.nzcv>>29)&1),before,after,mapped_before,mapped_after,(unsigned long long)page);fflush(stdout);return 0;
}
int main(void){unsigned failed=0;for(unsigned i=0;i<14;i++){pid_t p=fork();if(p<0)return 6;if(p==0)_exit(probe(i));int status=0;if(waitpid(p,&status,0)!=p)return 7;if(!WIFEXITED(status)||WEXITSTATUS(status)!=0){fprintf(stderr,"case %u status %d\n",i,status);failed++;}}return failed?1:0;}
