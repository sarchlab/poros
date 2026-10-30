// Owned Mach reply ports and acquired clock send rights; no clock reads.
#include <mach/mach.h>
#include <mach/mach_host.h>
#include <mach/ndr.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(void*p,uint32_t size,mach_port_t host,mach_port_t reply,uint32_t rsize){register uint64_t a __asm__("x0")=(uint64_t)p,b __asm__("x1")=0x200000003ULL,c __asm__("x2")=((uint64_t)size<<32)|0x1513,d __asm__("x3")=((uint64_t)reply<<32)|host,e __asm__("x4")=3418ULL<<32,f __asm__("x5")=(uint64_t)reply<<32,g __asm__("x6")=rsize,h __asm__("x7")=0,n __asm__("x16")=(uint64_t)-47;uint64_t flags,initial=0x50000000;__asm__ volatile("msr nzcv,%10\nsvc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(d),"r"(e),"r"(f),"r"(g),"r"(h),"r"(n),"r"(initial):"memory","cc");return(struct result){a,b,flags};}
static void word(unsigned char*p,uint32_t v){memcpy(p,&v,4);}static uint32_t get(const unsigned char*p){uint32_t v;memcpy(&v,p,4);return v;}static void hex(const unsigned char*p,unsigned n){for(unsigned i=0;i<n;i++)printf("%02x",p[i]);}

#include <mach/semaphore.h>
#include <limits.h>

#include <sys/mman.h>
#include <unistd.h>
struct names{mach_port_t p[512];unsigned count;};
static struct names names(void){struct names out={0};mach_port_name_array_t n;mach_port_type_array_t t;mach_msg_type_number_t nc=0,tc=0;if(mach_port_names(mach_task_self(),&n,&nc,&t,&tc)||nc!=tc||nc>512)_exit(31);for(unsigned i=0;i<nc;i++)if(t[i]&MACH_PORT_TYPE_SEND)out.p[out.count++]=n[i];if(vm_deallocate(mach_task_self(),(vm_address_t)n,nc*sizeof(*n))||vm_deallocate(mach_task_self(),(vm_address_t)t,tc*sizeof(*t)))_exit(32);return out;}
static int contains(struct names*s,mach_port_t n){for(unsigned i=0;i<s->count;i++)if(s->p[i]==n)return 1;return 0;}
int main(void){mach_port_t task=mach_task_self();names();printf("{\"cases\":[");
for(int i=0;i<8;i++){size_t page=getpagesize();unsigned char*allocation=mmap(0,page*2,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(allocation==MAP_FAILED)return 3;unsigned char*b=allocation+(i==1?page-32:i==2?page-16:0);memset(b,0xa5,160);memset(b,0,40);mach_port_t reply;if(mach_port_allocate(task,MACH_PORT_RIGHT_RECEIVE,&reply))return 4;word(b,0x1513);word(b+4,40);word(b+8,task);word(b+12,reply);word(b+20,3418);memcpy(b+24,&NDR_record,8);word(b+32,0);word(b+36,0);unsigned char before[160];memcpy(before,b,160);if(i<3&&mprotect(allocation+(i?page:0),i?page:page*2,PROT_READ))return 5;uint32_t sizes[]={128,128,128,0,23,24,47,48};struct names old=names();struct result r=call(b,40,task,reply,sizes[i]);struct names current=names();mach_port_t added=0;unsigned added_count=0;for(unsigned j=0;j<current.count;j++)if(!contains(&old,current.p[j])){added=current.p[j];added_count++;}if(added_count>1)return 6;mach_port_urefs_t refs=0;int lookup=-1,cleanup=-1,after_cleanup=-1;mach_port_type_t type=0;if(added_count){lookup=mach_port_get_refs(task,added,MACH_PORT_RIGHT_SEND,&refs);cleanup=semaphore_destroy(task,added);after_cleanup=mach_port_type(task,added,&type);if(lookup||refs!=1||cleanup||after_cleanup!=15)return 7;}
mach_port_status_t status={0};mach_msg_type_number_t sc=MACH_PORT_RECEIVE_STATUS_COUNT;int sr=mach_port_get_attributes(task,reply,MACH_PORT_RECEIVE_STATUS,(mach_port_info_t)&status,&sc);int dr=mach_port_mod_refs(task,reply,MACH_PORT_RIGHT_RECEIVE,-1);
const char*labels[]={"readonly","writable-prefix32","writable-prefix16","receive0","receive23","receive24","receive47","receive48"};
printf("%s{\"name\":\"%s\",\"receive_size\":%u,\"reply_port\":%u,\"x0\":%llu,\"x1\":%llu,\"input_nzcv\":1342177280,\"output_nzcv\":%llu,\"new_send_names\":%u,\"new_port\":%u,\"new_port_refs\":%u,\"semaphore_destroy\":%d,\"lookup_after_destroy\":%d,\"reply_status_result\":%d,\"queued_replies\":%u,\"reply_destroy\":%d,\"before_hex\":\"",i?",":"",labels[i],sizes[i],reply,r.x0,r.x1,r.nzcv,added_count,added,refs,cleanup,after_cleanup,sr,status.mps_msgcount,dr);hex(before,160);printf("\",\"after_hex\":\"");hex(b,160);printf("\"}");if(sr||dr||munmap(allocation,page*2))return 8;}
printf("]}\n");return 0;}
