#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <mach-o/dyld_images.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <sys/stat.h>
static int read_self(uint64_t address,void *dst,size_t size){mach_vm_size_t got=0;return mach_vm_read_overwrite(mach_task_self(),address,size,(mach_vm_address_t)dst,&got)==KERN_SUCCESS&&got==size;}
static uint64_t u64(const void*p){uint64_t v;memcpy(&v,p,8);return v;}
int main(void){task_dyld_info_data_t info;mach_msg_type_number_t count=TASK_DYLD_INFO_COUNT;if(task_info(mach_task_self(),TASK_DYLD_INFO,(task_info_t)&info,&count)!=KERN_SUCCESS)return 2;
 const struct dyld_all_image_infos*i=(const void*)(uintptr_t)info.all_image_info_addr;unsigned char descriptor[16],identity[16],buffer[1024];uint64_t base=i->sharedCacheBaseAddress;
 if(!read_self(base+496,descriptor,16))return 3;uint64_t off=u64(descriptor),size=u64(descriptor+8);if(size<32||size>16384||base>UINT64_MAX-off||!read_self(base+off+16,identity,16))return 4;
 memset(buffer,0xa5,sizeof(buffer));register uint64_t a __asm__("x0")=(uintptr_t)buffer,b __asm__("x1")=sizeof(buffer),c __asm__("x2")=(uintptr_t)identity,d __asm__("x3")=u64(identity+8),n __asm__("x16")=427;uint64_t flags;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(d),"r"(n):"memory","cc");
 if(flags&(1ull<<29)||a==0||a>sizeof(buffer)||buffer[a-1]!=0||strlen((char*)buffer)+1!=a)return 5;
 for(size_t j=a;j<sizeof(buffer);j++)if(buffer[j]!=0xa5)return 6;
 struct stat st;if(stat((char*)buffer,&st)||(uint32_t)st.st_dev!=(uint32_t)u64(identity)||st.st_ino!=u64(identity+8))return 7;
 printf("{\"cache_uuid\":\"");for(unsigned j=0;j<16;j++)printf("%02x",i->sharedCacheUUID[j]);printf("\",\"file_id_hex\":\"");for(unsigned j=0;j<16;j++)printf("%02x",identity[j]);printf("\",\"x0\":%llu,\"x1\":%llu,\"carry\":false,\"tail_unchanged\":true,\"stat_identity_matches\":true,\"path_hex\":\"",a,b);for(unsigned j=0;j<a;j++)printf("%02x",buffer[j]);puts("\"}");return 0;}
