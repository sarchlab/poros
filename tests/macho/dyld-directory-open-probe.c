#include <stdint.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/stat.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(int fd,const char*path){register uint64_t a __asm__("x0")=(uint64_t)fd,b __asm__("x1")=(uintptr_t)path,c __asm__("x2")=O_DIRECTORY,d __asm__("x3")=0,n __asm__("x16")=463;uint64_t f;__asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(c),"r"(d),"r"(n):"memory","cc");return(struct result){a,b,f};}
int main(void){int root=open("/",O_DIRECTORY),fd=open("/System/Cryptexes/OS",O_DIRECTORY);if(root!=3||fd!=4)return 2;
 const char*paths[]={"System/Library/dyld/","System/Library/dyld","/System/Volumes/Preboot/Cryptexes/OS/System/Library/dyld/"};
 struct stat expected;if(fstatat(fd,paths[0],&expected,0))return 3;
 puts("[");for(unsigned i=0;i<3;i++){struct result r=call(fd,paths[i]);if(r.nzcv&(1ull<<29))return 4;struct stat actual;if(fstat((int)r.x0,&actual)||actual.st_dev!=expected.st_dev||actual.st_ino!=expected.st_ino||actual.st_mode!=expected.st_mode)return 5;
 printf("%s{\"path\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":false,\"device\":%d,\"inode\":%llu,\"mode\":%u}",i?",\n":"",paths[i],(unsigned long long)r.x0,(unsigned long long)r.x1,actual.st_dev,(unsigned long long)actual.st_ino,actual.st_mode);if(close((int)r.x0))return 6;}
 puts("\n]");return close(fd)||close(root);}
