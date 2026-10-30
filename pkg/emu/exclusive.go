package emu

import (
	"golang.org/x/arch/arm64/arm64asm"
)

// ExclusivePolicy is effective implementation state, not a universal ARM rule.
type ExclusivePolicy uint8

const (
	ExclusiveUnspecified ExclusivePolicy = iota
	// ExclusiveM2Local64 models the measured single-owner M2 local monitor:
	// one 64-byte granule, write permission checked on failure. Alignment is
	// supplied separately by AtomicAlignment, including measured unaligned M2 behavior.
	ExclusiveM2Local64
)

type ExclusiveState struct {
	Policy ExclusivePolicy
	Valid  bool
	Base   uint64
}

func (r *Registers) ClearExclusive() { r.Exclusive.Valid = false; r.Exclusive.Base = 0 }
func (r *Registers) invalidateExclusiveStore(address uint64, size int) {
	if r.Exclusive.Valid && address < r.Exclusive.Base+64 && r.Exclusive.Base < address+uint64(size) {
		r.ClearExclusive()
	}
}

// ExclusiveMemory validates store permissions even when the local reservation
// fails. It writes atomically only when enabled; failure must not mutate bytes.
// The single execution owner performs all writes (including kernel calls).
type ExclusiveMemory interface {
	WriteExclusive(address uint64, data []byte, enabled bool) error
}

func IsExclusiveLoad(op arm64asm.Op) bool  { return op == arm64asm.LDXR || op == arm64asm.LDAXR }
func IsExclusiveStore(op arm64asm.Op) bool { return op == arm64asm.STXR || op == arm64asm.STLXR }
func (e *executor) execExclusive(inst arm64asm.Inst) error {
	if e.regs.Exclusive.Policy != ExclusiveM2Local64 {
		return &UnsupportedInstructionError{Instruction: inst}
	}
	if inst.Op == arm64asm.CLREX {
		e.regs.ClearExclusive()
		return nil
	}
	load := IsExclusiveLoad(inst.Op)
	operand := 0
	if !load {
		operand = 1
	}
	width := memoryWidth(inst.Args[operand])
	addr := e.resolveMemAddr(inst.Args[operand+1], false)
	if err := e.checkAtomicAlignment(addr, width); err != nil {
		return err
	}
	if load {
		if err := e.load(inst, width, 1, false); err != nil {
			return err
		}
		e.regs.Exclusive.Valid = true
		e.regs.Exclusive.Base = addr &^ uint64(63)
		return nil
	}
	word := inst.Enc
	rs, rt, rn := int((word>>16)&31), int(word&31), int((word>>5)&31)
	if rs != 31 && (rs == rt || rs == rn) {
		return &UnsupportedInstructionError{Instruction: inst}
	} // constrained unpredictable register overlap
	memory, ok := e.checked.(ExclusiveMemory)
	if !ok {
		return &UnsupportedInstructionError{Instruction: inst}
	}
	b := e.scratch[:width]
	e.storeRegister(inst.Args[1], b)
	success := e.regs.Exclusive.Valid && e.regs.Exclusive.Base == addr&^uint64(63)
	if err := memory.WriteExclusive(addr, b, success); err != nil {
		return &memoryAccessError{err}
	}
	e.regs.ClearExclusive()
	status := uint32(1)
	if success {
		status = 0
	}
	e.regs.WriteW(rs, status)
	return nil
}
