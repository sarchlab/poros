// Read only documented nonsecret CPU/runtime capabilities, never a raw commpage.
// XNU osfmk/arm/cpu_capabilities.h defines these stable ARM64 commpage offsets.
#include "macho-stat.h"
#include <stdint.h>
#include <stddef.h>
#include <fcntl.h>
#include <sys/stat.h>
#include <errno.h>
#include <sys/sysctl.h>
#include <mach-o/dyld_images.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>
#include <signal.h>
#include <dlfcn.h>
#include <stdio.h>
#include <mach/mach_time.h>
#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <mach/mach_host.h>
static int read_self(uint64_t address,void *dst,size_t size){mach_vm_size_t got=0;return mach_vm_read_overwrite(mach_task_self(),address,size,(mach_vm_address_t)dst,&got)==KERN_SUCCESS&&got==size;}
static uint64_t u64(const void*p){uint64_t v;memcpy(&v,p,8);return v;}
static uint32_t u32(const void*p){uint32_t v;memcpy(&v,p,4);return v;}
static void hex(const unsigned char *p,size_t n){putchar('"');for(size_t i=0;i<n;i++)printf("%02x",p[i]);putchar('"');}
static int path(uint64_t base,uint64_t size,uint32_t off){if(off==0){printf("null");return 1;}if(off>=size)return 0;char s[1025];size_t i;for(i=0;i<1024&&off+i<size;i++){if(!read_self(base+off+i,s+i,1))return 0;if(s[i]==0)break;}if(i==1024||off+i>=size||s[0]!='/')return 0;putchar('"');for(size_t j=0;j<i;j++){unsigned char c=s[j];if(c=='"'||c=='\\')printf("\\%c",c);else if(c<32)printf("\\u%04x",c);else putchar(c);}putchar('"');return 1;}
// Capture only the 16-byte strlen read envelope around the bounded path.
static int path_window(uint64_t base,uint64_t size,uint32_t off){
 if(off==0){printf("\"\"");return 1;}if(off>=size)return 0;
 size_t length=0;unsigned char c=1;
 while(length<1025&&off+length<size){if(!read_self(base+off+length,&c,1))return 0;length++;if(c==0)break;}
 if(c!=0)return 0;uint64_t start=(uint64_t)off&~15ULL,end=((uint64_t)off+length+15)&~15ULL;
 unsigned char window[1056];if(end>size||end-start>sizeof(window)||!read_self(base+start,window,(size_t)(end-start)))return 0;
 hex(window,(size_t)(end-start));return 1;
}
static int print_native_dyld(void) {
 task_dyld_info_data_t d;mach_msg_type_number_t n=TASK_DYLD_INFO_COUNT;
 if(task_info(mach_task_self(),TASK_DYLD_INFO,(task_info_t)&d,&n)!=KERN_SUCCESS)return 1;
 const struct dyld_all_image_infos*a=(const void*)(uintptr_t)d.all_image_info_addr;
 uint64_t base=(uint64_t)(uintptr_t)a->dyldImageLoadAddress;unsigned char h[32];
 if(!read_self(base,h,32)||u32(h)!=0xfeedfacf)return 2;
 uint32_t count=u32(h+16),size=u32(h+20),offset=32;unsigned char uuid[16];unsigned found=0;
 if(count>4096||size>65536)return 3;
 for(unsigned i=0;i<count;i++) {unsigned char command[8];if(offset>32+size||32+size-offset<8||!read_self(base+offset,command,8))return 4;
 uint32_t length=u32(command+4);if(length<8||length>32+size-offset)return 5;
 if(u32(command)==0x1b){if(length!=24||found++||!read_self(base+offset+8,uuid,16))return 6;}
 offset+=length;
 }
 if(found!=1)return 7;
 printf("{\"uuid\":");hex(uuid,16);printf(",\"subtype\":%u}",u32(h+8));return 0;
}
// Selected v4 roots fields, identified in the matching installed dyld methods.
// Read only the bounded descriptors and bitmaps, not the unknown header gap.
static int print_cache_roots(uint64_t base,uint64_t size) {
 unsigned char header[5],desc[16],image[8192],cryptex[8192];
 if(!read_self(base+80,header,5)||!read_self(base+96,desc,16))return 0;
 uint32_t count=u32(header),a=u32(desc),an=u32(desc+4),b=u32(desc+8),bn=u32(desc+12);
 if(!(header[4]&1)||!count||count>65536||an!=(count+7)/8||bn!=an||a<112||b<112||(uint64_t)a+an>size||(uint64_t)b+bn>size||((uint64_t)a<b+(uint64_t)bn&&(uint64_t)b<a+(uint64_t)an))return 0;
 if(!read_self(base+a,image,an)||!read_self(base+b,cryptex,bn))return 0;
 printf(",\"roots\":{\"roots_flags\":%u,\"image_count\":%u,\"image_roots_offset\":%u,\"image_roots_size\":%u,\"cryptex_roots_offset\":%u,\"cryptex_roots_size\":%u,\"image_roots_hex\":",header[4],count,a,an,b,bn);hex(image,an);printf(",\"cryptex_roots_hex\":");hex(cryptex,bn);printf("}");return 1;
}
static int print_cache_dynamic(void){task_dyld_info_data_t d;mach_msg_type_number_t n=TASK_DYLD_INFO_COUNT;if(task_info(mach_task_self(),TASK_DYLD_INFO,(task_info_t)&d,&n)!=KERN_SUCCESS)return 2;
const struct dyld_all_image_infos*a=(const void*)(uintptr_t)d.all_image_info_addr;
uint64_t base=a->sharedCacheBaseAddress;unsigned char h[512],r[96]={0};if(!read_self(base,h,sizeof(h)))return 3;
uint64_t off=u64(h+496),size=u64(h+504);if(!off||size<80||size>16384||base>UINT64_MAX-off||!read_self(base+off,r,32))return 4;
if(memcmp(r,"dyld_data    v",14)||r[14]<'0'||r[14]>'4'||r[15]!=0)return 7;
unsigned version=r[14]-'0';
if(version>=1&&!read_self(base+off+32,r+32,4))return 8;
if(version>=2&&!read_self(base+off+36,r+36,4))return 9;
if(version>=3&&!read_self(base+off+48,r+48,32))return 10;
if(version>=4&&(size<112||!read_self(base+off+88,r+88,8)))return 11;
printf("{\"cache_uuid\":");hex(a->sharedCacheUUID,16);printf(",\"dynamic_offset\":%llu,\"dynamic_size\":%llu,\"magic_hex\":",off,size);hex(r,16);printf(",\"file_id_hex\":");hex(r+16,16);printf(",\"system_flags_hex\":");hex(r+48,16);printf(",\"processor_flags_hex\":");hex(r+64,16);printf(",\"cryptex_offset\":%u,\"cache_path_offset\":%u,\"cryptex_path\":",u32(r+32),u32(r+36));if(!path(base+off,size,u32(r+32)))return 5;printf(",\"cache_path\":");if(!path(base+off,size,u32(r+36)))return 6;if(version>=4)printf(",\"cryptex_length\":%u,\"cache_path_length\":%u",u32(r+88),u32(r+92));if(u32(r+32)){printf(",\"cryptex_window_hex\":");if(!path_window(base+off,size,u32(r+32)))return 12;}if(u32(r+36)){printf(",\"cache_path_window_hex\":");if(!path_window(base+off,size,u32(r+36)))return 13;}if(version==4&&!print_cache_roots(base+off,size))return 14;printf("}");return 0;}

