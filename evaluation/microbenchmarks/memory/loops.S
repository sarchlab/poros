/* Identical function bodies in ELF and Mach-O. Darwin x18 is not used. */
#ifdef __APPLE__
#define NAME(x) _##x
#else
#define NAME(x) x
#endif
.text
.p2align 2
.global NAME(chase1)
NAME(chase1):
 stp x19, x20, [sp, #-16]!
 mov x20, x0
 mov x0, x3
 lsr x2, x2, #7
 mul x20, x20, x2
 add x20, x20, #17
 ldr x3, [x0, #0]
L_chase1:
 ldr x3, [x3]
 subs x20, x20, #1
 b.ne L_chase1
 mov x0, #0
 sub x3, x3, x1
 add x0, x0, x3
 ldp x19, x20, [sp], #16
 ret
.global NAME(chase1_end)
NAME(chase1_end):
.p2align 2
.global NAME(chase2)
NAME(chase2):
 stp x19, x20, [sp, #-16]!
 mov x20, x0
 mov x0, x3
 lsr x2, x2, #8
 mul x20, x20, x2
 add x20, x20, #17
 ldr x3, [x0, #0]
 ldr x4, [x0, #8]
L_chase2:
 ldr x3, [x3]
 ldr x4, [x4]
 subs x20, x20, #1
 b.ne L_chase2
 mov x0, #0
 sub x3, x3, x1
 add x0, x0, x3
 sub x4, x4, x1
 add x0, x0, x4
 ldp x19, x20, [sp], #16
 ret
.global NAME(chase2_end)
NAME(chase2_end):
.p2align 2
.global NAME(chase4)
NAME(chase4):
 stp x19, x20, [sp, #-16]!
 mov x20, x0
 mov x0, x3
 lsr x2, x2, #9
 mul x20, x20, x2
 add x20, x20, #17
 ldr x3, [x0, #0]
 ldr x4, [x0, #8]
 ldr x5, [x0, #16]
 ldr x6, [x0, #24]
L_chase4:
 ldr x3, [x3]
 ldr x4, [x4]
 ldr x5, [x5]
 ldr x6, [x6]
 subs x20, x20, #1
 b.ne L_chase4
 mov x0, #0
 sub x3, x3, x1
 add x0, x0, x3
 sub x4, x4, x1
 add x0, x0, x4
 sub x5, x5, x1
 add x0, x0, x5
 sub x6, x6, x1
 add x0, x0, x6
 ldp x19, x20, [sp], #16
 ret
.global NAME(chase4_end)
NAME(chase4_end):
.p2align 2
.global NAME(chase8)
NAME(chase8):
 stp x19, x20, [sp, #-16]!
 mov x20, x0
 mov x0, x3
 lsr x2, x2, #10
 mul x20, x20, x2
 add x20, x20, #17
 ldr x3, [x0, #0]
 ldr x4, [x0, #8]
 ldr x5, [x0, #16]
 ldr x6, [x0, #24]
 ldr x7, [x0, #32]
 ldr x8, [x0, #40]
 ldr x9, [x0, #48]
 ldr x10, [x0, #56]
L_chase8:
 ldr x3, [x3]
 ldr x4, [x4]
 ldr x5, [x5]
 ldr x6, [x6]
 ldr x7, [x7]
 ldr x8, [x8]
 ldr x9, [x9]
 ldr x10, [x10]
 subs x20, x20, #1
 b.ne L_chase8
 mov x0, #0
 sub x3, x3, x1
 add x0, x0, x3
 sub x4, x4, x1
 add x0, x0, x4
 sub x5, x5, x1
 add x0, x0, x5
 sub x6, x6, x1
 add x0, x0, x6
 sub x7, x7, x1
 add x0, x0, x7
 sub x8, x8, x1
 add x0, x0, x8
 sub x9, x9, x1
 add x0, x0, x9
 sub x10, x10, x1
 add x0, x0, x10
 ldp x19, x20, [sp], #16
 ret
.global NAME(chase8_end)
NAME(chase8_end):
.p2align 2
.global NAME(chase16)
NAME(chase16):
 stp x19, x20, [sp, #-16]!
 mov x20, x0
 mov x0, x3
 lsr x2, x2, #11
 mul x20, x20, x2
 add x20, x20, #17
 ldr x3, [x0, #0]
 ldr x4, [x0, #8]
 ldr x5, [x0, #16]
 ldr x6, [x0, #24]
 ldr x7, [x0, #32]
 ldr x8, [x0, #40]
 ldr x9, [x0, #48]
 ldr x10, [x0, #56]
 ldr x11, [x0, #64]
 ldr x12, [x0, #72]
 ldr x13, [x0, #80]
 ldr x14, [x0, #88]
 ldr x15, [x0, #96]
 ldr x16, [x0, #104]
 ldr x17, [x0, #112]
 ldr x19, [x0, #120]
L_chase16:
 ldr x3, [x3]
 ldr x4, [x4]
 ldr x5, [x5]
 ldr x6, [x6]
 ldr x7, [x7]
 ldr x8, [x8]
 ldr x9, [x9]
 ldr x10, [x10]
 ldr x11, [x11]
 ldr x12, [x12]
 ldr x13, [x13]
 ldr x14, [x14]
 ldr x15, [x15]
 ldr x16, [x16]
 ldr x17, [x17]
 ldr x19, [x19]
 subs x20, x20, #1
 b.ne L_chase16
 mov x0, #0
 sub x3, x3, x1
 add x0, x0, x3
 sub x4, x4, x1
 add x0, x0, x4
 sub x5, x5, x1
 add x0, x0, x5
 sub x6, x6, x1
 add x0, x0, x6
 sub x7, x7, x1
 add x0, x0, x7
 sub x8, x8, x1
 add x0, x0, x8
 sub x9, x9, x1
 add x0, x0, x9
 sub x10, x10, x1
 add x0, x0, x10
 sub x11, x11, x1
 add x0, x0, x11
 sub x12, x12, x1
 add x0, x0, x12
 sub x13, x13, x1
 add x0, x0, x13
 sub x14, x14, x1
 add x0, x0, x14
 sub x15, x15, x1
 add x0, x0, x15
 sub x16, x16, x1
 add x0, x0, x16
 sub x17, x17, x1
 add x0, x0, x17
 sub x19, x19, x1
 add x0, x0, x19
 ldp x19, x20, [sp], #16
 ret
.global NAME(chase16_end)
NAME(chase16_end):
.p2align 2
.global NAME(stream_read)
NAME(stream_read):
 stp x19, x20, [sp, #-32]!
 stp x21, x22, [sp, #16]
 mov x22, x0
 mov x3, #0
 mov x4, #0
 mov x5, #0
 mov x6, #0
 mov x7, #0
 mov x8, #0
 mov x9, #0
 mov x10, #0
L_read_pass:
 mov x20, x1
 lsr x21, x2, #6
L_read_chunk:
 ldr x11, [x20, #0]
 ldr x12, [x20, #8]
 ldr x13, [x20, #16]
 ldr x14, [x20, #24]
 ldr x15, [x20, #32]
 ldr x16, [x20, #40]
 ldr x17, [x20, #48]
 ldr x19, [x20, #56]
 add x3, x3, x11
 add x4, x4, x12
 add x5, x5, x13
 add x6, x6, x14
 add x7, x7, x15
 add x8, x8, x16
 add x9, x9, x17
 add x10, x10, x19
 add x20, x20, #64
 subs x21, x21, #1
 b.ne L_read_chunk
 subs x22, x22, #1
 b.ne L_read_pass
 mov x0, x3
 add x0, x0, x4
 add x0, x0, x5
 add x0, x0, x6
 add x0, x0, x7
 add x0, x0, x8
 add x0, x0, x9
 add x0, x0, x10
 ldp x21, x22, [sp, #16]
 ldp x19, x20, [sp], #32
 ret
.global NAME(stream_read_end)
NAME(stream_read_end):
.p2align 2
.global NAME(stream_write)
NAME(stream_write):
 mov x7, #0x5a5a
 movk x7, #0x5a5a, lsl #16
 movk x7, #0x5a5a, lsl #32
 movk x7, #0x5a5a, lsl #48
L_write_pass:
 mov x3, x1
 lsr x4, x2, #6
L_write_chunk:
 str x7, [x3, #0]
 str x7, [x3, #8]
 str x7, [x3, #16]
 str x7, [x3, #24]
 str x7, [x3, #32]
 str x7, [x3, #40]
 str x7, [x3, #48]
 str x7, [x3, #56]
 add x3, x3, #64
 subs x4, x4, #1
 b.ne L_write_chunk
 subs x0, x0, #1
 b.ne L_write_pass
 mov x0, x7
 ret
.global NAME(stream_write_end)
NAME(stream_write_end):
.p2align 2
.global NAME(stream_copy)
NAME(stream_copy):
 lsr x2, x2, #1
 mov x14, x0
L_copy_pass:
 mov x11, x1
 add x12, x1, x2
 lsr x13, x2, #6
L_copy_chunk:
 ldr x3, [x11, #0]
 ldr x4, [x11, #8]
 ldr x5, [x11, #16]
 ldr x6, [x11, #24]
 ldr x7, [x11, #32]
 ldr x8, [x11, #40]
 ldr x9, [x11, #48]
 ldr x10, [x11, #56]
 str x3, [x12, #0]
 str x4, [x12, #8]
 str x5, [x12, #16]
 str x6, [x12, #24]
 str x7, [x12, #32]
 str x8, [x12, #40]
 str x9, [x12, #48]
 str x10, [x12, #56]
 add x11, x11, #64
 add x12, x12, #64
 subs x13, x13, #1
 b.ne L_copy_chunk
 subs x14, x14, #1
 b.ne L_copy_pass
 lsr x0, x2, #3
 ret
.global NAME(stream_copy_end)
NAME(stream_copy_end):
#ifndef __APPLE__
.section .note.GNU-stack,"",@progbits
#endif
