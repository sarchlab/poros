package emu

import (
	"math/bits"

	"golang.org/x/arch/arm64/arm64asm"
)

// executor holds the emulator state needed for instruction execution.
type executor struct {
	regs    *Registers
	mem     *Memory
	checked CheckedMemory
	scratch [32]byte
}

// execute dispatches a decoded instruction to the appropriate handler.
// Returns true if the instruction updated PC (branch), false otherwise.
func (e *executor) execute(inst arm64asm.Inst) (branched bool, err error) {
	switch inst.Op {
	// Integer arithmetic
	case arm64asm.ADD:
		return false, e.execADD(inst)
	case arm64asm.ADDS:
		return false, e.execADDS(inst)
	case arm64asm.SUB:
		return false, e.execSUB(inst)
	case arm64asm.SUBS:
		return false, e.execSUBS(inst)
	case arm64asm.MUL:
		return false, e.execMUL(inst)
	case arm64asm.MADD:
		return false, e.execMADD(inst)
	case arm64asm.MSUB:
		return false, e.execMSUB(inst)
	case arm64asm.UMULL:
		return false, e.execUMULL(inst)
	case arm64asm.SMULL:
		return false, e.execSMULL(inst)
	case arm64asm.SMADDL:
		return false, e.execSMADDL(inst)
	case arm64asm.UMADDL:
		return false, e.execUMADDL(inst)
	case arm64asm.UMULH:
		return false, e.execUMULH(inst)
	case arm64asm.NEG:
		return false, e.execNEG(inst)
	case arm64asm.NEGS:
		return false, e.execNEGS(inst)
	case arm64asm.SDIV:
		return false, e.execSDIV(inst)
	case arm64asm.UDIV:
		return false, e.execUDIV(inst)

	// Logic / move
	case arm64asm.AND:
		return false, e.execAND(inst)
	case arm64asm.ANDS:
		return false, e.execANDS(inst)
	case arm64asm.ORR:
		return false, e.execORR(inst)
	case arm64asm.EOR:
		return false, e.execEOR(inst)
	case arm64asm.MOV:
		return false, e.execMOV(inst)
	case arm64asm.MOVZ:
		return false, e.execMOVZ(inst)
	case arm64asm.MOVN:
		return false, e.execMOVN(inst)
	case arm64asm.MOVK:
		return false, e.execMOVK(inst)
	case arm64asm.CMP:
		return false, e.execCMP(inst)
	case arm64asm.CMN:
		return false, e.execCMN(inst)
	case arm64asm.CCMP:
		return false, e.execCCMP(inst)
	case arm64asm.CCMN:
		return false, e.execCCMN(inst)
	case arm64asm.CSEL:
		return false, e.execCSEL(inst)
	case arm64asm.CSINC:
		return false, e.execCSINC(inst)
	case arm64asm.CSNEG:
		return false, e.execCSNEG(inst)
	case arm64asm.CNEG:
		return false, e.execCNEG(inst)
	case arm64asm.CSET:
		return false, e.execCSET(inst)
	case arm64asm.CINC:
		return false, e.execCINC(inst)
	case arm64asm.MVN:
		return false, e.execMVN(inst)
	case arm64asm.TST:
		return false, e.execTST(inst)

	// Shift / extend / bitfield
	case arm64asm.LSL:
		return false, e.execLSL(inst)
	case arm64asm.LSLV:
		return false, e.execLSLV(inst)
	case arm64asm.LSR:
		return false, e.execLSR(inst)
	case arm64asm.LSRV:
		return false, e.execLSRV(inst)
	case arm64asm.ASR:
		return false, e.execASR(inst)
	case arm64asm.ASRV:
		return false, e.execASRV(inst)
	case arm64asm.ROR:
		return false, e.execROR(inst)
	case arm64asm.RORV:
		return false, e.execRORV(inst)
	case arm64asm.SXTW:
		return false, e.execSXTW(inst)
	case arm64asm.SXTH:
		return false, e.execSXTH(inst)
	case arm64asm.SXTB:
		return false, e.execSXTB(inst)
	case arm64asm.UXTB:
		return false, e.execUXTB(inst)
	case arm64asm.UXTH:
		return false, e.execUXTH(inst)
	case arm64asm.UBFM:
		return false, e.execUBFM(inst)
	case arm64asm.UBFIZ:
		return false, e.execUBFIZ(inst)
	case arm64asm.SBFM:
		return false, e.execSBFM(inst)
	case arm64asm.SBFIZ:
		return false, e.execSBFIZ(inst)
	case arm64asm.UBFX:
		return false, e.execUBFX(inst)
	case arm64asm.SBFX:
		return false, e.execSBFX(inst)
	case arm64asm.BFM:
		return false, e.execBFM(inst)
	case arm64asm.BFI:
		return false, e.execBFI(inst)
	case arm64asm.EXTR:
		return false, e.execEXTR(inst)
	case arm64asm.CLZ:
		return false, e.execCLZ(inst)

	// Address
	case arm64asm.ADRP:
		return false, e.execADRP(inst)
	case arm64asm.ADR:
		return false, e.execADR(inst)

	// Load / Store
	case arm64asm.LDR:
		return false, e.execLDR(inst)
	case arm64asm.LDRB:
		return false, e.execLDRB(inst)
	case arm64asm.LDRH:
		return false, e.execLDRH(inst)
	case arm64asm.LDRSB:
		return false, e.execLDRSB(inst)
	case arm64asm.LDRSH:
		return false, e.execLDRSH(inst)
	case arm64asm.LDRSW:
		return false, e.execLDRSW(inst)
	case arm64asm.LDUR:
		return false, e.execLDUR(inst)
	case arm64asm.LDURB:
		return false, e.execLDURB(inst)
	case arm64asm.LDURH:
		return false, e.execLDURH(inst)
	case arm64asm.LDURSB:
		return false, e.execLDURSB(inst)
	case arm64asm.LDURSH:
		return false, e.execLDURSH(inst)
	case arm64asm.LDURSW:
		return false, e.execLDURSW(inst)
	case arm64asm.LDP:
		return false, e.execLDP(inst)
	case arm64asm.LDPSW:
		return false, e.execLDPSW(inst)
	case arm64asm.STR:
		return false, e.execSTR(inst)
	case arm64asm.STRB:
		return false, e.execSTRB(inst)
	case arm64asm.STRH:
		return false, e.execSTRH(inst)
	case arm64asm.STUR:
		return false, e.execSTUR(inst)
	case arm64asm.STURB:
		return false, e.execSTURB(inst)
	case arm64asm.STURH:
		return false, e.execSTURH(inst)
	case arm64asm.STP:
		return false, e.execSTP(inst)

	// Branch
	case arm64asm.B:
		return true, e.execB(inst)
	case arm64asm.BL:
		return true, e.execBL(inst)
	case arm64asm.BR:
		return true, e.execBR(inst)
	case arm64asm.BLR:
		return true, e.execBLR(inst)
	case arm64asm.RET:
		return true, e.execRET(inst)
	case arm64asm.CBZ:
		return e.execCBZ(inst)
	case arm64asm.CBNZ:
		return e.execCBNZ(inst)
	case arm64asm.TBZ:
		return e.execTBZ(inst)
	case arm64asm.TBNZ:
		return e.execTBNZ(inst)

	// FP arithmetic
	case arm64asm.FADD:
		return false, e.execFADD(inst)
	case arm64asm.FSUB:
		return false, e.execFSUB(inst)
	case arm64asm.FMUL:
		return false, e.execFMUL(inst)
	case arm64asm.FDIV:
		return false, e.execFDIV(inst)
	case arm64asm.FMADD:
		return false, e.execFMADD(inst)
	case arm64asm.FMSUB:
		return false, e.execFMSUB(inst)
	case arm64asm.FSQRT:
		return false, e.execFSQRT(inst)
	case arm64asm.FNEG:
		return false, e.execFNEG(inst)
	case arm64asm.FABS:
		return false, e.execFABS(inst)
	case arm64asm.FNMUL:
		return false, e.execFNMUL(inst)
	case arm64asm.FNMADD:
		return false, e.execFNMADD(inst)
	case arm64asm.FNMSUB:
		return false, e.execFNMSUB(inst)

	// SIMD scalar move / widen
	case arm64asm.MOVI:
		return false, e.execMOVI(inst)
	case arm64asm.SXTL:
		return false, e.execSXTL(inst)

	// FP move / convert / compare
	case arm64asm.FMOV:
		return false, e.execFMOV(inst)
	case arm64asm.FCMP, arm64asm.FCMPE:
		return false, e.execFCMP(inst)
	case arm64asm.FCCMP:
		return false, e.execFCCMP(inst)
	case arm64asm.FMAXNM:
		return false, e.execFMAXNM(inst)
	case arm64asm.FMINNM:
		return false, e.execFMINNM(inst)
	case arm64asm.FCSEL:
		return false, e.execFCSEL(inst)
	case arm64asm.SCVTF:
		return false, e.execSCVTF(inst)
	case arm64asm.UCVTF:
		return false, e.execUCVTF(inst)
	case arm64asm.FCVT:
		return false, e.execFCVT(inst)
	case arm64asm.FCVTZS:
		return false, e.execFCVTZS(inst)
	case arm64asm.FCVTZU:
		return false, e.execFCVTZU(inst)

	// NOP
	case arm64asm.NOP:
		return false, nil

	// Hints we can safely ignore
	case arm64asm.HINT:
		return false, nil

	default:
		return false, &UnsupportedInstructionError{Instruction: inst}
	}
}

