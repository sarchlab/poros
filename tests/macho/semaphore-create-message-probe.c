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
int main(void){mach_port_t task=mach_task_self();printf("{\"cases\":[");struct{const char*name;int32_t policy,value;uint32_t size,rsize,ndr;}cases[]={{"fifo0",0,0,40,128,1},{"fifo1",0,1,40,128,1},{"max-value",0,INT_MAX,40,128,1},{"negative-value",0,-1,40,128,1},{"policy1",1,0,40,128,1},{"policy2",2,0,40,128,1},{"policy3",3,0,40,128,1},{"policy4",4,0,40,128,1},{"policy6",6,0,40,128,1},{"policy8",8,0,40,128,1},{"policy-negative",-1,0,40,128,1},{"short-send",0,0,36,128,1},{"long-send",0,0,44,128,1},{"receive0",0,0,40,0,1},{"receive23",0,0,40,23,1},{"receive24",0,0,40,24,1},{"receive40",0,0,40,40,1},{"receive47",0,0,40,47,1},{"receive48",0,0,40,48,1},{"ndr-noncanonical",0,0,40,128,0}};
for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){mach_port_t reply;if(mach_port_allocate(task,MACH_PORT_RIGHT_RECEIVE,&reply))return 2;unsigned char b[160],before[160];memset(b,0xa5,sizeof b);memset(b,0,40);word(b,0x1513);word(b+4,cases[i].size);word(b+8,task);word(b+12,reply);word(b+20,3418);memcpy(b+24,&NDR_record,8);b[28]=cases[i].ndr;word(b+32,cases[i].policy);word(b+36,cases[i].value);memcpy(before,b,sizeof b);struct result r=call(b,cases[i].size,task,reply,cases[i].rsize);mach_port_t received=0;mach_port_urefs_t refs=0;int lookup=-1,destroy=-1,after=-1;mach_port_type_t type=0;
if(r.x0==0&&(get(b)&0x80000000)&&get(b+4)==40&&get(b+24)==1){received=get(b+28);lookup=mach_port_get_refs(task,received,MACH_PORT_RIGHT_SEND,&refs);destroy=semaphore_destroy(task,received);after=mach_port_type(task,received,&type);}
int dr=mach_port_mod_refs(task,reply,MACH_PORT_RIGHT_RECEIVE,-1);
printf("%s{\"name\":\"%s\",\"policy\":%d,\"value\":%d,\"send_size\":%u,\"receive_size\":%u,\"task_port\":%u,\"reply_port\":%u,\"x0\":%llu,\"x1\":%llu,\"input_nzcv\":1342177280,\"output_nzcv\":%llu,\"semaphore_port\":%u,\"lookup\":%d,\"refs\":%u,\"semaphore_destroy\":%d,\"lookup_after_destroy\":%d,\"type_after_destroy\":%u,\"reply_destroy\":%d,\"before_hex\":\"",i?",":"",cases[i].name,cases[i].policy,cases[i].value,cases[i].size,cases[i].rsize,task,reply,r.x0,r.x1,r.nzcv,received,lookup,refs,destroy,after,type,dr);hex(before,160);printf("\",\"after_hex\":\"");hex(b,160);printf("\"}");if(dr||(received&&(lookup||destroy||after!=KERN_INVALID_NAME)))return 3;}printf("]}\n");return 0;}
