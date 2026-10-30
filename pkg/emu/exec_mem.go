package emu

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/arch/arm64/arm64asm"
)

// readMemory and writeMemory preserve the legacy typed access widths (including
// Q pairs as four 8-byte accesses). Checked execution validates/transfers a whole
// instruction's span at once, so a pair cannot partially modify memory.
func (e *executor) readMemory(addr uint64, width, count int) ([32]byte, error) {
	var b [32]byte
	if e.checked != nil {
		err := e.checked.Read(addr, e.scratch[:width*count])
		if err != nil {
			return b, &memoryAccessError{err}
		}
		return e.scratch, nil
	}
	for i := 0; i < width*count; i += width {
		a := addr + uint64(i)
		switch width {
		case 1:
			b[i] = e.mem.ReadU8(a)
		case 2:
			binary.LittleEndian.PutUint16(b[i:], e.mem.ReadU16(a))
		case 4:
			binary.LittleEndian.PutUint32(b[i:], e.mem.ReadU32(a))
		case 8:
			binary.LittleEndian.PutUint64(b[i:], e.mem.ReadU64(a))
		}
	}
	return b, nil
}
func (e *executor) writeMemory(addr uint64, width int, b []byte) error {
	if e.checked != nil {
		if err := e.checked.Write(addr, b); err != nil {
			return &memoryAccessError{err}
		}
		return nil
	}
	for i := 0; i < len(b); i += width {
		a := addr + uint64(i)
		switch width {
		case 1:
			e.mem.WriteU8(a, b[i])
		case 2:
			e.mem.WriteU16(a, binary.LittleEndian.Uint16(b[i:]))
		case 4:
			e.mem.WriteU32(a, binary.LittleEndian.Uint32(b[i:]))
		case 8:
			e.mem.WriteU64(a, binary.LittleEndian.Uint64(b[i:]))
		}
	}
	return nil
}
func memoryWidth(arg arm64asm.Arg) int {
	if r, ok := arg.(arm64asm.Reg); ok {
		if qRegIndex(r) >= 0 {
			return 16
		}
		if dRegIndex(r) >= 0 {
			return 8
		}
		if sRegIndex(r) >= 0 {
			return 4
		}
	}
	if is32bit(arg) {
		return 4
	}
	return 8
}
func (e *executor) loadRegister(arg arm64asm.Arg, b []byte, signed bool) {
	var v uint64
	switch len(b) {
	case 1:
		v = uint64(b[0])
		if signed {
			v = uint64(int64(int8(v)))
		}
	case 2:
		v = uint64(binary.LittleEndian.Uint16(b))
		if signed {
			v = uint64(int64(int16(v)))
		}
	case 4:
		v = uint64(binary.LittleEndian.Uint32(b))
		if signed {
			v = uint64(int64(int32(v)))
		}
	case 8, 16:
		v = binary.LittleEndian.Uint64(b)
	}
	if r, ok := arg.(arm64asm.Reg); ok {
		if q := qRegIndex(r); q >= 0 {
			e.regs.WriteQ(q, v, binary.LittleEndian.Uint64(b[8:]))
			return
		}
		if d := dRegIndex(r); d >= 0 {
			e.regs.WriteDRaw(d, v)
			return
		}
		if s := sRegIndex(r); s >= 0 {
			e.regs.WriteSRaw(s, uint32(v))
			return
		}
	}
	e.writeGPR(arg, v)
}
func (e *executor) storeRegister(arg arm64asm.Arg, b []byte) {
	var lo, hi uint64
	if r, ok := arg.(arm64asm.Reg); ok && qRegIndex(r) >= 0 {
		lo, hi = e.regs.ReadQ(qRegIndex(r))
	} else if r, ok := arg.(arm64asm.Reg); ok && dRegIndex(r) >= 0 {
		lo = e.regs.ReadDRaw(dRegIndex(r))
	} else if r, ok := arg.(arm64asm.Reg); ok && sRegIndex(r) >= 0 {
		lo = uint64(e.regs.ReadSRaw(sRegIndex(r)))
	} else {
		lo = e.readOperandVal(arg, is32bit(arg))
	}
	switch len(b) {
	case 1:
		b[0] = byte(lo)
	case 2:
		binary.LittleEndian.PutUint16(b, uint16(lo))
	case 4:
		binary.LittleEndian.PutUint32(b, uint32(lo))
	case 8:
		binary.LittleEndian.PutUint64(b, lo)
	case 16:
		binary.LittleEndian.PutUint64(b, lo)
		binary.LittleEndian.PutUint64(b[8:], hi)
	}
}
func (e *executor) load(inst arm64asm.Inst, width, count int, signed bool) error {
	if width == 0 {
		width = memoryWidth(inst.Args[0])
	}
	addrArg := inst.Args[count]
	var addr uint64
	switch m := addrArg.(type) {
	case arm64asm.PCRel:
		addr = e.regs.PC + uint64(m)
	case arm64asm.MemImmediate, arm64asm.MemExtend:
		addr = e.resolveMemAddr(m, true)
	default:
		return fmt.Errorf("unexpected load address type: %T", addrArg)
	}
	b, err := e.readMemory(addr, min(width, 8), count*width/min(width, 8))
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		e.loadRegister(inst.Args[i], b[i*width:(i+1)*width], signed)
	}
	return nil
}
func (e *executor) store(inst arm64asm.Inst, width, count int) error {
	if width == 0 {
		width = memoryWidth(inst.Args[0])
	}
	addr := e.resolveMemAddr(inst.Args[count], true)
	b := e.scratch[:width*count]
	for i := 0; i < count; i++ {
		e.storeRegister(inst.Args[i], b[i*width:(i+1)*width])
	}
	return e.writeMemory(addr, min(width, 8), b[:width*count])
}
func (e *executor) execLDR(i arm64asm.Inst) error    { return e.load(i, 0, 1, false) }
func (e *executor) execLDRB(i arm64asm.Inst) error   { return e.load(i, 1, 1, false) }
func (e *executor) execLDRH(i arm64asm.Inst) error   { return e.load(i, 2, 1, false) }
func (e *executor) execLDRSB(i arm64asm.Inst) error  { return e.load(i, 1, 1, true) }
func (e *executor) execLDRSH(i arm64asm.Inst) error  { return e.load(i, 2, 1, true) }
func (e *executor) execLDRSW(i arm64asm.Inst) error  { return e.load(i, 4, 1, true) }
func (e *executor) execLDP(i arm64asm.Inst) error    { return e.load(i, 0, 2, false) }
func (e *executor) execLDPSW(i arm64asm.Inst) error  { return e.load(i, 4, 2, true) }
func (e *executor) execSTR(i arm64asm.Inst) error    { return e.store(i, 0, 1) }
func (e *executor) execSTRB(i arm64asm.Inst) error   { return e.store(i, 1, 1) }
func (e *executor) execSTRH(i arm64asm.Inst) error   { return e.store(i, 2, 1) }
func (e *executor) execSTP(i arm64asm.Inst) error    { return e.store(i, 0, 2) }
func (e *executor) execLDUR(i arm64asm.Inst) error   { return e.execLDR(i) }
func (e *executor) execLDURB(i arm64asm.Inst) error  { return e.execLDRB(i) }
func (e *executor) execLDURH(i arm64asm.Inst) error  { return e.execLDRH(i) }
func (e *executor) execLDURSB(i arm64asm.Inst) error { return e.execLDRSB(i) }
func (e *executor) execLDURSH(i arm64asm.Inst) error { return e.execLDRSH(i) }
func (e *executor) execLDURSW(i arm64asm.Inst) error { return e.execLDRSW(i) }
func (e *executor) execSTUR(i arm64asm.Inst) error   { return e.execSTR(i) }
func (e *executor) execSTURB(i arm64asm.Inst) error  { return e.execSTRB(i) }
func (e *executor) execSTURH(i arm64asm.Inst) error  { return e.execSTRH(i) }

// execOrderedMemory uses the same checked transaction. The single-owner
// stream is ordered; timing consumers conservatively serialize acquire/release.
func (e *executor) execOrderedMemory(inst arm64asm.Inst) error {
	width := memoryWidth(inst.Args[0])
	if err := e.checkAtomicAlignment(e.resolveMemAddr(inst.Args[1], false), width); err != nil {
		return err
	}
	if inst.Op == arm64asm.LDAR {
		return e.load(inst, width, 1, false)
	}
	return e.store(inst, width, 1)
}
