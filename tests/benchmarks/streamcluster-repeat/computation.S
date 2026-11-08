	.arch armv8-a
	.file	"streamcluster.c"
	.text
	.align	2
	.type	argument, %function
argument:
.LFB1:
	.cfi_startproc
	ldrb	w4, [x0]
	cbz	w4, .L5
	mov	w5, 0
	mov	w7, 52429
	movk	w7, 0xcccc, lsl 16
.L3:
	ldrb	w4, [x0]
	cbz	w4, .L10
	add	x0, x0, 1
	sub	w4, w4, #48
	cmp	w4, 9
	bhi	.L6
	sub	w6, w2, w4
	umull	x6, w6, w7
	lsr	x6, x6, 35
	cmp	w6, w5
	bcc	.L7
	add	w5, w5, w5, lsl 2
	add	w5, w4, w5, lsl 1
	cmp	w2, w5
	bcs	.L3
	mov	w0, 0
	b	.L1
.L10:
	cmp	w1, w5
	bhi	.L9
	str	w5, [x3]
	mov	w0, 1
	b	.L1
.L5:
	mov	w0, 0
.L1:
	ret
.L6:
	mov	w0, 0
	b	.L1
.L7:
	mov	w0, 0
	b	.L1
.L9:
	mov	w0, 0
	b	.L1
	.cfi_endproc
.LFE1:
	.size	argument, .-argument
	.align	2
	.global	compute_distances
	.type	compute_distances, %function
compute_distances:
.LFB0:
	.cfi_startproc
	cmp	w3, 0
	ble	.L11
	mov	x6, x2
	add	x7, x2, w3, sxtw 3
	mov	w5, 0
	sxtw	x8, w4
	b	.L13
.L15:
	sbfiz	x2, x5, 3, 32
	add	x3, x8, w5, sxtw
	lsl	x3, x3, 3
	movi	d30, #0
.L14:
	ldr	d31, [x0, x2]
	ldr	d29, [x1, x2]
	fsub	d31, d31, d29
	fmul	d31, d31, d31
	fadd	d30, d30, d31
	add	x2, x2, 8
	cmp	x2, x3
	bne	.L14
.L16:
	str	d30, [x6], 8
	add	w5, w5, w4
	cmp	x6, x7
	beq	.L11
.L13:
	cmp	w4, 0
	bgt	.L15
	movi	d30, #0
	b	.L16
.L11:
	ret
	.cfi_endproc
.LFE0:
	.size	compute_distances, .-compute_distances
	.align	2
	.global	workload
	.type	workload, %function
workload:
.LFB4:
	.cfi_startproc
	cmp	w0, 3
	beq	.L31
	mov	w0, 2
	ret
.L31:
	stp	x29, x30, [sp, -64]!
	.cfi_def_cfa_offset 64
	.cfi_offset 29, -64
	.cfi_offset 30, -56
	mov	x29, sp
	stp	x19, x20, [sp, 16]
	.cfi_offset 19, -48
	.cfi_offset 20, -40
	mov	w19, w0
	mov	x20, x1
	add	x3, sp, 60
	mov	w2, 41248
	movk	w2, 0x7, lsl 16
	mov	w1, 1
	ldr	x0, [x20, 8]
	bl	argument
	cbz	w0, .L23
	add	x3, sp, 56
	mov	w2, 32
	mov	w1, 1
	ldr	x0, [x20, 16]
	bl	argument
	cbz	w0, .L24
	str	x21, [sp, 32]
	.cfi_offset 21, -32
	ldr	w20, [sp, 60]
	ldr	w4, [sp, 56]
	mul	w5, w4, w20
	cmp	w5, 0
	ble	.L20
	sxtw	x5, w5
	mov	x1, 0
	adrp	x0, .LC0
	ldr	d27, [x0, #:lo12:.LC0]
	fmov	d28, 1.0e+0
	adrp	x7, p1
	add	x7, x7, :lo12:p1
	adrp	x0, .LC1
	ldr	d29, [x0, #:lo12:.LC1]
	fmov	d30, 2.0e+0
	adrp	x6, p2
	add	x6, x6, :lo12:p2
.L21:
	scvtf	d31, w1
	fmadd	d31, d31, d27, d28
	lsl	x0, x1, 3
	str	d31, [x7, x0]
	neg	w3, w1
	scvtf	d31, w3
	fmadd	d31, d31, d29, d30
	str	d31, [x6, x0]
	add	x1, x1, 1
	cmp	x1, x5
	bne	.L21
.L20:
	adrp	x21, result
	add	x21, x21, :lo12:result
	mov	w3, w20
	mov	x2, x21
	adrp	x1, p2
	add	x1, x1, :lo12:p2
	adrp	x0, p1
	add	x0, x0, :lo12:p1
	bl	compute_distances
	sbfiz	x2, x20, 3, 32
	mov	x0, 1
	mov	x1, x21
	mov	x8, 64
#APP
// 40 "/tmp/poros-pr64-frozen/tests/benchmarks/runtime.h" 1
	svc #0
// 0 "" 2
#NO_APP
	cmp	x2, x0
	bne	.L25
	mov	w0, 0
	ldr	x21, [sp, 32]
	.cfi_restore 21
.L18:
	ldp	x19, x20, [sp, 16]
	ldp	x29, x30, [sp], 64
	.cfi_remember_state
	.cfi_restore 30
	.cfi_restore 29
	.cfi_restore 19
	.cfi_restore 20
	.cfi_def_cfa_offset 0
	ret
.L23:
	.cfi_restore_state
	mov	w0, 2
	b	.L18
.L24:
	mov	w0, 2
	b	.L18
.L25:
	.cfi_offset 21, -32
	mov	w0, w19
	ldr	x21, [sp, 32]
	.cfi_restore 21
	b	.L18
	.cfi_endproc
.LFE4:
	.size	workload, .-workload
	.align	2
	.global	guest_main
	.type	guest_main, %function
guest_main:
.LFB3:
	.cfi_startproc
	stp	x29, x30, [sp, -16]!
	.cfi_def_cfa_offset 16
	.cfi_offset 29, -16
	.cfi_offset 30, -8
	mov	x29, sp
	mov	x1, x0
	ldr	x0, [x1], 8
	bl	workload
	ldp	x29, x30, [sp], 16
	.cfi_restore 30
	.cfi_restore 29
	.cfi_def_cfa_offset 0
	ret
	.cfi_endproc
.LFE3:
	.size	guest_main, .-guest_main
	.section	.rodata.cst8,"aM",@progbits,8
	.align	3
.LC0:
	.word	1202590843
	.word	1065646817
	.align	3
.LC1:
	.word	1202590843
	.word	1066695393
	.bss
	.align	3
	.type	result, %object
	.size	result, 4000000
result:
	.zero	4000000
	.type	p2, %object
	.size	p2, 128000000
p2:
	.zero	128000000
	.type	p1, %object
	.size	p1, 128000000
p1:
	.zero	128000000
	.ident	"GCC: (GNU) 14.2.0"
	.section	.note.GNU-stack,"",@progbits
