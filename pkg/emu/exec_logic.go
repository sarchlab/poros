package emu

import (
	"math/bits"

	"golang.org/x/arch/arm64/arm64asm"
)

func (e *executor) execAND(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)
	result := op1 & op2
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execANDS(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)
	result := op1 & op2
	if is32 {
		result = mask32(result)
		e.regs.SetNZ32(uint32(result))
	} else {
		e.regs.SetNZ64(result)
	}
	e.regs.C = false
	e.regs.V = false
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execORR(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)
	result := op1 | op2
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execEOR(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)
	result := op1 ^ op2
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(dst, result)
	return nil
}

func (e *executor) execMOV(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	val := e.readOperandVal(inst.Args[1], is32)
	if is32 {
		val = mask32(val)
	}
	e.writeGPR(dst, val)
	return nil
}

func (e *executor) execMOVZ(inst arm64asm.Inst) error {
	dst := inst.Args[0]
	val := resolveImm(inst.Args[1])
	is32 := is32bit(dst)
	if is32 {
		val = mask32(val)
	}
	e.writeGPR(dst, val)
	return nil
}

func (e *executor) execMOVN(inst arm64asm.Inst) error {
	dst := inst.Args[0]
	val := resolveImm(inst.Args[1])
	is32 := is32bit(dst)
	if is32 {
		val = mask32(^val)
	} else {
		val = ^val
	}
	e.writeGPR(dst, val)
	return nil
}

func (e *executor) execMOVK(inst arm64asm.Inst) error {
	dst := inst.Args[0]
	is32 := is32bit(dst)
	oldVal := e.readOperandVal(dst, is32)

	immArg, ok := inst.Args[1].(arm64asm.ImmShift)
	if !ok {
		// Fallback: just use the resolved value
		val := resolveImm(inst.Args[1])
		e.writeGPR(dst, val)
		return nil
	}
	immVal, shift := getImmShift(immArg)
	// MOVK keeps bits outside the 16-bit window
	mask := uint64(0xFFFF) << shift
	val := (oldVal &^ mask) | (uint64(immVal) << shift)
	if is32 {
		val = mask32(val)
	}
	e.writeGPR(dst, val)
	return nil
}

func (e *executor) execCMP(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	op1 := e.readOperandVal(inst.Args[0], is32)
	op2 := e.readOperandVal(inst.Args[1], is32)

	if is32 {
		_, n, z, c, v := addWithCarry32(uint32(op1), ^uint32(op2), 1)
		e.regs.N, e.regs.Z, e.regs.C, e.regs.V = n, z, c, v
	} else {
		_, n, z, c, v := addWithCarry64(op1, ^op2, 1)
		e.regs.N, e.regs.Z, e.regs.C, e.regs.V = n, z, c, v
	}
	return nil
}

func (e *executor) execCMN(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	op1 := e.readOperandVal(inst.Args[0], is32)
	op2 := e.readOperandVal(inst.Args[1], is32)

	if is32 {
		_, n, z, c, v := addWithCarry32(uint32(op1), uint32(op2), 0)
		e.regs.N, e.regs.Z, e.regs.C, e.regs.V = n, z, c, v
	} else {
		_, n, z, c, v := addWithCarry64(op1, op2, 0)
		e.regs.N, e.regs.Z, e.regs.C, e.regs.V = n, z, c, v
	}
	return nil
}

func (e *executor) execCCMP(inst arm64asm.Inst) error {
	// CCMP Rn, #imm, #nzcv, cond
	is32 := is32bit(inst.Args[0])
	cond := inst.Args[3].(arm64asm.Cond)

	if e.regs.EvalCond(cond) {
		op1 := e.readOperandVal(inst.Args[0], is32)
		op2 := e.readOperandVal(inst.Args[1], is32)
		if is32 {
			_, n, z, c, v := addWithCarry32(uint32(op1), ^uint32(op2), 1)
			e.regs.N, e.regs.Z, e.regs.C, e.regs.V = n, z, c, v
		} else {
			_, n, z, c, v := addWithCarry64(op1, ^op2, 1)
			e.regs.N, e.regs.Z, e.regs.C, e.regs.V = n, z, c, v
		}
	} else {
		nzcv := resolveImm(inst.Args[2])
		e.regs.N = (nzcv>>3)&1 != 0
		e.regs.Z = (nzcv>>2)&1 != 0
		e.regs.C = (nzcv>>1)&1 != 0
		e.regs.V = nzcv&1 != 0
	}
	return nil
}

