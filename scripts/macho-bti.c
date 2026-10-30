// Read-only generic-arm64 landing-check evidence, using a UUID-matched symbol.
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <signal.h>
#include <sys/wait.h>
#include <sys/resource.h>
#include <sys/sysctl.h>
#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <mach-o/loader.h>
#include <mach-o/dyld_images.h>
extern const struct mach_header_64 _mh_execute_header;
extern uint64_t bti_plain(void),bti_c(void),bti_j(void),bti_jc(void);
__asm__(".text\n.p2align 2\n_bti_plain:\nmov x0,#99\nret\n_bti_c:\n.inst 0xd503245f\nmov x0,#99\nret\n_bti_j:\n.inst 0xd503249f\nmov x0,#99\nret\n_bti_jc:\n.inst 0xd50324df\nmov x0,#99\nret\n");
static int read_self(uint64_t address,void*data,size_t size){mach_vm_size_t got=0;return mach_vm_read_overwrite(mach_task_self(),address,size,(mach_vm_address_t)data,&got)==KERN_SUCCESS&&got==size;}
static uint64_t branch(void*target,int jump){uint64_t result;if(jump)__asm__ volatile("adr x30,1f\nmov x12,%1\nbr x12\n1:mov %0,x0":"=r"(result):"r"(target):"x0","x12","x30","cc","memory");else __asm__ volatile("mov x12,%1\nblr x12\nmov %0,x0":"=r"(result):"r"(target):"x0","x12","x30","cc","memory");return result;}
static int stackprobe(void*target){uint64_t result;__asm__ volatile("mov x9,#0\nmov x0,#99\nblr %1\nmov %0,x0":"=r"(result):"r"(target):"x0","x9","x10","x16","x17","x30","cc","memory");return result==99;}
static void hex(const uint8_t*b,size_t n){for(size_t i=0;i<n;i++)printf("%02x",b[i]);}
int main(int argc,char**argv){
 if(argc!=3||strlen(argv[2])!=32||_mh_execute_header.cpusubtype!=0)return 2;
 errno=0;char*end;uint64_t offset=strtoull(argv[1],&end,0);if(errno||*end||offset>(64ULL<<20)||offset%4)return 2;
 uint8_t expected[16];for(unsigned i=0;i<16;i++){char v[3]={argv[2][i*2],argv[2][i*2+1],0};char*e;errno=0;unsigned long b=strtoul(v,&e,16);if(errno||e!=v+2)return 2;expected[i]=(uint8_t)b;}
 task_dyld_info_data_t task;mach_msg_type_number_t count=TASK_DYLD_INFO_COUNT;
 if(task_info(mach_task_self(),TASK_DYLD_INFO,(task_info_t)&task,&count)!=KERN_SUCCESS)return 3;
 struct dyld_all_image_infos info;if(!read_self(task.all_image_info_addr,&info,sizeof(info)))return 3;
 uint64_t base=(uint64_t)(uintptr_t)info.dyldImageLoadAddress;
 struct mach_header_64 header;if(!read_self(base,&header,sizeof(header))||header.magic!=MH_MAGIC_64||header.cputype!=CPU_TYPE_ARM64||header.ncmds>4096||header.sizeofcmds>(1<<20))return 3;
 uint64_t cursor=sizeof(header);unsigned found=0;uint8_t uuid[16];
 for(unsigned i=0;i<header.ncmds;i++){
  struct load_command cmd;if(cursor>sizeof(header)+header.sizeofcmds||sizeof(header)+header.sizeofcmds-cursor<sizeof(cmd)||!read_self(base+cursor,&cmd,sizeof(cmd))||cmd.cmdsize<8||cmd.cmdsize>sizeof(header)+header.sizeofcmds-cursor)return 3;
  if(cmd.cmd==LC_UUID){if(cmd.cmdsize!=24||found++||!read_self(base+cursor+8,uuid,16))return 3;}
  cursor+=cmd.cmdsize;
 }
 if(found!=1||cursor!=sizeof(header)+header.sizeofcmds||memcmp(uuid,expected,16)||base>UINT64_MAX-offset-8)return 3;
 uint32_t instruction;if(!read_self(base+offset,&instruction,4)||instruction!=0xd503245f)return 4;
 int feature=-1;size_t n=sizeof(feature);if(sysctlbyname("hw.optional.arm.FEAT_BTI",&feature,&n,NULL,0)||n!=sizeof(feature)||(feature!=0&&feature!=1))return 5;
 void*targets[]={(void*)bti_plain,(void*)bti_c,(void*)bti_j,(void*)bti_jc};int status[10];
 for(unsigned i=0;i<10;i++){
  pid_t child=fork();if(child<0)return 6;
  if(child==0){struct rlimit limit={0,0};setrlimit(RLIMIT_CORE,&limit);alarm(2);int ok=i<8?branch(targets[i%4],i/4)==99:stackprobe((void*)(uintptr_t)(base+offset+(i==9?4:0)));_exit(ok?0:1);}
  pid_t result;do{result=waitpid(child,&status[i],0);}while(result<0&&errno==EINTR);
  if(result!=child)return 6;
  if(!WIFEXITED(status[i])||WEXITSTATUS(status[i])!=0){fprintf(stderr,"branch target evidence failed at case %u\n",i);return 7;}
 }
 printf("{\"mode\":\"non-enforced\",\"generic_arm64\":true,\"feature\":%d,\"dyld_uuid\":\"",feature);hex(uuid,16);printf("\",\"cache_uuid\":\"");hex(info.sharedCacheUUID,16);printf("\",\"stack_probe_offset\":%llu,\"checks_passed\":10}\n",(unsigned long long)offset);
 return 0;
}
