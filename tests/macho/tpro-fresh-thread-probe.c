// Read-only register observation at the first instruction of an owned kernel-created
// thread. No pthread/libSystem initialization runs on that thread. Not an exec-entry trace.
#include <mach/mach.h>
#include <mach/arm/thread_status.h>
#include <stdint.h>
#include <stdio.h>
#include <sys/mman.h>
#include <stdatomic.h>
#include <unistd.h>
struct result {uint64_t value;_Atomic uint64_t ready;};
__attribute__((naked)) static void capture(void){__asm__ volatile("mrs x1, S3_6_C15_C1_5\nstr x1,[x0]\nmov x1,#1\nadd x2,x0,#8\nstlr x1,[x2]\n1:yield\nb 1b");}
int main(void){uint8_t c=*(const volatile uint8_t*)0xfffffc10cULL;if(c!=3)return 2;void*stack=mmap(0,65536,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(stack==MAP_FAILED)return 3;struct result r={0};arm_thread_state64_t state={0};state.__x[0]=(uint64_t)&r;arm_thread_state64_set_pc_fptr(state,capture);arm_thread_state64_set_sp(state,(char*)stack+65536);thread_act_t thread=0;kern_return_t kr=thread_create_running(mach_task_self(),ARM_THREAD_STATE64,(thread_state_t)&state,ARM_THREAD_STATE64_COUNT,&thread);if(kr){printf("{\"create_error\":%d}\n",kr);munmap(stack,65536);return 4;}for(unsigned i=0;i<1000&&!atomic_load_explicit(&r.ready,memory_order_acquire);i++)usleep(1000);int ready=atomic_load_explicit(&r.ready,memory_order_acquire);kern_return_t terminated=thread_terminate(thread);kern_return_t deallocated=mach_port_deallocate(mach_task_self(),thread);mach_port_type_t kind=0;kern_return_t lookup=mach_port_type(mach_task_self(),thread,&kind);printf("{\"stage\":\"first-instruction-of-new-mach-thread\",\"capability\":%u,\"ready\":%d,\"terminate\":%d,\"deallocate\":%d,\"lookup_after\":%d,\"value\":\"0x%016llx\"}\n",c,ready,terminated,deallocated,lookup,(unsigned long long)r.value);if(!terminated)munmap(stack,65536);return ready&&!terminated&&(deallocated==0||(deallocated==KERN_INVALID_NAME&&lookup==KERN_INVALID_NAME))?0:5;}
