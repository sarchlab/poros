#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
int main(void){
 _Alignas(16384) uint64_t memory[4096]={0}; unsigned ix=0;puts("{\"cases\":[");
 for(unsigned width=4;width<=8;width+=4)for(unsigned off=0;off<3;off++){
 unsigned offsets[]={0,16,2048},success=0,failure=0;uint64_t* p=memory,*q=memory+offsets[off];
 for(unsigned trial=0;trial<1000;trial++){
 uint64_t old,load;unsigned status;*p=9;*q=9;
 if(width==8)__asm__ volatile("ldxr %x[old],[%x[p]]\n\tldapr %x[load],[%x[q]]\n\tstxr %w[s],%x[old],[%x[p]]\n\tclrex" : [old]"=&r"(old),[load]"=&r"(load),[s]"=&r"(status) : [p]"r"(p),[q]"r"(q) : "memory");
 else __asm__ volatile("ldxr %w[old],[%x[p]]\n\tldapr %w[load],[%x[q]]\n\tstxr %w[s],%w[old],[%x[p]]\n\tclrex" : [old]"=&r"(old),[load]"=&r"(load),[s]"=&r"(status) : [p]"r"(p),[q]"r"(q) : "memory");
 if(old!=9||load!=9||*p!=9)return 4;if(status==0)success++;else failure++;
 }
 printf("%s{\"width\":%u,\"load_offset_bytes\":%u,\"trials\":1000,\"stxr_success\":%u,\"stxr_failure\":%u}",ix++?",":"",width,offsets[off]*8,success,failure);
 }puts("]}");return 0;}
