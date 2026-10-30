// Diagnostic only: intercept first ring initialization in an owned process,
// issue the raw allocation once, record its reply, then call the original
// wrapper (which now observes an existing ring). Never use for performance.
#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
extern int mach_vm_reclaim_ring_allocate(void**,uint32_t,uint32_t);
extern kern_return_t _kernelrpc_mach_port_construct_trap(mach_port_t,mach_port_options_t*,uint64_t,mach_port_t*);
extern uint64_t mach_msg2_trap(void*,uint64_t,uint64_t,uint64_t,uint64_t,uint64_t,uint64_t,uint64_t);
#ifndef PROBE_RECEIVE_SIZE
#define PROBE_RECEIVE_SIZE 60
#endif
#ifndef PROBE_READONLY
#define PROBE_READONLY 0
#endif
#ifndef PROBE_DEALLOCATE
#define PROBE_DEALLOCATE 0
#endif
static volatile unsigned calls;
static volatile uint32_t initial,maximum,reply_size,code,reply_id,port_result;
static volatile uint64_t trap_result,address,period,before_tick,after_tick;
static uint64_t absolute_tick(void){uint64_t tick;__asm__ volatile("mrs %0,cntpct_el0":"=r"(tick));return tick+*(const volatile uint64_t*)0xfffffc088ULL;}
static volatile int original_result;
static volatile int deallocate_result=-1;
static volatile unsigned char initial_bytes[128],reply_bytes[60];
static int observe(void**ring,uint32_t len,uint32_t max){
 unsigned call=calls++;if(call)return mach_vm_reclaim_ring_allocate(ring,len,max);
 initial=len;maximum=max;mach_port_options_t options={.flags=0x1000};mach_port_t reply=0,task=mach_task_self();
 port_result=_kernelrpc_mach_port_construct_trap(task,&options,0,&reply);
 if(!port_result){
  mach_vm_address_t storage=0; if(mach_vm_allocate(task,&storage,16384,VM_FLAGS_ANYWHERE))return 5; volatile uint64_t *b=(void*)(uintptr_t)storage;for(unsigned i=0;i<16;i++)b[i]=0;
  b[0]=((uint64_t)40<<32)|0x1513;b[1]=((uint64_t)reply<<32)|task;b[2]=(uint64_t)4822<<32;b[3]=0x100000000ULL;b[4]=((uint64_t)max<<32)|len;
  if(PROBE_READONLY&&mach_vm_protect(task,storage,16384,0,VM_PROT_READ))return 6;
  before_tick=absolute_tick();
  trap_result=mach_msg2_trap((void*)b,0x200000003ULL,((uint64_t)40<<32)|0x1513,((uint64_t)reply<<32)|task,(uint64_t)4822<<32,(uint64_t)reply<<32,PROBE_RECEIVE_SIZE,0);
  after_tick=absolute_tick();
  reply_size=b[0]>>32;reply_id=b[2]>>32;code=(uint32_t)b[4];for(unsigned i=0;i<60;i++)reply_bytes[i]=((volatile unsigned char*)b)[i];
  if(!trap_result&&!code&&reply_size==52){address=(b[4]>>32)|(b[5]<<32);period=(b[5]>>32)|(b[6]<<32);for(unsigned i=0;i<128;i++)initial_bytes[i]=((volatile unsigned char*)(uintptr_t)address)[i];}
 }
 if(PROBE_DEALLOCATE&&address)deallocate_result=mach_vm_deallocate(task,address,16384);
 int r=mach_vm_reclaim_ring_allocate(ring,len,max);original_result=r;return r;
}
__attribute__((used,section("__DATA,__interpose"))) static struct{const void*replacement;const void*original;}pair={observe,mach_vm_reclaim_ring_allocate};
__attribute__((constructor)) static void report(void){const char*s=getenv("POROS_QUERY_FD");if(!s)return;int fd=atoi(s);if(fd<3||fd>1024)return;mach_vm_address_t q=address;mach_vm_size_t size=0;vm_region_basic_info_data_64_t info={0};mach_msg_type_number_t count=VM_REGION_BASIC_INFO_COUNT_64;mach_port_t object=0;int region=address?mach_vm_region(mach_task_self(),&q,&size,VM_REGION_BASIC_INFO_64,(vm_region_info_t)&info,&count,&object):-1;dprintf(fd,"{\"diagnostic_consumes_first_allocation\":true,\"calls\":%u,\"length\":%u,\"max_length\":%u,\"port_result\":%u,\"trap_return\":%llu,\"reply_size\":%u,\"reply_id\":%u,\"code\":%u,\"address\":%llu,\"second_output\":%llu,\"original_wrapper_result\":%d,\"initial_hex\":\"",calls,initial,maximum,port_result,(unsigned long long)trap_result,reply_size,reply_id,code,(unsigned long long)address,(unsigned long long)period,original_result);for(unsigned i=0;i<128;i++)dprintf(fd,"%02x",initial_bytes[i]);dprintf(fd,"\",\"reply_hex\":\"");for(unsigned i=0;i<60;i++)dprintf(fd,"%02x",reply_bytes[i]);dprintf(fd,"\",\"receive_size\":%u,\"readonly\":%s,\"deallocate_result\":%d,\"before_absolute\":%llu,\"after_absolute\":%llu,\"region_result\":%d,\"region_base_matches\":%s,\"region_size\":%llu,\"protection\":%u,\"max_protection\":%u}\n",(unsigned)PROBE_RECEIVE_SIZE,PROBE_READONLY?"true":"false",deallocate_result,(unsigned long long)before_tick,(unsigned long long)after_tick,region,q==address?"true":"false",(unsigned long long)size,info.protection,info.max_protection);}
