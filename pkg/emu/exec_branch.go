package emu

import (
	"golang.org/x/arch/arm64/arm64asm"
)

func (e *executor) execB(inst arm64asm.Inst) error {
	// B can be unconditional (PCRel) or conditional (B.cond — PCRel + Cond)
	// Find the PCRel and optional Cond arguments
	var offset int64
	var hasCond bool
	var cond arm64asm.Cond

	for _, arg := range inst.Args {
		if arg == nil {
			break
		}
		switch a := arg.(type) {
		case arm64asm.PCRel:
			offset = int64(a)
		case arm64asm.Cond:
			hasCond = true
			cond = a
		}
	}

	if hasCond {
		if !e.regs.EvalCond(cond) {
			// Condition not met, fall through (don't branch)
			e.regs.PC += 4
			return nil
		}
	}

	e.regs.PC = uint64(int64(e.regs.PC) + offset)
	return nil
}

func (e *executor) execBL(inst arm64asm.Inst) error {
	offset := int64(inst.Args[0].(arm64asm.PCRel))
	e.regs.WriteX(30, e.regs.PC+4) // LR = PC + 4
	e.regs.PC = uint64(int64(e.regs.PC) + offset)
	return nil
}

func (e *executor) execBR(inst arm64asm.Inst) error {
	target := e.readOperandVal(inst.Args[0], false)
	e.regs.PC = target
	return nil
}

func (e *executor) execBLR(inst arm64asm.Inst) error {
	target := e.readOperandVal(inst.Args[0], false)
	e.regs.WriteX(30, e.regs.PC+4) // LR = PC + 4
	e.regs.PC = target
	return nil
}

func (e *executor) execRET(inst arm64asm.Inst) error {
	// RET {Xn} — default is X30
	reg := inst.Args[0]
	if reg == nil {
		e.regs.PC = e.regs.ReadX(30)
	} else {
		e.regs.PC = e.readOperandVal(reg, false)
	}
	return nil
}

func (e *executor) execCBZ(inst arm64asm.Inst) (branched bool, err error) {
	is32 := is32bit(inst.Args[0])
	val := e.readOperandVal(inst.Args[0], is32)
	offset := int64(inst.Args[1].(arm64asm.PCRel))

	if is32 {
		val = mask32(val)
	}

	if val == 0 {
		e.regs.PC = uint64(int64(e.regs.PC) + offset)
		return true, nil
	}
	return false, nil
}

func (e *executor) execCBNZ(inst arm64asm.Inst) (branched bool, err error) {
	is32 := is32bit(inst.Args[0])
	val := e.readOperandVal(inst.Args[0], is32)
	offset := int64(inst.Args[1].(arm64asm.PCRel))

	if is32 {
		val = mask32(val)
	}

	if val != 0 {
		e.regs.PC = uint64(int64(e.regs.PC) + offset)
		return true, nil
	}
	return false, nil
}

func (e *executor) execTBZ(inst arm64asm.Inst) (branched bool, err error) {
	w := is32bit(inst.Args[0])
	val := e.readOperandVal(inst.Args[0], w)
	bitNum := resolveImm(inst.Args[1])
	offset := int64(inst.Args[2].(arm64asm.PCRel))

	if (val>>bitNum)&1 == 0 {
		e.regs.PC = uint64(int64(e.regs.PC) + offset)
		return true, nil
	}
	return false, nil
}

func (e *executor) execTBNZ(inst arm64asm.Inst) (branched bool, err error) {
	w := is32bit(inst.Args[0])
	val := e.readOperandVal(inst.Args[0], w)
	bitNum := resolveImm(inst.Args[1])
	offset := int64(inst.Args[2].(arm64asm.PCRel))

	if (val>>bitNum)&1 != 0 {
		e.regs.PC = uint64(int64(e.regs.PC) + offset)
		return true, nil
	}
	return false, nil
}