static int atomic_probe(unsigned offset) {
    pid_t child=fork(); if(child<0)return -1;
    if(child==0) {
        _Alignas(16) uint8_t data[32]={0}; uint64_t compare=0,replacement=9;
        __asm__ volatile(".arch armv8.1-a\n\tcasal %x[c],%x[v],[%x[p]]"
            : [c]"+&r"(compare) : [v]"r"(replacement),[p]"r"(data+offset) : "memory");
        _exit(compare==0?0:1);
    }
    int status; if(waitpid(child,&status,0)!=child)return -1;
    if(WIFEXITED(status)&&WEXITSTATUS(status)==0)return 0;
    if(WIFSIGNALED(status)&&WTERMSIG(status)==SIGBUS)return 1;
    return -1;
}
static int ordered_probe(unsigned offset, unsigned load) {
 pid_t child=fork(); if(child<0)return -1;
 if(child==0) { _Alignas(16) unsigned char data[32]={0}; uint64_t value=9;
 if(load==2) __asm__ volatile(".arch armv8.3-a\n\tldapr %x[v],[%x[p]]" : [v]"=r"(value) : [p]"r"(data+offset) : "memory");
 else if(load) __asm__ volatile("ldar %x[v],[%x[p]]" : [v]"=r"(value) : [p]"r"(data+offset) : "memory");
 else __asm__ volatile("stlr %x[v],[%x[p]]" :: [v]"r"(value),[p]"r"(data+offset) : "memory");
 _exit(0); }
 int status;if(waitpid(child,&status,0)!=child)return -1;
 if(WIFEXITED(status)&&WEXITSTATUS(status)==0)return 0;
 if(WIFSIGNALED(status)&&WTERMSIG(status)==SIGBUS)return 1;
 return -1;
}
// Read the effective XPAC address/tag policy, independently of PAC enablement.
static int pointer_strip_probe(void) {
 const uint64_t input[]={0x0000ffffffffffffULL,0xab00123456789abcULL,0x0080123456789abcULL};
 const uint64_t data[]={0x00007fffffffffffULL,0xab00123456789abcULL,0xffff923456789abcULL};
 const uint64_t instruction[]={0x00007fffffffffffULL,0x0000123456789abcULL,0xffff923456789abcULL};
 for(unsigned i=0;i<3;i++) { uint64_t d=input[i],a=input[i];
 __asm__ volatile(".arch armv8.3-a\n\txpacd %0" : "+r"(d));
 __asm__ volatile(".arch armv8.3-a\n\txpaci %0" : "+r"(a));
 if(d!=data[i]||a!=instruction[i])return 0;
 } return 1;
}
static int exclusive_signature(void) {
 _Alignas(128) uint32_t data[64]={0};uint32_t *p=data,*q=data+32;
 for(unsigned kind=0;kind<4;kind++) {
 uint32_t loaded,status,compare=5,value=9;*p=7;*q=0;
 if(kind==0)__asm__ volatile(".arch armv8.1-a\nldxr %w[l],[%x[p]]\ncas %w[c],%w[v],[%x[q]]\nstxr %w[s],%w[v],[%x[p]]" : [l]"=&r"(loaded),[s]"=&r"(status),[c]"+&r"(compare):[p]"r"(p),[q]"r"(q),[v]"r"(value):"memory");
 else if(kind==1)__asm__ volatile(".arch armv8.1-a\nldxr %w[l],[%x[p]]\nswp %w[v],%w[c],[%x[q]]\nstxr %w[s],%w[v],[%x[p]]" : [l]"=&r"(loaded),[s]"=&r"(status),[c]"=&r"(compare):[p]"r"(p),[q]"r"(q),[v]"r"(value):"memory");
 else if(kind==2)__asm__ volatile(".arch armv8.1-a\nldxr %w[l],[%x[p]]\nldadd wzr,%w[c],[%x[q]]\nstxr %w[s],%w[v],[%x[p]]" : [l]"=&r"(loaded),[s]"=&r"(status),[c]"=&r"(compare):[p]"r"(p),[q]"r"(q),[v]"r"(value):"memory");
 else __asm__ volatile("ldxr %w[l],[%x[p]]\nmov x16,#20\nsvc #0x80\nstxr %w[s],%w[v],[%x[p]]" : [l]"=&r"(loaded),[s]"=&r"(status):[p]"r"(p),[v]"r"(value):"x0","x1","x16","cc","memory");
 if(status!=1||loaded!=7||*p!=7||*q!=(kind==1?9u:0u))return 0;
 }
 return 1;
}
// Identify the measured local exclusive-monitor granule with real instructions.
static int exclusive_probe(void) {
 _Alignas(128) uint32_t data[64];
 for(unsigned kind=0;kind<2;kind++)for(unsigned offset=0;offset<=128;offset+=4) {
 unsigned successes=0;uint32_t *p=data,*q=data+offset/4;
 for(unsigned attempt=0;attempt<16;attempt++) { *p=7;*q=0;uint32_t old,status,value=9,ordinary=11;
 if(kind==0)__asm__ volatile("ldxr %w[o],[%x[p]]\n\tstr %w[t],[%x[q]]\n\tstxr %w[s],%w[v],[%x[p]]" : [o]"=&r"(old),[s]"=&r"(status):[p]"r"(p),[q]"r"(q),[v]"r"(value),[t]"r"(ordinary):"memory");
 else __asm__ volatile("ldxr %w[o],[%x[p]]\n\tldxr %w[o],[%x[q]]\n\tstxr %w[s],%w[v],[%x[p]]" : [o]"=&r"(old),[s]"=&r"(status):[p]"r"(p),[q]"r"(q),[v]"r"(value):"memory");
 if(status>1)return 0;if(status==0){if(*p!=9)return 0;successes++;}
 }
 unsigned expected=kind==0?offset>=64:offset<64;
 if((successes!=0)!=expected)return 0;
 }
 return 1;
}
static int print_sysctl_names(void) {
 const char *names[]={"security.mac.lockdown_mode_state","kern.bootargs"};
 printf("[");
 for(unsigned entry=0;entry<2;entry++) {
  int mib[24];size_t count=24;
  if(sysctlnametomib(names[entry],mib,&count)||count==0||count>24)return 1;
  printf("%s{\"name\":\"%s\",\"mib\":[",entry?",":"",names[entry]);
  for(size_t i=0;i<count;i++)printf("%s%d",i?",":"",mib[i]);
  if(entry==0) {int32_t value;size_t size=sizeof(value);
   if(sysctl(mib,(u_int)count,&value,&size,NULL,0)||size!=sizeof(value))return 2;
   printf("],\"int32\":%d}",value);
  } else {char value[1024];size_t size=sizeof(value);
   // Do not emit potentially private custom boot arguments. The native empty
   // setting is the currently captured profile; other settings fail explicitly.
   if(sysctl(mib,(u_int)count,value,&size,NULL,0)||size!=1||value[0]!=0)return 3;
   printf("],\"string\":\"\"}");
  }
 }
 printf("]");return 0;
}

