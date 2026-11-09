// Same assembly bodies are linked into native Mach-O and freestanding ELF.
// MUL gates use integer registers, so their length does not consume FP renames.
// Constant operands do not remove architectural dependencies or instructions.
.text
.macro begin name
.p2align 6
.globl _\name
_\name:
.endm
.macro window name, gate, count, fp
begin \name
 mov x9, #1
 mov x10, #1
 mov x11, #1
 fmov d30, #1.0
 fmov d31, #2.0
.p2align 6
1:
.rept \gate
 mul x9, x9, x11
.endr
.set i, 0
.rept \count
.if \fp
 // Repeated overwrite allocates a fresh destination; operands are independent.
 fadd d0, d30, d31
.else
 nop
.endif
.endr
.rept \gate
 mul x10, x10, x11
.endr
 // Join both gate dependencies, retaining the nonzero fixed point (1,1).
 and x9, x9, x10
 orr x10, x9, #1
 add x12, x9, x10
 add x13, x12, #0
 subs x0, x0, #1
 b.ne 1b
.if \fp
 fcvtzu x0, d0
 add x0, x0, x13
.else
 mov x0, x13
.endif
 ret
.endm
.macro ports name, count
begin \name
 fmov d30, #1.0
 fmov d31, #2.0
.p2align 6
1:
.rept \count
 fadd d0, d30, d31
.endr
 subs x0, x0, #1
 b.ne 1b
 fcvtzu x0, d0
 ret
.endm
.macro divide r, normal
.if \normal
 fdiv d\r, d30, d\r
.else
 fdiv d\r, d\r, d31
.endif
.endm
.macro squareroot r, normal
.if \normal
 fadd d\r, d\r, d31
.endif
 fsqrt d\r, d\r
.endm
.macro arithmetic name, op, lanes, normal=0
begin \name
 fmov d31, #1.0
 fmov d30, #2.0
.irp r,0,1,2,3,4,5,6,7,16,17,18,19,20,21,22,23
.if \normal
 fmov d\r, #1.5
.else
 fmov d\r, #1.0
.endif
.endr
.p2align 6
1:
.rept 64/\lanes
.ifc \op,fdiv
 divide 0, \normal
.if \lanes > 1
.irp r,1,2,3
 divide \r, \normal
.endr
.endif
.if \lanes > 4
.irp r,4,5,6,7,16,17,18,19,20,21,22,23
 divide \r, \normal
.endr
.endif
.else
 squareroot 0, \normal
.if \lanes > 1
.irp r,1,2,3
 squareroot \r, \normal
.endr
.endif
.if \lanes > 4
.irp r,4,5,6,7,16,17,18,19,20,21,22,23
 squareroot \r, \normal
.endr
.endif
.endif
.endr
 subs x0, x0, #1
 b.ne 1b
 .if \lanes > 1
.irp r,1,2,3
 fadd d0, d0, d\r
.endr
.endif
.if \lanes > 4
.irp r,4,5,6,7,16,17,18,19,20,21,22,23
 fadd d0, d0, d\r
.endr
.endif
 fcvtzu x0, d0
 ret
.endm
window fp_g64_n64, 64, 64, 1
window fp_g64_n128, 64, 128, 1
window fp_g64_n192, 64, 192, 1
window fp_g64_n256, 64, 256, 1
window fp_g64_n288, 64, 288, 1
window fp_g64_n320, 64, 320, 1
window fp_g64_n336, 64, 336, 1
window fp_g64_n344, 64, 344, 1
window fp_g64_n352, 64, 352, 1
window fp_g64_n356, 64, 356, 1
window fp_g64_n360, 64, 360, 1
window fp_g64_n368, 64, 368, 1
window fp_g64_n384, 64, 384, 1
window fp_g64_n416, 64, 416, 1
window fp_g64_n448, 64, 448, 1
window nop_g64_n256, 64, 256, 0
window nop_g64_n352, 64, 352, 0
window nop_g64_n448, 64, 448, 0
window nop_g64_n576, 64, 576, 0
window nop_g64_n640, 64, 640, 0
window fp_g96_n64, 96, 64, 1
window fp_g96_n128, 96, 128, 1
window fp_g96_n192, 96, 192, 1
window fp_g96_n256, 96, 256, 1
window fp_g96_n288, 96, 288, 1
window fp_g96_n320, 96, 320, 1
window fp_g96_n336, 96, 336, 1
window fp_g96_n344, 96, 344, 1
window fp_g96_n352, 96, 352, 1
window fp_g96_n356, 96, 356, 1
window fp_g96_n360, 96, 360, 1
window fp_g96_n368, 96, 368, 1
window fp_g96_n384, 96, 384, 1
window fp_g96_n416, 96, 416, 1
window fp_g96_n448, 96, 448, 1
window nop_g96_n256, 96, 256, 0
window nop_g96_n352, 96, 352, 0
window nop_g96_n448, 96, 448, 0
window nop_g96_n576, 96, 576, 0
window nop_g96_n640, 96, 640, 0
ports fp_ports_n128, 128
ports fp_ports_n352, 352
ports fp_ports_n448, 448
arithmetic fdiv_lanes1, fdiv, 1
arithmetic fdiv_lanes4, fdiv, 4
arithmetic fdiv_lanes16, fdiv, 16
arithmetic fsqrt_lanes1, fsqrt, 1
arithmetic fsqrt_lanes4, fsqrt, 4
arithmetic fsqrt_lanes16, fsqrt, 16
window fp_g64_n348, 64, 348, 1
window fp_g64_n350, 64, 350, 1
window fp_g64_n354, 64, 354, 1
window fp_g64_n358, 64, 358, 1
window fp_g64_n362, 64, 362, 1
window fp_g64_n364, 64, 364, 1
window fp_g64_n366, 64, 366, 1
window fp_g64_n370, 64, 370, 1
window fp_g64_n372, 64, 372, 1
window fp_g96_n348, 96, 348, 1
window fp_g96_n350, 96, 350, 1
window fp_g96_n354, 96, 354, 1
window fp_g96_n358, 96, 358, 1
window fp_g96_n362, 96, 362, 1
window fp_g96_n364, 96, 364, 1
window fp_g96_n366, 96, 366, 1
window fp_g96_n370, 96, 370, 1
window fp_g96_n372, 96, 372, 1
arithmetic fdiv_normal_lanes1, fdiv, 1, 1
arithmetic fdiv_normal_lanes4, fdiv, 4, 1
arithmetic fdiv_normal_lanes16, fdiv, 16, 1
arithmetic fsqrt_normal_lanes1, fsqrt, 1, 1
arithmetic fsqrt_normal_lanes4, fsqrt, 4, 1
arithmetic fsqrt_normal_lanes16, fsqrt, 16, 1
