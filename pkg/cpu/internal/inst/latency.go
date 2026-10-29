package inst

import "golang.org/x/arch/arm64/arm64asm"

// ExecuteLatency returns the execute-stage latency in cycles for an instruction.
func ExecuteLatency(op arm64asm.Op) int {
	switch op {
	// Multiply
	case arm64asm.MUL, arm64asm.MADD, arm64asm.MSUB,
		arm64asm.UMULL, arm64asm.UMULH,
		arm64asm.SMULL, arm64asm.SMADDL, arm64asm.UMADDL:
		return 3

	// Division
	case arm64asm.SDIV, arm64asm.UDIV:
		return 10

	// FP simple (M2 Avalanche: 3-cycle FADD/FSUB)
	case arm64asm.FADD, arm64asm.FSUB, arm64asm.FCMP,
		arm64asm.FMOV, arm64asm.FCSEL,
		arm64asm.SCVTF, arm64asm.UCVTF,
		arm64asm.FCVTZS, arm64asm.FCVTZU,
		arm64asm.FNEG, arm64asm.FABS:
		return 3

	// FP multiply (standalone)
	case arm64asm.FMUL, arm64asm.FNMUL:
		return 4

	// FP multiply-accumulate (FMADD/FMSUB/FNMADD: multiply + accumulate pipeline stages)
	case arm64asm.FMADD, arm64asm.FMSUB, arm64asm.FNMADD:
		return 7

	// FP divide (M2 double-precision: ~22 cycles)
	case arm64asm.FDIV:
		return 22

	// FP square root
	case arm64asm.FSQRT:
		return 12

	default:
		// Simple INT (ADD, SUB, AND, ORR, MOV, CMP, LSL, LSR, CSEL, etc.)
		// Branch (B, BL, RET, CBZ, CBNZ, TBZ, TBNZ)
		// ADRP/ADR, NOP, HINT
		// Loads and stores (memory latency handled separately)
		return 1
	}
}

// IsLoad returns true if the instruction is a load operation.
func IsLoad(op arm64asm.Op) bool {
	switch op {
	case arm64asm.LDR, arm64asm.LDRB, arm64asm.LDRH,
		arm64asm.LDRSB, arm64asm.LDRSH, arm64asm.LDRSW,
		arm64asm.LDUR, arm64asm.LDURB, arm64asm.LDURH,
		arm64asm.LDURSB, arm64asm.LDURSH, arm64asm.LDURSW,
		arm64asm.LDP, arm64asm.LDPSW:
		return true
	}
	return false
}

// IsStore returns true if the instruction is a store operation.
func IsStore(op arm64asm.Op) bool {
	switch op {
	case arm64asm.STR, arm64asm.STRB, arm64asm.STRH,
		arm64asm.STUR, arm64asm.STURB, arm64asm.STURH,
		arm64asm.STP:
		return true
	}
	return false
}

// IsBranch returns true if the instruction is a branch.
func IsBranch(op arm64asm.Op) bool {
	switch op {
	case arm64asm.B, arm64asm.BL, arm64asm.BR, arm64asm.BLR,
		arm64asm.RET, arm64asm.CBZ, arm64asm.CBNZ,
		arm64asm.TBZ, arm64asm.TBNZ:
		return true
	}
	return false
}

// SetsFlags returns true if the instruction writes NZCV condition flags.
func SetsFlags(op arm64asm.Op) bool {
	switch op {
	case arm64asm.CMP, arm64asm.CMN, arm64asm.TST,
		arm64asm.CCMP, arm64asm.CCMN,
		arm64asm.ADDS, arm64asm.SUBS, arm64asm.ANDS,
		arm64asm.FCMP, arm64asm.FCCMP, arm64asm.FCCMPE:
		return true
	}
	return false
}

// UsesFlags returns true if the instruction reads NZCV condition flags.
// Branch instructions are excluded — they are predicted, not data-dependent.
func UsesFlags(op arm64asm.Op) bool {
	switch op {
	case arm64asm.CSEL, arm64asm.CSINC, arm64asm.CSINV, arm64asm.CSNEG, arm64asm.CNEG,
		arm64asm.CCMP, arm64asm.CCMN,
		arm64asm.FCSEL, arm64asm.FCCMP, arm64asm.FCCMPE:
		return true
	}
	return false
}
