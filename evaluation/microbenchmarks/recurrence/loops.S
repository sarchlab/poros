// Compare/select and memory-carried recurrence probes. The native Mach-O
// collector and the simulated ELF assemble these same bodies. Only caller-saved
// registers below x18 are used. x0 holds the outer trip count and x1 the buffer.
// Dense loops run 64 updates per outer trip, tight loops 64 single-update trips,
// and kernel replicas 1024 inner trips. README.md lists every analytic output.
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

// Cycle-scale references: one-cycle integer and three-cycle FP dependencies.
begin clock_add
 mov x2, #1
.p2align 6
1:
.rept 64
 add x2, x2, #1
.endr
endloop
 mov x0, x2
 ret

begin clock_fadd
 fmov d2, #1.0
 fmov d3, #0.5
.p2align 6
1:
.rept 64
 fadd d2, d2, d3
.endr
endloop
 fcvtzu x0, d2
 ret

// Register-carried compare/select recurrences. Every update has the same four
// instructions: X accumulates the previous selection, Y advances the candidate.
// "separated" moves X between the compare and the select, so adjacency is the
// only difference. pad shifts the loop by 4*pad bytes from a 64-byte boundary.
// form: 0 CMP Wn,Wm  1 CMP Xn,Xm  2 CMP Wn,#15  3 CMP Wn,#16
//       4 CMP Wn,#1,LSL #12  5 CMP Wn,Wm + CSINC
// Forms 0, 1 and 5 select between the two compared registers; 2-4 select two
// other registers. The remaining forms vary only that operand relation:
//       6 CMP Wn,Wm, select two other registers
//       7 CMP Wn,#15, select the compared register or one other
//       8 CMP Wn,Wm, select the compared registers in swapped order
.macro compare form
.if \form == 1
 cmp x2, x6
.elseif \form == 2
 cmp w2, #15
.elseif \form == 3
 cmp w2, #16
.elseif \form == 4
 cmp w2, #1, lsl #12
.elseif \form == 7
 cmp w2, #15
.else
 cmp w2, w6
.endif
.endm
.macro select form
.if \form == 0
 csel w2, w2, w6, ge
.elseif \form == 1
 csel x2, x2, x6, ge
.elseif \form == 5
 csinc w2, w2, w6, ge
.elseif \form == 6
 csel w2, w4, w5, lo
.elseif \form == 7
 csel w2, w6, w2, lo
.elseif \form == 8
 csel w2, w6, w2, lt
.else
 csel w2, w6, w5, lo
.endif
.endm
.macro selectloop name, form, separated=0, pad=0
begin \name
 mov x7, #0
 mov x5, #0
.if \form == 2
 mov x2, #0
 mov x6, #100
.elseif \form == 3
 mov x2, #0
 mov x6, #100
.elseif \form == 4
 mov x2, #0
 mov x6, #5000
.elseif \form == 6
 mov x2, #0
 mov x6, #1
 mov x4, #0x40000000
.elseif \form == 7
 mov x2, #0
 mov x6, #100
.else
 mov x2, #1
 mov x6, #1
.endif
.p2align 6
.rept \pad
 nop
.endr
1:
.rept 64
.if \separated
 add w6, w6, #1
 compare \form
 add x7, x7, x2
 select \form
.else
 add x7, x7, x2
 add w6, w6, #1
 compare \form
 select \form
.endif
.endr
endloop
 add x0, x7, x2
 ret
.endm
selectloop sel32_rr_adj, 0
selectloop sel32_rr_sep, 0, 1
selectloop sel64_rr_adj, 1
selectloop sel64_rr_sep, 1, 1
selectloop sel32_ri_adj, 2
selectloop sel32_ri_sep, 2, 1
selectloop sel32_ri16_adj, 3
selectloop sel32_shift_adj, 4
selectloop sel32_rr_adj_p8, 0, 0, 2
selectloop sel32_rr_sep_p8, 0, 1, 2
selectloop sel32_rr_adj_p4, 0, 0, 1
selectloop sel32_csinc_adj, 5

// Issue-throughput control: no instruction depends on another in the loop.
// Each unit is one compare, one select and two independent adds, which exceeds
// six integer operations per cycle at eight instructions per cycle unless the
// adjacent pair issues as one operation. Both orders have the same instructions,
// count and loop size. w3 changes once per outer trip.
.macro throughput name, separated
begin \name
 mov w3, #0
 mov w4, #0
