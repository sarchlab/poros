#include <stdint.h>
#include <stdio.h>
#include <stddef.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/stat.h>
#include <sys/mman.h>
#include <string.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(uint64_t fd,const void*path,void*buf,uint64_t flags){
 register uint64_t a __asm__("x0")=fd,b __asm__("x1")=(uintptr_t)path,c __asm__("x2")=(uintptr_t)buf,d __asm__("x3")=flags,n __asm__("x16")=470;uint64_t f;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(c),"r"(d),"r"(n):"memory","cc");return(struct result){a,b,f};
}
#define FIELD(f) printf("\"" #f "\":%zu,",offsetof(struct stat,f))
int main(void){
 int fd=open("/System/Cryptexes/OS",O_DIRECTORY);if(fd<0)return 2;
 printf("{\"size\":%zu,\"offsets\":{",sizeof(struct stat));
 FIELD(st_dev);FIELD(st_mode);FIELD(st_nlink);FIELD(st_ino);FIELD(st_uid);FIELD(st_gid);FIELD(st_rdev);FIELD(st_atimespec);FIELD(st_mtimespec);FIELD(st_ctimespec);FIELD(st_birthtimespec);FIELD(st_size);FIELD(st_blocks);FIELD(st_blksize);FIELD(st_flags);FIELD(st_gen);FIELD(st_lspare);printf("\"st_qspare\":%zu},\"cases\":[\n",offsetof(struct stat,st_qspare));
 long pagesize=sysconf(_SC_PAGESIZE);unsigned char*guard=mmap(NULL,2*pagesize,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(guard==MAP_FAILED||mprotect(guard+pagesize,pagesize,PROT_NONE))return 3;
 unsigned char buf[sizeof(struct stat)+16];
 const char*abs="/System/Cryptexes/OS/System/Library/dyld/";
 const struct{const char*label;int fd;const void*path;int dest;}cases[]={
 {"selected_relative_directory",fd,"System/Library/dyld/",0},
 {"selected_absolute_directory",fd,abs,0},{"absolute_ignores_bad_fd",-1,abs,0},
 {"relative_bad_fd",-1,"System/Library/dyld/",0},
 {"empty_path",fd,"",0},{"null_path",fd,NULL,0},{"bad_path",fd,(void*)1,0},
 {"null_output",fd,"System/Library/dyld/",1},{"readonly_output",fd,"System/Library/dyld/",2},
 {"short_output_prefix",fd,"System/Library/dyld/",3}};
 for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){
  memset(buf,0xa5,sizeof(buf));memset(guard,0xa5,pagesize);
  void*out=buf;size_t shown=sizeof(buf);if(cases[i].dest==1){out=NULL;shown=0;}if(cases[i].dest==2){if(mprotect(guard,pagesize,PROT_READ))return 4;out=guard;shown=sizeof(buf);}if(cases[i].dest==3){out=guard+pagesize-8;shown=8;}
  struct result r=call(cases[i].fd,cases[i].path,out,0);
  printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"bytes\":\"",i?",\n":"",cases[i].label,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false");
  for(size_t j=0;j<shown;j++)printf("%02x",((unsigned char*)out)[j]);printf("\"}");
  if(cases[i].dest==2&&mprotect(guard,pagesize,PROT_READ|PROT_WRITE))return 5;
 }
 puts("\n]}");munmap(guard,2*pagesize);return close(fd);
}
