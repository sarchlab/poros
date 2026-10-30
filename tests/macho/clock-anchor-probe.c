#include <sys/time.h>
#include <sys/sysctl.h>
#include <mach/mach_time.h>
#include <stdint.h>
#include <stdio.h>
#include <dlfcn.h>
#include <string.h>
static uint64_t cp(unsigned offset){return *(volatile uint64_t*)(uintptr_t)(0xfffffc000ULL+offset);}
static uint64_t raw(void){uint64_t v;__asm__ volatile("isb\nmrs %0,CNTVCT_EL0":"=r"(v)::"memory");return v;}
int main(void){uint64_t(*bootfn)(void)=dlsym(RTLD_DEFAULT,"mach_boottime_usec");int(*fast)(struct timeval*)=dlsym(RTLD_DEFAULT,"__commpage_gettimeofday");if(!bootfn||!fast)return 2;mach_timebase_info_data_t tb; if(mach_timebase_info(&tb))return 3;printf("{\"numer\":%u,\"denom\":%u,\"samples\":[",tb.numer,tb.denom);for(unsigned round=0;round<3;round++){uint64_t v[5],again[5];uint64_t boot_before=cp(0xa0),offset=cp(0x88),cont=cp(0x98),hw=cp(0xa8);for(unsigned i=0;i<5;i++)v[i]=cp(0x120+8*i);uint64_t rb=raw(),ab=mach_absolute_time(),cb=mach_continuous_time();struct timeval now={0},sysboot={0};int fr=fast(&now);size_t bs=sizeof(sysboot);int sr=sysctlbyname("kern.boottime",&sysboot,&bs,NULL,0);uint64_t api=bootfn(),ca=mach_continuous_time(),aa=mach_absolute_time(),ra=raw();for(unsigned i=0;i<5;i++)again[i]=cp(0x120+8*i);uint64_t boot_after=cp(0xa0);if(memcmp(v,again,sizeof(v))||boot_before!=boot_after||offset!=cp(0x88)||cont!=cp(0x98)||hw!=cp(0xa8)||fr||sr||bs!=sizeof(sysboot))return 4;printf("%s{\"boot_before\":%llu,\"boot_after\":%llu,\"boot_api\":%llu,\"boot_sysctl_usec\":%llu,\"absolute_offset\":%llu,\"continuous_offset\":%llu,\"continuous_hw_offset\":%llu,\"tuple\":[",round?",":"",boot_before,boot_after,api,(uint64_t)sysboot.tv_sec*1000000+sysboot.tv_usec,offset,cont,hw);for(unsigned i=0;i<5;i++)printf("%s%llu",i?",":"",v[i]);printf("],\"counter_before\":%llu,\"counter_after\":%llu,\"absolute_before\":%llu,\"absolute_after\":%llu,\"continuous_before\":%llu,\"continuous_after\":%llu,\"timeofday_sec\":%lld,\"timeofday_usec\":%d}",rb,ra,ab,aa,cb,ca,(long long)now.tv_sec,now.tv_usec);}printf("]}\n");}
