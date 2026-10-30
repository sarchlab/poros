#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <string.h>
#include <limits.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(const char*path,uint64_t flags){register uint64_t a __asm__("x0")=(uintptr_t)path,b __asm__("x1")=flags,c __asm__("x2")=0,n __asm__("x16")=5;uint64_t f;__asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(c),"r"(n):"memory","cc");return(struct result){a,b,f};}
int main(int argc,char**argv){char path[PATH_MAX];struct stat expected;if(argc!=1||!realpath(argv[0],path)||stat(path,&expected))return 2;long ps=sysconf(_SC_PAGESIZE);void*guard=mmap(NULL,ps,PROT_NONE,MAP_PRIVATE|MAP_ANON,-1,0);if(guard==MAP_FAILED)return 3;
 const struct{const char*label;int kind;uint64_t flags;}cases[]={{"readonly",0,0},{"flags_low32",0,1ULL<<32},{"cloexec",0,O_CLOEXEC},{"nofollow_regular",0,O_NOFOLLOW},{"empty_path",1,0},{"null_path",2,0},{"unmapped_path",3,0}};
 puts("[");for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){const char*p=path;if(cases[i].kind==1)p="";if(cases[i].kind==2)p=NULL;if(cases[i].kind==3)p=guard;struct result r=call(p,cases[i].flags);int failed=!!(r.nzcv&(1ULL<<29)),status=-1,descriptor=-1,identity=0;unsigned char magic[4]={0};if(!failed){struct stat got;status=fcntl((int)r.x0,F_GETFL);descriptor=fcntl((int)r.x0,F_GETFD);if(fstat((int)r.x0,&got)||pread((int)r.x0,magic,4,0)!=4)return 4;identity=got.st_dev==expected.st_dev&&got.st_ino==expected.st_ino&&got.st_size==expected.st_size;if(close((int)r.x0))return 5;}
 printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"status_flags\":%d,\"descriptor_flags\":%d,\"same_identity\":%s,\"first_bytes\":\"%02x%02x%02x%02x\"}",i?",\n":"",cases[i].label,r.x0,r.x1,failed?"true":"false",status,descriptor,identity?"true":"false",magic[0],magic[1],magic[2],magic[3]);
 }puts("\n]");return munmap(guard,ps);}
