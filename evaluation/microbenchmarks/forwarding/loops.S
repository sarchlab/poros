// Identical ELF/Mach-O loops. Every outer iteration performs 64 updates.
// Only caller-saved registers are used. Memory fits in one aligned 64-byte line.
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
.macro gprloop name, memory, width=64, indexed=0
begin \name
 mov x2, #1
 mov x3, #1
 mov x4, #0
 str x2, [x1]
.p2align 6
1:
.rept 64
.if \memory == 1
.if \width == 32
 ldr w2, [x1]
.elseif (\indexed & 1)
 ldr x2, [x1, x4]
.else
 ldr x2, [x1]
.endif
.elseif \memory == 2
 ldr x2, [x1]
.endif
.if \memory == 2
 add x2, x2, x3
 add x3, x3, #1
.elseif \width == 32
 add w2, w2, #1
.else
 add x2, x2, #1
.endif
.if \memory == 1
.if \width == 32
 str w2, [x1]
.elseif (\indexed & 2)
 str x2, [x1, x4]
.else
 str x2, [x1]
.endif
.elseif \memory == 2
 str x2, [x1, #8]
.endif
.endr
endloop
 mov x0, x2
 ret
.endm
gprloop gpr64_register, 0
gprloop gpr64_same, 1
gprloop gpr64_disjoint, 2
gprloop gpr32_register, 0, 32
gprloop gpr32_same, 1, 32
gprloop gpr64_indexed, 1, 64, 3
gprloop gpr64_indexed_load, 1, 64, 1
gprloop gpr64_indexed_store, 1, 64, 2

// Increment and wrap modulo 65536: both CSEL outcomes occur in long runs.
// This probes load/compare/select/store dependencies; it is not Nussinov itself.
.macro selectloop name, memory
begin \name
 mov x2, #0
 str x2, [x1]
.p2align 6
1:
.rept 64
.if \memory
 ldr x2, [x1]
.endif
 add x2, x2, #1
 cmp x2, #16, lsl #12
 csel x2, x2, xzr, lo
.if \memory
 str x2, [x1]
.endif
.endr
endloop
 mov x0, x2
 ret
.endm
selectloop select_register, 0
selectloop select_same, 1

.macro fploop name, memory
begin \name
 fmov d2, #1.0
 fmov d3, #0.5
 fmov d4, #0.5
 str d2, [x1]
.p2align 6
1:
.rept 64
.if \memory
 ldr d2, [x1]
.endif
.if \memory == 2
 fadd d2, d2, d4
 fadd d4, d4, d3
 str d2, [x1, #8]
.else
 fadd d2, d2, d3
.if \memory
 str d2, [x1]
.endif
.endif
.endr
endloop
 fcvtzu x0, d2
 ret
.endm
fploop fp_register, 0
fploop fp_same, 1
fploop fp_disjoint, 2
