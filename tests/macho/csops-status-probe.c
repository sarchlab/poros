#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/mman.h>
#include <string.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(uint64_t pid,void*buf,uint64_t size){register uint64_t a __asm__("x0")=pid,b __asm__("x1")=0,c __asm__("x2")=(uintptr_t)buf,d __asm__("x3")=size,n __asm__("x16")=169;uint64_t f;__asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(c),"r"(d),"r"(n):"memory","cc");return(struct result){a,b,f};}
int main(void){long ps=sysconf(_SC_PAGESIZE);unsigned char*guard=mmap(NULL,2*ps,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(guard==MAP_FAILED||mprotect(guard+ps,ps,PROT_NONE))return 2;
 const struct{const char*label;uint64_t pid,size;int kind;size_t capacity;}cases[]={
 {"self_zero_size",0,0,0,16},{"self_one_size",0,1,0,16},{"self_three_size",0,3,0,16},{"self_four_size",0,4,0,16},{"self_large_size",0,4096,0,16},
 {"explicit_own_pid",(uint64_t)getpid(),4,0,16},{"pid_low32_zero",0x100000000ULL,4,0,16},
 {"null_output_four_size",0,4,1,0},{"null_output_zero_size",0,0,1,0},{"readonly_output",0,4,2,16},
 {"unmapped_output",0,4,3,0},{"partial_one",0,4,3,1},{"partial_three",0,4,3,3},{"exact_four_guarded",0,4,3,4}};
 puts("[");for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){memset(guard,0xa5,ps);void*out=guard;if(cases[i].kind==1)out=NULL;if(cases[i].kind==2&&mprotect(guard,ps,PROT_READ))return 3;if(cases[i].kind==3)out=guard+ps-cases[i].capacity;
 struct result r=call(cases[i].pid,out,cases[i].size);printf("%s{\"case\":\"%s\",\"size\":%llu,\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"bytes\":\"",i?",\n":"",cases[i].label,(unsigned long long)cases[i].size,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false");for(size_t j=0;j<cases[i].capacity;j++)printf("%02x",((unsigned char*)out)[j]);printf("\"}");if(cases[i].kind==2&&mprotect(guard,ps,PROT_READ|PROT_WRITE))return 4;
 }puts("\n]");munmap(guard,2*ps);return 0;}
