#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result allocate(mach_port_t task,mach_vm_address_t*slot,mach_vm_size_t size,uint32_t flags){register uint64_t a __asm__("x0")=task,b __asm__("x1")=(uintptr_t)slot,c __asm__("x2")=size,d __asm__("x3")=flags,n __asm__("x16")=(uint64_t)(int64_t)-10;uint64_t f,prior=0xa0000000;__asm__ volatile("msr nzcv,%[prior]\nsvc #0x80\nmrs %[f],nzcv":[f]"=&r"(f),"+r"(a),"+r"(b):"r"(c),"r"(d),"r"(n),[prior]"r"(prior):"memory","cc");return(struct result){a,b,f};}
static int run(unsigned which){const char*names[]={"fixed_36","anywhere_tag60_36","invalid_task","dead_task","null_address_pointer","unmapped_address_pointer","zero_size","fixed_unaligned","readonly_address_pointer","fixed_collision","overflow_size"};
 unsigned char*arena=mmap(NULL,2<<20,PROT_NONE,MAP_PRIVATE|MAP_ANON,-1,0);mach_vm_address_t*slot=mmap(NULL,16384,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(arena==MAP_FAILED||slot==MAP_FAILED)return 2;
 mach_vm_address_t gap=(mach_vm_address_t)arena+65536;if(munmap((void*)gap,131072))return 3;
 *slot=gap;mach_vm_address_t*arg=slot;mach_vm_size_t size=36;uint32_t flags=0;mach_port_t task=mach_task_self();
 switch(which){case 1:flags=0x3c000001;*slot=0;break;case 2:task=0;break;case 3:task=~(mach_port_t)0;break;case 4:arg=NULL;break;case 5:arg=(void*)1;break;case 6:size=0;break;case 7:*slot=gap+1;break;case 8:if(mprotect(slot,16384,PROT_READ))return 4;break;case 9:*slot=(mach_vm_address_t)arena;break;case 10:size=UINT64_MAX;break;}
 uint64_t before=*slot;struct result r=allocate(task,arg,size,flags);unsigned char byte=255;mach_vm_size_t got=0;int gapread=mach_vm_read_overwrite(mach_task_self(),gap,1,(mach_vm_address_t)&byte,&got)==KERN_SUCCESS&&got==1;
 int zero=0,prot=-1,maxprot=-1;mach_vm_size_t region_size=0;
 if(r.x0==0&&size&&*slot){mach_vm_address_t address=*slot;vm_region_basic_info_data_64_t info;mach_msg_type_number_t count=VM_REGION_BASIC_INFO_COUNT_64;mach_port_t object=0;if(mach_vm_region(mach_task_self(),&address,&region_size,VM_REGION_BASIC_INFO_64,(vm_region_info_t)&info,&count,&object)!=KERN_SUCCESS)return 5;prot=info.protection;maxprot=info.max_protection;if(object)mach_port_deallocate(mach_task_self(),object);unsigned char bytes[36];if(!mach_vm_read_overwrite(mach_task_self(),*slot,36,(mach_vm_address_t)bytes,&got)&&got==36){zero=1;for(unsigned j=0;j<36;j++)if(bytes[j])zero=0;}}
 printf("%s{\"case\":\"%s\",\"return\":%llu,\"x1_preserved\":%s,\"nzcv\":\"%08llx\",\"address_unchanged\":%s,\"address_zero\":%s,\"address_equals_gap\":%s,\"gap_readable\":%s,\"gap_zero\":%s,\"allocation_zero\":%s,\"region_size\":%llu,\"protection\":%d,\"max_protection\":%d}",which?",\n":"",names[which],(unsigned long long)r.x0,r.x1==(uintptr_t)arg?"true":"false",(unsigned long long)r.nzcv,*slot==before?"true":"false",*slot==0?"true":"false",*slot==gap?"true":"false",gapread?"true":"false",gapread&&byte==0?"true":"false",zero?"true":"false",(unsigned long long)region_size,prot,maxprot);fflush(stdout);return 0;}
int main(void){puts("[");fflush(stdout);for(unsigned i=0;i<11;i++){pid_t p=fork();if(p<0)return 6;if(p==0)_exit(run(i));int s;if(waitpid(p,&s,0)!=p||!WIFEXITED(s)||WEXITSTATUS(s))return 7;}puts("\n]");return 0;}
