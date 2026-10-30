#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/mman.h>
#include <mach/mach.h>
#include <mach/mach_vm.h>
int main(void){char name[]="/tmp/poros-vmcopy-XXXXXX";int fd=mkstemp(name);if(fd<0)return 2;unlink(name);unsigned char data[16384];memset(data,0x5a,sizeof(data));if(write(fd,data,sizeof(data))!=sizeof(data))return 3;
 unsigned char*a=mmap(0,16384,PROT_READ|PROT_WRITE,MAP_PRIVATE,fd,0),*b=mmap(0,16384,PROT_READ,MAP_PRIVATE,fd,0);if(a==MAP_FAILED||b==MAP_FAILED)return 4;
 a[0]=0x6b;if(mach_vm_protect(mach_task_self(),(uint64_t)a,16384,1,3)||mach_vm_protect(mach_task_self(),(uint64_t)a,16384,0,1))return 5;
 register uint64_t x0 __asm__("x0")=mach_task_self(),x1 __asm__("x1")=(uint64_t)a,x2 __asm__("x2")=16384,x3 __asm__("x3")=0,x4 __asm__("x4")=19,n __asm__("x16")=(uint64_t)-14;
 __asm__ volatile("svc #0x80":"+r"(x0),"+r"(x1):"r"(x2),"r"(x3),"r"(x4),"r"(n):"cc","memory");uint64_t result=x0;unsigned preserved=a[0];unsigned final=0;if(result==0){a[0]=0x7c;final=a[0];}unsigned char disk=0;if(pread(fd,&disk,1,0)!=1)return 6;
 printf("{\"requested\":19,\"return\":%llu,\"private_before\":107,\"private_after_copy\":%u,\"private_after_write\":%u,\"other_mapping\":%u,\"backing_file\":%u}\n",(unsigned long long)result,preserved,final,b[0],disk);munmap(a,16384);munmap(b,16384);close(fd);return 0;}
