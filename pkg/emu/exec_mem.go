package emu

import (
	"fmt"

	"golang.org/x/arch/arm64/arm64asm"
)

// Load instructions

func (e *executor) execLDR(inst arm64asm.Inst) error {
	dst := inst.Args[0]

	// Check if destination is FP register
	if r, ok := dst.(arm64asm.Reg); ok {
		if di := dRegIndex(r); di >= 0 {
			addr := e.resolveMemAddr(inst.Args[1], true)
			e.regs.WriteDRaw(di, e.mem.ReadU64(addr))
			return nil
		}
		if si := sRegIndex(r); si >= 0 {
			addr := e.resolveMemAddr(inst.Args[1], true)
			e.regs.WriteSRaw(si, e.mem.ReadU32(addr))
			return nil
		}
	}

	is32 := is32bit(dst)
	addr := e.resolveMemAddr(inst.Args[1], true)

	if is32 {
		val := e.mem.ReadU32(addr)
		e.writeGPR(dst, uint64(val))
	} else {
		val := e.mem.ReadU64(addr)
		e.writeGPR(dst, val)
	}
	return nil
}

func (e *executor) execLDRB(inst arm64asm.Inst) error {
	dst := inst.Args[0]
	addr := e.resolveMemAddr(inst.Args[1], true)
	val := e.mem.ReadU8(addr)
	e.writeGPR(dst, uint64(val))
	return nil
}

func (e *executor) execLDRH(inst arm64asm.Inst) error {
	dst := inst.Args[0]
	addr := e.resolveMemAddr(inst.Args[1], true)
	val := e.mem.ReadU16(addr)
	e.writeGPR(dst, uint64(val))
	return nil
}

func (e *executor) execLDRSB(inst arm64asm.Inst) error {
	dst := inst.Args[0]
	addr := e.resolveMemAddr(inst.Args[1], true)
	val := e.mem.ReadS8(addr)
	is32 := is32bit(dst)
	if is32 {
		e.writeGPR(dst, uint64(uint32(int32(val))))
	} else {
		e.writeGPR(dst, uint64(int64(val)))
	}
	return nil
}

func (e *executor) execLDRSH(inst arm64asm.Inst) error {
	dst := inst.Args[0]
	addr := e.resolveMemAddr(inst.Args[1], true)
	val := int16(e.mem.ReadU16(addr))
	is32 := is32bit(dst)
	if is32 {
		e.writeGPR(dst, uint64(uint32(int32(val))))
	} else {
		e.writeGPR(dst, uint64(int64(val)))
	}
	return nil
}

func (e *executor) execLDRSW(inst arm64asm.Inst) error {
	dst := inst.Args[0]
	addr := e.resolveMemAddr(inst.Args[1], true)
	val := e.mem.ReadS32(addr)
	e.writeGPR(dst, uint64(int64(val)))
	return nil
}

func (e *executor) execLDUR(inst arm64asm.Inst) error {
	return e.execLDR(inst) // Same logic, unscaled offset handled by decoder
}

func (e *executor) execLDURB(inst arm64asm.Inst) error {
	return e.execLDRB(inst)
}

func (e *executor) execLDURH(inst arm64asm.Inst) error {
	return e.execLDRH(inst)
}

func (e *executor) execLDURSB(inst arm64asm.Inst) error {
	return e.execLDRSB(inst)
}

func (e *executor) execLDURSH(inst arm64asm.Inst) error {
	return e.execLDRSH(inst)
}

func (e *executor) execLDURSW(inst arm64asm.Inst) error {
	return e.execLDRSW(inst)
}

func (e *executor) execLDP(inst arm64asm.Inst) error {
	rt1 := inst.Args[0]
	rt2 := inst.Args[1]
	is32 := is32bit(rt1)

	// Check for FP/SIMD register pair
	if r, ok := rt1.(arm64asm.Reg); ok {
		// Q register pair (128-bit each)
		if qi := qRegIndex(r); qi >= 0 {
			addr := e.resolveMemAddr(inst.Args[2], true)
			r2 := inst.Args[1].(arm64asm.Reg)
			qi2 := qRegIndex(r2)
			lo1 := e.mem.ReadU64(addr)
			hi1 := e.mem.ReadU64(addr + 8)
			lo2 := e.mem.ReadU64(addr + 16)
			hi2 := e.mem.ReadU64(addr + 24)
			e.regs.WriteQ(qi, lo1, hi1)
			e.regs.WriteQ(qi2, lo2, hi2)
			return nil
		}
		if di := dRegIndex(r); di >= 0 {
			addr := e.resolveMemAddr(inst.Args[2], true)
			r2 := inst.Args[1].(arm64asm.Reg)
			di2 := dRegIndex(r2)
			e.regs.WriteDRaw(di, e.mem.ReadU64(addr))
			e.regs.WriteDRaw(di2, e.mem.ReadU64(addr+8))
			return nil
		}
		if si := sRegIndex(r); si >= 0 {
			addr := e.resolveMemAddr(inst.Args[2], true)
			r2 := inst.Args[1].(arm64asm.Reg)
			si2 := sRegIndex(r2)
			e.regs.WriteSRaw(si, e.mem.ReadU32(addr))
			e.regs.WriteSRaw(si2, e.mem.ReadU32(addr+4))
			return nil
		}
	}

	addr := e.resolveMemAddr(inst.Args[2], true)
	if is32 {
		v1 := uint64(e.mem.ReadU32(addr))
		v2 := uint64(e.mem.ReadU32(addr + 4))
		e.writeGPR(rt1, v1)
		e.writeGPR(rt2, v2)
	} else {
		v1 := e.mem.ReadU64(addr)
		v2 := e.mem.ReadU64(addr + 8)
		e.writeGPR(rt1, v1)
		e.writeGPR(rt2, v2)
	}
	return nil
}

