#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <unistd.h>
typedef kern_return_t(*construct_fn)(mach_port_name_t,mach_port_options_t*,uint64_t,mach_port_name_t*);
struct snapshot{mach_port_name_array_t names;mach_msg_type_number_t n;mach_port_type_array_t types;mach_msg_type_number_t nt;};
static int snapshot(struct snapshot*s){return mach_port_names(mach_task_self(),&s->names,&s->n,&s->types,&s->nt);}
static void release(struct snapshot*s){mach_vm_deallocate(mach_task_self(),(mach_vm_address_t)s->names,s->n*sizeof(*s->names));mach_vm_deallocate(mach_task_self(),(mach_vm_address_t)s->types,s->nt*sizeof(*s->types));}
static int probe(unsigned which){construct_fn fn=(construct_fn)dlsym(RTLD_DEFAULT,"_kernelrpc_mach_port_construct_trap");if(!fn)return 2;mach_port_options_t options={.flags=0x1000};mach_port_options_t*op=&options;uint64_t context=0;mach_port_name_t task=mach_task_self(),name=0xdeadbeef,*output=&name;void*ro=0;
 switch(which){case 0:break;case 1:options.flags=0;break;case 2:context=0x12345678;break;case 3:options.mpl.mpl_qlimit=99;break;case 4:options.reserved[0]=UINT64_MAX;options.reserved[1]=UINT64_MAX;break;case 5:op=0;break;case 6:op=(void*)1;break;case 7:output=0;break;case 8:ro=mmap(0,(size_t)getpagesize(),PROT_READ,MAP_PRIVATE|MAP_ANON,-1,0);if(ro==MAP_FAILED)return 3;output=ro;break;case 9:task=0;break;case 10:options.flags=0x80000000;break;case 11:options.flags|=MPO_QLIMIT;options.mpl.mpl_qlimit=UINT32_MAX;break;}
 struct snapshot warm={0},before={0},after={0};if(snapshot(&warm))return 4;release(&warm);if(snapshot(&before))return 5;kern_return_t result=fn(task,op,context,output);if(snapshot(&after))return 6;unsigned added=0;mach_port_name_t newname=0;mach_port_type_t type=0;for(unsigned i=0;i<after.n;i++){int seen=0;for(unsigned j=0;j<before.n;j++)if(after.names[i]==before.names[j])seen=1;if(!seen){added++;newname=after.names[i];type=after.types[i];}}
 mach_port_urefs_t receive=0,send=0;mach_port_context_t observed_context=0;kern_return_t receive_result=-1,send_result=-1,context_result=-1;if(added==1){receive_result=mach_port_get_refs(mach_task_self(),newname,MACH_PORT_RIGHT_RECEIVE,&receive);send_result=mach_port_get_refs(mach_task_self(),newname,MACH_PORT_RIGHT_SEND,&send);context_result=mach_port_get_context(mach_task_self(),newname,&observed_context);}
 printf("{\"case\":%u,\"options_size\":%zu,\"return\":%d,\"output_changed\":%s,\"output_matches_new\":%s,\"ports_added\":%u,\"new_type\":%u,\"receive_result\":%d,\"receive_refs\":%u,\"send_result\":%d,\"send_refs\":%u,\"context_result\":%d,\"context\":%llu}\n",which,sizeof(options),result,name!=0xdeadbeef?"true":"false",name==newname?"true":"false",added,type,receive_result,receive,send_result,send,context_result,(unsigned long long)observed_context);fflush(stdout);release(&before);release(&after);return 0;}
int main(void){unsigned failed=0;for(unsigned i=0;i<12;i++){pid_t p=fork();if(p<0)return 7;if(p==0)_exit(probe(i));int status=0;if(waitpid(p,&status,0)!=p)return 8;if(!WIFEXITED(status)||WEXITSTATUS(status)!=0){fprintf(stderr,"case %u status %d\n",i,status);failed++;}}return failed?1:0;}
