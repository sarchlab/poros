#define main values_main
#include "ld1-probe.c"
#undef main
#include <signal.h>
#include <sys/mman.h>
#include <sys/ucontext.h>
#include <unistd.h>
#include <stdlib.h>
static volatile sig_atomic_t caught;
static volatile uintptr_t fault_address;
static void handler(int signo,siginfo_t*info,void*context){ucontext_t*u=context;caught=signo;fault_address=(uintptr_t)info->si_addr;u->uc_mcontext->__ss.__pc+=4;}
int main(void){size_t page=(size_t)sysconf(_SC_PAGESIZE);unsigned char*memory=mmap(NULL,page*2,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(memory==MAP_FAILED)return 2;for(size_t i=0;i<page;i++)memory[i]=(i*37+11)&255;if(mprotect(memory+page,page,PROT_NONE))return 3;struct sigaction sa={0};sa.sa_sigaction=handler;sa.sa_flags=SA_SIGINFO;sigemptyset(&sa.sa_mask);if(sigaction(SIGBUS,&sa,NULL)||sigaction(SIGSEGV,&sa,NULL))return 4;_Alignas(16) unsigned char initial[512],out[512];for(unsigned i=0;i<512;i++)initial[i]=(i*13+0xa5)&255;unsigned count=0,partial=0;printf("{\"page_size\":%zu,\"cases\":[",page);for(unsigned c=0;c<sizeof(code)/sizeof(code[0]);c++){unsigned bytes=code[c].n*(code[c].q?16:8);unsigned available[]={0,1,8,16,31,32,47,48,63};for(unsigned a=0;a<sizeof(available)/sizeof(available[0]);a++){unsigned span=available[a];if(span>=bytes)continue;caught=0;fault_address=0;uint64_t result[2]={0};code[c].run(memory+page-span,initial,out,0xa0000000,17,result);if(!caught)return 5;int unchanged=!memcmp(initial,out,512);partial+=!unchanged;printf("%s{\"encoding\":%u,\"readable_prefix\":%u,\"signal\":%d,\"fault_offset\":%llu,\"base_delta\":%llu,\"output_nzcv\":%llu,\"registers_unchanged\":%s,\"loaded_hex\":\"",count++?",":"",code[c].word,span,caught,(uint64_t)(fault_address-(uintptr_t)(memory+page-span)),result[0],result[1],unchanged?"true":"false");for(unsigned r=0;r<code[c].n;r++)hex(out+((code[c].first+r)%32)*16,16);printf("\"}");}}printf("],\"case_count\":%u,\"partial_register_cases\":%u}\n",count,partial);return munmap(memory,page*2)!=0;}
