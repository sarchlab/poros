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
 const char *key="security.mac.lockdown_mode_state";int native_oid[24]={0};size_t count=24;if(sysctlnametomib(key,native_oid,&count))return 2;
 int32_t value=-1;size_t value_length=4;errno=0;int value_result=sysctl(native_oid,(u_int)count,&value,&value_length,NULL,0);int value_errno=errno;
 printf("{\"name\":\"%s\",\"mib\":[",key);for(size_t i=0;i<count;i++)printf("%s%d",i?",":"",native_oid[i]);printf("],\"value_result\":%d,\"value_errno\":%d,\"value\":%d,\"value_length\":%zu,\"cases\":[\n",value_result,value_errno,value,value_length);
 size_t page=(size_t)getpagesize();unsigned char*memory=mmap(NULL,2*page,PROT_READ|PROT_WRITE,MAP_ANON|MAP_PRIVATE,-1,0);if(memory==MAP_FAILED)return 3;size_t*length=(size_t*)(memory+page);int selector[2]={0,3};
 const char*names[]={"ordinary","include_nul","trailing_dot","embedded_nul_tail","size_only","zero_capacity","one_byte_capacity","four_byte_capacity","short_capacity","exact_capacity","readonly_output","null_length","bad_mib","bad_new_pointer","empty_input","too_long_input","unknown_name"};
 unsigned emitted=0;for(unsigned i=0;i<sizeof(names)/sizeof(names[0]);i++){
  if(mprotect(memory,2*page,PROT_READ|PROT_WRITE))return 4;memset(memory,0xa5,64);*length=56;
  const void*mib=selector,*input=key;void*out=memory,*lenptr=length;uint64_t input_length=strlen(key);
  if(i==1)input_length++;
  if(i==2){input="security.mac.lockdown_mode_state.";input_length=strlen(input);}
  if(i==3){input="security.mac.lockdown_mode_state\0garbage";input_length=40;}
  if(i==4)out=NULL;
  if(i==5)*length=0;
  if(i==6)*length=1;
  if(i==7)*length=4;
  if(i==8)*length=count*4-1;
  if(i==9)*length=count*4;
  if(i==10&&mprotect(memory,page,PROT_READ))return 5;
  if(i==11)lenptr=NULL;
  if(i==12)mib=(void*)1;
  if(i==13)input=(void*)1;
  if(i==14)input_length=0;
  if(i==15)input_length=1024;
  if(i==16){input="poros.nonexistent";input_length=strlen(input);}
  size_t before=*length;struct result r=query(mib,2,out,lenptr,input,input_length);
  printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"initial_capacity\":%zu,\"final_length\":%zu,\"output_hex\":\"",emitted++?",\n":"",names[i],(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false",before,*length);for(int j=0;j<32;j++)printf("%02x",memory[j]);printf("\"}");
 }
 puts("\n]}");munmap(memory,2*page);return 0;
}
