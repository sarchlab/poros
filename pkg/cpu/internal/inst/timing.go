package inst

import "golang.org/x/arch/arm64/arm64asm"

type Unit int

const (
	UnitINT Unit = iota
	UnitFP
	UnitLDST
)
const (
	ConventionalForwardLatency = 5
	ExactGPRForwardLatency     = 1
)

// classifyUnit determines which execution unit type an instruction needs.
func ClassifyUnit(instruction arm64asm.Inst) Unit {
	op := instruction.Op
	if (op == arm64asm.ORR || op == arm64asm.MOVI) && ArgFP(instruction.Args[0]) >= 0 {
		return UnitFP
	}
	if IsLoad(op) || IsStore(op) {
		return UnitLDST
	}
	switch op {
	case arm64asm.FADD, arm64asm.FSUB, arm64asm.FMUL,
		arm64asm.FMADD, arm64asm.FMSUB,
		arm64asm.FNMUL, arm64asm.FNMADD,
		arm64asm.FDIV, arm64asm.FSQRT,
		arm64asm.FCMP, arm64asm.FCMPE, arm64asm.FMOV, arm64asm.FCSEL,
		arm64asm.SCVTF, arm64asm.UCVTF,
		arm64asm.FCVTZS, arm64asm.FCVTZU,
		arm64asm.FNEG, arm64asm.FABS:
		return UnitFP
	default:
		return UnitINT
	}
}

// Occupancy controls issue throughput; it is independent of result latency.
func UnitOccupancy(op arm64asm.Op) uint64 {
	switch op {
	case arm64asm.SDIV, arm64asm.UDIV:
		return uint64(ExecuteLatency(op))
	case arm64asm.FDIV:
		return 10
	case arm64asm.FSQRT:
		return 6
	default:
		return 1
	}
}

// DirectGPRTransfer restricts the fast hypothesis to the measured instruction
// class. Both producer and consumer must use scalar 32/64-bit immediate-offset
// LDR/STR without writeback. Register-offset, pair and unscaled forms retain
// conventional timing; see evaluation/microbenchmarks/forwarding/README.md.
func DirectGPRTransfer(inst arm64asm.Inst) bool {
	if inst.Op != arm64asm.LDR && inst.Op != arm64asm.STR {
		return false
	}
	reg, ok := inst.Args[0].(arm64asm.Reg)
	if !ok || !((reg >= arm64asm.W0 && reg <= arm64asm.W30) || (reg >= arm64asm.X0 && reg <= arm64asm.X30)) {
		return false
	}
	address, ok := inst.Args[1].(arm64asm.MemImmediate)
	return ok && address.Mode == arm64asm.AddrOffset
}
