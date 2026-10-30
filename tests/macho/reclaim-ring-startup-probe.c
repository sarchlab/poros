// Diagnostic interposition only. Preserve the genuine startup call and observe
// its results before any libc helpers are available; report after initialization.
#include <mach/mach.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
extern int mach_vm_reclaim_ring_allocate(void**,uint32_t,uint32_t);
static volatile unsigned calls;
static volatile uint32_t initial,max_len;
static volatile kern_return_t result;
static volatile uint64_t address,before_tick,after_tick;
static uint64_t absolute_tick(void){uint64_t tick;__asm__ volatile("mrs %0,cntpct_el0":"=r"(tick));return tick+*(const volatile uint64_t*)0xfffffc088ULL;}
static volatile unsigned char bytes[128];
static int observe(void**addr,uint32_t len,uint32_t max){
 calls++;initial=len;max_len=max;before_tick=absolute_tick();int r=mach_vm_reclaim_ring_allocate(addr,len,max);after_tick=absolute_tick();result=r;
 if(!r){address=(uintptr_t)*addr;for(unsigned i=0;i<128;i++)bytes[i]=((volatile unsigned char*)*addr)[i];}
 return r;
}
__attribute__((used,section("__DATA,__interpose"))) static struct{const void*replacement;const void*original;}pair={observe,mach_vm_reclaim_ring_allocate};
__attribute__((constructor)) static void report(void){const char*s=getenv("POROS_QUERY_FD");if(!s)return;int fd=atoi(s);if(fd<3||fd>1024)return;dprintf(fd,"{\"calls\":%u,\"length\":%u,\"max_length\":%u,\"result\":%u,\"address\":%llu,\"initial_hex\":\"",calls,initial,max_len,result,(unsigned long long)address);for(unsigned i=0;i<128;i++)dprintf(fd,"%02x",bytes[i]);dprintf(fd,"\",\"before_absolute\":%llu,\"after_absolute\":%llu}\n",(unsigned long long)before_tick,(unsigned long long)after_tick);}
