#include <stdint.h>
#include <stdio.h>
extern void run0(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run0:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe0\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe0:\ncneg w0,w1,eq\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run1(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run1:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe1\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe1:\ncneg w1,w1,eq\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run2(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run2:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe2\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe2:\ncneg w0,w1,ne\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run3(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run3:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe3\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe3:\ncneg w1,w1,ne\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run4(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run4:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe4\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe4:\ncneg w0,w1,cs\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run5(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run5:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe5\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe5:\ncneg w1,w1,cs\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run6(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run6:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe6\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe6:\ncneg w0,w1,cc\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run7(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run7:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe7\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe7:\ncneg w1,w1,cc\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run8(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run8:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe8\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe8:\ncneg w0,w1,mi\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run9(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run9:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe9\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe9:\ncneg w1,w1,mi\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run10(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run10:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe10\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe10:\ncneg w0,w1,pl\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run11(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run11:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe11\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe11:\ncneg w1,w1,pl\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run12(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run12:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe12\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe12:\ncneg w0,w1,vs\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run13(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run13:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe13\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe13:\ncneg w1,w1,vs\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run14(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run14:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe14\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe14:\ncneg w0,w1,vc\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run15(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run15:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe15\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe15:\ncneg w1,w1,vc\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run16(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run16:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe16\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe16:\ncneg w0,w1,hi\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run17(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run17:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe17\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe17:\ncneg w1,w1,hi\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run18(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run18:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe18\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe18:\ncneg w0,w1,ls\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run19(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run19:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe19\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe19:\ncneg w1,w1,ls\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run20(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run20:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe20\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe20:\ncneg w0,w1,ge\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run21(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run21:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe21\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe21:\ncneg w1,w1,ge\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run22(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run22:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe22\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe22:\ncneg w0,w1,lt\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run23(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run23:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe23\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe23:\ncneg w1,w1,lt\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run24(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run24:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe24\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe24:\ncneg w0,w1,gt\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run25(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run25:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe25\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe25:\ncneg w1,w1,gt\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run26(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run26:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe26\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe26:\ncneg w0,w1,le\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run27(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run27:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe27\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe27:\ncneg w1,w1,le\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run28(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run28:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe28\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe28:\ncneg x0,x1,eq\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run29(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run29:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe29\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe29:\ncneg x1,x1,eq\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run30(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run30:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe30\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe30:\ncneg x0,x1,ne\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run31(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run31:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe31\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe31:\ncneg x1,x1,ne\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run32(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run32:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe32\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe32:\ncneg x0,x1,cs\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run33(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run33:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe33\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe33:\ncneg x1,x1,cs\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run34(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run34:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe34\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe34:\ncneg x0,x1,cc\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run35(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run35:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe35\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe35:\ncneg x1,x1,cc\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run36(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run36:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe36\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe36:\ncneg x0,x1,mi\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run37(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run37:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe37\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe37:\ncneg x1,x1,mi\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run38(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run38:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe38\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe38:\ncneg x0,x1,pl\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run39(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run39:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe39\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe39:\ncneg x1,x1,pl\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run40(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run40:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe40\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe40:\ncneg x0,x1,vs\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run41(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run41:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe41\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe41:\ncneg x1,x1,vs\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run42(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run42:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe42\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe42:\ncneg x0,x1,vc\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run43(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run43:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe43\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe43:\ncneg x1,x1,vc\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run44(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run44:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe44\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe44:\ncneg x0,x1,hi\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run45(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run45:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe45\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe45:\ncneg x1,x1,hi\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run46(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run46:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe46\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe46:\ncneg x0,x1,ls\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run47(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run47:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe47\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe47:\ncneg x1,x1,ls\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run48(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run48:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe48\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe48:\ncneg x0,x1,ge\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run49(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run49:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe49\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe49:\ncneg x1,x1,ge\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run50(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run50:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe50\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe50:\ncneg x0,x1,lt\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run51(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run51:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe51\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe51:\ncneg x1,x1,lt\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run52(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run52:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe52\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe52:\ncneg x0,x1,gt\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run53(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run53:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe53\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe53:\ncneg x1,x1,gt\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run54(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run54:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe54\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe54:\ncneg x0,x1,le\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
extern void run55(uint64_t*,uint64_t*);
__asm__(".text\n.p2align 2\n_run55:\nmov x9,x0\nmov x10,x1\nldp x0,x1,[x9]\nldr x2,[x9,#16]\nldr x11,[x9,#24]\nmsr nzcv,x11\nadr x12,Lprobe55\nldr w12,[x12]\nstr x12,[x10,#32]\nLprobe55:\ncneg x1,x1,le\nstp x0,x1,[x10]\nstr x2,[x10,#16]\nmrs x11,nzcv\nstr x11,[x10,#24]\nret\n");
typedef void (*fn)(uint64_t*,uint64_t*);
struct variant {fn f;const char*ins;int width,d,s,c;const char*cond;};
static struct variant variants[]={
{run0,"cneg w0,w1,eq",32,0,1,2,"eq"},
{run1,"cneg w1,w1,eq",32,1,1,2,"eq"},
{run2,"cneg w0,w1,ne",32,0,1,2,"ne"},
{run3,"cneg w1,w1,ne",32,1,1,2,"ne"},
{run4,"cneg w0,w1,cs",32,0,1,2,"cs"},
{run5,"cneg w1,w1,cs",32,1,1,2,"cs"},
{run6,"cneg w0,w1,cc",32,0,1,2,"cc"},
{run7,"cneg w1,w1,cc",32,1,1,2,"cc"},
{run8,"cneg w0,w1,mi",32,0,1,2,"mi"},
{run9,"cneg w1,w1,mi",32,1,1,2,"mi"},
{run10,"cneg w0,w1,pl",32,0,1,2,"pl"},
{run11,"cneg w1,w1,pl",32,1,1,2,"pl"},
{run12,"cneg w0,w1,vs",32,0,1,2,"vs"},
{run13,"cneg w1,w1,vs",32,1,1,2,"vs"},
{run14,"cneg w0,w1,vc",32,0,1,2,"vc"},
{run15,"cneg w1,w1,vc",32,1,1,2,"vc"},
{run16,"cneg w0,w1,hi",32,0,1,2,"hi"},
{run17,"cneg w1,w1,hi",32,1,1,2,"hi"},
{run18,"cneg w0,w1,ls",32,0,1,2,"ls"},
{run19,"cneg w1,w1,ls",32,1,1,2,"ls"},
{run20,"cneg w0,w1,ge",32,0,1,2,"ge"},
{run21,"cneg w1,w1,ge",32,1,1,2,"ge"},
{run22,"cneg w0,w1,lt",32,0,1,2,"lt"},
{run23,"cneg w1,w1,lt",32,1,1,2,"lt"},
{run24,"cneg w0,w1,gt",32,0,1,2,"gt"},
{run25,"cneg w1,w1,gt",32,1,1,2,"gt"},
{run26,"cneg w0,w1,le",32,0,1,2,"le"},
{run27,"cneg w1,w1,le",32,1,1,2,"le"},
{run28,"cneg x0,x1,eq",64,0,1,2,"eq"},
{run29,"cneg x1,x1,eq",64,1,1,2,"eq"},
{run30,"cneg x0,x1,ne",64,0,1,2,"ne"},
{run31,"cneg x1,x1,ne",64,1,1,2,"ne"},
{run32,"cneg x0,x1,cs",64,0,1,2,"cs"},
{run33,"cneg x1,x1,cs",64,1,1,2,"cs"},
{run34,"cneg x0,x1,cc",64,0,1,2,"cc"},
{run35,"cneg x1,x1,cc",64,1,1,2,"cc"},
{run36,"cneg x0,x1,mi",64,0,1,2,"mi"},
{run37,"cneg x1,x1,mi",64,1,1,2,"mi"},
{run38,"cneg x0,x1,pl",64,0,1,2,"pl"},
{run39,"cneg x1,x1,pl",64,1,1,2,"pl"},
{run40,"cneg x0,x1,vs",64,0,1,2,"vs"},
{run41,"cneg x1,x1,vs",64,1,1,2,"vs"},
{run42,"cneg x0,x1,vc",64,0,1,2,"vc"},
{run43,"cneg x1,x1,vc",64,1,1,2,"vc"},
{run44,"cneg x0,x1,hi",64,0,1,2,"hi"},
{run45,"cneg x1,x1,hi",64,1,1,2,"hi"},
{run46,"cneg x0,x1,ls",64,0,1,2,"ls"},
{run47,"cneg x1,x1,ls",64,1,1,2,"ls"},
{run48,"cneg x0,x1,ge",64,0,1,2,"ge"},
{run49,"cneg x1,x1,ge",64,1,1,2,"ge"},
{run50,"cneg x0,x1,lt",64,0,1,2,"lt"},
{run51,"cneg x1,x1,lt",64,1,1,2,"lt"},
{run52,"cneg x0,x1,gt",64,0,1,2,"gt"},
{run53,"cneg x1,x1,gt",64,1,1,2,"gt"},
{run54,"cneg x0,x1,le",64,0,1,2,"le"},
{run55,"cneg x1,x1,le",64,1,1,2,"le"},
};
static uint64_t values[]={0x0ULL,0x1ULL,0x64ULL,0xffffffffffffffffULL,0x80000000ULL,0x8000000000000000ULL,0x7fffffffffffffffULL,0x123456789abcdefULL};
static uint64_t counts[]={0x0ULL,0x1ULL,0xfULL,0x10ULL,0x1fULL,0x20ULL,0x21ULL,0x3fULL,0x40ULL,0x41ULL,0x7fULL,0xffULL,0xffffffffffffffffULL,0x100000020ULL};
int main(void){int first=1;puts("[");
for(unsigned i=0;i<sizeof(variants)/sizeof(variants[0]);i++){struct variant v=variants[i];
for(int j=0;j<16;j++)for(int k=0;k<8;k++){uint64_t flags=(uint64_t)j<<28,source=values[k],count=0x5aa55aa55aa55aa5ULL;
uint64_t in[4]={0xa5a5a5a55a5a5a5aULL,source,count,flags},out[5];v.f(in,out);
printf("%s{\"instruction\":\"%s\",\"encoding\":\"%08x\",\"width\":%d,\"destination\":%d,\"source\":%d,\"count_register\":%d,\"condition\":\"%s\",\"before\":[\"%016llx\",\"%016llx\",\"%016llx\"],\"after\":[\"%016llx\",\"%016llx\",\"%016llx\"],\"nzcv_before\":%llu,\"nzcv_after\":%llu}",first?"":",\n",v.ins,(unsigned)out[4],v.width,v.d,v.s,v.c,v.cond,in[0],in[1],in[2],out[0],out[1],out[2],in[3],out[3]);first=0;
}}puts("\n]");return 0;}