func (e *executor) execCSEL(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	cond := inst.Args[3].(arm64asm.Cond)

	var result uint64
	if e.regs.EvalCond(cond) {
		result = e.readOperandVal(inst.Args[1], is32)
	} else {
		result = e.readOperandVal(inst.Args[2], is32)
	}
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(inst.Args[0], result)
	return nil
}

func (e *executor) execCSINC(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	cond := inst.Args[3].(arm64asm.Cond)

	var result uint64
	if e.regs.EvalCond(cond) {
		result = e.readOperandVal(inst.Args[1], is32)
	} else {
		result = e.readOperandVal(inst.Args[2], is32) + 1
	}
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(inst.Args[0], result)
	return nil
}

func (e *executor) execCSET(inst arm64asm.Inst) error {
	// CSET Rd, cond — decoded form. Cond is in Args[1].
	cond := inst.Args[1].(arm64asm.Cond)
	var result uint64
	if e.regs.EvalCond(cond) {
		result = 1
	}
	e.writeGPR(inst.Args[0], result)
	return nil
}

func (e *executor) execCINC(inst arm64asm.Inst) error {
	// CINC Rd, Rn, cond
	is32 := is32bit(inst.Args[0])
	cond := inst.Args[2].(arm64asm.Cond)
	val := e.readOperandVal(inst.Args[1], is32)
	if e.regs.EvalCond(cond) {
		val++
	}
	if is32 {
		val = mask32(val)
	}
	e.writeGPR(inst.Args[0], val)
	return nil
}

func (e *executor) execMVN(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	val := e.readOperandVal(inst.Args[1], is32)
	result := ^val
	if is32 {
		result = mask32(result)
	}
	e.writeGPR(inst.Args[0], result)
	return nil
}

func (e *executor) execTST(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	op1 := e.readOperandVal(inst.Args[0], is32)
	op2 := e.readOperandVal(inst.Args[1], is32)
	result := op1 & op2
	if is32 {
		result = mask32(result)
		e.regs.SetNZ32(uint32(result))
	} else {
		e.regs.SetNZ64(result)
	}
	e.regs.C = false
	e.regs.V = false
	return nil
}

// Shift/extend instructions

func (e *executor) execLSL(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	val := e.readOperandVal(inst.Args[1], is32)
	shift := resolveImm(inst.Args[2])
	if is32 {
		result := uint64(uint32(val) << (shift & 31))
		e.writeGPR(dst, mask32(result))
	} else {
		e.writeGPR(dst, val<<(shift&63))
	}
	return nil
}

func (e *executor) execLSLV(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	val := e.readOperandVal(inst.Args[1], is32)
	shift := e.readOperandVal(inst.Args[2], is32)
	if is32 {
		result := uint64(uint32(val) << (uint32(shift) & 31))
		e.writeGPR(dst, mask32(result))
	} else {
		e.writeGPR(dst, val<<(shift&63))
	}
	return nil
}

func (e *executor) execLSR(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	val := e.readOperandVal(inst.Args[1], is32)
	shift := resolveImm(inst.Args[2])
	if is32 {
		result := uint32(val) >> (shift & 31)
		e.writeGPR(dst, uint64(result))
	} else {
		e.writeGPR(dst, val>>(shift&63))
	}
	return nil
}

func (e *executor) execLSRV(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	val := e.readOperandVal(inst.Args[1], is32)
	shift := e.readOperandVal(inst.Args[2], is32)
	if is32 {
		result := uint32(val) >> (uint32(shift) & 31)
		e.writeGPR(dst, uint64(result))
	} else {
		e.writeGPR(dst, val>>(shift&63))
	}
	return nil
}

func (e *executor) execASR(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	val := e.readOperandVal(inst.Args[1], is32)
	shift := resolveImm(inst.Args[2])
	if is32 {
		result := int32(val) >> (shift & 31)
		e.writeGPR(dst, uint64(uint32(result)))
	} else {
		result := int64(val) >> (shift & 63)
		e.writeGPR(dst, uint64(result))
	}
	return nil
}

