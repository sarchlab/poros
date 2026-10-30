// Diagnostic only: interpose native startup calls while preserving their real
// arguments/results. Hooks call no libc helpers before libSystem initialization.
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <pthread.h>
#include <unistd.h>
extern int csops(pid_t,unsigned int,void*,size_t);
extern int __bsdthread_register(void*,void*,int,void*,uintptr_t,uintptr_t);
extern void _dyld_call_with_writable_tpro_memory(void(*)(void*),void*);
static volatile unsigned registration_calls,tpro_calls;
static volatile uint64_t args_seen[6],tpro_before[16],tpro_after[16];
static volatile unsigned char before[128],after[128],capability;
static volatile int registration_result;
static int observe_registration(void*a,void*b,int c,void*d,uintptr_t e,uintptr_t f){registration_calls++;args_seen[0]=(uintptr_t)a;args_seen[1]=(uintptr_t)b;args_seen[2]=(uint32_t)c;args_seen[3]=(uintptr_t)d;args_seen[4]=e;args_seen[5]=f;if(e<=128&&d)for(unsigned i=0;i<e;i++)before[i]=((volatile unsigned char*)d)[i];int r=__bsdthread_register(a,b,c,d,e,f);registration_result=r;if(e<=128&&d)for(unsigned i=0;i<e;i++)after[i]=((volatile unsigned char*)d)[i];return r;}
static void observe_tpro(void(*f)(void*),void*c){unsigned i=tpro_calls++;capability=*(const volatile unsigned char*)0xfffffc10cULL;uint64_t v=0;if(capability==3)__asm__ volatile("mrs %0,S3_6_C15_C1_5":"=r"(v));if(i<16)tpro_before[i]=v;_dyld_call_with_writable_tpro_memory(f,c);if(capability==3)__asm__ volatile("mrs %0,S3_6_C15_C1_5":"=r"(v));if(i<16)tpro_after[i]=v;}
__attribute__((used,section("__DATA,__interpose"))) static struct{const void*replacement;const void*original;}pairs[]={{observe_registration,__bsdthread_register},{observe_tpro,_dyld_call_with_writable_tpro_memory}};
__attribute__((constructor)) static void report(void){const char*s=getenv("POROS_QUERY_FD");if(!s)return;int fd=atoi(s);if(fd<3||fd>1024)return;uint32_t cs=0;int cr=csops(getpid(),0,&cs,sizeof(cs));dprintf(fd,"{\"cs_status\":%u,\"cs_result\":%d,\"main_stack_top\":%llu,",cs,cr,(unsigned long long)(uintptr_t)pthread_get_stackaddr_np(pthread_self()));dprintf(fd,"\"registration_calls\":%u,\"result\":%d,\"args\":[",registration_calls,registration_result);for(unsigned i=0;i<6;i++)dprintf(fd,"%s%llu",i?",":"",(unsigned long long)args_seen[i]);dprintf(fd,"],\"before_hex\":\"");for(unsigned i=0;i<args_seen[4]&&i<128;i++)dprintf(fd,"%02x",before[i]);dprintf(fd,"\",\"after_hex\":\"");for(unsigned i=0;i<args_seen[4]&&i<128;i++)dprintf(fd,"%02x",after[i]);dprintf(fd,"\",\"tpro_capability\":%u,\"tpro_calls\":%u,\"tpro_values\":[",capability,tpro_calls);for(unsigned i=0;i<tpro_calls&&i<16;i++)dprintf(fd,"%s[\"%016llx\",\"%016llx\"]",i?",":"",(unsigned long long)tpro_before[i],(unsigned long long)tpro_after[i]);dprintf(fd,"]}\n");}
