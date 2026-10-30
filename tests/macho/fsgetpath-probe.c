#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/fsgetpath.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <mach-o/dyld.h>
typedef struct {uint64_t x0,x1,carry;} result;
static result raw(char *b,uint64_t n,fsid_t *f,uint64_t ino){register uint64_t x0 __asm__("x0")=(uintptr_t)b;register uint64_t x1 __asm__("x1")=n;register uint64_t x2 __asm__("x2")=(uintptr_t)f;register uint64_t x3 __asm__("x3")=ino;register uint64_t x16 __asm__("x16")=427;uint64_t flags;__asm__ volatile("svc #0x80\n mrs %2,nzcv":"+r"(x0),"+r"(x1),"=r"(flags):"r"(x2),"r"(x3),"r"(x16):"cc","memory");return(result){x0,x1,(flags>>29)&1};}
int main(void){char exe[4096],expected[4096];uint32_t cap=sizeof(exe);if(_NSGetExecutablePath(exe,&cap))return 2;struct stat st;struct statfs fs;if(stat(exe,&st)||statfs(exe,&fs))return 3;ssize_t len=fsgetpath(expected,sizeof(expected),&fs.f_fsid,st.st_ino);if(len<=0)return 4;
for(unsigned i=0;i<16;i++){unsigned char b[4096];memset(b,0xa5,sizeof(b));char*out=(char*)b;uint64_t size=sizeof(b),ino=st.st_ino;fsid_t f=fs.f_fsid,*fp=&f;
switch(i){case 0:break;case 1:size=0;break;case 2:size=(uint64_t)len-1;break;case 3:size=(uint64_t)len;break;case 4:out=0;break;case 5:fp=0;break;case 6:fp=(void*)1;size=0;break;case 7:f.val[0]=0;break;case 8:f.val[1]^=0x12345678;break;case 9:ino=UINT64_MAX;break;case 10:size=UINT64_MAX;break;case 11:size=1;break;case 12:size=8192;break;case 13:size=8193;break;case 14:out=0;size=0;break;case 15:fp=0;size=UINT64_MAX;break;}
result r=raw(out,size,fp,ino);size_t changed=0;for(size_t j=0;j<sizeof(b);j++)if(b[j]!=0xa5)changed=j+1;printf("{\"case\":%u,\"x0\":%llu,\"x1\":%llu,\"carry\":%llu,\"path_length_with_nul\":%zd,\"path_matches\":%s,\"changed_extent\":%zu}\n",i,(unsigned long long)r.x0,(unsigned long long)r.x1,(unsigned long long)r.carry,len,memcmp(b,expected,(size_t)len)==0?"true":"false",changed);}
return 0;}
