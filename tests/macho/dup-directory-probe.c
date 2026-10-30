#include <stdint.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/stat.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(uint64_t number,uint64_t a0){
 register uint64_t a __asm__("x0")=a0,b __asm__("x1")=0xabcdef,n __asm__("x16")=number;uint64_t f;
 __asm__ volatile("svc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(f):"r"(n):"memory","cc");return(struct result){a,b,f};
}
static void emit(const char*label,struct result r,int comma){printf("%s{\"case\":\"%s\",\"x0\":%llu,\"x1\":%llu,\"carry\":%s}",comma?",\n":"",label,(unsigned long long)r.x0,(unsigned long long)r.x1,(r.nzcv&(1ull<<29))?"true":"false");}
int main(void){
 int root=open("/",O_DIRECTORY|O_NOFOLLOW_ANY),fd=open("/System/Cryptexes/OS",O_DIRECTORY);
 if(root!=3||fd!=4)return 2;
 struct stat expected,actual;if(fstat(fd,&expected))return 3;
 puts("[");struct result r=call(41,fd);emit("duplicate_directory",r,0);
 if(r.x0!=5||r.nzcv&(1ull<<29))return 4;
 emit("close_original",call(6,fd),1);
 if(fstat(5,&actual)||actual.st_dev!=expected.st_dev||actual.st_ino!=expected.st_ino)return 5;
 r=call(41,5);emit("duplicate_survives_close_and_reuses_lowest",r,1);
 if(r.x0!=4||r.nzcv&(1ull<<29))return 6;
 emit("close_first_duplicate",call(6,5),1);
 r=call(41,0x100000004ULL);emit("duplicate_low_signed32_fd",r,1);
 if(r.x0!=5||r.nzcv&(1ull<<29))return 7;
 if(fstat(5,&actual)||actual.st_dev!=expected.st_dev||actual.st_ino!=expected.st_ino)return 8;
 emit("duplicate_invalid_minus1",call(41,UINT64_MAX),1);
 emit("duplicate_invalid_minus2",call(41,UINT64_MAX-1),1);
 emit("duplicate_unopened_fd",call(41,500),1);
 emit("close_last_alias4",call(6,4),1);
 emit("close_last_alias5",call(6,5),1);
 emit("duplicate_closed_fd",call(41,4),1);
 emit("close_root",call(6,root),1);
 puts("\n]");return 0;
}
