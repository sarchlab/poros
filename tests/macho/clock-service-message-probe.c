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
int main(void){mach_port_t host=mach_host_self(),clocks[2];for(int i=0;i<2;i++)if(host_get_clock_service(host,i,&clocks[i]))return 2;
printf("{\"cases\":[");struct{const char*name;uint32_t id,size,rsize,ndr;}cases[]={{"system",0,36,128,1},{"calendar",1,36,128,1},{"invalid-id2",2,36,128,1},{"invalid-id-max",UINT32_MAX,36,128,1},{"short-send",0,32,128,1},{"long-send",0,40,128,1},{"receive-zero",0,36,0,1},{"receive23",0,36,23,1},{"receive24",0,36,24,1},{"receive39",0,36,39,1},{"receive40",0,36,40,1},{"receive47",0,36,47,1},{"receive48",0,36,48,1},{"ndr-noncanonical",0,36,128,0}};
for(unsigned i=0;i<sizeof(cases)/sizeof(cases[0]);i++){mach_port_t reply;int allocated=mach_port_allocate(mach_task_self(),MACH_PORT_RIGHT_RECEIVE,&reply);if(allocated)return 3;unsigned char b[160],before[160];memset(b,0xa5,sizeof b);memset(b,0,36);word(b,0x1513);word(b+4,cases[i].size);word(b+8,host);word(b+12,reply);word(b+20,206);memcpy(b+24,&NDR_record,8);b[28]=cases[i].ndr;word(b+32,cases[i].id);memcpy(before,b,sizeof b);
mach_port_urefs_t refs_before=0,refs_received=0,refs_after=0;int rb=mach_port_get_refs(mach_task_self(),clocks[cases[i].id==1],MACH_PORT_RIGHT_SEND,&refs_before);struct result r=call(b,cases[i].size,host,reply,cases[i].rsize);
mach_port_t received=0;int lookup=-1,release=-1;
if(r.x0==0&&(get(b)&0x80000000)&&get(b+4)==40&&get(b+24)==1){received=get(b+28);lookup=mach_port_get_refs(mach_task_self(),received,MACH_PORT_RIGHT_SEND,&refs_received);release=mach_port_deallocate(mach_task_self(),received);}
int destroyed=mach_port_mod_refs(mach_task_self(),reply,MACH_PORT_RIGHT_RECEIVE,-1);int ra=mach_port_get_refs(mach_task_self(),clocks[cases[i].id==1],MACH_PORT_RIGHT_SEND,&refs_after);
printf("%s{\"name\":\"%s\",\"clock_id\":%u,\"send_size\":%u,\"receive_size\":%u,\"reply_port\":%u,\"x0\":%llu,\"x1\":%llu,\"input_nzcv\":1342177280,\"output_nzcv\":%llu,\"received_port\":%u,\"received_lookup\":%d,\"received_refs\":%u,\"release\":%d,\"destroy\":%d,\"refs_before_result\":%d,\"refs_before\":%u,\"refs_after_result\":%d,\"refs_after\":%u,\"before_hex\":\"",i?",":"",cases[i].name,cases[i].id,cases[i].size,cases[i].rsize,reply,(unsigned long long)r.x0,(unsigned long long)r.x1,(unsigned long long)r.nzcv,received,lookup,refs_received,release,destroyed,rb,refs_before,ra,refs_after);hex(before,sizeof b);printf("\",\"after_hex\":\"");hex(b,sizeof b);printf("\"}");if(rb||ra||refs_before!=refs_after||destroyed||(received&&(lookup||release)))return 4;}
int a=mach_port_deallocate(mach_task_self(),clocks[0]),b=mach_port_deallocate(mach_task_self(),clocks[1]),h=mach_port_deallocate(mach_task_self(),host);printf("],\"cleanup\":[%d,%d,%d]}\n",a,b,h);return a||b||h?5:0;}
