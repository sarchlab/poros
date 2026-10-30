#include <sys/mman.h>
#include <sys/stat.h>
#include <unistd.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include "macho-stat.h"
struct result{uint64_t x0,x1,nzcv;};
static struct result call(uint64_t x,uint64_t y,uint64_t z){register uint64_t a __asm__("x0")=x,b __asm__("x1")=y,c __asm__("x2")=z,n __asm__("x16")=266;uint64_t flags,initial=0x50000000;__asm__ volatile("msr nzcv,%5\nsvc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(n),"r"(initial):"memory","cc");return(struct result){a,b,flags};}
int main(void){const char*name="com.apple.featureflags.shm";struct{const char*label;uint64_t ptr,flags,mode;}cases[]={{"valid-mode0",(uintptr_t)name,0,0},{"valid-mode1",(uintptr_t)name,0,1},{"valid-mode-max",(uintptr_t)name,0,UINT64_MAX},{"flags-high32",(uintptr_t)name,1ULL<<32,1},{"null-name",0,0,0},{"invalid-name",1,0,0}};printf("{\"cases\":[");for(unsigned i=0;i<6;i++){struct result r=call(cases[i].ptr,cases[i].flags,cases[i].mode);int closed=-1;printf("%s{\"name\":\"%s\",\"flags\":%llu,\"mode\":%llu,\"x0\":%llu,\"x1\":%llu,\"input_nzcv\":1342177280,\"output_nzcv\":%llu",i?",":"",cases[i].label,cases[i].flags,cases[i].mode,r.x0,r.x1,r.nzcv);if(!(r.nzcv&(1ULL<<29))){struct stat st={0};int sr=fstat((int)r.x0,&st);printf(",\"stat_result\":%d,\"stat\":",sr);if(sr||print_file_stat(&st))return 2;closed=close((int)r.x0);}printf(",\"close\":%d}",closed);}printf("]}\n");return 0;}