// Selected runtime directories only; no directory listings or host namespace
// access is granted to the guest. Both aliases must identify the same node.
static int print_runtime_directories(void) {
 const char *names[]={"/System/Cryptexes/OS","/System/Cryptexes/OS/System/Library/dyld","/System/Cryptexes/Rosetta"};
 printf("[");unsigned emitted=0;
 for(unsigned entry=0;entry<sizeof(names)/sizeof(names[0]);entry++) {
 const char *name=names[entry];char canonical[1024]={0};struct stat st,other;
 int fd=open(name,O_RDONLY|O_DIRECTORY);
 // Rosetta is optional on ARM64 hosts. Omission leaves guest lookup explicitly
 // unsupported; it does not assert a negative guest namespace entry.
 if(fd<0) {if(entry==2&&errno==ENOENT)continue;return 1;}
 int failed=fstat(fd,&st)||!S_ISDIR(st.st_mode)||fcntl(fd,F_GETPATH,canonical);
 close(fd);if(failed||canonical[0]!='/'||strnlen(canonical,sizeof(canonical))==sizeof(canonical))return 2;
 fd=open(name,O_RDONLY|O_DIRECTORY|O_NOFOLLOW_ANY);int symlink=0;
 if(fd<0) {if(errno!=ELOOP)return 3;symlink=1;}
 else {failed=fstat(fd,&other)||other.st_dev!=st.st_dev||other.st_ino!=st.st_ino;close(fd);if(failed)return 4;}
 fd=open(canonical,O_RDONLY|O_DIRECTORY|O_NOFOLLOW_ANY);if(fd<0)return 5;
 failed=fstat(fd,&other)||other.st_dev!=st.st_dev||other.st_ino!=st.st_ino;close(fd);if(failed)return 6;
 printf("%s{\"path\":\"%s\",\"canonical_path\":\"",emitted++?",":"",name);
 for(const unsigned char *p=(const unsigned char *)canonical;*p;p++) {if(*p=='\"'||*p=='\\')printf("\\%c",*p);else if(*p<32)printf("\\u%04x",*p);else putchar(*p);}
 printf("\",\"symlink\":%s,\"stat\":",symlink?"true":"false");if(print_file_stat(&st))return 7;printf("}");
 }
 printf("]");return 0;
}
// Only observed stat ENOENT is retained; this does not claim lstat/open absence.
static int print_stat_enoent_paths(void){
 const char*paths[]={"/usr/lib/libSystem.B.dylib"};unsigned emitted=0;
 printf("[");for(unsigned i=0;i<sizeof(paths)/sizeof(paths[0]);i++){
  struct stat metadata;errno=0;int result=stat(paths[i],&metadata);
  if(result==0)continue;
  if(errno!=ENOENT){fprintf(stderr,"selected runtime stat observation failed: %d\n",errno);return 1;}
  printf("%s\"%s\"",emitted++?",":"",paths[i]);
 }printf("]");return 0;
}
static int print_access_enoent_paths(void){
 const char *name="/AppleInternal/XBS/.isChrooted";errno=0;int result=access(name,F_OK);
 if(result==0){printf("[]");return 0;}
 if(errno!=ENOENT){fprintf(stderr,"selected access observation failed: %d\n",errno);return 1;}
 printf("[\"%s\"]",name);return 0;
}
int main(void) {
    host_priority_info_data_t priority={0};mach_msg_type_number_t priority_count=HOST_PRIORITY_INFO_COUNT;
    mach_port_t host=mach_host_self();kern_return_t priority_result=host_info(host,HOST_PRIORITY_INFO,(host_info_t)&priority,&priority_count);
    kern_return_t released_host=mach_port_deallocate(mach_task_self(),host);
    if(priority_result!=KERN_SUCCESS || priority_count!=8 || released_host!=KERN_SUCCESS)return 31;
    if(!pointer_strip_probe()||!exclusive_probe()||!exclusive_signature())return 5;
    const volatile uint8_t *p = (const volatile uint8_t *)0xfffffc000ULL;
    mach_timebase_info_data_t tb;
    if (mach_timebase_info(&tb) != 0) return 1;
    uint8_t ro_user = 0, ro_kernel = 0;
    mach_vm_size_t got;
    int ro = mach_vm_read_overwrite(mach_task_self(), 0xfffff4025ULL, 1,
                  (mach_vm_address_t)&ro_user, &got) == KERN_SUCCESS && got == 1;
    ro = ro && mach_vm_read_overwrite(mach_task_self(), 0xfffff4037ULL, 1,
                  (mach_vm_address_t)&ro_kernel, &got) == KERN_SUCCESS && got == 1;
    int aligned=atomic_probe(0),within=atomic_probe(1),crossing=atomic_probe(9);
    if(aligned!=0 || crossing!=1 || within<0)return 3;
    for(unsigned load=0;load<3;load++)if(ordered_probe(0,load)!=aligned||ordered_probe(1,load)!=within||ordered_probe(9,load)!=crossing)return 6;
    const char *atomic_alignment=within==0?"within-16-byte":"natural";
    if(p[0x92]>1)return 15;
    // Exact byte read by this native libdyld TPRO entry; this offset is not a
    // published XNU field. The collector binds it to native dyld/cache identity.
    printf("{\"host_priorities\":[");for(unsigned i=0;i<8;i++)printf("%s%d",i?",":"",((int*)&priority)[i]);
    printf("],\"thread_protection_capability_byte\":%u,\"dtrace_dof_enabled\":%u,",p[0x10c],p[0x92]);
    uint32_t csr;
    int (*get_csr)(uint32_t *) = (int (*)(uint32_t *))dlsym(RTLD_DEFAULT, "csr_get_active_config");
    if (!get_csr || get_csr(&csr) != 0) return 2;
    printf("\"cpu_capabilities\":%llu,\"version\":%u,"
           "\"user_page_shift_64_legacy\":%u,\"cache_line_size\":%u,"
           "\"cpu_family\":%u,\"user_timebase\":%u,"
           "\"continuous_hwclock\":%u,\"kdebug\":%u,"
           "\"timebase_numer\":%u,\"timebase_denom\":%u,\"kernel_page_shift\":%u,\"dyld_flags\":%llu,\"csr_config\":%u,\"atomic_alignment\":\"%s\"",
           *(const volatile uint64_t *)(p + 0x10),
           *(const volatile uint16_t *)(p + 0x1e), p[0x25],
           *(const volatile uint16_t *)(p + 0x26),
           *(const volatile uint32_t *)(p + 0x80), p[0x90], p[0x91],
           *(const volatile uint32_t *)(p + 0x100), tb.numer, tb.denom, p[0x37], *(const volatile uint64_t *)(p + 0x160), csr, atomic_alignment);
    uint32_t ro_devfirm=0;
    if(read_self(0xfffff4084ULL,&ro_devfirm,sizeof(ro_devfirm)))printf(",\"read_only_development_firmware\":%u",ro_devfirm);
    printf(",\"development_firmware\":%u",*(const volatile uint32_t *)(p+0x84));
    if (ro) printf(",\"read_only_page_shifts\":{\"user\":%u,\"kernel\":%u}",ro_user,ro_kernel);
    printf(",\"exclusive_monitor\":\"m2-local-64-byte\",\"native_dyld\":");if(print_native_dyld()!=0)return 7;
    printf(",\"pointer_strip\":\"darwin-47-bit\",\"cache_dynamic\":");
    if(print_cache_dynamic()!=0)return 4;
    printf(",\"sysctl_names\":");if(print_sysctl_names()!=0)return 8;
    printf(",\"directories\":");int directory_error=print_runtime_directories();if(directory_error){fprintf(stderr,"runtime directory capture failed at check %d\n",directory_error);return 9;}
    printf(",\"stat_enoent_paths\":");if(print_stat_enoent_paths()!=0)return 10;
 printf(",\"access_enoent_paths\":");if(print_access_enoent_paths()!=0)return 10;
    puts("}");
}
