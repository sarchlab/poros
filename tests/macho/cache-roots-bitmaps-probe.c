#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <mach-o/dyld_images.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
static int read_self(uint64_t address,void *dst,size_t size){mach_vm_size_t got=0;return mach_vm_read_overwrite(mach_task_self(),address,size,(mach_vm_address_t)dst,&got)==KERN_SUCCESS&&got==size;}
static uint64_t u64(const void*p){uint64_t v;memcpy(&v,p,8);return v;}
static uint32_t u32(const void*p){uint32_t v;memcpy(&v,p,4);return v;}
int main(void){task_dyld_info_data_t d;mach_msg_type_number_t n=TASK_DYLD_INFO_COUNT;if(task_info(mach_task_self(),TASK_DYLD_INFO,(task_info_t)&d,&n)!=KERN_SUCCESS)return 2;
 const struct dyld_all_image_infos*a=(const void*)(uintptr_t)d.all_image_info_addr;
 uint64_t base=a->sharedCacheBaseAddress;unsigned char range[16],magic[16],flags;if(base>UINT64_MAX-512||!read_self(base+496,range,16))return 3;
 uint64_t off=u64(range),size=u64(range+8);if(size<112||size>16384||base>UINT64_MAX-off||!read_self(base+off,magic,16)||memcmp(magic,"dyld_data    v4",15))return 4;
 if(!read_self(base+off+0x54,&flags,1))return 5;
 unsigned char number[4],desc[16],first[8192],second[8192];
 if(!read_self(base+off+0x50,number,4)||!read_self(base+off+0x60,desc,16))return 6;
 uint32_t count=u32(number),firstOff=u32(desc),firstSize=u32(desc+4),secondOff=u32(desc+8),secondSize=u32(desc+12);
 if(!(flags&1)||count==0||count>65536||firstSize!=(count+7)/8||secondSize!=firstSize||firstOff<112||secondOff<112||(uint64_t)firstOff+firstSize>size||(uint64_t)secondOff+secondSize>size)return 7;
 if(!read_self(base+off+firstOff,first,firstSize)||!read_self(base+off+secondOff,second,secondSize))return 8;
 printf("{\"cache_uuid\":\"");for(unsigned i=0;i<16;i++)printf("%02x",a->sharedCacheUUID[i]);printf("\",\"dynamic_offset\":%llu,\"dynamic_size\":%llu,\"version\":4,\"roots_flags\":%u,\"image_count\":%u,\"image_roots_offset\":%u,\"image_roots_size\":%u,\"cryptex_roots_offset\":%u,\"cryptex_roots_size\":%u,\"image_roots_hex\":\"",off,size,flags,count,firstOff,firstSize,secondOff,secondSize);
 for(unsigned i=0;i<firstSize;i++)printf("%02x",first[i]);printf("\",\"cryptex_roots_hex\":\"");for(unsigned i=0;i<secondSize;i++)printf("%02x",second[i]);puts("\"}");return 0;}
