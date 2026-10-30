#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/mman.h>
#include <string.h>
#include <sys/stat.h>
#include <stdlib.h>
#include <limits.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(const char*path,void*buf){register uint64_t a __asm__("x0")= (uintptr_t)path,b __asm__("x1")=(uintptr_t)buf,n __asm__("x16")=338;uint64_t f;__asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(n):"memory","cc");return(struct result){a,b,f};}
int main(int argc,char **argv){char path[PATH_MAX];if(argc!=1||!realpath(argv[0],path))return 2;
 struct stat expected;memset(&expected,0xa5,sizeof(expected));if(stat(path,&expected)||!S_ISREG(expected.st_mode)||sizeof(expected)!=144)return 3;
 long ps=sysconf(_SC_PAGESIZE);unsigned char*guard=mmap(NULL,2*ps,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(guard==MAP_FAILED||mprotect(guard+ps,ps,PROT_NONE))return 4;
 const struct{const char*label;int pathkind;int kind;size_t capacity;}cases[]={
 {"owned_executable",0,0,160},{"empty_path",1,0,160},{"null_path",2,0,160},{"unmapped_path",3,0,160},
 {"null_output",0,1,0},{"readonly_output",0,2,160},{"unmapped_output",0,3,0},
 {"partial_eight",0,3,8},{"partial_128",0,3,128},{"missing_last_byte",0,3,143},{"exact_144",0,3,144}};
 puts("[");for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){memset(guard,0xa5,ps);void*out=guard;if(cases[i].kind==1)out=NULL;if(cases[i].kind==2&&mprotect(guard,ps,PROT_READ))return 5;if(cases[i].kind==3)out=guard+ps-cases[i].capacity;
 struct result r=call(cases[i].pathkind==1?"":cases[i].pathkind==2?NULL:cases[i].pathkind==3?(char*)guard+ps:path,out);if(!(r.nzcv&(1ull<<29))){if(cases[i].capacity<144||memcmp(out,&expected,144))return 6;for(size_t j=144;j<cases[i].capacity;j++)if(((unsigned char*)out)[j]!=0xa5)return 7;}
 printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"bytes\":\"",i?",\n":"",cases[i].label,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false");for(size_t j=0;j<cases[i].capacity;j++)printf("%02x",((unsigned char*)out)[j]);printf("\"}");if(cases[i].kind==2&&mprotect(guard,ps,PROT_READ|PROT_WRITE))return 8;
 }puts("\n]");munmap(guard,2*ps);return 0;}
