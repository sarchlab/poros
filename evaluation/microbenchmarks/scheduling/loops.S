// Caller-saved scalar functions shared by native Mach-O and simulated ELF.
// Each block creates a long chain, then a dependent fan-out that reserves all
// modeled units in the future. Mixed cases add an independent chain afterward,
// which can use earlier gaps. Controls retain the long chain and fan-out only.
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
.macro intloop name, mixed
begin \name
 mov x2, #1
 mov x3, #1
 mov x4, #1
 mov x5, #0
.p2align 6
1:
.rept 8
 mov x5, #0
.rept 8
 madd x2, x2, x3, x4
.endr
.irp r,9,10,11,12,13,14
 add x\r, x2, #1
.endr
.if \mixed
.rept 8
 add x5, x5, #1
.endr
.endif
 add x2, x2, x5
.endr
endloop
 mov x0, #0
.irp r,9,10,11,12,13,14
 add x0, x0, x\r
.endr
.if \mixed
 add x0, x0, x5
.endif
 ret
.endm
intloop int_chain, 0
intloop int_interleaved, 1

.macro fploop name, mixed
begin \name
 fmov d0, #1.0
 fmov d30, #0.5
 fmov d31, #1.0
 fmov d1, xzr
.p2align 6
1:
.rept 8
 fmov d1, xzr
.rept 8
 fmadd d0, d0, d31, d30
.endr
.irp r,4,5,6,7
 fadd d\r, d0, d30
.endr
.if \mixed
.rept 8
 fadd d1, d1, d30
.endr
.endif
 fadd d0, d0, d1
.endr
endloop
 fadd d0, d4, d5
 fadd d0, d0, d6
 fadd d0, d0, d7
.if \mixed
 fadd d0, d0, d1
.endif
 fcvtzu x0, d0
 ret
.endm
fploop fp_chain, 0
fploop fp_interleaved, 1
