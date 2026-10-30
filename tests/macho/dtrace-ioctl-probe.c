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
static void hex(const unsigned char*p,size_t n){for(size_t i=0;i<n;i++)printf("%02x",p[i]);}
static void run(unsigned mode){
 int fd=open("/dev/dtracehelper",mode==8?O_RDONLY:O_RDWR);if(fd<0)_exit(99);
 dof_hdr_t dof={0};memcpy(dof.dofh_ident,"\177DOF",4);dof.dofh_ident[DOF_ID_MODEL]=DOF_MODEL_LP64;dof.dofh_ident[DOF_ID_ENCODING]=DOF_ENCODE_LSB;dof.dofh_ident[DOF_ID_VERSION]=3;dof.dofh_ident[DOF_ID_DIFVERS]=2;dof.dofh_ident[DOF_ID_DIFIREG]=8;dof.dofh_ident[DOF_ID_DIFTREG]=8;dof.dofh_hdrsize=sizeof(dof);dof.dofh_secsize=sizeof(dof_sec_t);dof.dofh_secoff=sizeof(dof);dof.dofh_loadsz=sizeof(dof);dof.dofh_filesz=sizeof(dof);
 unsigned char* mem=mmap(0,16384,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(mem==MAP_FAILED)_exit(98);dof_ioctl_data_t*p=(void*)mem;unsigned count=mode==1?2:1;p->dofiod_count=count;
 for(unsigned i=0;i<count;i++){snprintf(p->dofiod_helpers[i].dofhp_mod,64,"poros-owned-empty-%u",i);p->dofiod_helpers[i].dofhp_addr=p->dofiod_helpers[i].dofhp_dof=(uint64_t)&dof;}
 if(mode==2)p->dofiod_count=0;if(mode==3)p->dofiod_count=1025;
 if(mode==10)memset(&dof,0,sizeof(dof));
 uint64_t known_generation=0;
 if(mode==6){uint64_t preptr=(uint64_t)p;struct result pre=raw(fd,0x80086804ULL,(uint64_t)&preptr);if(pre.carry)_exit(97);known_generation=p->dofiod_helpers[0].dofhp_dof;struct result removed=raw(fd,0x20006802ULL,known_generation);if(removed.carry)_exit(96);p->dofiod_helpers[0].dofhp_dof=(uint64_t)&dof;}
 unsigned bytes=8+80*count;unsigned char before[168];memcpy(before,mem,bytes);
 uint64_t ptr=(uint64_t)p,outer=(uint64_t)&ptr;if(mode==4)outer=0;if(mode==5)ptr=0;if(mode==6)mprotect(mem,16384,PROT_READ);if(mode==7)fd=999999;
 uint64_t cmd=mode==9?0x200068ffULL:0x80086804ULL;struct result r=raw(fd,cmd,outer);
 printf("{\"mode\":%u,\"dof_header_size\":%zu,\"helper_size\":%zu,\"x0\":%llu,\"x1\":%llu,\"carry\":%llu,\"before\":\"",mode,sizeof(dof),sizeof(dof_helper_t),(unsigned long long)r.x0,(unsigned long long)r.x1,(unsigned long long)r.carry);hex(before,bytes);printf("\",\"after\":\"");hex(mem,bytes);printf("\",\"removals\":[");
 if((r.carry==0 || mode==6) && cmd==0x80086804ULL){for(unsigned i=0;i<count;i++){uint64_t gen=mode==6?known_generation:p->dofiod_helpers[i].dofhp_dof;struct result q=raw(fd,0x20006802ULL,gen);printf("%s{\"generation\":%llu,\"x0\":%llu,\"x1\":%llu,\"carry\":%llu}",i?",":"",(unsigned long long)gen,(unsigned long long)q.x0,(unsigned long long)q.x1,(unsigned long long)q.carry);}}
 puts("]}");fflush(stdout);_exit(0);
}
int main(void){for(unsigned mode=0;mode<11;mode++){fflush(stdout);pid_t p=fork();if(p<0)return 2;if(p==0)run(mode);int status;if(waitpid(p,&status,0)!=p||!WIFEXITED(status)||WEXITSTATUS(status))return 3;}return 0;}
