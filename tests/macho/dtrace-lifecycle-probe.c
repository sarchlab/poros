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

static unsigned row_index;
static void emit(const char*label,struct result r,uint64_t a,uint64_t b){printf("%s{\"operation\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%llu,\"generation_a\":%llu,\"generation_b\":%llu}",row_index++?",":"",label,(unsigned long long)r.x0,(unsigned long long)r.x1,(unsigned long long)r.carry,(unsigned long long)a,(unsigned long long)b);}
int main(void){
 int fd=open("/dev/dtracehelper",O_RDWR);if(fd<0)return 2;
 dof_hdr_t dof={0};
 struct {uint64_t count;dof_helper_t helpers[2];} data={.count=2};
 for(int i=0;i<2;i++){strcpy(data.helpers[i].dofhp_mod,"poros-owned-lazy-lifecycle");data.helpers[i].dofhp_addr=data.helpers[i].dofhp_dof=(uint64_t)&dof;}
 uint64_t pointer=(uint64_t)&data;struct result r=raw(fd,0x80086804ULL,(uint64_t)&pointer);if(r.carry)return 3;uint64_t a=data.helpers[0].dofhp_dof,b=data.helpers[1].dofhp_dof;
 puts("{\"cases\":[");emit("add_two",r,a,b);close(fd);fd=open("/dev/dtracehelper",O_RDWR);if(fd<0)return 4;
 r=raw(fd,0x20006802ULL,a);emit("remove_first_after_reopen",r,a,b);r=raw(fd,0x20006802ULL,a);emit("remove_first_again",r,a,b);
 data.count=1;data.helpers[0].dofhp_dof=(uint64_t)&dof;r=raw(fd,0x80086804ULL,(uint64_t)&pointer);uint64_t c=data.helpers[0].dofhp_dof;emit("add_after_first_removed",r,c,b);
 r=raw(fd,0x20006802ULL,c);emit("remove_last",r,c,b);
 data.helpers[0].dofhp_dof=(uint64_t)&dof;r=raw(fd,0x80086804ULL,(uint64_t)&pointer);uint64_t d=data.helpers[0].dofhp_dof;emit("add_after_last_removed",r,d,b);
 r=raw(fd,0x20006802ULL,b);emit("remove_original_second",r,b,d);r=raw(fd,0x20006802ULL,d);emit("remove_new_last",r,d,b);
 r=raw(fd,0x20006802ULL,0);emit("remove_zero",r,0,0);r=raw(fd,0x20006802ULL,0xffffffffffffffffULL);emit("remove_negative",r,0,0);
 close(fd);puts("]}");return 0;}
