// Identical loop bodies on Mach-O and ELF. Only caller-saved registers are used.
.text
.macro begin name
.p2align 6
.globl _\name
_\name:
.endm
.macro endloop
 subs x0, x0, #1
 b.ne 1b
.endm
.macro noploop name, count
begin \name
.p2align 6
1:
.rept \count
 nop
.endr
endloop
 mov x0, #0
 ret
.endm
noploop nop_32, 32
noploop nop_128, 128
noploop nop_256, 256
noploop nop_512, 512

.macro addloop name, chains
begin \name
.irp reg,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17
.if (\reg - 2) < \chains
 mov x\reg, #0
.endif
.endr
.p2align 6
1:
.rept 256 / \chains
.irp reg,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17
.if (\reg - 2) < \chains
 add x\reg, x\reg, #1
.endif
.endr
.endr
endloop
 mov x0, #0
.irp reg,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17
.if (\reg - 2) < \chains
 add x0, x0, x\reg
.endif
.endr
 ret
.endm
addloop add_1, 1
addloop add_2, 2
addloop add_4, 4
addloop add_8, 8
addloop add_16, 16

.macro fploop name, chains
begin \name
 fmov d31, #0.5
.irp reg,0,1,2,3,4,5,6,7,16,17,18,19,20,21,22,23
.if (\reg < 8 && \reg < \chains) || (\reg >= 16 && (\reg - 8) < \chains)
 fmov d\reg, xzr
.endif
.endr
.p2align 6
1:
.rept 256 / \chains
.irp reg,0,1,2,3,4,5,6,7,16,17,18,19,20,21,22,23
.if (\reg < 8 && \reg < \chains) || (\reg >= 16 && (\reg - 8) < \chains)
 fadd d\reg, d\reg, d31
.endif
.endr
.endr
endloop
 mov x0, #0
.irp reg,0,1,2,3,4,5,6,7,16,17,18,19,20,21,22,23
.if (\reg < 8 && \reg < \chains) || (\reg >= 16 && (\reg - 8) < \chains)
 fcvtzu x2, d\reg
 add x0, x0, x2
.endif
.endr
 ret
.endm
fploop fp_1, 1
fploop fp_4, 4
fploop fp_8, 8

fploop fp_16, 16

begin load_8
.p2align 6
1:
.rept 32
.irp reg,2,3,4,5,6,7,8,9
 ldr x\reg, [x1, #(\reg - 2) * 8]
.endr
.endr
endloop
 mov x0, #0
.irp reg,2,3,4,5,6,7,8,9
 add x0, x0, x\reg
.endr
 ret

begin store_8
.irp reg,2,3,4,5,6,7,8,9
 mov x\reg, #(\reg + 15)
.endr
.p2align 6
1:
.rept 32
.irp reg,2,3,4,5,6,7,8,9
 str x\reg, [x1, #(\reg - 2) * 8]
.endr
.endr
endloop
 mov x0, #0
.irp reg,2,3,4,5,6,7,8,9
 ldr x2, [x1, #(\reg - 2) * 8]
 add x0, x0, x2
.endr
 ret

begin mixed
.irp reg,2,3,4,5
 mov x\reg, #0
.endr
.irp reg,0,1,2,3
 fmov d\reg, xzr
.endr
 fmov d16, #0.5
.p2align 6
1:
.rept 16
 add x2, x2, #1
 fadd d0, d0, d16
 ldr x6, [x1]
 add x3, x3, #1
 fadd d1, d1, d16
 ldr x7, [x1, #8]
 add x4, x4, #1
 fadd d2, d2, d16
 ldr x8, [x1, #16]
 add x5, x5, #1
 fadd d3, d3, d16
 ldr x9, [x1, #24]
.endr
endloop
 mov x0, #0
.irp reg,0,1,2,3
 fcvtzu x10, d\reg
 add x0, x0, x10
.endr
.irp reg,2,3,4,5,6,7,8,9
 add x0, x0, x\reg
.endr
 ret

begin mixed_16
.irp reg,2,3,4,5,6,7,8,9
 mov x\reg, #0
.endr
.irp reg,0,1,2,3,4,5,6,7,16,17,18,19,20,21,22,23
 fmov d\reg, xzr
.endr
 fmov d31, #0.5
.p2align 6
1:
.rept 4
 add x2, x2, #1
 fadd d0, d0, d31
 ldr x10, [x1, #0]
 add x3, x3, #1
 fadd d1, d1, d31
 ldr x11, [x1, #8]
 add x4, x4, #1
 fadd d2, d2, d31
 ldr x12, [x1, #16]
 add x5, x5, #1
 fadd d3, d3, d31
 ldr x13, [x1, #24]
 add x6, x6, #1
 fadd d4, d4, d31
 ldr x10, [x1, #0]
 add x7, x7, #1
 fadd d5, d5, d31
 ldr x11, [x1, #8]
 add x8, x8, #1
 fadd d6, d6, d31
 ldr x12, [x1, #16]
 add x9, x9, #1
 fadd d7, d7, d31
 ldr x13, [x1, #24]
 add x2, x2, #1
 fadd d16, d16, d31
 ldr x10, [x1, #0]
 add x3, x3, #1
 fadd d17, d17, d31
 ldr x11, [x1, #8]
 add x4, x4, #1
 fadd d18, d18, d31
 ldr x12, [x1, #16]
 add x5, x5, #1
 fadd d19, d19, d31
 ldr x13, [x1, #24]
 add x6, x6, #1
 fadd d20, d20, d31
 ldr x10, [x1, #0]
 add x7, x7, #1
 fadd d21, d21, d31
 ldr x11, [x1, #8]
 add x8, x8, #1
 fadd d22, d22, d31
 ldr x12, [x1, #16]
 add x9, x9, #1
 fadd d23, d23, d31
 ldr x13, [x1, #24]
.endr
endloop
 mov x0, #0
.irp reg,0,1,2,3,4,5,6,7,16,17,18,19,20,21,22,23
 fcvtzu x14, d\reg
 add x0, x0, x14
.endr
.irp reg,2,3,4,5,6,7,8,9,10,11,12,13
 add x0, x0, x\reg
.endr
 ret
