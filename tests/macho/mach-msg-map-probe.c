#define __MigPackStructs 1
#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <sys/wait.h>
#include <sys/mman.h>
#include <unistd.h>
typedef kern_return_t(*construct_fn)(mach_port_name_t,mach_port_options_t*,uint64_t,mach_port_name_t*);
typedef uint64_t(*msg_fn)(void*,uint64_t,uint64_t,uint64_t,uint64_t,uint64_t,uint64_t,uint64_t);
_Static_assert(sizeof(__Request__mach_vm_map_t)==100,"request layout");
_Static_assert(sizeof(__Reply__mach_vm_map_t)==44,"reply layout");
static int probe(unsigned which){alarm(5);construct_fn construct=(construct_fn)dlsym(RTLD_DEFAULT,"_kernelrpc_mach_port_construct_trap");msg_fn msg=(msg_fn)dlsym(RTLD_DEFAULT,"mach_msg2_trap");if(!construct||!msg)return 2;mach_port_name_t task=mach_task_self(),reply=0;mach_port_options_t opts={.flags=0x1000};if(construct(task,&opts,0,&reply))return 3;
 mach_vm_address_t reserved=0;mach_vm_size_t length=0x16c000;if(mach_vm_allocate(task,&reserved,length,VM_FLAGS_ANYWHERE))return 4;
 union{__Request__mach_vm_map_t request;unsigned char raw[128];} b={0};b.request.Head.msgh_bits=0x80001513;b.request.Head.msgh_size=100;b.request.Head.msgh_remote_port=task;b.request.Head.msgh_local_port=reply;b.request.Head.msgh_id=4811;b.request.msgh_body.msgh_descriptor_count=1;b.request.object.name=0;b.request.object.disposition=19;b.request.object.type=0;b.request.NDR=NDR_record;b.request.address=reserved;b.request.size=length;b.request.inheritance=1;
 switch(which){case 0:case 1:break;case 2:b.request.size=0;break;case 3:b.request.cur_protection=VM_PROT_READ;break;case 4:b.request.flags=VM_FLAGS_ANYWHERE;b.request.cur_protection=3;b.request.max_protection=3;break;case 5:b.request.max_protection=8;break;}
 void* buffer=&b;uint64_t receive_size=52;void*readonly=0;
 if(which==6)receive_size=44;if(which==7)receive_size=35;if(which==9)receive_size=0;
 if(which>=12&&which<=17){static const uint64_t bounds[]={24,31,32,51,52,45};receive_size=bounds[which-12];}
 if(which>=19){static const uint64_t bounds[]={1,16,23,25,26,27,28,29,30,48,49,50};receive_size=bounds[which-19];memset(&b,0xa5,28);}
 if(which==10)memset(&b.request.Head,0,sizeof(b.request.Head));
 if(which==11)b.request.msgh_body.msgh_descriptor_count=0;
 if(which==8){readonly=mmap(0,(size_t)getpagesize(),PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(readonly==MAP_FAILED)return 8;memcpy(readonly,&b,sizeof(b));if(mprotect(readonly,(size_t)getpagesize(),PROT_READ))return 9;buffer=readonly;}
 unsigned char before_buffer[128];memcpy(before_buffer,&b,sizeof(b));if(which==18)buffer=(void*)1;
 if(which!=1&&mach_vm_deallocate(task,reserved,length))return 5;
 uint64_t result=msg(buffer,0x200000003ULL,((uint64_t)100<<32)|0x80001513,((uint64_t)reply<<32)|task,(uint64_t)4811<<32,((uint64_t)reply<<32)|1,receive_size,0);if(readonly)memcpy(&b,readonly,sizeof(b));
 __Reply__mach_vm_map_t response;memcpy(&response,b.raw,sizeof(response));mach_vm_address_t address=response.address;mach_vm_address_t query=reserved;mach_vm_size_t region_size=0;vm_region_basic_info_data_64_t info={0};mach_msg_type_number_t count=VM_REGION_BASIC_INFO_COUNT_64;mach_port_t object=0;kern_return_t region=mach_vm_region(task,&query,&region_size,VM_REGION_BASIC_INFO_64,(vm_region_info_t)&info,&count,&object);int mapped=region==0&&query<=reserved&&reserved-query<region_size;
 const char*kind=result==0?"mig_reply":result==0x10004004?"header_only":"none";size_t written=result==0?response.Head.msgh_size+8:result==0x10004004?32:0;int suffix_preserved=written<=sizeof(b)&&memcmp(b.raw+written,before_buffer+written,sizeof(b)-written)==0;
 printf("{\"case\":%u,\"trap_return\":%llu,\"response_kind\":\"%s\",\"receive_size\":%llu,\"suffix_preserved\":%s,\"hint_mapped\":%s,\"hint_protection\":%d,\"hint_max_protection\":%d",which,(unsigned long long)result,kind,(unsigned long long)receive_size,suffix_preserved?"true":"false",mapped?"true":"false",mapped?info.protection:-1,mapped?info.max_protection:-1);
 if(result==0||result==0x10004004){printf(",\"reply_bits\":%u,\"reply_size\":%u,\"reply_remote_null\":%s,\"reply_local_matches\":%s,\"reply_voucher\":%u,\"reply_id\":%d",response.Head.msgh_bits,response.Head.msgh_size,response.Head.msgh_remote_port==0?"true":"false",response.Head.msgh_local_port==reply?"true":"false",response.Head.msgh_voucher_port,response.Head.msgh_id);
 if(result==0){printf(",\"reply_ndr_hex\":\"");for(unsigned i=24;i<32;i++)printf("%02x",b.raw[i]);printf("\",\"kernel_return\":%d",response.RetCode);if(response.RetCode==0)printf(",\"address_matches_hint\":%s",address==reserved?"true":"false");}
 printf(",\"trailer_hex\":\"");for(unsigned i=response.Head.msgh_size;i<response.Head.msgh_size+8&&i<128;i++)printf("%02x",b.raw[i]);printf("\"");}printf(",\"raw_bytes_24_48_hex\":\"");for(unsigned i=24;i<48;i++)printf("%02x",b.raw[i]);printf("\"}\n");fflush(stdout);return 0;}
int main(void){unsigned failed=0;for(unsigned i=0;i<31;i++){pid_t p=fork();if(p<0)return 6;if(p==0)_exit(probe(i));int status=0;if(waitpid(p,&status,0)!=p)return 7;if(!WIFEXITED(status)||WEXITSTATUS(status)!=0){fprintf(stderr,"case %u status %d\n",i,status);failed++;}}return failed?1:0;}
