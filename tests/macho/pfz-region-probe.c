#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <stdio.h>
#include <stdint.h>
int main(void){mach_vm_address_t wanted=0xfffe78000ull,address=wanted;mach_vm_size_t size=0;natural_t depth=0;vm_region_submap_info_data_64_t info;for(unsigned i=0;i<32;i++){mach_msg_type_number_t count=VM_REGION_SUBMAP_INFO_COUNT_64;kern_return_t kr=mach_vm_region_recurse(mach_task_self(),&address,&size,&depth,(vm_region_recurse_info_t)&info,&count);if(kr)return 2;if(address>wanted||size<=wanted-address)return 3;if(info.is_submap){depth++;continue;}printf("{\"queried_pfz\":\"%llx\",\"start\":\"%llx\",\"size\":%llu,\"protection\":%u,\"max_protection\":%u,\"depth\":%u}\n",(unsigned long long)wanted,(unsigned long long)address,(unsigned long long)size,info.protection,info.max_protection,depth);return 0;}return 4;}
