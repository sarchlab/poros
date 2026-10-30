#include <stdint.h>
#include <stdio.h>
extern void run0(void*,void*);
__asm__(".text\n.p2align 2\n_run0:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lprobe0\nldr w10,[x10]\nstr x10,[x1,#56]\nLprobe0:\nbsl v0.8b,v1.8b,v2.8b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run1(void*,void*);
__asm__(".text\n.p2align 2\n_run1:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lprobe1\nldr w10,[x10]\nstr x10,[x1,#56]\nLprobe1:\nbsl v1.8b,v1.8b,v2.8b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run2(void*,void*);
__asm__(".text\n.p2align 2\n_run2:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lprobe2\nldr w10,[x10]\nstr x10,[x1,#56]\nLprobe2:\nbsl v2.8b,v1.8b,v2.8b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run3(void*,void*);
__asm__(".text\n.p2align 2\n_run3:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lprobe3\nldr w10,[x10]\nstr x10,[x1,#56]\nLprobe3:\nbsl v0.8b,v1.8b,v1.8b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run4(void*,void*);
__asm__(".text\n.p2align 2\n_run4:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lprobe4\nldr w10,[x10]\nstr x10,[x1,#56]\nLprobe4:\nbsl v1.8b,v1.8b,v1.8b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run5(void*,void*);
__asm__(".text\n.p2align 2\n_run5:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lprobe5\nldr w10,[x10]\nstr x10,[x1,#56]\nLprobe5:\nbsl v0.16b,v1.16b,v2.16b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run6(void*,void*);
__asm__(".text\n.p2align 2\n_run6:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lprobe6\nldr w10,[x10]\nstr x10,[x1,#56]\nLprobe6:\nbsl v1.16b,v1.16b,v2.16b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run7(void*,void*);
__asm__(".text\n.p2align 2\n_run7:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lprobe7\nldr w10,[x10]\nstr x10,[x1,#56]\nLprobe7:\nbsl v2.16b,v1.16b,v2.16b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run8(void*,void*);
__asm__(".text\n.p2align 2\n_run8:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lprobe8\nldr w10,[x10]\nstr x10,[x1,#56]\nLprobe8:\nbsl v0.16b,v1.16b,v1.16b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run9(void*,void*);
__asm__(".text\n.p2align 2\n_run9:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lprobe9\nldr w10,[x10]\nstr x10,[x1,#56]\nLprobe9:\nbsl v1.16b,v1.16b,v1.16b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
struct entry{void(*fn)(void*,void*);const char*ins;int w,n,d,a,b;};
static struct entry variants[]={
{run0,"bsl v0.8b,v1.8b,v2.8b",1,8,0,1,2},
{run1,"bsl v1.8b,v1.8b,v2.8b",1,8,1,1,2},
{run2,"bsl v2.8b,v1.8b,v2.8b",1,8,2,1,2},
{run3,"bsl v0.8b,v1.8b,v1.8b",1,8,0,1,1},
{run4,"bsl v1.8b,v1.8b,v1.8b",1,8,1,1,1},
{run5,"bsl v0.16b,v1.16b,v2.16b",1,16,0,1,2},
{run6,"bsl v1.16b,v1.16b,v2.16b",1,16,1,1,2},
{run7,"bsl v2.16b,v1.16b,v2.16b",1,16,2,1,2},
{run8,"bsl v0.16b,v1.16b,v1.16b",1,16,0,1,1},
{run9,"bsl v1.16b,v1.16b,v1.16b",1,16,1,1,1}};

struct state{unsigned char q[48];uint64_t flags,encoding;};
static void regs(unsigned char*q){putchar('[');for(int r=0;r<3;r++){printf("%s\"",r?",":"");for(int k=0;k<16;k++)printf("%02x",q[r*16+k]);putchar('"');}putchar(']');}
int main(void){int first=1;puts("[");for(unsigned i=0;i<sizeof(variants)/sizeof(variants[0]);i++)for(int p=0;p<8;p++)for(int f=0;f<16;f++){
struct state in={0},out={0};for(int k=0;k<16;k++){in.q[k]=p==0?0:p==1?255:p==2?0x55:p==3?0xaa:(unsigned char)(k*31+p*7);in.q[32+k]=(unsigned char)((k*37+0x93)^(p*19));in.q[16+k]=p==0?0:p==1?255:p==2?k:p==3?15-k:p==4?(1u<<(k%8)):p==5?(0x80^(k*29)):p==6?(k*17+3):(k%4==0?0xff:0);}
in.flags=(uint64_t)f<<28;struct entry e=variants[i];e.fn(&in,&out);printf("%s{\"instruction\":\"%s\",\"encoding\":\"%08x\",\"element_bytes\":%d,\"active_bytes\":%d,\"destination\":%d,\"source\":%d,\"second_source\":%d,\"before\":",first?"":",\n",e.ins,(unsigned)out.encoding,e.w,e.n,e.d,e.a,e.b);regs(in.q);printf(",\"after\":");regs(out.q);printf(",\"nzcv_before\":%llu,\"nzcv_after\":%llu}",(unsigned long long)in.flags,(unsigned long long)out.flags);first=0;
}puts("\n]");return 0;}
