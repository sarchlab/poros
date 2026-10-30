#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/mman.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result query(const void *policy,const void *request){
 register uint64_t a __asm__("x0")=(uint64_t)policy,b __asm__("x1")=2,c __asm__("x2")=(uint64_t)request,n __asm__("x16")=381;uint64_t flags;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(n):"memory","cc");return(struct result){a,b,flags};
}
int main(void){
 size_t page=(size_t)getpagesize();unsigned char *memory=mmap(NULL,4*page,PROT_READ|PROT_WRITE,MAP_ANON|MAP_PRIVATE,-1,0);if(memory==MAP_FAILED)return 2;
 uint64_t *request=(uint64_t*)memory;unsigned char *output=memory+page;char *operation=(char*)(memory+2*page);strcpy(operation,"syscall-unix");
 const char*names[]={"zero_output","sentinel_output","readonly_request","null_request","null_output","readonly_output","null_policy","unknown_policy","bad_operation","truncated_request","request48","request56","request64","request80","request96","request112","request128","request144","request152","request160","request159"};puts("[");
 for(unsigned i=0;i<sizeof(names)/sizeof(names[0]);i++){
  if(mprotect(memory,4*page,PROT_READ|PROT_WRITE))return 3;
  memset(request,0,160);memset(output,i?0xa5:0,32);request[0]=(uint64_t)output;request[1]=(uint64_t)getpid();request[2]=(uint64_t)operation;request[3]=0x41;request[4]=550;request[5]=1;
  const void*arg=request;const void*policy="Sandbox";uint64_t before[20];memcpy(before,request,160);
  if(i==2&&mprotect(memory,page,PROT_READ))return 4;
  if(i==3)arg=NULL;
  if(i==4)request[0]=0;
  if(i==5&&mprotect(output,page,PROT_READ))return 4;
  if(i==6)policy=NULL;
  if(i==7)policy="PorosMissingPolicy";
  if(i==8)request[2]=1;
  if(i>=9){const size_t sizes[]={8,48,56,64,80,96,112,128,144,152,160,159};size_t count=sizes[i-9];arg=memory+3*page-count;memcpy((void*)arg,request,count);if(mprotect(memory+3*page,page,PROT_NONE))return 4;}
  if(i!=2)memcpy(before,request,160);
  struct result r=query(policy,arg);
  printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s,\"request_unchanged\":%s,\"output_hex\":\"",i?",\n":"",names[i],(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false",memcmp(before,request,160)?"false":"true");for(int j=0;j<32;j++)printf("%02x",output[j]);printf("\"}");
 }
 puts("\n]");munmap(memory,4*page);return 0;
}
