#include <mach/mach.h>
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
typedef mach_port_name_t(*self_fn)(void);
typedef kern_return_t(*mod_fn)(mach_port_name_t,mach_port_name_t,mach_port_right_t,mach_port_delta_t);
int main(void){self_fn self=(self_fn)dlsym(RTLD_DEFAULT,"mach_task_self");mod_fn mod=(mod_fn)dlsym(RTLD_DEFAULT,"_kernelrpc_mach_port_mod_refs_trap");if(!self||!mod)return 2;
 mach_port_name_t task=mach_task_self();mach_port_urefs_t before=0,after1=0,after2=0,afterdrop=0,final=0;
 if(mach_port_get_refs(task,task,MACH_PORT_RIGHT_SEND,&before))return 3;
 mach_port_name_t first=self();if(mach_port_get_refs(task,task,MACH_PORT_RIGHT_SEND,&after1))return 4;
 mach_port_name_t second=self();if(mach_port_get_refs(task,task,MACH_PORT_RIGHT_SEND,&after2))return 5;
 if(first!=task||second!=task||after1!=before+1||after2!=before+2)return 6;
 kern_return_t zero=mod(task,task,MACH_PORT_RIGHT_SEND,0),drop=mod(task,task,MACH_PORT_RIGHT_SEND,-1);
 if(mach_port_get_refs(task,task,MACH_PORT_RIGHT_SEND,&afterdrop))return 7;
 kern_return_t restore=mod(task,task,MACH_PORT_RIGHT_SEND,-1);if(mach_port_get_refs(task,task,MACH_PORT_RIGHT_SEND,&final))return 8;
 printf("{\"before\":%u,\"after_first_self\":%u,\"after_second_self\":%u,\"zero_delta_return\":%d,\"drop_return\":%d,\"after_drop\":%u,\"restore_return\":%d,\"final\":%u,\"same_name\":true}\n",before,after1,after2,zero,drop,afterdrop,restore,final);return final==before?0:9;}
