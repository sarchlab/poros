// Diagnostic constructor only, never part of timed workload launches.
// Read the original main executable's dyld policy and code-signing status;
// the collector checks this status against its empty-environment suspended
// launch and requires no entitlements plus unchanged bytes/correct output.
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/mman.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result query(const void *policy,const void *request){
 register uint64_t a __asm__("x0")=(uint64_t)policy,b __asm__("x1")=90,c __asm__("x2")=(uint64_t)request,n __asm__("x16")=381;uint64_t flags;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(n):"memory","cc");return(struct result){a,b,flags};
}
#include <stdlib.h>
#include <errno.h>
extern int csops(pid_t,unsigned int,void*,size_t);
__attribute__((constructor)) static void capture(void){
 const char*t=getenv("POROS_QUERY_FD");if(!t)return;char*end;long fd=strtol(t,&end,10);if(*end||fd<3||fd>1024)return;
 uint64_t out=0xa5a5a5a5a5a5a5a5ULL,req[2]={0,(uintptr_t)&out};struct result r=query("AMFI",req);
 uint32_t cs=0;errno=0;int cs_result=csops(getpid(),0,&cs,sizeof(cs)),cs_errno=errno;
 dprintf((int)fd,"{\"cs_status\":%u,\"cs_result\":%d,\"cs_errno\":%d,\"input_flags\":0,\"output_flags\":%llu,\"x0\":%llu,\"x1\":%llu,\"carry\":%s}\n",cs,cs_result,cs_errno,(unsigned long long)out,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false");
}