// readGPR reads a 64-bit GPR value, handling SP and XZR contexts.
func (e *executor) readGPR(arg arm64asm.Arg) uint64 {
	switch r := arg.(type) {
	case arm64asm.Reg:
		idx := xRegIndex(r)
		if idx >= 0 {
			return e.regs.ReadX(idx)
		}
		widx := wRegIndex(r)
		if widx >= 0 {
			return uint64(e.regs.ReadW(widx))
		}
	case arm64asm.RegSP:
		reg := arm64asm.Reg(r)
		if reg == arm64asm.XZR { // SP context
			return e.regs.SP
		}
		if reg == arm64asm.WZR { // WSP context
			return uint64(uint32(e.regs.SP))
		}
		idx := xRegIndex(reg)
		if idx >= 0 {
			return e.regs.ReadX(idx)
		}
		widx := wRegIndex(reg)
		if widx >= 0 {
			return uint64(e.regs.ReadW(widx))
		}
	}
	return 0
}

// writeGPR writes a 64-bit value to a GPR, handling SP and XZR contexts.
func (e *executor) writeGPR(arg arm64asm.Arg, val uint64) {
	switch r := arg.(type) {
	case arm64asm.Reg:
		idx := xRegIndex(r)
		if idx >= 0 {
			e.regs.WriteX(idx, val)
			return
		}
		widx := wRegIndex(r)
		if widx >= 0 {
			e.regs.WriteW(widx, uint32(val))
			return
		}
	case arm64asm.RegSP:
		reg := arm64asm.Reg(r)
		if reg == arm64asm.XZR { // SP context
			e.regs.SP = val
			return
		}
		if reg == arm64asm.WZR { // WSP context
			e.regs.SP = uint64(uint32(val))
			return
		}
		idx := xRegIndex(reg)
		if idx >= 0 {
			e.regs.WriteX(idx, val)
			return
		}
		widx := wRegIndex(reg)
		if widx >= 0 {
			e.regs.WriteW(widx, uint32(val))
			return
		}
	}
}

