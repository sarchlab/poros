package emu

import (
	"math/bits"

	"golang.org/x/arch/arm64/arm64asm"
)

func (e *executor) execADD(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)
	result := op1 + op2
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execADDS(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)

	if is32 {
		result, n, z, c, v := addWithCarry32(uint32(op1), uint32(op2), 0)
		e.regs.N, e.regs.Z, e.regs.C, e.regs.V = n, z, c, v
		e.writeGPR(dst, uint64(result))
	} else {
		result, n, z, c, v := addWithCarry64(op1, op2, 0)
		e.regs.N, e.regs.Z, e.regs.C, e.regs.V = n, z, c, v
		e.writeGPR(dst, result)
	}
	return nil
}

func (e *executor) execSUB(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)
	result := op1 - op2
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execSUBS(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)

	// SUBS is AddWithCarry(op1, NOT(op2), 1)
	if is32 {
		result, n, z, c, v := addWithCarry32(uint32(op1), ^uint32(op2), 1)
		e.regs.N, e.regs.Z, e.regs.C, e.regs.V = n, z, c, v
		e.writeGPR(dst, uint64(result))
	} else {
		result, n, z, c, v := addWithCarry64(op1, ^op2, 1)
		e.regs.N, e.regs.Z, e.regs.C, e.regs.V = n, z, c, v
		e.writeGPR(dst, result)
	}
	return nil
}

func (e *executor) execMUL(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)
	result := op1 * op2
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execMADD(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)
	addend := e.readOperandVal(inst.Args[3], is32)
	result := addend + op1*op2
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execMSUB(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)
	addend := e.readOperandVal(inst.Args[3], is32)
	result := addend - op1*op2
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execUMULL(inst arm64asm.Inst) error {
	// UMULL Xd, Wn, Wm — 32x32→64 unsigned multiply
	dst := inst.Args[0]
	op1 := uint64(e.readOperandVal(inst.Args[1], true)) & 0xFFFFFFFF
	op2 := uint64(e.readOperandVal(inst.Args[2], true)) & 0xFFFFFFFF
	result := op1 * op2
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execSMULL(inst arm64asm.Inst) error {
	// SMULL Xd, Wn, Wm — 32x32→64 signed multiply
	dst := inst.Args[0]
	op1 := int64(int32(e.readOperandVal(inst.Args[1], true)))
	op2 := int64(int32(e.readOperandVal(inst.Args[2], true)))
	result := uint64(op1 * op2)
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execSMADDL(inst arm64asm.Inst) error {
	// SMADDL Xd, Wn, Wm, Xa — Xd = Xa + (sext(Wn) * sext(Wm))
	dst := inst.Args[0]
	op1 := int64(int32(e.readOperandVal(inst.Args[1], true)))
	op2 := int64(int32(e.readOperandVal(inst.Args[2], true)))
	addend := int64(e.readOperandVal(inst.Args[3], false))
	result := uint64(addend + op1*op2)
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execUMADDL(inst arm64asm.Inst) error {
	// UMADDL Xd, Wn, Wm, Xa — Xd = Xa + (zext(Wn) * zext(Wm))
	dst := inst.Args[0]
	op1 := uint64(uint32(e.readOperandVal(inst.Args[1], true)))
	op2 := uint64(uint32(e.readOperandVal(inst.Args[2], true)))
	addend := e.readOperandVal(inst.Args[3], false)
	result := addend + op1*op2
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execUMULH(inst arm64asm.Inst) error {
	// UMULH Xd, Xn, Xm — 64x64→128, return high 64 bits
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], false)
	op2 := e.readOperandVal(inst.Args[2], false)
	hi, _ := bits.Mul64(op1, op2)
	e.writeGPR(dst, hi)
	return nil
}

func (e *executor) execNEG(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op := e.readOperandVal(inst.Args[1], is32)
	result := -op
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execSDIV(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)

	if is32 {
		a := int32(op1)
		b := int32(op2)
		if b == 0 {
			e.writeGPR(dst, 0)
		} else if a == -2147483648 && b == -1 {
			e.writeGPR(dst, uint64(uint32(a)))
		} else {
			e.writeGPR(dst, uint64(uint32(a/b)))
		}
	} else {
		a := int64(op1)
		b := int64(op2)
		if b == 0 {
			e.writeGPR(dst, 0)
		} else if a == -9223372036854775808 && b == -1 {
			e.writeGPR(dst, uint64(a))
		} else {
			e.writeGPR(dst, uint64(a/b))
		}
	}
	return nil
}

func (e *executor) execUDIV(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)

	if is32 {
		a := uint32(op1)
		b := uint32(op2)
		if b == 0 {
			e.writeGPR(dst, 0)
		} else {
			e.writeGPR(dst, uint64(a/b))
		}
	} else {
		if op2 == 0 {
			e.writeGPR(dst, 0)
		} else {
			e.writeGPR(dst, op1/op2)
		}
	}
	return nil
}

// execADC consumes carry without changing NZCV. Unsigned addition deliberately
// wraps; writeGPR applies the architectural W-register zero extension.
func (e *executor) execADC(inst arm64asm.Inst) error {
	a, b := e.readGPR(inst.Args[1]), e.readGPR(inst.Args[2])
	carry := uint64(0)
	if e.regs.C {
		carry = 1
	}
	e.writeGPR(inst.Args[0], a+b+carry)
	return nil
}
