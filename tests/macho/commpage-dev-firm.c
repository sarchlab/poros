#include <stdint.h>
#include <stdio.h>
#include <mach/mach.h>
#include <mach/mach_vm.h>
int main(void) {
 const uint64_t addresses[]={0xfffffc084ULL,0xfffff4084ULL};
 uint32_t values[2];
 for(unsigned i=0;i<2;i++) {
  mach_vm_size_t got=0;
  kern_return_t kr=mach_vm_read_overwrite(mach_task_self(),addresses[i],sizeof(values[i]),(mach_vm_address_t)&values[i],&got);
  if(kr!=KERN_SUCCESS||got!=sizeof(values[i]))return 2;
 }
 printf("{\"legacy_dev_firm\":%u,\"read_only_dev_firm\":%u}\n",values[0],values[1]);
 return 0;
}
