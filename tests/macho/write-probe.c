#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <string.h>
struct result{uint64_t x0,x1,x2,nzcv;};
static struct result call(uint64_t fd,const void*buf,uint64_t len){register uint64_t a __asm__("x0")=fd,b __asm__("x1")=(uintptr_t)buf,c __asm__("x2")=len,n __asm__("x16")=4;uint64_t f,initial=0xa0000000;__asm__ volatile("msr nzcv,%5\nsvc #0x80\nmrs %3,nzcv":"+r"(a),"+r"(b),"+r"(c),"=&r"(f):"r"(n),"r"(initial):"memory","cc");return(struct result){a,b,c,f};}
int main(void){char path[]="/tmp/poros-write-native-XXXXXX";int fd=mkstemp(path);if(fd<0)return 2;int ro=open(path,O_RDONLY),alias=dup(fd);if(ro<0||alias<0)return 3;if(unlink(path))return 4;
 long ps=sysconf(_SC_PAGESIZE);unsigned char*guard=mmap(NULL,ps*2,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(guard==MAP_FAILED||mprotect(guard+ps,ps,PROT_NONE))return 5;memset(guard,0x5a,ps);
 const struct{const char*label;int which,kind;size_t count,capacity;int prefix;}cases[]={
 {"regular_bytes",0,0,7,7,0},{"low32_fd",3,0,7,7,0},{"zero_null",0,1,0,0,0},{"zero_unmapped",0,2,0,0,0},
 {"invalid_fd",1,0,7,7,0},{"invalid_fd_zero",1,1,0,0,0},{"readonly_fd",2,0,7,7,0},{"readonly_fd_zero",2,1,0,0,0},
 {"null_nonzero",0,1,7,0,0},{"unmapped_nonzero",0,2,7,0,0},{"readonly_input",0,3,7,7,0},
 {"exact_boundary",0,2,7,7,0},{"partial_one",0,2,7,1,0},{"partial_six",0,2,7,6,0},{"dup_shared_offset",4,0,7,7,1}};
 puts("[");for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){
 if(ftruncate(fd,0)||lseek(fd,0,SEEK_SET))return 6;if(cases[i].prefix&&write(fd,"prefix:",7)!=7)return 7;
 const void*buf="hello!\n";if(cases[i].kind==1)buf=NULL;if(cases[i].kind==2)buf=guard+ps-cases[i].capacity;if(cases[i].kind==3){buf=guard;if(mprotect(guard,ps,PROT_READ))return 8;}
 uint64_t dest=fd;if(cases[i].which==1)dest=UINT64_MAX;else if(cases[i].which==2)dest=ro;else if(cases[i].which==3)dest=(1ULL<<32)|fd;else if(cases[i].which==4)dest=alias;
 struct result r=call(dest,buf,cases[i].count);if(cases[i].kind==3&&mprotect(guard,ps,PROT_READ|PROT_WRITE))return 9;
 unsigned char bytes[64];ssize_t len=pread(fd,bytes,sizeof(bytes),0);if(len<0)return 10;
 printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"x2\":%llu,\"carry\":%s,\"offset\":%lld,\"dup_offset\":%lld,\"contents\":\"",i?",\n":"",cases[i].label,r.x0,r.x1,r.x2,(r.nzcv&(1ull<<29))?"true":"false",(long long)lseek(fd,0,SEEK_CUR),(long long)lseek(alias,0,SEEK_CUR));for(ssize_t j=0;j<len;j++)printf("%02x",bytes[j]);printf("\"}");
 }puts("\n]");munmap(guard,2*ps);close(ro);close(alias);return close(fd);}
