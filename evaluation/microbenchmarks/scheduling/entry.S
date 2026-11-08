.global _start
.text
_start:
 ldr x0, [sp]
 add x1, sp, #8
 bl run
 mov x8, #93
 svc #0
.global emit
emit:
 sub sp, sp, #16
 str x0, [sp]
 mov x0, #1
 mov x1, sp
 mov x2, #8
 mov x8, #64
 svc #0
 add sp, sp, #16
 ret
.section .rodata
.byte 0
.section .data
.byte 0
.section .note.GNU-stack,"",@progbits
