#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <string.h>
#include <errno.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(const char*path,void*buf){register uint64_t a __asm__("x0")=(uintptr_t)path,b __asm__("x1")=(uintptr_t)buf,n __asm__("x16")=338;uint64_t f;__asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(n):"memory","cc");return(struct result){a,b,f};}
int main(void){const char*path="/usr/lib/libSystem.B.dylib";struct stat st;errno=0;int rc=stat(path,&st),saved=errno;if(rc!=-1||saved!=ENOENT)return 2;
long ps=sysconf(_SC_PAGESIZE);if(ps<144)return 3;unsigned char*m=mmap(NULL,2*ps,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(m==MAP_FAILED||mprotect(m+ps,ps,PROT_NONE))return 4;
const struct{const char*label;int kind;int cap;} cases[]={{"writable_output",0,160},{"null_output",1,0},{"unmapped_output",2,0},{"readonly_output",3,160},{"partial_eight",4,8},{"missing_last_byte",4,143}};
printf("{\"path\":\"%s\",\"libc_return\":%d,\"libc_errno\":%d,\"cases\":[",path,rc,saved);
for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){memset(m,0xa5,ps);void*out=m;if(cases[i].kind==1)out=NULL;if(cases[i].kind==2)out=m+ps;if(cases[i].kind==3&&mprotect(m,ps,PROT_READ))return 5;if(cases[i].kind==4)out=m+ps-cases[i].cap;struct result r=call(path,out);int unchanged=1;for(long j=0;j<ps;j++)if(m[j]!=0xa5)unchanged=0;
printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"output_unchanged\":%s}",i?",":"",cases[i].label,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ULL<<29))?"true":"false",unchanged?"true":"false");if(cases[i].kind==3&&mprotect(m,ps,PROT_READ|PROT_WRITE))return 6;}
printf("]}\n");return munmap(m,2*ps)!=0;}
