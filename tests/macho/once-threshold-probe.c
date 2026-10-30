// Synthetic owned completed-generation predicates; kernel counter is read only.
#include <stdint.h>
#include <stdio.h>
#include <dlfcn.h>
static void callback(void *p){(*(unsigned*)p)++;}
static uint64_t epoch(void){return *(volatile const uint64_t*)0xfffffc180ULL;}
int main(void){void(*once)(uintptr_t*,void*,void(*)(void*))=dlsym(RTLD_DEFAULT,"_os_once");if(!once)return 2;unsigned deltas[]={0,1,2,3,4,5,8};printf("{\"synthetic_completed_generation_cases\":[");for(unsigned i=0;i<7;i++){unsigned tries,calls=0;uint64_t before=0,after=0,predicate=0,old=0;for(tries=0;tries<100;tries++){before=epoch();if(before<deltas[i])return 3;predicate=((before-deltas[i])<<2)|1;old=predicate;once((uintptr_t*)&predicate,&calls,callback);after=epoch();if(before==after)break;}if(tries==100)return 4;printf("%s{\"delta\":%u,\"counter_before\":%llu,\"counter_after\":%llu,\"predicate_before\":\"%llu\",\"predicate_after\":\"%llu\",\"callback_calls\":%u,\"attempts\":%u}",i?",":"",deltas[i],(unsigned long long)before,(unsigned long long)after,(unsigned long long)old,(unsigned long long)predicate,calls,tries+1);}printf("]}\n");return 0;}
