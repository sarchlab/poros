package emu

import "golang.org/x/arch/arm64/arm64asm"

// Extension opcodes share the reserved range documented alongside OpCAS.
const (
	OpXPACI arm64asm.Op = 0xff02
	OpXPACD arm64asm.Op = 0xff03
)

// PointerStripPolicy describes effective translation/tag state, independently
// of authentication enablement. The Darwin policy is measured on the reference
// M2: 47 payload bits, bit 55 selecting upper region, data-only top-byte tags.
// Unspecified state fails closed; this does not implement tagged memory access.
type PointerStripPolicy uint8

const (
	PointerStripUnspecified PointerStripPolicy = iota
	PointerStripDarwin47Bit
)

func IsPointerStrip(op arm64asm.Op) bool { return op == OpXPACI || op == OpXPACD }
func decodePointerStrip(word uint32) (arm64asm.Inst, bool) {
	op := OpXPACI
	switch word & 0xffffffe0 {
	case 0xdac143e0:
	case 0xdac147e0:
		op = OpXPACD
	default:
		return arm64asm.Inst{}, false
	}
	return arm64asm.Inst{Op: op, Enc: word, Args: arm64asm.Args{arm64asm.X0 + arm64asm.Reg(word&31)}}, true
}
func (e *executor) execPointerStrip(inst arm64asm.Inst) error {
	if e.regs.PointerStrip != PointerStripDarwin47Bit {
		return &UnsupportedInstructionError{Instruction: inst}
	}
	rd := int(inst.Enc & 31)
	value := e.regs.ReadX(rd)
	result := value & ((uint64(1) << 47) - 1)
	if value&(uint64(1)<<55) != 0 {
		result |= ^((uint64(1) << 47) - 1)
	} else if inst.Op == OpXPACD {
		result |= value & (uint64(0xff) << 56)
	}
	e.regs.WriteX(rd, result)
	return nil
}
