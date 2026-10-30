.file "start.S"
.global _start
.type _start,%function
.text
_start:
 mov x0, sp
 bl guest_main
 mov x8, #93
 svc #0
// Every fixture has three real permission-separated segments.
.section .rodata
.byte 0
.section .data
.byte 0
.section .note.GNU-stack,"",@progbits
