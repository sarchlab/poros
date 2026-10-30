// Read-only launch clock capture. No clock adjustment or target process access.
#include <sys/time.h>
#include <sys/sysctl.h>
#include <mach/mach_time.h>
#include <stdint.h>
#include <stdio.h>
#include <dlfcn.h>
#include <string.h>

static uint64_t field(unsigned offset) {
 return *(volatile uint64_t *)(uintptr_t)(0xfffffc000ULL+offset);
}
static uint64_t counter(unsigned mode) {
 uint64_t value;
 if(mode==3)__asm__ volatile("mrs %0,S3_4_C15_C10_6":"=r"(value)::"memory");
 else if(mode==2)__asm__ volatile("mrs %0,S3_3_C14_C0_6":"=r"(value)::"memory");
 else __asm__ volatile("isb\nmrs %0,CNTVCT_EL0":"=r"(value)::"memory");
 return value;
}
int main(void) {
 uint64_t (*boot)(void)=dlsym(RTLD_DEFAULT,"mach_boottime_usec");
 int (*fast)(struct timeval *)=dlsym(RTLD_DEFAULT,"__commpage_gettimeofday");
 mach_timebase_info_data_t timebase;
 unsigned mode=*(volatile uint8_t *)(uintptr_t)0xfffffc090ULL;
 unsigned continuous=*(volatile uint8_t *)(uintptr_t)0xfffffc091ULL;
 if(!boot||!fast||mode<1||mode>3||continuous>1||mach_timebase_info(&timebase))return 2;
 for(unsigned attempt=0;attempt<64;attempt++){
  uint64_t tuple[5],again[5];
  uint64_t epoch=field(0xa0),absolute=field(0x88),cont=field(0x98),hardware=field(0xa8);
  for(unsigned i=0;i<5;i++)tuple[i]=field(0x120+8*i);
  uint64_t before=counter(mode),absolute_before=mach_absolute_time(),continuous_before=mach_continuous_time();
  struct timeval value={0},sysboot={0};size_t size=sizeof(sysboot);
  int status=fast(&value),sysstatus=sysctlbyname("kern.boottime",&sysboot,&size,NULL,0);
  uint64_t api=boot(),continuous_after=mach_continuous_time(),absolute_after=mach_absolute_time(),after=counter(mode);
  for(unsigned i=0;i<5;i++)again[i]=field(0x120+8*i);
  if(status||memcmp(tuple,again,sizeof(tuple))||epoch!=field(0xa0)||absolute!=field(0x88)||cont!=field(0x98)||hardware!=field(0xa8))continue;
  if(sysstatus||size!=sizeof(sysboot)||sysboot.tv_sec<0||sysboot.tv_usec<0||sysboot.tv_usec>=1000000||value.tv_sec<0||value.tv_usec<0||value.tv_usec>=1000000)return 3;
  uint64_t sysusec=(uint64_t)sysboot.tv_sec*1000000+(uint32_t)sysboot.tv_usec;
  if(!epoch||epoch!=api||epoch!=sysusec||!tuple[0]||after<before||after+absolute<tuple[0]||after+absolute-tuple[0]>=tuple[4])continue;
  printf("{\"user_timebase\":%u,\"continuous_hwclock\":%u,\"numer\":%u,\"denom\":%u,",mode,continuous,timebase.numer,timebase.denom);
  printf("\"boot_time_microseconds\":%llu,\"initial_counter\":%llu,\"absolute_offset\":%llu,\"continuous_offset\":%llu,\"continuous_hardware_offset\":%llu,",epoch,after,absolute,cont,hardware);
  printf("\"time_of_day\":{\"tick\":%llu,\"seconds\":%llu,\"fraction\":%llu,\"scale\":%llu,\"ticks_per_second\":%llu},",tuple[0],tuple[1],tuple[2],tuple[3],tuple[4]);
  printf("\"observation\":{\"counter_before\":%llu,\"counter_after\":%llu,\"absolute_before\":%llu,\"absolute_after\":%llu,\"continuous_before\":%llu,\"continuous_after\":%llu,\"boot_api\":%llu,\"boot_sysctl_usec\":%llu,\"timeofday_sec\":%lld,\"timeofday_usec\":%d}}\n",before,after,absolute_before,absolute_after,continuous_before,continuous_after,api,sysusec,(long long)value.tv_sec,value.tv_usec);
  return 0;
 }
 return 4;
}
