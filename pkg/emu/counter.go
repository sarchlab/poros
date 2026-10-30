package emu

import "golang.org/x/arch/arm64/arm64asm"

// CounterState is explicitly supplied virtual time. The zero value leaves
// system-counter reads unsupported; no host clock is consulted by the emulator.
type CounterState struct {
	Enabled   bool
	Value     uint64
	Frequency uint64
}

// ThreadPointerState supplies the effective read-only user TLS register. Its
// zero value rejects reads until the process runtime has initialized it.
type ThreadPointerState struct {
	Enabled bool
	Value   uint64
}

// AppleThreadProtectionState supplies an explicit implementation-defined user
// register value. Reads do not infer a native default or enable protection writes.
type AppleThreadProtectionState struct {
	Enabled bool
	Value   uint64
}

func (e *executor) execMRS(inst arm64asm.Inst) error {
	if inst.Enc&^31 == 0xd53ef1a0 {
		if !e.regs.AppleThreadProtection.Enabled {
			return &UnsupportedInstructionError{Instruction: inst}
		}
		e.regs.WriteX(int(inst.Enc&31), e.regs.AppleThreadProtection.Value)
		return nil
	}
	if inst.Enc&^31 == 0xd53bd060 {
		if !e.regs.ThreadPointer.Enabled {
			return &UnsupportedInstructionError{Instruction: inst}
		}
		e.regs.WriteX(int(inst.Enc&31), e.regs.ThreadPointer.Value)
		return nil
	}

	if !e.regs.Counter.Enabled {
		return &UnsupportedInstructionError{Instruction: inst}
	}
	var value uint64
	switch inst.Enc &^ 31 {
	case 0xd53be040, 0xd53be0c0, 0xd53cfac0: // CNTVCT_EL0, CNTVCTSS_EL0, Apple's APCNTVCT_EL0
		value = e.regs.Counter.Value
	case 0xd53be000: // CNTFRQ_EL0
		value = e.regs.Counter.Frequency
	default:
		return &UnsupportedInstructionError{Instruction: inst}
	}
	e.regs.WriteX(int(inst.Enc&31), value)
	return nil
}
