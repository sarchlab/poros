#include <stdint.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/mman.h>
#include <string.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(uint64_t fd,void*buf){register uint64_t a __asm__("x0")=fd,b __asm__("x1")=F_GETPATH,c __asm__("x2")=(uintptr_t)buf,n __asm__("x16")=92;uint64_t f;__asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(c),"r"(n):"memory","cc");return(struct result){a,b,f};}
int main(void){_Static_assert(F_GETPATH==50,"command");int fd=open("/System/Cryptexes/OS/System/Library/dyld",O_DIRECTORY);if(fd<0)return 2;
 char expected[1024]={0};if(fcntl(fd,F_GETPATH,expected)||strpbrk(expected,"\"\\\n\r"))return 3;
 size_t size=strlen(expected)+1;long ps=sysconf(_SC_PAGESIZE);unsigned char*guard=mmap(NULL,2*ps,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(guard==MAP_FAILED||mprotect(guard+ps,ps,PROT_NONE))return 4;
 const struct{const char*label;uint64_t fd;int kind;size_t capacity;}cases[]={
 {"full_buffer",fd,0,1024},{"low32_fd",0x100000000ULL+(uint64_t)fd,0,1024},{"invalid_fd",UINT64_MAX,0,1024},
 {"null_output",fd,1,0},{"readonly_output",fd,2,1024},{"unmapped_output",fd,3,0},
 {"partial_one",fd,3,1},{"partial_eight",fd,3,8},{"partial_sixteen",fd,3,16},{"missing_nul",fd,3,size-1},{"exact_size",fd,3,size}};
 printf("{\"expected\":\"%s\",\"size\":%zu,\"cases\":[\n",expected,size);
 for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){
  memset(guard,0xa5,ps);void*out=guard;if(cases[i].kind==1)out=NULL;if(cases[i].kind==2&&mprotect(guard,ps,PROT_READ))return 5;if(cases[i].kind==3)out=guard+ps-cases[i].capacity;
  struct result r=call(cases[i].fd,out);
  printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"bytes\":\"",i?",\n":"",cases[i].label,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false");for(size_t j=0;j<cases[i].capacity;j++)printf("%02x",((unsigned char*)out)[j]);printf("\"}");
  if(cases[i].kind==2&&mprotect(guard,ps,PROT_READ|PROT_WRITE))return 6;
 }
 puts("\n]}");munmap(guard,2*ps);return close(fd);}
