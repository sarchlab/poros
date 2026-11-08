	.arch armv8-a
	.file	"repeat.c"
	.text
	.align	2
	.global	repeat_main
	.type	repeat_main, %function
repeat_main:
.LFB0:
	.cfi_startproc
	stp	x29, x30, [sp, -48]!
	.cfi_def_cfa_offset 48
	.cfi_offset 29, -48
	.cfi_offset 30, -40
	mov	x29, sp
	stp	x19, x20, [sp, 16]
	.cfi_offset 19, -32
	.cfi_offset 20, -24
	mov	x20, x0
	ldr	x0, [x0]
	cmp	x0, 4
	bne	.L6
	ldr	x3, [x20, 32]
	ldrb	w1, [x3]
	mov	w0, 91
	mov	x19, 0
	cbz	w1, .L1
.L3:
	ldrb	w1, [x3]
	cbz	w1, .L14
	sub	w2, w1, #48
	and	w2, w2, 255
	cmp	w2, 9
	bhi	.L8
	add	x19, x19, x19, lsl 2
	add	x3, x3, 1
	sub	w1, w1, #48
	sxtw	x1, w1
	add	x19, x1, x19, lsl 1
	cmp	x19, 64
	bls	.L3
	mov	w0, 92
	b	.L1
.L14:
	mov	w0, 93
	cbz	x19, .L1
	str	x21, [sp, 32]
	.cfi_offset 21, -16
	mov	x0, 3
	str	x0, [x20]
	mov	x21, 0
.L5:
	mov	x0, x20
	bl	guest_main
	cbnz	w0, .L13
	add	x21, x21, 1
	cmp	x19, x21
	bne	.L5
	ldr	x21, [sp, 32]
	.cfi_restore 21
	b	.L1
.L6:
	mov	w0, 90
.L1:
	ldp	x19, x20, [sp, 16]
	ldp	x29, x30, [sp], 48
	.cfi_remember_state
	.cfi_restore 30
	.cfi_restore 29
	.cfi_restore 19
	.cfi_restore 20
	.cfi_def_cfa_offset 0
	ret
.L8:
	.cfi_restore_state
	mov	w0, 91
	b	.L1
.L13:
	.cfi_offset 21, -16
	ldr	x21, [sp, 32]
	.cfi_restore 21
	b	.L1
	.cfi_endproc
.LFE0:
	.size	repeat_main, .-repeat_main
	.ident	"GCC: (GNU) 14.2.0"
	.section	.note.GNU-stack,"",@progbits
