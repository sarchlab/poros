#include <stdint.h>
#include <stdio.h>
extern void run0(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run0:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe0\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe0:\nbics w0,w1,w2,lsl #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run1(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run1:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe1\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe1:\nbics w1,w1,w2,lsl #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run2(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run2:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe2\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe2:\nbics w2,w1,w2,lsl #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run3(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run3:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe3\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe3:\nbics wzr,w1,w2,lsl #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run4(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run4:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe4\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe4:\nbics w0,w1,w2,lsl #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run5(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run5:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe5\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe5:\nbics w1,w1,w2,lsl #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run6(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run6:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe6\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe6:\nbics w2,w1,w2,lsl #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run7(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run7:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe7\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe7:\nbics wzr,w1,w2,lsl #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run8(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run8:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe8\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe8:\nbics w0,w1,w2,lsl #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run9(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run9:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe9\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe9:\nbics w1,w1,w2,lsl #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run10(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run10:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe10\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe10:\nbics w2,w1,w2,lsl #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run11(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run11:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe11\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe11:\nbics wzr,w1,w2,lsl #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run12(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run12:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe12\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe12:\nbics w0,w1,w2,lsl #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run13(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run13:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe13\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe13:\nbics w1,w1,w2,lsl #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run14(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run14:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe14\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe14:\nbics w2,w1,w2,lsl #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run15(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run15:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe15\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe15:\nbics wzr,w1,w2,lsl #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run16(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run16:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe16\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe16:\nbics w0,w1,w2,lsr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run17(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run17:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe17\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe17:\nbics w1,w1,w2,lsr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run18(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run18:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe18\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe18:\nbics w2,w1,w2,lsr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run19(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run19:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe19\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe19:\nbics wzr,w1,w2,lsr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run20(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run20:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe20\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe20:\nbics w0,w1,w2,lsr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run21(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run21:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe21\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe21:\nbics w1,w1,w2,lsr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run22(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run22:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe22\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe22:\nbics w2,w1,w2,lsr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run23(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run23:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe23\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe23:\nbics wzr,w1,w2,lsr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run24(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run24:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe24\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe24:\nbics w0,w1,w2,lsr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run25(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run25:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe25\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe25:\nbics w1,w1,w2,lsr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run26(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run26:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe26\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe26:\nbics w2,w1,w2,lsr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run27(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run27:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe27\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe27:\nbics wzr,w1,w2,lsr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run28(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run28:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe28\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe28:\nbics w0,w1,w2,lsr #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run29(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run29:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe29\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe29:\nbics w1,w1,w2,lsr #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run30(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run30:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe30\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe30:\nbics w2,w1,w2,lsr #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run31(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run31:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe31\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe31:\nbics wzr,w1,w2,lsr #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run32(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run32:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe32\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe32:\nbics w0,w1,w2,asr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run33(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run33:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe33\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe33:\nbics w1,w1,w2,asr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run34(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run34:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe34\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe34:\nbics w2,w1,w2,asr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run35(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run35:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe35\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe35:\nbics wzr,w1,w2,asr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run36(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run36:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe36\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe36:\nbics w0,w1,w2,asr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run37(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run37:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe37\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe37:\nbics w1,w1,w2,asr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run38(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run38:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe38\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe38:\nbics w2,w1,w2,asr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run39(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run39:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe39\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe39:\nbics wzr,w1,w2,asr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run40(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run40:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe40\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe40:\nbics w0,w1,w2,asr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run41(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run41:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe41\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe41:\nbics w1,w1,w2,asr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run42(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run42:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe42\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe42:\nbics w2,w1,w2,asr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run43(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run43:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe43\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe43:\nbics wzr,w1,w2,asr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run44(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run44:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe44\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe44:\nbics w0,w1,w2,asr #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run45(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run45:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe45\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe45:\nbics w1,w1,w2,asr #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run46(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run46:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe46\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe46:\nbics w2,w1,w2,asr #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run47(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run47:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe47\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe47:\nbics wzr,w1,w2,asr #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run48(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run48:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe48\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe48:\nbics w0,w1,w2,ror #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run49(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run49:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe49\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe49:\nbics w1,w1,w2,ror #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run50(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run50:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe50\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe50:\nbics w2,w1,w2,ror #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run51(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run51:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe51\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe51:\nbics wzr,w1,w2,ror #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run52(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run52:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe52\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe52:\nbics w0,w1,w2,ror #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run53(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run53:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe53\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe53:\nbics w1,w1,w2,ror #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run54(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run54:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe54\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe54:\nbics w2,w1,w2,ror #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run55(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run55:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe55\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe55:\nbics wzr,w1,w2,ror #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run56(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run56:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe56\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe56:\nbics w0,w1,w2,ror #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run57(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run57:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe57\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe57:\nbics w1,w1,w2,ror #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run58(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run58:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe58\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe58:\nbics w2,w1,w2,ror #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run59(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run59:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe59\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe59:\nbics wzr,w1,w2,ror #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run60(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run60:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe60\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe60:\nbics w0,w1,w2,ror #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run61(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run61:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe61\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe61:\nbics w1,w1,w2,ror #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run62(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run62:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe62\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe62:\nbics w2,w1,w2,ror #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run63(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run63:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe63\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe63:\nbics wzr,w1,w2,ror #31\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run64(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run64:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe64\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe64:\nbics x0,x1,x2,lsl #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run65(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run65:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe65\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe65:\nbics x1,x1,x2,lsl #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run66(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run66:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe66\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe66:\nbics x2,x1,x2,lsl #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run67(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run67:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe67\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe67:\nbics xzr,x1,x2,lsl #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run68(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run68:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe68\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe68:\nbics x0,x1,x2,lsl #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run69(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run69:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe69\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe69:\nbics x1,x1,x2,lsl #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run70(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run70:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe70\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe70:\nbics x2,x1,x2,lsl #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run71(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run71:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe71\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe71:\nbics xzr,x1,x2,lsl #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run72(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run72:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe72\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe72:\nbics x0,x1,x2,lsl #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run73(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run73:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe73\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe73:\nbics x1,x1,x2,lsl #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run74(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run74:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe74\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe74:\nbics x2,x1,x2,lsl #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run75(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run75:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe75\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe75:\nbics xzr,x1,x2,lsl #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run76(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run76:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe76\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe76:\nbics x0,x1,x2,lsl #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run77(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run77:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe77\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe77:\nbics x1,x1,x2,lsl #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run78(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run78:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe78\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe78:\nbics x2,x1,x2,lsl #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run79(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run79:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe79\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe79:\nbics xzr,x1,x2,lsl #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run80(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run80:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe80\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe80:\nbics x0,x1,x2,lsl #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run81(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run81:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe81\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe81:\nbics x1,x1,x2,lsl #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run82(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run82:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe82\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe82:\nbics x2,x1,x2,lsl #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run83(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run83:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe83\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe83:\nbics xzr,x1,x2,lsl #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run84(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run84:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe84\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe84:\nbics x0,x1,x2,lsr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run85(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run85:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe85\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe85:\nbics x1,x1,x2,lsr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run86(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run86:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe86\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe86:\nbics x2,x1,x2,lsr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run87(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run87:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe87\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe87:\nbics xzr,x1,x2,lsr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run88(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run88:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe88\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe88:\nbics x0,x1,x2,lsr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run89(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run89:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe89\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe89:\nbics x1,x1,x2,lsr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run90(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run90:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe90\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe90:\nbics x2,x1,x2,lsr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run91(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run91:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe91\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe91:\nbics xzr,x1,x2,lsr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run92(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run92:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe92\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe92:\nbics x0,x1,x2,lsr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run93(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run93:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe93\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe93:\nbics x1,x1,x2,lsr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run94(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run94:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe94\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe94:\nbics x2,x1,x2,lsr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run95(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run95:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe95\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe95:\nbics xzr,x1,x2,lsr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run96(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run96:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe96\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe96:\nbics x0,x1,x2,lsr #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run97(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run97:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe97\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe97:\nbics x1,x1,x2,lsr #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run98(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run98:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe98\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe98:\nbics x2,x1,x2,lsr #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run99(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run99:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe99\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe99:\nbics xzr,x1,x2,lsr #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run100(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run100:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe100\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe100:\nbics x0,x1,x2,lsr #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run101(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run101:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe101\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe101:\nbics x1,x1,x2,lsr #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run102(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run102:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe102\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe102:\nbics x2,x1,x2,lsr #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run103(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run103:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe103\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe103:\nbics xzr,x1,x2,lsr #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run104(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run104:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe104\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe104:\nbics x0,x1,x2,asr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run105(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run105:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe105\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe105:\nbics x1,x1,x2,asr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run106(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run106:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe106\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe106:\nbics x2,x1,x2,asr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run107(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run107:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe107\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe107:\nbics xzr,x1,x2,asr #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run108(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run108:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe108\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe108:\nbics x0,x1,x2,asr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run109(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run109:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe109\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe109:\nbics x1,x1,x2,asr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run110(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run110:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe110\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe110:\nbics x2,x1,x2,asr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run111(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run111:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe111\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe111:\nbics xzr,x1,x2,asr #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run112(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run112:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe112\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe112:\nbics x0,x1,x2,asr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run113(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run113:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe113\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe113:\nbics x1,x1,x2,asr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run114(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run114:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe114\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe114:\nbics x2,x1,x2,asr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run115(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run115:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe115\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe115:\nbics xzr,x1,x2,asr #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run116(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run116:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe116\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe116:\nbics x0,x1,x2,asr #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run117(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run117:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe117\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe117:\nbics x1,x1,x2,asr #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run118(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run118:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe118\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe118:\nbics x2,x1,x2,asr #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run119(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run119:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe119\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe119:\nbics xzr,x1,x2,asr #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run120(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run120:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe120\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe120:\nbics x0,x1,x2,asr #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run121(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run121:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe121\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe121:\nbics x1,x1,x2,asr #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run122(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run122:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe122\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe122:\nbics x2,x1,x2,asr #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run123(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run123:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe123\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe123:\nbics xzr,x1,x2,asr #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run124(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run124:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe124\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe124:\nbics x0,x1,x2,ror #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run125(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run125:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe125\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe125:\nbics x1,x1,x2,ror #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run126(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run126:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe126\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe126:\nbics x2,x1,x2,ror #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run127(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run127:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe127\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe127:\nbics xzr,x1,x2,ror #0\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run128(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run128:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe128\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe128:\nbics x0,x1,x2,ror #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run129(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run129:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe129\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe129:\nbics x1,x1,x2,ror #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run130(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run130:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe130\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe130:\nbics x2,x1,x2,ror #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run131(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run131:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe131\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe131:\nbics xzr,x1,x2,ror #1\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run132(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run132:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe132\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe132:\nbics x0,x1,x2,ror #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run133(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run133:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe133\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe133:\nbics x1,x1,x2,ror #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run134(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run134:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe134\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe134:\nbics x2,x1,x2,ror #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run135(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run135:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe135\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe135:\nbics xzr,x1,x2,ror #7\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run136(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run136:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe136\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe136:\nbics x0,x1,x2,ror #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run137(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run137:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe137\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe137:\nbics x1,x1,x2,ror #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run138(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run138:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe138\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe138:\nbics x2,x1,x2,ror #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run139(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run139:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe139\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe139:\nbics xzr,x1,x2,ror #32\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run140(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run140:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe140\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe140:\nbics x0,x1,x2,ror #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run141(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run141:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe141\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe141:\nbics x1,x1,x2,ror #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run142(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run142:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe142\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe142:\nbics x2,x1,x2,ror #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run143(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run143:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe143\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe143:\nbics xzr,x1,x2,ror #63\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
typedef void (*fn)(uint64_t*,uint64_t*);
struct variant {fn f;const char*ins;int width,d;const char*shift;int amount;};
static struct variant variants[]={
{run0,"bics w0,w1,w2,lsl #0",32,0,"lsl",0},
{run1,"bics w1,w1,w2,lsl #0",32,1,"lsl",0},
{run2,"bics w2,w1,w2,lsl #0",32,2,"lsl",0},
{run3,"bics wzr,w1,w2,lsl #0",32,31,"lsl",0},
{run4,"bics w0,w1,w2,lsl #1",32,0,"lsl",1},
{run5,"bics w1,w1,w2,lsl #1",32,1,"lsl",1},
{run6,"bics w2,w1,w2,lsl #1",32,2,"lsl",1},
{run7,"bics wzr,w1,w2,lsl #1",32,31,"lsl",1},
{run8,"bics w0,w1,w2,lsl #7",32,0,"lsl",7},
{run9,"bics w1,w1,w2,lsl #7",32,1,"lsl",7},
{run10,"bics w2,w1,w2,lsl #7",32,2,"lsl",7},
{run11,"bics wzr,w1,w2,lsl #7",32,31,"lsl",7},
{run12,"bics w0,w1,w2,lsl #31",32,0,"lsl",31},
{run13,"bics w1,w1,w2,lsl #31",32,1,"lsl",31},
{run14,"bics w2,w1,w2,lsl #31",32,2,"lsl",31},
{run15,"bics wzr,w1,w2,lsl #31",32,31,"lsl",31},
{run16,"bics w0,w1,w2,lsr #0",32,0,"lsr",0},
{run17,"bics w1,w1,w2,lsr #0",32,1,"lsr",0},
{run18,"bics w2,w1,w2,lsr #0",32,2,"lsr",0},
{run19,"bics wzr,w1,w2,lsr #0",32,31,"lsr",0},
{run20,"bics w0,w1,w2,lsr #1",32,0,"lsr",1},
{run21,"bics w1,w1,w2,lsr #1",32,1,"lsr",1},
{run22,"bics w2,w1,w2,lsr #1",32,2,"lsr",1},
{run23,"bics wzr,w1,w2,lsr #1",32,31,"lsr",1},
{run24,"bics w0,w1,w2,lsr #7",32,0,"lsr",7},
{run25,"bics w1,w1,w2,lsr #7",32,1,"lsr",7},
{run26,"bics w2,w1,w2,lsr #7",32,2,"lsr",7},
{run27,"bics wzr,w1,w2,lsr #7",32,31,"lsr",7},
{run28,"bics w0,w1,w2,lsr #31",32,0,"lsr",31},
{run29,"bics w1,w1,w2,lsr #31",32,1,"lsr",31},
{run30,"bics w2,w1,w2,lsr #31",32,2,"lsr",31},
{run31,"bics wzr,w1,w2,lsr #31",32,31,"lsr",31},
{run32,"bics w0,w1,w2,asr #0",32,0,"asr",0},
{run33,"bics w1,w1,w2,asr #0",32,1,"asr",0},
{run34,"bics w2,w1,w2,asr #0",32,2,"asr",0},
{run35,"bics wzr,w1,w2,asr #0",32,31,"asr",0},
{run36,"bics w0,w1,w2,asr #1",32,0,"asr",1},
{run37,"bics w1,w1,w2,asr #1",32,1,"asr",1},
{run38,"bics w2,w1,w2,asr #1",32,2,"asr",1},
{run39,"bics wzr,w1,w2,asr #1",32,31,"asr",1},
{run40,"bics w0,w1,w2,asr #7",32,0,"asr",7},
{run41,"bics w1,w1,w2,asr #7",32,1,"asr",7},
{run42,"bics w2,w1,w2,asr #7",32,2,"asr",7},
{run43,"bics wzr,w1,w2,asr #7",32,31,"asr",7},
{run44,"bics w0,w1,w2,asr #31",32,0,"asr",31},
{run45,"bics w1,w1,w2,asr #31",32,1,"asr",31},
{run46,"bics w2,w1,w2,asr #31",32,2,"asr",31},
{run47,"bics wzr,w1,w2,asr #31",32,31,"asr",31},
{run48,"bics w0,w1,w2,ror #0",32,0,"ror",0},
{run49,"bics w1,w1,w2,ror #0",32,1,"ror",0},
{run50,"bics w2,w1,w2,ror #0",32,2,"ror",0},
{run51,"bics wzr,w1,w2,ror #0",32,31,"ror",0},
{run52,"bics w0,w1,w2,ror #1",32,0,"ror",1},
{run53,"bics w1,w1,w2,ror #1",32,1,"ror",1},
{run54,"bics w2,w1,w2,ror #1",32,2,"ror",1},
{run55,"bics wzr,w1,w2,ror #1",32,31,"ror",1},
{run56,"bics w0,w1,w2,ror #7",32,0,"ror",7},
{run57,"bics w1,w1,w2,ror #7",32,1,"ror",7},
{run58,"bics w2,w1,w2,ror #7",32,2,"ror",7},
{run59,"bics wzr,w1,w2,ror #7",32,31,"ror",7},
{run60,"bics w0,w1,w2,ror #31",32,0,"ror",31},
{run61,"bics w1,w1,w2,ror #31",32,1,"ror",31},
{run62,"bics w2,w1,w2,ror #31",32,2,"ror",31},
{run63,"bics wzr,w1,w2,ror #31",32,31,"ror",31},
{run64,"bics x0,x1,x2,lsl #0",64,0,"lsl",0},
{run65,"bics x1,x1,x2,lsl #0",64,1,"lsl",0},
{run66,"bics x2,x1,x2,lsl #0",64,2,"lsl",0},
{run67,"bics xzr,x1,x2,lsl #0",64,31,"lsl",0},
{run68,"bics x0,x1,x2,lsl #1",64,0,"lsl",1},
{run69,"bics x1,x1,x2,lsl #1",64,1,"lsl",1},
{run70,"bics x2,x1,x2,lsl #1",64,2,"lsl",1},
{run71,"bics xzr,x1,x2,lsl #1",64,31,"lsl",1},
{run72,"bics x0,x1,x2,lsl #7",64,0,"lsl",7},
{run73,"bics x1,x1,x2,lsl #7",64,1,"lsl",7},
{run74,"bics x2,x1,x2,lsl #7",64,2,"lsl",7},
{run75,"bics xzr,x1,x2,lsl #7",64,31,"lsl",7},
{run76,"bics x0,x1,x2,lsl #32",64,0,"lsl",32},
{run77,"bics x1,x1,x2,lsl #32",64,1,"lsl",32},
{run78,"bics x2,x1,x2,lsl #32",64,2,"lsl",32},
{run79,"bics xzr,x1,x2,lsl #32",64,31,"lsl",32},
{run80,"bics x0,x1,x2,lsl #63",64,0,"lsl",63},
{run81,"bics x1,x1,x2,lsl #63",64,1,"lsl",63},
{run82,"bics x2,x1,x2,lsl #63",64,2,"lsl",63},
{run83,"bics xzr,x1,x2,lsl #63",64,31,"lsl",63},
{run84,"bics x0,x1,x2,lsr #0",64,0,"lsr",0},
{run85,"bics x1,x1,x2,lsr #0",64,1,"lsr",0},
{run86,"bics x2,x1,x2,lsr #0",64,2,"lsr",0},
{run87,"bics xzr,x1,x2,lsr #0",64,31,"lsr",0},
{run88,"bics x0,x1,x2,lsr #1",64,0,"lsr",1},
{run89,"bics x1,x1,x2,lsr #1",64,1,"lsr",1},
{run90,"bics x2,x1,x2,lsr #1",64,2,"lsr",1},
{run91,"bics xzr,x1,x2,lsr #1",64,31,"lsr",1},
{run92,"bics x0,x1,x2,lsr #7",64,0,"lsr",7},
{run93,"bics x1,x1,x2,lsr #7",64,1,"lsr",7},
{run94,"bics x2,x1,x2,lsr #7",64,2,"lsr",7},
{run95,"bics xzr,x1,x2,lsr #7",64,31,"lsr",7},
{run96,"bics x0,x1,x2,lsr #32",64,0,"lsr",32},
{run97,"bics x1,x1,x2,lsr #32",64,1,"lsr",32},
{run98,"bics x2,x1,x2,lsr #32",64,2,"lsr",32},
{run99,"bics xzr,x1,x2,lsr #32",64,31,"lsr",32},
{run100,"bics x0,x1,x2,lsr #63",64,0,"lsr",63},
{run101,"bics x1,x1,x2,lsr #63",64,1,"lsr",63},
{run102,"bics x2,x1,x2,lsr #63",64,2,"lsr",63},
{run103,"bics xzr,x1,x2,lsr #63",64,31,"lsr",63},
{run104,"bics x0,x1,x2,asr #0",64,0,"asr",0},
{run105,"bics x1,x1,x2,asr #0",64,1,"asr",0},
{run106,"bics x2,x1,x2,asr #0",64,2,"asr",0},
{run107,"bics xzr,x1,x2,asr #0",64,31,"asr",0},
{run108,"bics x0,x1,x2,asr #1",64,0,"asr",1},
{run109,"bics x1,x1,x2,asr #1",64,1,"asr",1},
{run110,"bics x2,x1,x2,asr #1",64,2,"asr",1},
{run111,"bics xzr,x1,x2,asr #1",64,31,"asr",1},
{run112,"bics x0,x1,x2,asr #7",64,0,"asr",7},
{run113,"bics x1,x1,x2,asr #7",64,1,"asr",7},
{run114,"bics x2,x1,x2,asr #7",64,2,"asr",7},
{run115,"bics xzr,x1,x2,asr #7",64,31,"asr",7},
{run116,"bics x0,x1,x2,asr #32",64,0,"asr",32},
{run117,"bics x1,x1,x2,asr #32",64,1,"asr",32},
{run118,"bics x2,x1,x2,asr #32",64,2,"asr",32},
{run119,"bics xzr,x1,x2,asr #32",64,31,"asr",32},
{run120,"bics x0,x1,x2,asr #63",64,0,"asr",63},
{run121,"bics x1,x1,x2,asr #63",64,1,"asr",63},
{run122,"bics x2,x1,x2,asr #63",64,2,"asr",63},
{run123,"bics xzr,x1,x2,asr #63",64,31,"asr",63},
{run124,"bics x0,x1,x2,ror #0",64,0,"ror",0},
{run125,"bics x1,x1,x2,ror #0",64,1,"ror",0},
{run126,"bics x2,x1,x2,ror #0",64,2,"ror",0},
{run127,"bics xzr,x1,x2,ror #0",64,31,"ror",0},
{run128,"bics x0,x1,x2,ror #1",64,0,"ror",1},
{run129,"bics x1,x1,x2,ror #1",64,1,"ror",1},
{run130,"bics x2,x1,x2,ror #1",64,2,"ror",1},
{run131,"bics xzr,x1,x2,ror #1",64,31,"ror",1},
{run132,"bics x0,x1,x2,ror #7",64,0,"ror",7},
{run133,"bics x1,x1,x2,ror #7",64,1,"ror",7},
{run134,"bics x2,x1,x2,ror #7",64,2,"ror",7},
{run135,"bics xzr,x1,x2,ror #7",64,31,"ror",7},
{run136,"bics x0,x1,x2,ror #32",64,0,"ror",32},
{run137,"bics x1,x1,x2,ror #32",64,1,"ror",32},
{run138,"bics x2,x1,x2,ror #32",64,2,"ror",32},
{run139,"bics xzr,x1,x2,ror #32",64,31,"ror",32},
{run140,"bics x0,x1,x2,ror #63",64,0,"ror",63},
{run141,"bics x1,x1,x2,ror #63",64,1,"ror",63},
{run142,"bics x2,x1,x2,ror #63",64,2,"ror",63},
{run143,"bics xzr,x1,x2,ror #63",64,31,"ror",63},
};
static uint64_t pairs[][2]={{0,0},{0,~0ULL},{~0ULL,0},{~0ULL,~0ULL},{0x8000000080000000ULL,0x0123456789abcdefULL},{0x7fffffff7fffffffULL,0x8000000080000000ULL},{0x0123456789abcdefULL,0xfedcba9876543210ULL},{0xa55aa55a5aa55aa5ULL,0x00ff0080ff00ff80ULL}};
static uint64_t flags[]={0,0x50000000,0xa0000000,0xf0000000};
int main(void){int first=1;puts("[");for(unsigned i=0;i<sizeof(variants)/sizeof(variants[0]);i++)for(int j=0;j<8;j++)for(int k=0;k<4;k++){struct variant v=variants[i];uint64_t in[4]={0xa5a5a5a55a5a5a5aULL,pairs[j][0],pairs[j][1],flags[k]},out[5];v.f(in,out);
printf("%s{\"instruction\":\"%s\",\"encoding\":\"%08x\",\"width\":%d,\"destination\":%d,\"shift\":\"%s\",\"amount\":%d,\"before\":[\"%016llx\",\"%016llx\",\"%016llx\"],\"after\":[\"%016llx\",\"%016llx\",\"%016llx\"],\"nzcv_before\":%llu,\"nzcv_after\":%llu}",first?"":",\n",v.ins,(unsigned)out[4],v.width,v.d,v.shift,v.amount,in[0],in[1],in[2],out[0],out[1],out[2],in[3],out[3]);first=0;
}puts("\n]");return 0;}