// is32bit returns true if the arg is a W register.
func is32bit(arg arm64asm.Arg) bool {
	switch r := arg.(type) {
	case arm64asm.Reg:
		return r >= arm64asm.W0 && r <= arm64asm.WZR
	case arm64asm.RegSP:
		reg := arm64asm.Reg(r)
		return reg >= arm64asm.W0 && reg <= arm64asm.WZR
	}
	return false
}

// resolveImm resolves an immediate value from various arm64asm immediate types.
func resolveImm(arg arm64asm.Arg) uint64 {
	switch imm := arg.(type) {
	case arm64asm.Imm:
		return uint64(imm.Imm)
	case arm64asm.Imm64:
		return imm.Imm
	case arm64asm.ImmShift:
		immVal, shift := getImmShift(imm)
		if shift >= 128 {
			// MSL shift (ones shifted in)
			realShift := shift - 128
			mask := uint64((1 << realShift) - 1)
			return (uint64(immVal) << realShift) | mask
		}
		return uint64(immVal) << shift
	}
	return 0
}

// readOperandVal reads the value of a source operand (register or immediate).
func (e *executor) readOperandVal(arg arm64asm.Arg, is32 bool) uint64 {
	switch a := arg.(type) {
	case arm64asm.Reg:
		if is32 {
			widx := wRegIndex(a)
			if widx >= 0 {
				return uint64(e.regs.ReadW(widx))
			}
		}
		idx := xRegIndex(a)
		if idx >= 0 {
			return e.regs.ReadX(idx)
		}
	case arm64asm.RegSP:
		return e.readGPR(a)
	case arm64asm.Imm, arm64asm.Imm64, arm64asm.ImmShift:
		return resolveImm(a)
	case arm64asm.RegExtshiftAmount:
		reg, extShift, amount := getRegExtshiftAmount(a)
		val := e.readRegRaw(reg)
		return applyExtShift(val, extShift, amount, is32)
	}
	return 0
}

