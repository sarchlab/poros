// Read only documented nonsecret CPU/runtime capabilities, never a raw commpage.
// XNU osfmk/arm/cpu_capabilities.h defines these stable ARM64 commpage offsets.
#include <stdint.h>
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
static int read_self(uint64_t address,void *dst,size_t size){mach_vm_size_t got=0;return mach_vm_read_overwrite(mach_task_self(),address,size,(mach_vm_address_t)dst,&got)==KERN_SUCCESS&&got==size;}
static uint64_t u64(const void*p){uint64_t v;memcpy(&v,p,8);return v;}
static uint32_t u32(const void*p){uint32_t v;memcpy(&v,p,4);return v;}
static void hex(const unsigned char *p,size_t n){putchar('"');for(size_t i=0;i<n;i++)printf("%02x",p[i]);putchar('"');}
static int path(uint64_t base,uint64_t size,uint32_t off){if(off==0){printf("null");return 1;}if(off>=size)return 0;char s[1025];size_t i;for(i=0;i<1024&&off+i<size;i++){if(!read_self(base+off+i,s+i,1))return 0;if(s[i]==0)break;}if(i==1024||off+i>=size||s[0]!='/')return 0;putchar('"');for(size_t j=0;j<i;j++){unsigned char c=s[j];if(c=='"'||c=='\\')printf("\\%c",c);else if(c<32)printf("\\u%04x",c);else putchar(c);}putchar('"');return 1;}
static int print_cache_dynamic(void){task_dyld_info_data_t d;mach_msg_type_number_t n=TASK_DYLD_INFO_COUNT;if(task_info(mach_task_self(),TASK_DYLD_INFO,(task_info_t)&d,&n)!=KERN_SUCCESS)return 2;
const struct dyld_all_image_infos*a=(const void*)(uintptr_t)d.all_image_info_addr;
uint64_t base=a->sharedCacheBaseAddress;unsigned char h[512],r[80]={0};if(!read_self(base,h,sizeof(h)))return 3;
uint64_t off=u64(h+496),size=u64(h+504);if(!off||size<80||size>16384||base>UINT64_MAX-off||!read_self(base+off,r,32))return 4;
if(memcmp(r,"dyld_data    v",14)||r[14]<'0'||r[14]>'4'||r[15]!=0)return 7;
unsigned version=r[14]-'0';
if(version>=1&&!read_self(base+off+32,r+32,4))return 8;
if(version>=2&&!read_self(base+off+36,r+36,4))return 9;
if(version>=3&&!read_self(base+off+48,r+48,32))return 10;
printf("{\"cache_uuid\":");hex(a->sharedCacheUUID,16);printf(",\"dynamic_offset\":%llu,\"dynamic_size\":%llu,\"magic_hex\":",off,size);hex(r,16);printf(",\"file_id_hex\":");hex(r+16,16);printf(",\"system_flags_hex\":");hex(r+48,16);printf(",\"processor_flags_hex\":");hex(r+64,16);printf(",\"cryptex_offset\":%u,\"cache_path_offset\":%u,\"cryptex_path\":",u32(r+32),u32(r+36));if(!path(base+off,size,u32(r+32)))return 5;printf(",\"cache_path\":");if(!path(base+off,size,u32(r+36)))return 6;printf("}");return 0;}

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
 if(load) __asm__ volatile("ldar %x[v],[%x[p]]" : [v]"=r"(value) : [p]"r"(data+offset) : "memory");
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
int main(void) {
    if(!pointer_strip_probe())return 5;
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
    for(unsigned load=0;load<2;load++)if(ordered_probe(0,load)!=aligned||ordered_probe(1,load)!=within||ordered_probe(9,load)!=crossing)return 6;
    const char *atomic_alignment=within==0?"within-16-byte":"natural";
    uint32_t csr;
    int (*get_csr)(uint32_t *) = (int (*)(uint32_t *))dlsym(RTLD_DEFAULT, "csr_get_active_config");
    if (!get_csr || get_csr(&csr) != 0) return 2;
    printf("{\"cpu_capabilities\":%llu,\"version\":%u,"
           "\"user_page_shift_64_legacy\":%u,\"cache_line_size\":%u,"
           "\"cpu_family\":%u,\"user_timebase\":%u,"
           "\"continuous_hwclock\":%u,\"kdebug\":%u,"
           "\"timebase_numer\":%u,\"timebase_denom\":%u,\"kernel_page_shift\":%u,\"dyld_flags\":%llu,\"csr_config\":%u,\"atomic_alignment\":\"%s\"",
           *(const volatile uint64_t *)(p + 0x10),
           *(const volatile uint16_t *)(p + 0x1e), p[0x25],
           *(const volatile uint16_t *)(p + 0x26),
           *(const volatile uint32_t *)(p + 0x80), p[0x90], p[0x91],
           *(const volatile uint32_t *)(p + 0x100), tb.numer, tb.denom, p[0x37], *(const volatile uint64_t *)(p + 0x160), csr, atomic_alignment);
    if (ro) printf(",\"read_only_page_shifts\":{\"user\":%u,\"kernel\":%u}",ro_user,ro_kernel);
    printf(",\"pointer_strip\":\"darwin-47-bit\",\"cache_dynamic\":");
    if(print_cache_dynamic()!=0)return 4;
    puts("}");
}