.p2align 6
1:
.rept 16
.irp r,8,9,10,11
 cmp w3, w4
.if \separated
 add w12, w3, #1
 csel w\r, w3, w4, ge
.else
 csel w\r, w3, w4, ge
 add w12, w3, #1
.endif
 add w13, w3, #2
.endr
.endr
 add w3, w3, #1
endloop
 add w0, w8, w9
 add w0, w0, w10
 add w0, w0, w11
 add w0, w0, w12
 add w0, w0, w13
 ret
.endm
throughput thr32_mix_adj, 0
throughput thr32_mix_sep, 1

// Memory-carried maximum, six instructions per update. delta 0 keeps the
// stored maximum constant; delta 1 makes the candidate win every update.
// Both orders keep one instruction between the store and the next load, and
// no adjacent pair straddles a 64-byte line.
.macro maxloop name, delta, start, separated=0
begin \name
 mov x7, #0
 mov w5, #\delta
 mov w6, #\start
 mov w2, #5
 str w2, [x1]
.p2align 6
1:
.rept 64
 add w6, w6, w5
 ldr w3, [x1]
 cmp w6, w3
.if \separated
 add x7, x7, x6
 csel w2, w6, w3, ge
.else
 csel w2, w6, w3, ge
 add x7, x7, x6
.endif
 str w2, [x1]
.endr
endloop
 ldr w2, [x1]
 add x0, x7, x2
 ret
.endm
maxloop mem32_max_const, 0, 1
maxloop mem32_max_chg, 1, 5
maxloop mem32_max_const_sep, 0, 1, 1

