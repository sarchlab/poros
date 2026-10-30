// Owned Mach reply ports and acquired clock send rights; no clock reads.
#include <mach/mach.h>
#include <mach/mach_host.h>
#include <mach/ndr.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
struct result{uint64_t x0,x1,nzcv;};
static struct result call(void*p,uint32_t size,mach_port_t host,mach_port_t reply,uint32_t rsize){register uint64_t a __asm__("x0")=(uint64_t)p,b __asm__("x1")=0x200000003ULL,c __asm__("x2")=((uint64_t)size<<32)|0x1513,d __asm__("x3")=((uint64_t)reply<<32)|host,e __asm__("x4")=206ULL<<32,f __asm__("x5")=(uint64_t)reply<<32,g __asm__("x6")=rsize,h __asm__("x7")=0,n __asm__("x16")=(uint64_t)-47;uint64_t flags,initial=0x50000000;__asm__ volatile("msr nzcv,%10\nsvc #0x80\nmrs %2,nzcv":"+r"(a),"+r"(b),"=&r"(flags):"r"(c),"r"(d),"r"(e),"r"(f),"r"(g),"r"(h),"r"(n),"r"(initial):"memory","cc");return(struct result){a,b,flags};}
static void word(unsigned char*p,uint32_t v){memcpy(p,&v,4);}static uint32_t get(const unsigned char*p){uint32_t v;memcpy(&v,p,4);return v;}static void hex(const unsigned char*p,unsigned n){for(unsigned i=0;i<n;i++)printf("%02x",p[i]);}

#include <sys/mman.h>
#include <unistd.h>
int main(void){mach_port_t host=mach_host_self(),clock;if(host_get_clock_service(host,0,&clock))return 2;printf("{\"cases\":[");
for(int i=0;i<3;i++){size_t page=getpagesize();unsigned char*allocation=mmap(0,page*2,PROT_READ|PROT_WRITE,MAP_PRIVATE|MAP_ANON,-1,0);if(allocation==MAP_FAILED)return 3;unsigned char*b=allocation+(i==0?0:page-(i==1?32:16));memset(b,0xa5,160);memset(b,0,36);mach_port_t reply;if(mach_port_allocate(mach_task_self(),MACH_PORT_RIGHT_RECEIVE,&reply))return 4;word(b,0x1513);word(b+4,36);word(b+8,host);word(b+12,reply);word(b+20,206);memcpy(b+24,&NDR_record,8);word(b+32,0);unsigned char before[160];memcpy(before,b,160);if(mprotect(allocation+(i? page:0),i?page:page*2,PROT_READ))return 5;
mach_port_urefs_t refs_before=0,refs_returned=0,refs_cleaned=0;if(mach_port_get_refs(mach_task_self(),clock,MACH_PORT_RIGHT_SEND,&refs_before))return 6;struct result r=call(b,36,host,reply,128);int lookup=mach_port_get_refs(mach_task_self(),clock,MACH_PORT_RIGHT_SEND,&refs_returned);int releases=0;if(lookup==0&&refs_returned>=refs_before&&refs_returned-refs_before<=1){while(refs_returned-refs_before>releases){if(mach_port_deallocate(mach_task_self(),clock))return 7;releases++;}}else return 8;
int destroyed=mach_port_mod_refs(mach_task_self(),reply,MACH_PORT_RIGHT_RECEIVE,-1);int checked=mach_port_get_refs(mach_task_self(),clock,MACH_PORT_RIGHT_SEND,&refs_cleaned);
printf("%s{\"name\":\"%s\",\"reply_port\":%u,\"clock_port\":%u,\"x0\":%llu,\"x1\":%llu,\"input_nzcv\":1342177280,\"output_nzcv\":%llu,\"refs_before\":%u,\"refs_returned\":%u,\"owned_added_refs_released\":%d,\"refs_cleaned\":%u,\"destroy\":%d,\"before_hex\":\"",i?",":"",i==0?"readonly":i==1?"writable-prefix32":"writable-prefix16",reply,clock,(unsigned long long)r.x0,(unsigned long long)r.x1,(unsigned long long)r.nzcv,refs_before,refs_returned,releases,refs_cleaned,destroyed);hex(before,160);printf("\",\"after_hex\":\"");hex(b,160);printf("\"}");if(checked||destroyed||refs_cleaned!=refs_before)return 9;if(munmap(allocation,page*2))return 10;}
int c=mach_port_deallocate(mach_task_self(),clock),h=mach_port_deallocate(mach_task_self(),host);printf("],\"cleanup\":[%d,%d]}\n",c,h);return c||h?11:0;}