func (e *executor) execASRV(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	val := e.readOperandVal(inst.Args[1], is32)
	shift := e.readOperandVal(inst.Args[2], is32)
	if is32 {
		result := int32(val) >> (uint32(shift) & 31)
		e.writeGPR(dst, uint64(uint32(result)))
	} else {
		result := int64(val) >> (shift & 63)
		e.writeGPR(dst, uint64(result))
	}
	return nil
}

func (e *executor) execROR(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	val := e.readOperandVal(inst.Args[1], is32)
	shift := resolveImm(inst.Args[2])
	if is32 {
		result := bits.RotateLeft32(uint32(val), -int(shift&31))
		e.writeGPR(dst, uint64(result))
	} else {
		result := bits.RotateLeft64(val, -int(shift&63))
		e.writeGPR(dst, result)
	}
	return nil
}

func (e *executor) execRORV(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := inst.Args[0]
	val := e.readOperandVal(inst.Args[1], is32)
	shift := e.readOperandVal(inst.Args[2], is32)
	if is32 {
		result := bits.RotateLeft32(uint32(val), -int(uint32(shift)&31))
		e.writeGPR(dst, uint64(result))
	} else {
		result := bits.RotateLeft64(val, -int(shift&63))
		e.writeGPR(dst, result)
	}
	return nil
}

func (e *executor) execSXTW(inst arm64asm.Inst) error {
	// SXTW Xd, Wn — sign-extend W to X
	val := e.readOperandVal(inst.Args[1], true)
	result := uint64(int64(int32(val)))
	e.writeGPR(inst.Args[0], result)
	return nil
}

func (e *executor) execSXTH(inst arm64asm.Inst) error {
	val := e.readOperandVal(inst.Args[1], true)
	is32 := is32bit(inst.Args[0])
	if is32 {
		result := uint64(uint32(int32(int16(val))))
		e.writeGPR(inst.Args[0], result)
	} else {
		result := uint64(int64(int16(val)))
		e.writeGPR(inst.Args[0], result)
	}
	return nil
}

func (e *executor) execSXTB(inst arm64asm.Inst) error {
	val := e.readOperandVal(inst.Args[1], true)
	is32 := is32bit(inst.Args[0])
	if is32 {
		result := uint64(uint32(int32(int8(val))))
		e.writeGPR(inst.Args[0], result)
	} else {
		result := uint64(int64(int8(val)))
		e.writeGPR(inst.Args[0], result)
	}
	return nil
}

func (e *executor) execUXTB(inst arm64asm.Inst) error {
	val := e.readOperandVal(inst.Args[1], true)
	e.writeGPR(inst.Args[0], val&0xFF)
	return nil
}

func (e *executor) execUXTH(inst arm64asm.Inst) error {
	val := e.readOperandVal(inst.Args[1], true)
	e.writeGPR(inst.Args[0], val&0xFFFF)
	return nil
}

func (e *executor) execUBFM(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	src := e.readOperandVal(inst.Args[1], is32)
	immr := resolveImm(inst.Args[2])
	imms := resolveImm(inst.Args[3])

	var regSize uint64 = 64
	if is32 {
		regSize = 32
	}

	// UBFM: extract bitfield
	r := immr % regSize
	s := imms % regSize

	var result uint64
	if s >= r {
		width := s - r + 1
		mask := (uint64(1) << width) - 1
		result = (src >> r) & mask
	} else {
		// Wrap case
		lowWidth := regSize - r
		lowMask := (uint64(1) << lowWidth) - 1
		low := (src >> r) & lowMask
		highWidth := s + 1
		highMask := (uint64(1) << highWidth) - 1
		high := src & highMask
		result = low | (high << lowWidth)
	}

	if is32 {
		result = mask32(result)
	}
	e.writeGPR(inst.Args[0], result)
	return nil
}

