// Read only documented nonsecret CPU/runtime capabilities, never a raw commpage.
// XNU osfmk/arm/cpu_capabilities.h defines these stable ARM64 commpage offsets.
#include <stdint.h>
#include <sys/wait.h>
#include <unistd.h>
#include <signal.h>
#include <dlfcn.h>
#include <stdio.h>
#include <mach/mach_time.h>
#include <mach/mach.h>
#include <mach/mach_vm.h>
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
int main(void) {
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
    puts("}");
}
