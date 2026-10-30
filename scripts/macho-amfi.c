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
#include <mach/mach.h>
#include <mach/arm/thread_status.h>
#include <stdatomic.h>
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
// The first instruction runs before any user runtime on this owned Mach
// thread. Observe the kernel-created default, not a synthesized exec trace.
struct thread_observation { uint64_t value; _Atomic uint64_t ready; };
__attribute__((naked)) static void read_thread_default(void) {
 __asm__ volatile("mrs x1, S3_6_C15_C1_5\nstr x1,[x0]\nmov x1,#1\nadd x2,x0,#8\nstlr x1,[x2]\n1:yield\nb 1b");
}
static uint64_t capture_thread_default(void) {
 uint32_t entry=0;memcpy(&entry,(const void*)(uintptr_t)&read_thread_default,sizeof(entry));
 if(entry!=0xd53ef1a1U)_exit(80);
 void *stack=mmap(NULL,65536,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);
 if(stack==MAP_FAILED)_exit(81);
 struct thread_observation result={0}; arm_thread_state64_t state={0};
 state.__x[0]=(uint64_t)&result;
 arm_thread_state64_set_pc_fptr(state,read_thread_default);
 arm_thread_state64_set_sp(state,(char*)stack+65536);
 thread_act_t thread=MACH_PORT_NULL;
 kern_return_t created=thread_create_running(mach_task_self(),ARM_THREAD_STATE64,
   (thread_state_t)&state,ARM_THREAD_STATE64_COUNT,&thread);
 if(created){munmap(stack,65536);_exit(82);}
 for(unsigned i=0;i<1000&&!atomic_load_explicit(&result.ready,memory_order_acquire);i++)usleep(1000);
 int ready=atomic_load_explicit(&result.ready,memory_order_acquire);
 // Never return while an owned thread can still reference this frame/stack.
 // Any cleanup failure terminates only this diagnostic process.
 if(thread_terminate(thread)!=KERN_SUCCESS)_exit(83);
 kern_return_t released=mach_port_deallocate(mach_task_self(),thread);
 if(released!=KERN_SUCCESS){
  // Some kernel thread ports disappear when the thread terminates. Require
  // an independent namespace query to prove that no owned right remains.
  mach_port_type_t kind=0;
  if(released!=KERN_INVALID_NAME || mach_port_type(mach_task_self(),thread,&kind)!=KERN_INVALID_NAME)_exit(84);
 }
 if(munmap(stack,65536)||!ready)_exit(85);
 return result.value;
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
 dprintf((int)fd,"{\"cs_status\":%u,\"cs_result\":%d,\"cs_errno\":%d,\"input_flags\":0,\"output_flags\":%llu,\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"dtrace\":{\"opened\":%s,\"lazy_invalid_header_accepted\":%s,\"own_registration_removed\":%s,\"cs_status_after\":%u,\"cs_result_after\":%d},",cs,cs_result,cs_errno,(unsigned long long)out,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false",opened?"true":"false",lazy?"true":"false",removed?"true":"false",after,after_result);
 uint8_t capability=*(const volatile uint8_t*)0xfffffc10cULL;
 uint64_t protection=0;
 if(capability==3)__asm__ volatile("mrs %0, S3_6_C15_C1_5":"=r"(protection));
 dprintf((int)fd,"\"thread_protection_observation\":{\"stage\":\"post-loader-pre-main\",\"capability\":%u,\"register\":\"S3_6_C15_C1_5\",\"value\":",capability);
 if(capability==3)dprintf((int)fd,"\"%llu\"",(unsigned long long)protection);else dprintf((int)fd,"null");
 if(capability==3){
  uint64_t initial=capture_thread_default(),rechecked=0;
  __asm__ volatile("mrs %0, S3_6_C15_C1_5":"=r"(rechecked));
  uint32_t final_cs=0;int final_result=csops(getpid(),0,&final_cs,sizeof(final_cs));
  dprintf((int)fd,",\"kernel_thread_default\":{\"stage\":\"first-instruction-of-new-mach-thread\",\"value\":\"%llu\",\"main_rechecked\":\"%llu\",\"terminated\":true,\"entry_encoding\":3577672097,\"cs_status_after\":%u,\"cs_result_after\":%d}",(unsigned long long)initial,(unsigned long long)rechecked,final_cs,final_result);
 }
 dprintf((int)fd,"}}\n");

}
