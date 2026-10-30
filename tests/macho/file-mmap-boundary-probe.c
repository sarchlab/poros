#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <string.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(uint64_t addr,uint64_t len,uint64_t prot,uint64_t flags,uint64_t fd,uint64_t offset){register uint64_t a __asm__("x0")=addr,b __asm__("x1")=len,c __asm__("x2")=prot,d __asm__("x3")=flags,e __asm__("x4")=fd,f __asm__("x5")=offset,n __asm__("x16")=197;uint64_t nz;__asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(nz):"r"(c),"r"(d),"r"(e),"r"(f),"r"(n):"memory","cc");return(struct result){a,b,nz};}
static int readbytes(uint64_t a,void*out,size_t size){mach_vm_size_t got=0;return mach_vm_read_overwrite(mach_task_self(),a,size,(mach_vm_address_t)out,&got)==KERN_SUCCESS&&got==size;}

int main(void){char path[]="/tmp/poros-mmap-boundary-XXXXXX";int wr=mkstemp(path);if(wr<0)return 2;unsigned char data[33472];for(unsigned i=0;i<sizeof(data);i++)data[i]=(i*17+(i>>14)*29+3)&255;if(write(wr,data,sizeof(data))!=sizeof(data))return 3;int fd=open(path,O_RDONLY);if(fd<0||unlink(path)||close(wr))return 4;
 const char*names[]={"legacy_no_sharing_type","legacy_zero_length","write_only_protection","unaligned_hint","legacy_unaligned_offset"};puts("[");
 for(unsigned i=0;i<5;i++){uint64_t hint=0,len=33472,prot=1,flags=0x40002,offset=0;void*arena=MAP_FAILED;
 if(i==0)flags=0;if(i==1){flags=2;len=0;}if(i==2)prot=2;if(i==3){arena=mmap(NULL,5*16384,PROT_NONE,MAP_PRIVATE|MAP_ANON,-1,0);if(arena==MAP_FAILED||munmap((unsigned char*)arena+16384,3*16384))return 5;hint=(uintptr_t)arena+16384+1;len=16;}
 if(i==4){flags=2;offset=1;len=16;}
 struct result r=call(hint,len,prot,flags,fd,offset);int fail=!!(r.nzcv&(1ULL<<29)),current=-1,max=-1,readable=0;uint64_t span=0;unsigned char bytes[16]={0};
 if(!fail&&len){mach_vm_address_t start=r.x0;mach_vm_size_t size=0;vm_region_basic_info_data_64_t info;mach_msg_type_number_t count=VM_REGION_BASIC_INFO_COUNT_64;mach_port_t object=0;if(mach_vm_region(mach_task_self(),&start,&size,VM_REGION_BASIC_INFO_64,(vm_region_info_t)&info,&count,&object)!=KERN_SUCCESS)return 6;if(object)mach_port_deallocate(mach_task_self(),object);span=size;current=info.protection;max=info.max_protection;readable=readbytes(r.x0,bytes,16);}
 printf("%s{\"case\":\"%s\",\"success\":%s,\"errno\":%llu,\"x1\":%llu,\"address_zero\":%s,\"hint_round_up_matches\":%s,\"region_size\":%llu,\"protection\":%d,\"max_protection\":%d,\"readable\":%s,\"bytes\":\"",i?",\n":"",names[i],fail?"false":"true",fail?r.x0:0,r.x1,r.x0==0?"true":"false",(!fail&&hint&&r.x0==((hint+16383)&~16383ULL))?"true":"false",span,current,max,readable?"true":"false");for(unsigned j=0;j<16;j++)printf("%02x",bytes[j]);printf("\"}");if(!fail&&len)munmap((void*)(uintptr_t)r.x0,(len+16383)&~16383ULL);if(arena!=MAP_FAILED)munmap(arena,5*16384);
 }puts("\n]");return close(fd);}
