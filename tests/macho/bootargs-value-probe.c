#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/mman.h>
#include <sys/sysctl.h>
#include <errno.h>
#include <stdlib.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result query(const void*mib,uint64_t count,void*out,void*length,const void*input,uint64_t input_length){
 register uint64_t a __asm__("x0")=(uintptr_t)mib,b __asm__("x1")=count,c __asm__("x2")=(uintptr_t)out,d __asm__("x3")=(uintptr_t)length,e __asm__("x4")=(uintptr_t)input,f __asm__("x5")=input_length,n __asm__("x16")=202;uint64_t flags;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(d),"r"(e),"r"(f),"r"(n):"memory","cc");return(struct result){a,b,flags};
}
int main(void){setbuf(stdout,NULL);alarm(2);
 const char*key="kern.bootargs";int oid[24]={0};size_t count=24;if(sysctlnametomib(key,oid,&count))return 2;
 char value[4096];size_t size=sizeof(value);if(sysctl(oid,(u_int)count,value,&size,NULL,0)||size!=1||value[0]!=0)return 3;
 printf("{\"name\":\"%s\",\"mib\":[",key);for(size_t i=0;i<count;i++)printf("%s%d",i?",":"",oid[i]);printf("],\"value_hex\":\"00\",\"value_length\":%zu,\"cases\":[",size);
 size_t page=(size_t)getpagesize();unsigned char*mem=mmap(NULL,page*3,PROT_READ|PROT_WRITE,MAP_ANON|MAP_PRIVATE,-1,0);if(mem==MAP_FAILED)return 4;
 size_t*len=(size_t*)(mem+2*page);
 const char*names[]={"ordinary","zero_capacity","one_byte_capacity","three_byte_capacity","exact_capacity","eight_byte_capacity","size_only","size_only_zero","readonly_output","bad_output","null_length","bad_mib","null_input_nonzero_length","size_only_null_length"};
 for(unsigned i=0;i<sizeof(names)/sizeof(names[0]);i++){
  if(mprotect(mem,page*3,PROT_READ|PROT_WRITE))return 5;
  memset(mem,0xa5,64);*len=8;void*out=mem,*lp=len;const void*mp=oid;uint64_t newlen=0;
  if(i==1)*len=0;if(i==2)*len=1;if(i==3)*len=3;if(i==4)*len=4;
  if(i==6||i==7)out=NULL;if(i==7)*len=0;
  if(i==8&&mprotect(mem,page,PROT_READ))return 6;
  if(i==9)out=(void*)1;if(i==10)lp=NULL;if(i==11)mp=(void*)1;if(i==12)newlen=4;
  if(i==13){out=NULL;lp=NULL;}
  size_t before=*len;struct result r=query(mp,count,out,lp,NULL,newlen);
  printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"initial_capacity\":%zu,\"final_length\":%zu,\"output_hex\":\"",i?",":"",names[i],(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false",before,*len);for(int j=0;j<32;j++)printf("%02x",mem[j]);printf("\"}");
 }
 puts("]}");munmap(mem,page*3);return 0;
}
