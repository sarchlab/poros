#include <mach/mach.h>
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
typedef kern_return_t(*mod_fn)(mach_port_name_t,mach_port_name_t,mach_port_right_t,mach_port_delta_t);
int main(void){mod_fn mod=(mod_fn)dlsym(RTLD_DEFAULT,"_kernelrpc_mach_port_mod_refs_trap");if(!mod)return 2;mach_port_name_t names[]={MACH_PORT_NULL,MACH_PORT_DEAD};mach_port_right_t rights[]={MACH_PORT_RIGHT_SEND,MACH_PORT_RIGHT_SEND_ONCE,MACH_PORT_RIGHT_RECEIVE,MACH_PORT_RIGHT_NUMBER};int deltas[]={-1,0,1};printf("[\n");unsigned n=0;for(unsigned name=0;name<2;name++)for(unsigned right=0;right<4;right++)for(unsigned d=0;d<3;d++){kern_return_t result=mod(mach_task_self(),names[name],rights[right],deltas[d]);printf("%s{\"name\":%u,\"right\":%u,\"delta\":%d,\"return\":%d}",n++?",\n":"",names[name],rights[right],deltas[d],result);}printf("\n]\n");return 0;}