// Load, increment, store. form: 0 same base  1 second base register holding
// the same address  2 displacement 8  3 64-bit store, 32-bit load
.macro addloop name, form
begin \name
 mov x2, #1
 mov x9, x1
 str x2, [x1]
 str x2, [x1, #8]
.p2align 6
1:
.rept 64
.if \form == 1
 ldr x2, [x9]
.elseif \form == 2
 ldr x2, [x1, #8]
.elseif \form == 3
 ldr w2, [x1]
.else
 ldr x2, [x1]
.endif
 add x2, x2, #1
.if \form == 2
 str x2, [x1, #8]
.else
 str x2, [x1]
.endif
.endr
endloop
 mov x0, x2
 ret
.endm
addloop mem64_add, 0
addloop mem64_add_alias, 1
addloop mem64_add_disp, 2
addloop mem64_partial, 3

// Same instruction classes without a memory dependency: the load address is
// never stored to, and the register accumulator carries the only recurrence.
begin mem64_independent
 mov x2, #1
 mov x3, #0
 str x2, [x1]
.p2align 6
1:
.rept 64
 ldr x2, [x1]
 add x3, x3, x2
 str x3, [x1, #8]
.endr
endloop
 mov x0, x3
 ret

begin mem_fadd
 fmov d2, #1.0
 fmov d3, #0.5
 str d2, [x1]
.p2align 6
1:
.rept 64
 ldr d2, [x1]
 fadd d2, d2, d3
 str d2, [x1]
.endr
endloop
 fcvtzu x0, d2
 ret

// Store-to-load distance control. Each chain is load, two increments, store.
// "near" reloads immediately after its store. "mid" and "far" interleave two
// and three chains on separate lines, placing four and eight instructions
// between a store and its reload.
begin fwd_near
 mov x2, #1
 str x2, [x1]
.p2align 6
1:
.rept 64
 ldr x2, [x1]
 add x2, x2, #1
 add x2, x2, #1
 str x2, [x1]
.endr
endloop
 mov x0, x2
 ret

.macro farloop name, chains
begin \name
 mov x2, #1
 add x9, x1, #64
 add x10, x1, #128
 str x2, [x1]
 str x2, [x9]
 str x2, [x10]
 mov x4, #0
.p2align 6
1:
.rept 32
 ldr x2, [x1]
 add x2, x2, #1
 add x2, x2, #1
 str x2, [x1]
 ldr x3, [x9]
 add x3, x3, #1
 add x3, x3, #1
 str x3, [x9]
.if \chains == 3
 ldr x4, [x10]
 add x4, x4, #1
 add x4, x4, #1
 str x4, [x10]
.endif
.endr
endloop
 add x0, x2, x3
 add x0, x0, x4
 ret
.endm
farloop fwd_mid, 2
farloop fwd_far, 3

// The recurrence runs through the load address while the stored data is ready
// early. Eight NOPs separate each store from its reload; +8/-8 alternate.
begin mem64_addr_far
 mov x3, #8
 mov x4, #-8
 mov x7, #0
 mov x9, x1
.p2align 6
1:
.rept 32
 str x3, [x1]
.rept 8
 nop
.endr
 ldr x2, [x1]
 add x1, x1, x2
 str x4, [x1]
.rept 8
 nop
.endr
 ldr x2, [x1]
 add x1, x1, x2
.endr
 add x7, x7, #1
endloop
 sub x0, x1, x9
 add x0, x0, x7
 ret

// One static store/load pair per taken branch. 64 trips per outer trip.
begin tight_add_mem
 lsl x0, x0, #6
 mov x2, #1
 str x2, [x1]
.p2align 6
1:
 ldr x2, [x1]
 add x2, x2, #1
 str x2, [x1]
endloop
 mov x0, x2
 ret

.macro tightmax name, memory
begin \name
 lsl x0, x0, #6
 mov x7, #0
 mov w6, #1
 mov w3, #5
 str w3, [x1]
.p2align 6
1:
.if \memory == 1
 ldr w3, [x1]
 cmp w6, w3
 csel w2, w6, w3, ge
 str w2, [x1]
.elseif \memory == 2
 ldr w3, [x1]
 cmp w6, w3
 csel w3, w6, w3, ge
 str w3, [x1]
.else
 cmp w6, w3
 csel w3, w6, w3, ge
 nop
 nop
.endif
 add x7, x7, x6
endloop
.if \memory
 ldr w3, [x1]
.endif
 add x0, x7, x3
 ret
.endm
tightmax tight_max_mem, 1
tightmax tight_max_reg, 0

// Nussinov inner-loop replica: the eleven application instructions on
// L1-resident data, at the application's offset within a 64-byte line.
// variant: 0 application order  1 index increment between CMP and CSEL
//          2 register-carried maximum with an unrelated third load
.macro nussloop name, variant, reset
begin \name
 mov x13, x0
 add x6, x1, #1, lsl #12
 add x10, x1, #2, lsl #12
 mov x4, x1
 mov x8, #4
 mov w7, #1024
 mov w9, #\reset
 mov x11, #0
.p2align 6
1:
 str w9, [x4]
.if \variant == 2
 mov w3, w9
.else
 mov w12, w9
.endif
 mov x1, #0
 mov x2, x10
 nop
 nop
 nop
2:
 ldr w0, [x6, x1, lsl #2]
.if \variant == 2
 ldr w5, [x2]
 add w0, w0, w5
 ldr w5, [x4, #64]
 cmp w0, w3
 csel w3, w0, w3, ge
 str w3, [x4]
 add x1, x1, #1
.else
 ldr w3, [x2]
 add w0, w0, w3
 ldr w3, [x4]
 cmp w0, w3
.if \variant == 1
 add x1, x1, #1
 csel w0, w0, w3, ge
 str w0, [x4]
.else
 csel w0, w0, w3, ge
 str w0, [x4]
 add x1, x1, #1
.endif
.endif
 add x2, x2, x8
 cmp w7, w1
 b.gt 2b
 ldr w12, [x4]
 add x11, x11, x12
 subs x13, x13, #1
 b.ne 1b
 mov x0, x11
 ret
.endm
nussloop nuss_const, 0, 4096
nussloop nuss_chg, 0, 0
nussloop nuss_sep, 1, 4096
nussloop nuss_reg, 2, 4096

// Operand-relation controls, placed last so earlier probes keep their layout.
selectloop sel32_rr_other_adj, 6
selectloop sel32_ri_same_adj, 7
selectloop sel32_rr_swap_adj, 8

// The same running maximum with a destination that is neither compared
// register: updates alternate between w2 and w3.
begin sel32_rr_dst_adj
 mov x7, #0
 mov x2, #1
 mov x6, #1
.p2align 6
1:
.rept 32
 add x7, x7, x2
 add w6, w6, #1
 cmp w2, w6
 csel w3, w2, w6, ge
 add x7, x7, x3
 add w6, w6, #1
 cmp w3, w6
 csel w2, w3, w6, ge
.endr
endloop
 add x0, x7, x2
 ret

// The loaded register is also the selected and stored register.
tightmax tight_max_mem_same, 2
#ifdef __ELF__
.section .note.GNU-stack,"",%progbits
#endif
