#include <stdint.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/stat.h>
#include <string.h>
#include <errno.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(uint64_t number,uint64_t a0,uint64_t a1,uint64_t a2,uint64_t a3){
 register uint64_t a __asm__("x0")=a0,b __asm__("x1")=a1,c __asm__("x2")=a2,d __asm__("x3")=a3,n __asm__("x16")=number;uint64_t f;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(c),"r"(d),"r"(n):"memory","cc");return(struct result){a,b,f};
}
static int safe(const char*s){return strpbrk(s,"\"\\\n\r")==NULL;}
int main(void){
 const char*path="/System/Cryptexes/OS";int root=open("/",O_DIRECTORY|O_NOFOLLOW_ANY);if(root<0)return 2;
 const struct {const char*label;int dirfd;const void*path;int flags;} cases[]={
 {"relative_directory",root,"System/Cryptexes/OS",O_DIRECTORY},
 {"relative_nofollow_any",root,"System/Cryptexes/OS",O_DIRECTORY|O_NOFOLLOW_ANY},
 {"absolute_directory",root,path,O_DIRECTORY},
 {"absolute_ignores_bad_fd",-1,path,O_DIRECTORY},
 {"relative_bad_fd",-1,"System/Cryptexes/OS",O_DIRECTORY},
 {"empty_path",root,"",O_DIRECTORY},
 {"null_path",root,NULL,O_DIRECTORY},
 {"unmapped_path",root,(void*)1,O_DIRECTORY},
 {"root_dot",root,".",O_DIRECTORY},
 {"root_dotdot",root,"..",O_DIRECTORY}};
 puts("{\"cases\":[");
 for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){
  struct result r=call(463,(uint64_t)cases[i].dirfd,(uintptr_t)cases[i].path,cases[i].flags,0);
  printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s}",i?",\n":"",cases[i].label,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false");
  if(!(r.nzcv&(1ull<<29))&&close((int)r.x0))return 3;
 }
 int fd=openat(root,"System/Cryptexes/OS",O_DIRECTORY);if(fd<0)return 4;
 struct stat ls,s;if(lstat(path,&ls)||fstat(fd,&s))return 5;
 char canonical[1024]={0},link[1024]={0};if(fcntl(fd,F_GETPATH,canonical))return 6;
 ssize_t n=readlink(path,link,sizeof(link)-1);if(n<0)return 7;link[n]=0;
 if(!safe(canonical)||!safe(link))return 8;
 printf("\n],\"directory\":{\"path\":\"%s\",\"canonical_path\":\"%s\",\"symlink_target\":\"%s\",\"symlink_mode\":%u,\"device\":%d,\"inode\":%llu,\"mode\":%u,\"uid\":%u,\"gid\":%u}}\n",path,canonical,link,ls.st_mode,s.st_dev,(unsigned long long)s.st_ino,s.st_mode,s.st_uid,s.st_gid);
 return close(fd)||close(root);
}
