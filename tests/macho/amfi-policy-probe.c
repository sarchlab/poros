#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/mman.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result query(const void *policy,const void *request){
 register uint64_t a __asm__("x0")=(uint64_t)policy,b __asm__("x1")=90,c __asm__("x2")=(uint64_t)request,n __asm__("x16")=381;uint64_t flags;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(n):"memory","cc");return(struct result){a,b,flags};
}
int main(void){setbuf(stdout,NULL);alarm(2);
 size_t page=(size_t)getpagesize();unsigned char*mem=mmap(NULL,page*4,PROT_READ|PROT_WRITE,MAP_ANON|MAP_PRIVATE,-1,0);if(mem==MAP_FAILED)return 2;
 uint64_t*req=(uint64_t*)mem;unsigned char*out=mem+page;
 const char*names[]={"ordinary","readonly_request","null_request","null_output","readonly_output","bad_output","null_policy","unknown_policy","request8","request15","request16","output_tail1","output_tail7","output_tail8"};puts("[");
 for(unsigned i=0;i<sizeof(names)/sizeof(names[0]);i++){
  if(mprotect(mem,page*4,PROT_READ|PROT_WRITE))return 3;
  memset(mem,0,32);memset(out,0xa5,32);memset(mem+3*page-32,0xa5,32);req[0]=0;req[1]=(uintptr_t)out;
  const void*arg=req,*policy="AMFI";unsigned char*observed=out;
  if(i==1&&mprotect(mem,page,PROT_READ))return 4;
  if(i==2)arg=NULL;if(i==3)req[1]=0;
  if(i==4&&mprotect(out,page,PROT_READ))return 4;
  if(i==5)req[1]=1;if(i==6)policy=NULL;if(i==7)policy="PorosMissingPolicy";
  if(i>=8&&i<=10){size_t n=i==8?8:i==9?15:16;arg=mem+3*page-n;memcpy((void*)arg,req,n);if(mprotect(mem+3*page,page,PROT_NONE))return 4;}
  if(i>=11){size_t n=i==11?1:i==12?7:8;req[1]=(uintptr_t)(mem+3*page-n);observed=mem+3*page-32;if(mprotect(mem+3*page,page,PROT_NONE))return 4;}
  uint64_t before[2];memcpy(before,req,16);struct result r=query(policy,arg);
  printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"request_unchanged\":%s,\"output_hex\":\"",i?",":"",names[i],(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false",memcmp(before,req,16)?"false":"true");for(int j=0;j<32;j++)printf("%02x",observed[j]);printf("\"}");
 }
 puts("]");munmap(mem,page*4);return 0;
}
