package emu

import "golang.org/x/arch/arm64/arm64asm"

// DITState holds FEAT_DIT availability and PSTATE.DIT. It does not change the
// modeled instruction latencies; data-independent timing is not calibrated.
type DITState struct {
	Available bool
	Value     bool
}

// OpSB shares the reserved extension range documented alongside OpCAS.
const OpSB arm64asm.Op = 0xff06

func decodeDITBarrier(word uint32) (arm64asm.Inst, bool) {
	switch word {
	case 0xd503405f, 0xd503415f:
		return arm64asm.Inst{Op: arm64asm.MSR, Enc: word}, true
	case 0xd50330ff:
		return arm64asm.Inst{Op: OpSB, Enc: word}, true
	}
	return arm64asm.Inst{}, false
}

func (e *executor) execMSR(inst arm64asm.Inst) error {
	if !e.regs.DIT.Available {
		return &UnsupportedInstructionError{Instruction: inst}
	}
	switch {
	case inst.Enc == 0xd503405f || inst.Enc == 0xd503415f:
		e.regs.DIT.Value = inst.Enc&0x100 != 0
	case inst.Enc&^31 == 0xd51b42a0:
		e.regs.DIT.Value = e.regs.ReadX(int(inst.Enc&31))&(1<<24) != 0
	default:
		return &UnsupportedInstructionError{Instruction: inst}
	}
	return nil
}
