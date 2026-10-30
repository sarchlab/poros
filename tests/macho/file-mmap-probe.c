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
static int run(unsigned which,int fd){const char*names[]={"readonly_private_unix03","private_cow","zero_length","bad_fd","low32_fd","aligned_offset","unaligned_offset","negative_offset","beyond_eof_page","partial_eof_page","private_without_unix03","no_sharing_type","fixed_owned_replacement","overflow_length","after_close"};uint64_t address=0,len=33472,prot=PROT_READ,flags=0x40002,offset=0,fdarg=fd;void*owned=MAP_FAILED;
 switch(which){case 1:prot|=PROT_WRITE;break;case 2:len=0;break;case 3:fdarg=UINT64_MAX;break;case 4:fdarg|=1ULL<<32;break;case 5:offset=16384;len=16;break;case 6:offset=1;len=16;break;case 7:offset=UINT64_MAX;len=16;break;case 8:offset=49152;len=16;break;case 9:offset=32768;len=16;break;case 10:flags=2;break;case 11:flags=0x40000;break;case 12:owned=mmap(NULL,49152,PROT_NONE,MAP_PRIVATE|MAP_ANON,-1,0);if(owned==MAP_FAILED)return 2;address=(uintptr_t)owned;flags|=MAP_FIXED;break;case 13:len=UINT64_MAX;break;}
 struct result r=call(address,len,prot,flags,fdarg,offset);int fail=!!(r.nzcv&(1ULL<<29)),p=-1,max=-1,readable=0,tailread=0,cow=-1,lifetime=-1;uint64_t span=0;unsigned char bytes[16]={0},tail[16]={0};
 if(!fail){mach_vm_address_t start=r.x0;mach_vm_size_t size=0;vm_region_basic_info_data_64_t info;mach_msg_type_number_t count=VM_REGION_BASIC_INFO_COUNT_64;mach_port_t object=0;if(mach_vm_region(mach_task_self(),&start,&size,VM_REGION_BASIC_INFO_64,(vm_region_info_t)&info,&count,&object)!=KERN_SUCCESS)return 3;if(object)mach_port_deallocate(mach_task_self(),object);span=size;p=info.protection;max=info.max_protection;readable=readbytes(r.x0,bytes,16);
 if(offset<33472&&33472-offset+16<=span)tailread=readbytes(r.x0+33472-offset,tail,16);
 if(which==1){unsigned char original;if(pread(fd,&original,1,0)!=1)return 4;*(volatile unsigned char*)(uintptr_t)r.x0=0xa7;unsigned char after;if(pread(fd,&after,1,0)!=1)return 5;void*second=mmap(NULL,16384,PROT_READ,MAP_PRIVATE,fd,0);if(second==MAP_FAILED)return 6;cow=original==after&&*(unsigned char*)second==original&&*(unsigned char*)(uintptr_t)r.x0==0xa7;munmap(second,16384);}
 if(which==14){if(close(fd))return 7;unsigned char after[16];lifetime=readbytes(r.x0,after,16)&&!memcmp(bytes,after,16);}
 }
 printf("%s{\"case\":\"%s\",\"success\":%s,\"errno\":%llu,\"x1\":%llu,\"page_aligned\":%s,\"fixed_address_matches\":%s,\"region_size\":%llu,\"protection\":%d,\"max_protection\":%d,\"readable\":%s,\"bytes\":\"",which?",\n":"",names[which],fail?"false":"true",fail?r.x0:0,r.x1,(!fail&&r.x0%16384==0)?"true":"false",(!fail&&address&&r.x0==address)?"true":"false",span,p,max,readable?"true":"false");for(unsigned i=0;i<16;i++)printf("%02x",bytes[i]);printf("\",\"eof_tail_readable\":%s,\"eof_tail\":\"",tailread?"true":"false");for(unsigned i=0;i<16;i++)printf("%02x",tail[i]);printf("\",\"private_cow_preserved_file\":%d,\"mapping_survived_close\":%d}",cow,lifetime);fflush(stdout);return 0;}
int main(void){char path[]="/tmp/poros-mmap-native-XXXXXX";int wr=mkstemp(path);if(wr<0)return 8;unsigned char*data=malloc(33472);if(!data)return 9;for(unsigned i=0;i<33472;i++)data[i]=(i*17+(i>>14)*29+3)&255;if(write(wr,data,33472)!=33472)return 10;free(data);int fd=open(path,O_RDONLY);if(fd<0||unlink(path)||close(wr))return 11;
 puts("[");fflush(stdout);for(unsigned i=0;i<15;i++){pid_t p=fork();if(p<0)return 12;if(p==0)_exit(run(i,fd));int s;if(waitpid(p,&s,0)!=p||!WIFEXITED(s)||WEXITSTATUS(s))return 13;}puts("\n]");return close(fd);}
