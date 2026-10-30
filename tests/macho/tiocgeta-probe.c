#include <sys/mman.h>
#include <sys/filio.h>
#include <sys/ioctl.h>
#include <termios.h>
#include <sys/stat.h>
#include <unistd.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
struct result { uint64_t x0,x1,nzcv; };
static struct result call(uint64_t fd,uint64_t cmd,uint64_t ptr) {register uint64_t a __asm__("x0")=fd,b __asm__("x1")=cmd,c __asm__("x2")=ptr,n __asm__("x16")=54;uint64_t flags,initial=0x50000000;__asm__ volatile("msr nzcv,%5\nsvc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(n),"r"(initial):"memory","cc");return(struct result){a,b,flags};}
int main(void){char name[]="/tmp/poros-fiodtype-XXXXXX";int fd=mkstemp(name);if(fd<0||unlink(name))return 2;int nullfd=open("/dev/null",O_RDONLY),dirfd=open("/tmp",O_RDONLY),shmfd=shm_open("com.apple.featureflags.shm",O_RDONLY,0);if(nullfd<0||dirfd<0||shmfd<0)return 3;int closedfd=dup(fd);if(closedfd<0||close(closedfd))return 6;size_t page=(size_t)sysconf(_SC_PAGESIZE);unsigned char *mapped=mmap(NULL,page*2,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(mapped==MAP_FAILED)return 4;memset(mapped,0xa5,page*2);if(mprotect(mapped+page,page,PROT_READ))return 5;uint8_t buf[96];
struct {const char*name;uint64_t fd,cmd,ptr;int map;} cases[]={
{"regular",fd,TIOCGETA,(uintptr_t)(buf+4),0},{"null",nullfd,TIOCGETA,(uintptr_t)(buf+4),0},{"directory",dirfd,TIOCGETA,(uintptr_t)(buf+4),0},{"shared-memory",shmfd,TIOCGETA,(uintptr_t)(buf+4),0},{"fd-high32",(1ULL<<32)|(unsigned)fd,TIOCGETA,(uintptr_t)(buf+4),0},{"cmd-high32",fd,(1ULL<<32)|TIOCGETA,(uintptr_t)(buf+4),0},{"null-output",fd,TIOCGETA,0,0},{"invalid-output",fd,TIOCGETA,1,0},{"invalid-fd",UINT32_MAX,TIOCGETA,(uintptr_t)(buf+4),0},{"invalid-both",UINT32_MAX,TIOCGETA,1,0},{"readonly-output",fd,TIOCGETA,(uintptr_t)(mapped+page),1},{"partial-output",fd,TIOCGETA,(uintptr_t)(mapped+page-2),2},{"unaligned-output",fd,TIOCGETA,(uintptr_t)(buf+5),0},{"closed-fd",closedfd,TIOCGETA,(uintptr_t)(buf+4),0},{"null-device-null-output",nullfd,TIOCGETA,0,0},{"null-device-invalid-output",nullfd,TIOCGETA,1,0},{"null-device-readonly-output",nullfd,TIOCGETA,(uintptr_t)(mapped+page),1},{"null-device-partial-output",nullfd,TIOCGETA,(uintptr_t)(mapped+page-2),2},{"null-device-unaligned-output",nullfd,TIOCGETA,(uintptr_t)(buf+5),0}};
printf("{\"command\":%lu,\"cases\":[",(unsigned long)TIOCGETA);for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){memset(buf,0xa5,sizeof(buf));memset(mapped,0xa5,page);struct result r=call(cases[i].fd,cases[i].cmd,cases[i].ptr);printf("%s{\"name\":\"%s\",\"fd\":%llu,\"command\":%llu,\"x0\":%llu,\"x1\":%llu,\"input_nzcv\":1342177280,\"output_nzcv\":%llu,\"buffer_hex\":\"",i?",":"",cases[i].name,cases[i].fd,cases[i].cmd,r.x0,r.x1,r.nzcv);for(unsigned j=0;j<96;j++)printf("%02x",buf[j]);printf("\",\"mapping_window_hex\":\"");for(unsigned j=0;j<8;j++)printf("%02x",mapped[page-4+j]);printf("\"}");}int a=munmap(mapped,page*2),b=close(fd),c=close(nullfd),d=close(dirfd),e=close(shmfd);printf("],\"cleanup\":[%d,%d,%d,%d,%d]}\n",a,b,c,d,e);return a||b||c||d||e;}
