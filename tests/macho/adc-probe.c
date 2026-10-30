#include <stdint.h>
#include <stdio.h>
struct result {uint64_t a,b,c,flags;};
#define RUN(word) __asm__ volatile("msr nzcv,x3\n.inst " #word "\nmrs x4,nzcv":"+r"(a),"+r"(b),"+r"(c),"=r"(out):"r"(flags):"memory","cc")
static struct result run(unsigned index,uint64_t av,uint64_t bv,uint64_t flagv){register uint64_t a __asm__("x0")=av,b __asm__("x1")=bv,c __asm__("x2")=0xabcdef0123456789ULL,flags __asm__("x3")=flagv,out __asm__("x4");switch(index){
case 0:RUN(0x1a010002);break;
case 1:RUN(0x1a010000);break;
case 2:RUN(0x1a010001);break;
case 3:RUN(0x1a01001f);break;
case 4:RUN(0x1a000002);break;
case 5:RUN(0x1a0103e2);break;
case 6:RUN(0x1a1f0002);break;
case 7:RUN(0x1a1f03e2);break;
case 8:RUN(0x9a010002);break;
case 9:RUN(0x9a010000);break;
case 10:RUN(0x9a010001);break;
case 11:RUN(0x9a01001f);break;
case 12:RUN(0x9a000002);break;
case 13:RUN(0x9a0103e2);break;
case 14:RUN(0x9a1f0002);break;
case 15:RUN(0x9a1f03e2);break;
default:__builtin_trap();}return(struct result){a,b,c,out};}
int main(void){uint64_t values[][2]={{0,0},{0,1},{UINT64_MAX,0},{UINT64_MAX,1},{UINT64_MAX,UINT64_MAX},{0x7fffffffffffffffULL,1},{0x8000000000000000ULL,0x8000000000000000ULL},{0xffffffffULL,1},{0x123456789abcdef0ULL,0xfedcba9876543210ULL},{0xdeadbeefffffffffULL,0x0123456700000001ULL}};struct {unsigned width,rn,rm,rd,word;}code[]={
{32,0,1,2,0x1a010002},
{32,0,1,0,0x1a010000},
{32,0,1,1,0x1a010001},
{32,0,1,31,0x1a01001f},
{32,0,0,2,0x1a000002},
{32,31,1,2,0x1a0103e2},
{32,0,31,2,0x1a1f0002},
{32,31,31,2,0x1a1f03e2},
{64,0,1,2,0x9a010002},
{64,0,1,0,0x9a010000},
{64,0,1,1,0x9a010001},
{64,0,1,31,0x9a01001f},
{64,0,0,2,0x9a000002},
{64,31,1,2,0x9a0103e2},
{64,0,31,2,0x9a1f0002},
{64,31,31,2,0x9a1f03e2},
};unsigned count=0;printf("{\"cases\":[");for(unsigned i=0;i<sizeof(code)/sizeof(code[0]);i++)for(unsigned v=0;v<sizeof(values)/sizeof(values[0]);v++)for(unsigned f=0;f<16;f++){struct result r=run(i,values[v][0],values[v][1],(uint64_t)f<<28);printf("%s{\"width\":%u,\"rn\":%u,\"rm\":%u,\"rd\":%u,\"encoding\":%u,\"a\":%llu,\"b\":%llu,\"initial_c\":12379813738877118345,\"input_nzcv\":%llu,\"out_a\":%llu,\"out_b\":%llu,\"out_c\":%llu,\"output_nzcv\":%llu}",count++?",":"",code[i].width,code[i].rn,code[i].rm,code[i].rd,code[i].word,values[v][0],values[v][1],(uint64_t)f<<28,r.a,r.b,r.c,r.flags);}printf("],\"count\":%u}\n",count);}
