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
#include <pthread.h>
#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <mach/mach_time.h>

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
// Native startup observations only. These wrappers invoke the original native
// routines with unchanged arguments/results. They call no libc before libSystem
// initialization, and never participate in timed launches or guest execution.
extern int __bsdthread_register(void*,void*,int,void*,uintptr_t,uintptr_t);
static volatile unsigned registration_calls;
static volatile uint64_t registration_args[3];
static volatile unsigned char registration_before[128],registration_after[128];
static volatile int registration_result;
static int observe_registration(void*a,void*b,int c,void*d,uintptr_t size,uintptr_t offset){
 unsigned call=registration_calls++;
 if(call==0){registration_args[0]=(uint32_t)c;registration_args[1]=size;registration_args[2]=offset;
  if(d&&size<=128)for(unsigned i=0;i<size;i++)registration_before[i]=((volatile unsigned char*)d)[i];}
 int result=__bsdthread_register(a,b,c,d,size,offset);
 if(call==0){registration_result=result;if(d&&size<=128)for(unsigned i=0;i<size;i++)registration_after[i]=((volatile unsigned char*)d)[i];}
 return result;
}
#if POROS_CAPTURE_TPRO
extern void _dyld_call_with_writable_tpro_memory(void(*)(void*),void*);
static volatile unsigned tpro_calls;
static volatile uint64_t tpro_before[16],tpro_after[16];
static volatile unsigned char tpro_capability;
static void observe_tpro(void(*callback)(void*),void*context){
 unsigned i=tpro_calls++;uint64_t value=0;
 tpro_capability=*(const volatile unsigned char*)0xfffffc10cULL;
 if(tpro_capability==3)__asm__ volatile("mrs %0,S3_6_C15_C1_5":"=r"(value));
 if(i<16)tpro_before[i]=value;
 _dyld_call_with_writable_tpro_memory(callback,context);
 if(tpro_capability==3)__asm__ volatile("mrs %0,S3_6_C15_C1_5":"=r"(value));
 if(i<16)tpro_after[i]=value;
}
#endif
#if POROS_CAPTURE_RECLAIM
extern int mach_vm_reclaim_ring_allocate(void**,uint32_t,uint32_t);
static volatile unsigned reclaim_calls;
static volatile uint32_t reclaim_length,reclaim_max_length;
static volatile int reclaim_result;
static volatile uint64_t reclaim_address,reclaim_before_tick,reclaim_after_tick;
static volatile unsigned char reclaim_initial[128];
static int observe_reclaim(void**address,uint32_t length,uint32_t maximum){
 unsigned call=reclaim_calls++;
 if(call==0){reclaim_length=length;reclaim_max_length=maximum;reclaim_before_tick=mach_absolute_time();}
 int result=mach_vm_reclaim_ring_allocate(address,length,maximum);
 if(call==0){reclaim_after_tick=mach_absolute_time();reclaim_result=result;
  if(result==0&&address&&*address&&length>0){reclaim_address=(uint64_t)(uintptr_t)*address;
   for(unsigned i=0;i<128;i++)reclaim_initial[i]=((volatile unsigned char*)*address)[i];}}
 return result;
}
#endif
__attribute__((used,section("__DATA,__interpose"))) static struct {
 const void *replacement,*original;
} startup_interpose[]={
 {observe_registration,__bsdthread_register},
#if POROS_CAPTURE_TPRO
 {observe_tpro,_dyld_call_with_writable_tpro_memory},
#endif
#if POROS_CAPTURE_RECLAIM
 {observe_reclaim,mach_vm_reclaim_ring_allocate},
#endif
};
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
 dprintf((int)fd,"\"pthread_registration\":{\"stage\":\"native-first-registration\",\"calls\":%u,\"result\":%d,\"pthread_size\":%llu,\"data_size\":%llu,\"dispatch_queue_offset\":%llu,\"main_stack_top\":\"%llu\",\"input_hex\":\"",registration_calls,registration_result,(unsigned long long)registration_args[0],(unsigned long long)registration_args[1],(unsigned long long)registration_args[2],(unsigned long long)(uintptr_t)pthread_get_stackaddr_np(pthread_self()));
 for(unsigned i=0;i<registration_args[1]&&i<128;i++)dprintf((int)fd,"%02x",registration_before[i]);
 dprintf((int)fd,"\",\"output_hex\":\"");
 for(unsigned i=0;i<registration_args[1]&&i<128;i++)dprintf((int)fd,"%02x",registration_after[i]);
 dprintf((int)fd,"\"},\"thread_protection_observation\":{\"stage\":\"native-tpro-call-entry-and-return\",\"capability\":%u,\"register\":\"S3_6_C15_C1_5\",",capability);
#if POROS_CAPTURE_TPRO
 dprintf((int)fd,"\"interposed\":true,\"entry_capability\":%u,\"calls\":%u,\"values\":[",tpro_capability,tpro_calls);
 for(unsigned i=0;i<tpro_calls&&i<16;i++)dprintf((int)fd,"%s[\"%llu\",\"%llu\"]",i?",":"",(unsigned long long)tpro_before[i],(unsigned long long)tpro_after[i]);
 dprintf((int)fd,"]");
#else
 dprintf((int)fd,"\"interposed\":false,\"calls\":0,\"values\":[]");
#endif
 dprintf((int)fd,"},\"reclaim\":{\"stage\":\"native-reclaim-ring-return\",");
#if POROS_CAPTURE_RECLAIM
 mach_vm_address_t region_address=reclaim_address;mach_vm_size_t region_size=0;
 vm_region_basic_info_data_64_t region={0};mach_msg_type_number_t region_count=VM_REGION_BASIC_INFO_COUNT_64;
 mach_port_t object=MACH_PORT_NULL;
 kern_return_t region_result=reclaim_address?mach_vm_region(mach_task_self(),&region_address,&region_size,VM_REGION_BASIC_INFO_64,(vm_region_info_t)&region,&region_count,&object):KERN_INVALID_ADDRESS;
 kern_return_t object_cleanup=object?mach_port_deallocate(mach_task_self(),object):0;
 dprintf((int)fd,"\"interposed\":true,\"calls\":%u,\"length\":%u,\"max_length\":%u,\"result\":%d,\"before_absolute\":\"%llu\",\"after_absolute\":\"%llu\",\"region_result\":%u,\"region_base_matches\":%s,\"region_size\":\"%llu\",\"protection\":%u,\"max_protection\":%u,\"object_cleanup\":%u,\"initial_hex\":\"",reclaim_calls,reclaim_length,reclaim_max_length,reclaim_result,(unsigned long long)reclaim_before_tick,(unsigned long long)reclaim_after_tick,region_result,region_address==reclaim_address?"true":"false",(unsigned long long)region_size,region.protection,region.max_protection,object_cleanup);
 for(unsigned i=0;i<128;i++)dprintf((int)fd,"%02x",reclaim_initial[i]);
 dprintf((int)fd,"\"");
#else
 dprintf((int)fd,"\"interposed\":false,\"calls\":0");
#endif
 dprintf((int)fd,"}}\n");
}
