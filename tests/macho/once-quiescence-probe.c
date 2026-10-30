// Owned once predicate; observe but never modify the kernel's counter.
#include <stdint.h>
#include <stdio.h>
#include <unistd.h>
#include <dlfcn.h>
static unsigned calls;
static void callback(void *p){(*(unsigned*)p)++;}
static uint64_t epoch(void){return *(volatile const uint64_t*)0xfffffc180ULL;}
int main(void){void(*once)(uintptr_t*,void*,void(*)(void*))=dlsym(RTLD_DEFAULT,"_os_once");if(!once)return 2;uintptr_t predicate=0;printf("{\"cases\":[");for(unsigned i=0;i<14;i++){if(i>=4)usleep(10000);uint64_t before=epoch(),old=predicate;unsigned previous=calls;once(&predicate,&calls,callback);uint64_t after=epoch();printf("%s{\"index\":%u,\"delay_us\":%u,\"counter_before\":%llu,\"counter_after\":%llu,\"predicate_before\":\"%llu\",\"predicate_after\":\"%llu\",\"calls_before\":%u,\"calls_after\":%u}",i?",":"",i,i>=4?10000:0,(unsigned long long)before,(unsigned long long)after,(unsigned long long)old,(unsigned long long)predicate,previous,calls);}printf("]}\n");return calls==1?0:3;}
