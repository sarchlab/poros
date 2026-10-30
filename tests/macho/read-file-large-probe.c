#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <string.h>
#include <limits.h>
struct result{uint64_t x0,x1,x2,nzcv;};
static struct result call(uint64_t number,uint64_t a0,uint64_t a1,uint64_t a2){register uint64_t a __asm__("x0")=a0,b __asm__("x1")=a1,c __asm__("x2")=a2,n __asm__("x16")=number;uint64_t f;__asm__ volatile("svc #0x80\nmrs %3,nzcv":"+r"(a),"+r"(b),"+r"(c),"=&r"(f):"r"(n):"memory","cc");return(struct result){a,b,c,f};}
int main(void){char temp[]="/tmp/poros-read-native-XXXXXX",path[PATH_MAX];int owned=mkstemp(temp);if(owned<0||!realpath(temp,path))return 2;const char payload[]="0123456789abcdefghijklmnopqrstuv";if(write(owned,payload,32)!=32||close(owned))return 3;
 long ps=sysconf(_SC_PAGESIZE);unsigned char*guard=mmap(NULL,ps*2,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(guard==MAP_FAILED||mprotect(guard+ps,ps,PROT_NONE))return 4;
 const struct{const char*label;int which,kind;size_t count,capacity;int offset;}cases[]={
 {"regular_read",0,0,16,32,0},{"low32_fd",3,0,16,32,0},{"zero_null",0,1,0,0,0},{"zero_unmapped",0,2,0,0,0},
 {"invalid_fd",1,0,16,32,0},{"invalid_fd_zero",1,1,0,0,0},{"null_nonzero",0,1,16,0,0},{"unmapped_nonzero",0,2,16,0,0},
 {"readonly_output",0,3,16,32,0},{"exact_boundary",0,2,16,16,0},{"partial_eight",0,2,16,8,0},{"partial_fifteen",0,2,16,15,0},
 {"dup_shared_offset",4,0,16,32,8},{"short_at_eof",0,0,16,32,24},{"eof",0,0,16,32,32},{"independent_open_offset",0,0,16,32,0},{"eof_null_output",0,1,16,0,32},{"short_eof_exact_boundary",0,2,16,8,24},{"oversize_int32",0,0,0x80000000ULL,32,0},{"oversize_uint64",0,0,UINT64_MAX,32,0},{"oversize_eof_null",0,1,UINT64_MAX,0,32},{"int32_max_read",0,0,0x7fffffffULL,32,0},{"int32_max_eof_null",0,1,0x7fffffffULL,0,32},{"oversize_invalid_fd",1,0,UINT64_MAX,32,0}};
 puts("[");int independent=open(path,O_RDONLY);if(independent<0||lseek(independent,20,SEEK_SET)!=20)return 5;
 for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){
 struct result opened=call(5,(uintptr_t)path,O_RDONLY,0);if(opened.nzcv&(1ULL<<29))return 6;int fd=(int)opened.x0,alias=dup(fd);if(alias<0||lseek(fd,cases[i].offset,SEEK_SET)!=cases[i].offset)return 7;
 memset(guard,0xa5,ps);void*buf=guard;if(cases[i].kind==1)buf=NULL;if(cases[i].kind==2)buf=guard+ps-cases[i].capacity;if(cases[i].kind==3&&mprotect(guard,ps,PROT_READ))return 8;
 uint64_t source=fd;if(cases[i].which==1)source=UINT64_MAX;else if(cases[i].which==3)source=(1ULL<<32)|fd;else if(cases[i].which==4)source=alias;
 struct result r=call(3,source,(uintptr_t)buf,cases[i].count);if(cases[i].kind==3&&mprotect(guard,ps,PROT_READ|PROT_WRITE))return 9;
 printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"x2\":%llu,\"carry\":%s,\"offset\":%lld,\"dup_offset\":%lld,\"independent_offset\":%lld,\"bytes\":\"",i?",\n":"",cases[i].label,r.x0,r.x1,r.x2,(r.nzcv&(1ull<<29))?"true":"false",(long long)lseek(fd,0,SEEK_CUR),(long long)lseek(alias,0,SEEK_CUR),(long long)lseek(independent,0,SEEK_CUR));for(size_t j=0;j<cases[i].capacity;j++)printf("%02x",((unsigned char*)buf)[j]);printf("\"}");close(alias);close(fd);
 }puts("\n]");munmap(guard,2*ps);close(independent);return unlink(path);}
