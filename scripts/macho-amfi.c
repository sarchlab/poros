// Diagnostic constructor only, never part of timed workload launches.
// Read the original main executable's dyld policy and code-signing status;
// the collector checks this status against its empty-environment suspended
// launch and requires no entitlements plus unchanged bytes/correct output.
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/mman.h>
#include <sys/dtrace.h>
#include <fcntl.h>
#include <limits.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result query(const void *policy,const void *request){
 register uint64_t a __asm__("x0")=(uint64_t)policy,b __asm__("x1")=90,c __asm__("x2")=(uint64_t)request,n __asm__("x16")=381;uint64_t flags;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(n):"memory","cc");return(struct result){a,b,flags};
}
#include <stdlib.h>
#include <errno.h>
extern int csops(pid_t,unsigned int,void*,size_t);
static struct result helper_ioctl(int fd,uint64_t command,uint64_t argument){
 register uint64_t a __asm__("x0")=(uint32_t)fd,b __asm__("x1")=command,c __asm__("x2")=argument,n __asm__("x16")=54;uint64_t flags;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(n):"memory","cc");return(struct result){a,b,flags};
}
__attribute__((constructor)) static void capture(void){
 const char*t=getenv("POROS_QUERY_FD");if(!t)return;char*end;long fd=strtol(t,&end,10);if(*end||fd<3||fd>1024)return;
 uint64_t out=0xa5a5a5a5a5a5a5a5ULL,req[2]={0,(uintptr_t)&out};struct result r=query("AMFI",req);
 uint32_t cs=0;errno=0;int cs_result=csops(getpid(),0,&cs,sizeof(cs)),cs_errno=errno;
 // An invalid, all-zero DOF header distinguishes lazy storage from eager
 // parsing. Remove only the generation returned for this owned registration.
 int device=open("/dev/dtracehelper",O_RDWR),opened=device>=0,removed=0,lazy=0;
 struct result added={UINT64_MAX,UINT64_MAX,1ull<<29},cleanup=added;
 dof_hdr_t invalid={0};struct {uint64_t count;dof_helper_t helper;} data={.count=1};
 strcpy(data.helper.dofhp_mod,"poros-owned-mode-capture");
 data.helper.dofhp_addr=data.helper.dofhp_dof=(uint64_t)&invalid;
 uint64_t pointer=(uint64_t)&data;
 if(opened){
  added=helper_ioctl(device,0x80086804ULL,(uint64_t)&pointer);
  uint64_t generation=data.helper.dofhp_dof;
  if(!(added.nzcv&(1ull<<29)) && added.x0==0 && added.x1==0 && generation>0 && generation<=INT_MAX){
   lazy=1;cleanup=helper_ioctl(device,0x20006802ULL,generation);
   removed=!(cleanup.nzcv&(1ull<<29)) && cleanup.x0==0 && cleanup.x1==0;
  }
  close(device);
 }
 uint32_t after=0;int after_result=csops(getpid(),0,&after,sizeof(after));
 dprintf((int)fd,"{\"cs_status\":%u,\"cs_result\":%d,\"cs_errno\":%d,\"input_flags\":0,\"output_flags\":%llu,\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"dtrace\":{\"opened\":%s,\"lazy_invalid_header_accepted\":%s,\"own_registration_removed\":%s,\"cs_status_after\":%u,\"cs_result_after\":%d}}\n",cs,cs_result,cs_errno,(unsigned long long)out,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false",opened?"true":"false",lazy?"true":"false",removed?"true":"false",after,after_result);
}
