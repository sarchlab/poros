#include <stdint.h>
#include <stdio.h>
extern void run0(void*,void*);
__asm__(".text\n.p2align 2\n_run0:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev0\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev0:\nrev64 v0.8b,v1.8b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run1(void*,void*);
__asm__(".text\n.p2align 2\n_run1:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev1\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev1:\nrev64 v1.8b,v1.8b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run2(void*,void*);
__asm__(".text\n.p2align 2\n_run2:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev2\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev2:\nrev64 v2.8b,v1.8b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run3(void*,void*);
__asm__(".text\n.p2align 2\n_run3:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev3\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev3:\nrev64 v0.16b,v1.16b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run4(void*,void*);
__asm__(".text\n.p2align 2\n_run4:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev4\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev4:\nrev64 v1.16b,v1.16b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run5(void*,void*);
__asm__(".text\n.p2align 2\n_run5:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev5\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev5:\nrev64 v2.16b,v1.16b\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run6(void*,void*);
__asm__(".text\n.p2align 2\n_run6:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev6\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev6:\nrev64 v0.4h,v1.4h\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run7(void*,void*);
__asm__(".text\n.p2align 2\n_run7:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev7\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev7:\nrev64 v1.4h,v1.4h\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run8(void*,void*);
__asm__(".text\n.p2align 2\n_run8:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev8\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev8:\nrev64 v2.4h,v1.4h\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run9(void*,void*);
__asm__(".text\n.p2align 2\n_run9:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev9\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev9:\nrev64 v0.8h,v1.8h\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run10(void*,void*);
__asm__(".text\n.p2align 2\n_run10:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev10\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev10:\nrev64 v1.8h,v1.8h\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run11(void*,void*);
__asm__(".text\n.p2align 2\n_run11:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev11\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev11:\nrev64 v2.8h,v1.8h\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run12(void*,void*);
__asm__(".text\n.p2align 2\n_run12:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev12\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev12:\nrev64 v0.2s,v1.2s\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run13(void*,void*);
__asm__(".text\n.p2align 2\n_run13:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev13\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev13:\nrev64 v1.2s,v1.2s\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run14(void*,void*);
__asm__(".text\n.p2align 2\n_run14:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev14\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev14:\nrev64 v2.2s,v1.2s\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run15(void*,void*);
__asm__(".text\n.p2align 2\n_run15:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev15\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev15:\nrev64 v0.4s,v1.4s\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run16(void*,void*);
__asm__(".text\n.p2align 2\n_run16:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev16\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev16:\nrev64 v1.4s,v1.4s\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
extern void run17(void*,void*);
__asm__(".text\n.p2align 2\n_run17:\nldp q0,q1,[x0]\nldr q2,[x0,#32]\nldr x9,[x0,#48]\nmsr nzcv,x9\nadr x10,Lrev17\nldr w10,[x10]\nstr x10,[x1,#56]\nLrev17:\nrev64 v2.4s,v1.4s\nstp q0,q1,[x1]\nstr q2,[x1,#32]\nmrs x9,nzcv\nstr x9,[x1,#48]\nret\n");
struct entry {void(*fn)(void*,void*);const char*ins;int width,n,d;};
static struct entry entries[]={
{run0,"rev64 v0.8b,v1.8b",1,8,0},
{run1,"rev64 v1.8b,v1.8b",1,8,1},
{run2,"rev64 v2.8b,v1.8b",1,8,2},
{run3,"rev64 v0.16b,v1.16b",1,16,0},
{run4,"rev64 v1.16b,v1.16b",1,16,1},
{run5,"rev64 v2.16b,v1.16b",1,16,2},
{run6,"rev64 v0.4h,v1.4h",2,8,0},
{run7,"rev64 v1.4h,v1.4h",2,8,1},
{run8,"rev64 v2.4h,v1.4h",2,8,2},
{run9,"rev64 v0.8h,v1.8h",2,16,0},
{run10,"rev64 v1.8h,v1.8h",2,16,1},
{run11,"rev64 v2.8h,v1.8h",2,16,2},
{run12,"rev64 v0.2s,v1.2s",4,8,0},
{run13,"rev64 v1.2s,v1.2s",4,8,1},
{run14,"rev64 v2.2s,v1.2s",4,8,2},
{run15,"rev64 v0.4s,v1.4s",4,16,0},
{run16,"rev64 v1.4s,v1.4s",4,16,1},
{run17,"rev64 v2.4s,v1.4s",4,16,2}};
struct state {unsigned char q[48];uint64_t flags,encoding;};
static void regs(unsigned char*q){putchar('[');for(int r=0;r<3;r++){printf("%s\"",r?",":"");for(int k=0;k<16;k++)printf("%02x",q[r*16+k]);putchar('"');}putchar(']');}
int main(void){int first=1;puts("[");for(unsigned i=0;i<sizeof(entries)/sizeof(entries[0]);i++)for(int p=0;p<8;p++)for(int f=0;f<16;f++){
struct state in={0},out={0};for(int k=0;k<48;k++)in.q[k]=(unsigned char)(k*37+0x93);for(int k=0;k<16;k++)in.q[16+k]=p==0?0:p==1?255:p==2?k:p==3?15-k:p==4?(1u<<(k%8)):p==5?(0x80^(k*29)):p==6?(k*17+3):(k%4==0?0xff:0);
in.flags=(uint64_t)f<<28;struct entry e=entries[i];e.fn(&in,&out);printf("%s{\"instruction\":\"%s\",\"encoding\":\"%08x\",\"element_bytes\":%d,\"active_bytes\":%d,\"destination\":%d,\"source\":1,\"before\":",first?"":",\n",e.ins,(unsigned)out.encoding,e.width,e.n,e.d);regs(in.q);printf(",\"after\":");regs(out.q);printf(",\"nzcv_before\":%llu,\"nzcv_after\":%llu}",(unsigned long long)in.flags,(unsigned long long)out.flags);first=0;
}puts("\n]");return 0;}