func (e *executor) execLDPSW(inst arm64asm.Inst) error {
	rt1 := inst.Args[0]
	rt2 := inst.Args[1]
	addr := e.resolveMemAddr(inst.Args[2], true)
	v1 := int64(e.mem.ReadS32(addr))
	v2 := int64(e.mem.ReadS32(addr + 4))
	e.writeGPR(rt1, uint64(v1))
	e.writeGPR(rt2, uint64(v2))
	return nil
}

// Store instructions

func (e *executor) execSTR(inst arm64asm.Inst) error {
	src := inst.Args[0]

	// Check if source is FP register
	if r, ok := src.(arm64asm.Reg); ok {
		if di := dRegIndex(r); di >= 0 {
			addr := e.resolveMemAddr(inst.Args[1], true)
			e.mem.WriteU64(addr, e.regs.ReadDRaw(di))
			return nil
		}
		if si := sRegIndex(r); si >= 0 {
			addr := e.resolveMemAddr(inst.Args[1], true)
			e.mem.WriteU32(addr, e.regs.ReadSRaw(si))
			return nil
		}
	}

	is32 := is32bit(src)
	addr := e.resolveMemAddr(inst.Args[1], true)

	if is32 {
		val := uint32(e.readOperandVal(src, true))
		e.mem.WriteU32(addr, val)
	} else {
		val := e.readOperandVal(src, false)
		e.mem.WriteU64(addr, val)
	}
	return nil
}

func (e *executor) execSTRB(inst arm64asm.Inst) error {
	src := inst.Args[0]
	addr := e.resolveMemAddr(inst.Args[1], true)
	val := uint8(e.readOperandVal(src, true))
	e.mem.WriteU8(addr, val)
	return nil
}

func (e *executor) execSTRH(inst arm64asm.Inst) error {
	src := inst.Args[0]
	addr := e.resolveMemAddr(inst.Args[1], true)
	val := uint16(e.readOperandVal(src, true))
	e.mem.WriteU16(addr, val)
	return nil
}

func (e *executor) execSTUR(inst arm64asm.Inst) error {
	return e.execSTR(inst)
}

func (e *executor) execSTURB(inst arm64asm.Inst) error {
	return e.execSTRB(inst)
}

func (e *executor) execSTURH(inst arm64asm.Inst) error {
	return e.execSTRH(inst)
}

func (e *executor) execSTP(inst arm64asm.Inst) error {
	rt1 := inst.Args[0]
	rt2 := inst.Args[1]
	is32 := is32bit(rt1)

	// Check for FP/SIMD register pair
	if r, ok := rt1.(arm64asm.Reg); ok {
		// Q register pair (128-bit each)
		if qi := qRegIndex(r); qi >= 0 {
			addr := e.resolveMemAddr(inst.Args[2], true)
			r2 := inst.Args[1].(arm64asm.Reg)
			qi2 := qRegIndex(r2)
			lo1, hi1 := e.regs.ReadQ(qi)
			lo2, hi2 := e.regs.ReadQ(qi2)
			e.mem.WriteU64(addr, lo1)
			e.mem.WriteU64(addr+8, hi1)
			e.mem.WriteU64(addr+16, lo2)
			e.mem.WriteU64(addr+24, hi2)
			return nil
		}
		if di := dRegIndex(r); di >= 0 {
			addr := e.resolveMemAddr(inst.Args[2], true)
			r2 := inst.Args[1].(arm64asm.Reg)
			di2 := dRegIndex(r2)
			e.mem.WriteU64(addr, e.regs.ReadDRaw(di))
			e.mem.WriteU64(addr+8, e.regs.ReadDRaw(di2))
			return nil
		}
		if si := sRegIndex(r); si >= 0 {
			addr := e.resolveMemAddr(inst.Args[2], true)
			r2 := inst.Args[1].(arm64asm.Reg)
			si2 := sRegIndex(r2)
			e.mem.WriteU32(addr, e.regs.ReadSRaw(si))
			e.mem.WriteU32(addr+4, e.regs.ReadSRaw(si2))
			return nil
		}
	}

	addr := e.resolveMemAddr(inst.Args[2], true)
	if is32 {
		e.mem.WriteU32(addr, uint32(e.readOperandVal(rt1, true)))
		e.mem.WriteU32(addr+4, uint32(e.readOperandVal(rt2, true)))
	} else {
		e.mem.WriteU64(addr, e.readOperandVal(rt1, false))
		e.mem.WriteU64(addr+8, e.readOperandVal(rt2, false))
	}
	return nil
}

// resolveLoadAddr is a helper for load-with-literal (PC-relative).
func (e *executor) resolveLoadAddr(inst arm64asm.Inst) (uint64, error) {
	memArg := inst.Args[1]
	switch m := memArg.(type) {
	case arm64asm.PCRel:
		return uint64(int64(e.regs.PC) + int64(m)), nil
	case arm64asm.MemImmediate, arm64asm.MemExtend:
		return e.resolveMemAddr(m, true), nil
	default:
		return 0, fmt.Errorf("unexpected load address type: %T", memArg)
	}
}
