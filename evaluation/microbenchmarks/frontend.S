.text
.p2align 6
.globl _nop_probe
_nop_probe:
.p2align 6
1:
.rept 256
 nop
.endr
 subs x0, x0, #1
 b.ne 1b
 ret
.p2align 6
.globl _add_probe
_add_probe:
.p2align 6
1:
.rept 32
 add x1,x1,#1
 add x2,x2,#1
 add x3,x3,#1
 add x4,x4,#1
 add x5,x5,#1
 add x6,x6,#1
 add x7,x7,#1
 add x8,x8,#1
.endr
 subs x0,x0,#1
 b.ne 1b
 ret
