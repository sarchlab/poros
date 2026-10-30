#include <stdint.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(uint64_t number,uint64_t a0,uint64_t a1,uint64_t a2){
 register uint64_t a __asm__("x0")=a0,b __asm__("x1")=a1,c __asm__("x2")=a2,n __asm__("x16")=number;uint64_t f;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(c),"r"(n):"memory","cc");return(struct result){a,b,f};
}
static void emit(const char*label,struct result r,int comma){printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s}",comma?",\n":"",label,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false");}
int main(void){
 _Static_assert((O_DIRECTORY|O_NOFOLLOW_ANY)==0x20100000,"native flags");
 const struct {const char*label;const void*path;int flags;} cases[]={
 {"root_directory_nofollow","/",O_DIRECTORY|O_NOFOLLOW_ANY},
 {"root_directory","/",O_DIRECTORY},{"root_readonly","/",O_RDONLY},
 {"empty_path","",O_DIRECTORY|O_NOFOLLOW_ANY},
 {"null_path",NULL,O_DIRECTORY|O_NOFOLLOW_ANY},
 {"unmapped_path",(void*)1,O_DIRECTORY|O_NOFOLLOW_ANY},
 {"regular_file_via_symlink_parent","/tmp/poros-macho-pauth/root-directory-probe.c",O_DIRECTORY|O_NOFOLLOW_ANY}};
 puts("[");int comma=0;
 for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){
  struct result r=call(5,(uintptr_t)cases[i].path,cases[i].flags,0);emit(cases[i].label,r,comma++);
  if(!(r.nzcv&(1ull<<29))){
   uint64_t fd=r.x0;
   struct result close_result=call(6,fd,0xabcdef,0);emit("close_owned_directory",close_result,comma++);
   if(close_result.x0||close_result.nzcv&(1ull<<29))return 3;
   emit("close_already_closed",call(6,fd,0xabcdef,0),comma++);
  }
 }
 emit("close_invalid_minus1",call(6,UINT64_MAX,0xabcdef,0),comma++);
 puts("\n]");return 0;
}