func (e *executor) execSBFM(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	src := e.readOperandVal(inst.Args[1], is32)
	immr := resolveImm(inst.Args[2])
	imms := resolveImm(inst.Args[3])

	var regSize uint64 = 64
	if is32 {
		regSize = 32
	}

	r := immr % regSize
	s := imms % regSize

	var result uint64
	if s >= r {
		width := s - r + 1
		mask := (uint64(1) << width) - 1
		result = (src >> r) & mask
		// Sign extend from bit (width-1)
		signBit := (result >> (width - 1)) & 1
		if signBit != 0 {
			if is32 {
				result |= (0xFFFFFFFF << width) & 0xFFFFFFFF
			} else {
				result |= ^((uint64(1) << width) - 1)
			}
		}
	} else {
		lowWidth := regSize - r
		lowMask := (uint64(1) << lowWidth) - 1
		low := (src >> r) & lowMask
		highWidth := s + 1
		highMask := (uint64(1) << highWidth) - 1
		high := src & highMask
		result = low | (high << lowWidth)
		totalWidth := lowWidth + highWidth
		signBit := (result >> (totalWidth - 1)) & 1
		if signBit != 0 {
			if is32 {
				result |= (0xFFFFFFFF << totalWidth) & 0xFFFFFFFF
			} else {
				result |= ^((uint64(1) << totalWidth) - 1)
			}
		}
	}

	if is32 {
		result = mask32(result)
	}
	e.writeGPR(inst.Args[0], result)
	return nil
}

func (e *executor) execUBFX(inst arm64asm.Inst) error {
	// UBFX is an alias of UBFM
	return e.execUBFM(inst)
}

func (e *executor) execSBFX(inst arm64asm.Inst) error {
	// SBFX is an alias of SBFM
	return e.execSBFM(inst)
}

func (e *executor) execBFM(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	dst := e.readOperandVal(inst.Args[0], is32)
	src := e.readOperandVal(inst.Args[1], is32)
	immr := resolveImm(inst.Args[2])
	imms := resolveImm(inst.Args[3])

	var regSize uint64 = 64
	if is32 {
		regSize = 32
	}

	r := immr % regSize
	s := imms % regSize

	var result uint64
	if s >= r {
		width := s - r + 1
		mask := (uint64(1) << width) - 1
		bits := (src >> r) & mask
		dstMask := mask
		result = (dst &^ dstMask) | bits
	} else {
		pos := regSize - r
		width := s + 1
		mask := (uint64(1) << width) - 1
		bits := src & mask
		dstMask := mask << pos
		result = (dst &^ dstMask) | (bits << pos)
	}

	if is32 {
		result = mask32(result)
	}
	e.writeGPR(inst.Args[0], result)
	return nil
}

func (e *executor) execEXTR(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	op1 := e.readOperandVal(inst.Args[1], is32)
	op2 := e.readOperandVal(inst.Args[2], is32)
	lsb := resolveImm(inst.Args[3])

	if is32 {
		concat := (uint64(uint32(op1)) << 32) | uint64(uint32(op2))
		result := uint32(concat >> (lsb & 31))
		e.writeGPR(inst.Args[0], uint64(result))
	} else {
		if lsb == 0 {
			e.writeGPR(inst.Args[0], op2)
		} else {
			result := (op2 >> lsb) | (op1 << (64 - lsb))
			e.writeGPR(inst.Args[0], result)
		}
	}
	return nil
}

func (e *executor) execCLZ(inst arm64asm.Inst) error {
	is32 := is32bit(inst.Args[0])
	val := e.readOperandVal(inst.Args[1], is32)
	var result uint64
	if is32 {
		result = uint64(bits.LeadingZeros32(uint32(val)))
	} else {
		result = uint64(bits.LeadingZeros64(val))
	}
	e.writeGPR(inst.Args[0], result)
	return nil
}

// Address computation

func (e *executor) execADRP(inst arm64asm.Inst) error {
	offset := int64(inst.Args[1].(arm64asm.PCRel))
	base := e.regs.PC &^ 0xFFF // Page-aligned PC
	result := uint64(int64(base) + offset)
	e.writeGPR(inst.Args[0], result)
	return nil
}

func (e *executor) execADR(inst arm64asm.Inst) error {
	offset := int64(inst.Args[1].(arm64asm.PCRel))
	result := uint64(int64(e.regs.PC) + offset)
	e.writeGPR(inst.Args[0], result)
	return nil
}
