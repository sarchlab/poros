// Minimal real Linux/AArch64 entry point. Runtime execution is added in #46.
.file "start.S"
.global _start
.type _start, %function
.section .text
_start:
    mov x9, sp
    tst x9, #15
    b.ne fail
    adrp x10, zero_data
    add x10, x10, :lo12:zero_data
    ldr x11, [x10]
    cbnz x11, fail
    mov x0, #0
    b exit
fail:
    mov x0, #1
exit:
    mov x8, #93
    svc #0
.size _start, .-_start
.section .rodata
.global message
message: .asciz "poros ELF fixture"
.section .data
.global initialized_data
initialized_data: .quad 0x0123456789abcdef
.section .bss
.balign 8
.global zero_data
zero_data: .skip 64
.section .note.GNU-stack,"",%progbits
