// Arguments supplied by the report harness: {nop,add} {1,2}.
// Each unit of length is 4096 iterations of the native benchmark's loop.
.global _start
.text
_start:
 ldr x9, [sp, #24]
 ldrb w0, [x9]
 sub w0, w0, #48
 lsl x0, x0, #12
 ldr x9, [sp, #16]
 ldrb w9, [x9]
 cmp w9, #110
 b.ne 1f
 bl _nop_probe
 b 2f
1:
 bl _add_probe
2:
 mov x0, #0
 mov x8, #93
 svc #0
.section .rodata
.byte 0
.section .data
.byte 0
.section .note.GNU-stack,"",@progbits
