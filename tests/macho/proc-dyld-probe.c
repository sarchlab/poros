#include <mach/mach.h>
#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
struct outcome{uint64_t x0,x1,carry;};
static struct outcome call(uint64_t pid,uint64_t address,uint64_t size,uint64_t flavor){register uint64_t r0 __asm__("x0")=15,r1 __asm__("x1")=pid,r2 __asm__("x2")=flavor,r3 __asm__("x3")=123,r4 __asm__("x4")=address,r5 __asm__("x5")=size,r16 __asm__("x16")=336;uint64_t carry;__asm__ volatile("svc #0x80\ncset %w2,cs":"+r"(r0),"+r"(r1),"=&r"(carry):"r"(r2),"r"(r3),"r"(r4),"r"(r5),"r"(r16):"cc","memory");return(struct outcome){r0,r1,carry};}
static int info(task_dyld_info_data_t*d){mach_msg_type_number_t count=TASK_DYLD_INFO_COUNT;return task_info(mach_task_self(),TASK_DYLD_INFO,(task_info_t)d,&count)==KERN_SUCCESS;}
int main(void){task_dyld_info_data_t before,after;if(!info(&before))return 2;uint64_t pid=(uint64_t)getpid();printf("{\"startup_already_completed\":true,\"cases\":[");
for(unsigned i=0;i<8;i++){uint64_t target=pid,address=before.all_image_info_addr,size=before.all_image_info_size,flavor=0;switch(i){case 1:address=0;break;case 2:target=pid+1;break;case 3:address=1;size=0;break;case 4:address=UINT64_MAX;size=1;break;case 5:size=0;break;case 6:size=UINT32_MAX;break;case 7:flavor=99;break;}
struct outcome o=call(target,address,size,flavor);if(!info(&after))return 3;unsigned unchanged=before.all_image_info_addr==after.all_image_info_addr&&before.all_image_info_size==after.all_image_info_size;
if(i)putchar(',');printf("{\"case\":%u,\"x0\":%llu,\"x1\":%llu,\"carry\":%llu,\"metadata_unchanged\":%s}",i,(unsigned long long)o.x0,(unsigned long long)o.x1,(unsigned long long)o.carry,unchanged?"true":"false");if(!unchanged)return 4;
}printf("]}\n");return 0;}
