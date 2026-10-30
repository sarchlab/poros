#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
#include <sys/mman.h>
#include <string.h>
#include <sys/stat.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(uint64_t fd,void*buf){register uint64_t a __asm__("x0")=fd,b __asm__("x1")=(uintptr_t)buf,n __asm__("x16")=339;uint64_t f;__asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(n):"memory","cc");return(struct result){a,b,f};}
int main(void){FILE*temp=tmpfile();if(!temp||dup2(fileno(temp),2)!=2)return 2;
 struct stat expected;memset(&expected,0xa5,sizeof(expected));if(fstat(2,&expected)||!S_ISREG(expected.st_mode)||sizeof(expected)!=144)return 3;
 long ps=sysconf(_SC_PAGESIZE);unsigned char*guard=mmap(NULL,2*ps,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(guard==MAP_FAILED||mprotect(guard+ps,ps,PROT_NONE))return 4;
 const struct{const char*label;uint64_t fd;int kind;size_t capacity;}cases[]={
 {"owned_regular_stderr",2,0,160},{"low32_fd",0x100000002ULL,0,160},{"invalid_fd",UINT64_MAX,0,160},
 {"null_output",2,1,0},{"readonly_output",2,2,160},{"unmapped_output",2,3,0},
 {"partial_eight",2,3,8},{"partial_128",2,3,128},{"missing_last_byte",2,3,143},{"exact_144",2,3,144}};
 puts("[");for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){memset(guard,0xa5,ps);void*out=guard;if(cases[i].kind==1)out=NULL;if(cases[i].kind==2&&mprotect(guard,ps,PROT_READ))return 5;if(cases[i].kind==3)out=guard+ps-cases[i].capacity;
 struct result r=call(cases[i].fd,out);if(!(r.nzcv&(1ull<<29))){if(cases[i].capacity<144||memcmp(out,&expected,144))return 6;for(size_t j=144;j<cases[i].capacity;j++)if(((unsigned char*)out)[j]!=0xa5)return 7;}
 printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"bytes\":\"",i?",\n":"",cases[i].label,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false");for(size_t j=0;j<cases[i].capacity;j++)printf("%02x",((unsigned char*)out)[j]);printf("\"}");if(cases[i].kind==2&&mprotect(guard,ps,PROT_READ|PROT_WRITE))return 8;
 }puts("\n]");munmap(guard,2*ps);return fclose(temp);}