// readRegRaw reads a register value without context.
func (e *executor) readRegRaw(reg arm64asm.Reg) uint64 {
	idx := xRegIndex(reg)
	if idx >= 0 {
		return e.regs.ReadX(idx)
	}
	widx := wRegIndex(reg)
	if widx >= 0 {
		return uint64(e.regs.ReadW(widx))
	}
	return 0
}

// applyExtShift applies an extend/shift operation to a value.
func applyExtShift(val uint64, extShift uint8, amount uint8, is32 bool) uint64 {
	// ExtShift constants from arm64asm (unexported):
	// uxtb=1, uxth=2, uxtw=3, uxtx=4, sxtb=5, sxth=6, sxtw=7, sxtx=8, lsl=9, lsr=10, asr=11, ror=12
	switch extShift {
	case 1: // uxtb
		val = val & 0xFF
	case 2: // uxth
		val = val & 0xFFFF
	case 3: // uxtw
		val = val & 0xFFFFFFFF
	case 4: // uxtx
		// no extension needed
	case 5: // sxtb
		val = uint64(int64(int8(val)))
	case 6: // sxth
		val = uint64(int64(int16(val)))
	case 7: // sxtw
		val = uint64(int64(int32(val)))
	case 8: // sxtx
		// no extension needed
	case 9: // lsl
		// just shift
	case 10: // lsr
		if is32 {
			return (val & 0xFFFFFFFF) >> amount
		}
		return val >> amount
	case 11: // asr
		if is32 {
			return uint64(uint32(int32(val) >> amount))
		}
		return uint64(int64(val) >> amount)
	case 12: // ror
		if is32 {
			return uint64(bits.RotateLeft32(uint32(val), -int(amount)))
		}
		return bits.RotateLeft64(val, -int(amount))
	}
	// Apply left shift for uxt*/sxt*/lsl
	if extShift != 10 && extShift != 11 && extShift != 12 {
		val = val << amount
	}
	return val
}

// resolveMemAddr resolves a memory address from a MemImmediate or MemExtend operand.
// Returns the effective address and performs any pre/post-index writeback.
func (e *executor) resolveMemAddr(arg arm64asm.Arg, prePhase bool) uint64 {
	switch m := arg.(type) {
	case arm64asm.MemImmediate:
		base, mode, imm := getMemImmediate(m)
		baseVal := e.readGPR(base)

		switch mode {
		case arm64asm.AddrOffset:
			return baseVal + uint64(int64(imm))
		case arm64asm.AddrPreIndex:
			addr := baseVal + uint64(int64(imm))
			if prePhase {
				e.writeGPR(base, addr)
			}
			return addr
		case arm64asm.AddrPostIndex:
			addr := baseVal
			if prePhase {
				e.writeGPR(base, baseVal+uint64(int64(imm)))
			}
			return addr
		}
		return baseVal

	case arm64asm.MemExtend:
		baseVal := e.readGPR(m.Base)
		idxVal := e.readRegRaw(m.Index)
		shifted := applyExtShift(idxVal, uint8(m.Extend), m.Amount, false)
		return baseVal + shifted
	}
	return 0
}

// mask32 returns 0xFFFFFFFF for 32-bit ops.
func mask32(v uint64) uint64 {
	return v & 0xFFFFFFFF
}

// addWithCarry64 performs 64-bit addition with carry, returning result and flags.
func addWithCarry64(x, y uint64, carryIn uint64) (result uint64, n, z, c, v bool) {
	result = x + y + carryIn
	n = (result >> 63) != 0
	z = result == 0
	// Carry: unsigned overflow
	if carryIn == 0 {
		c = result < x
	} else {
		c = result <= x
	}
	// Overflow: sign bit wrong
	signX := (x >> 63) != 0
	signY := (y >> 63) != 0
	signR := (result >> 63) != 0
	v = (signX == signY) && (signX != signR)
	return
}

// addWithCarry32 performs 32-bit addition with carry.
func addWithCarry32(x, y uint32, carryIn uint32) (result uint32, n, z, c, v bool) {
	sum := uint64(x) + uint64(y) + uint64(carryIn)
	result = uint32(sum)
	n = (result >> 31) != 0
	z = result == 0
	c = sum > 0xFFFFFFFF
	signX := (x >> 31) != 0
	signY := (y >> 31) != 0
	signR := (result >> 31) != 0
	v = (signX == signY) && (signX != signR)
	return
}
