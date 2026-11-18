package ooo

import (
	"math/bits"

	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"golang.org/x/arch/arm64/arm64asm"
)

// Latencies replaces the timing of the instruction classes that differ between
// cores. Every field is in cycles and a zero field keeps the default, which is
// the Apple M2 value of instinfo.ExecuteLatency and unitOccupancy, so the zero
// value changes nothing.
type Latencies struct {
	// Multiply is the result latency of MUL, MADD, MSUB and the long
	// multiplies. UMULH keeps its default.
	Multiply int `json:"multiply,omitempty"`
	// MultiplyAdd overrides Multiply for MADD and MSUB when positive.
	MultiplyAdd int `json:"multiply_add,omitempty"`
	// DivideBase and DivideBitsPerCycle describe an integer divider that ends
	// early on a short quotient: SDIV and UDIV take DivideBase cycles plus one
	// for every DivideBitsPerCycle significant bits of the quotient's
	// magnitude, at least one. DivideBitsPerCycle zero makes every divide take
	// DivideBase cycles.
	DivideBase         int `json:"divide_base,omitempty"`
	DivideBitsPerCycle int `json:"divide_bits_per_cycle,omitempty"`
	// FPArithmetic is the result latency of FADD, FSUB, FCMP, FCMPE, FCSEL,
	// FNEG and FABS. Conversions and FMOV keep their default.
	FPArithmetic int `json:"fp_arithmetic,omitempty"`
	// FPMultiply is the result latency of FMUL and FNMUL.
	FPMultiply int `json:"fp_multiply,omitempty"`
	// FPMultiplyAdd is the full result latency of scalar fused multiply-adds.
	FPMultiplyAdd int `json:"fp_multiply_add,omitempty"`
	// FPAccumulate is the latency from the addend of a fused multiply-add to
	// its result when an FP multiply or another fused multiply-add produced
	// that addend: the instruction may then issue before the addend is ready,
	// by its result latency less FPAccumulate. Zero needs every operand at
	// issue.
	FPAccumulate int `json:"fp_accumulate,omitempty"`
	// FPDivide is the result latency of FDIV. FPDivideOccupancy, when
	// positive, gives the core Spec.NumFPDiv FP dividers of their own: an FDIV
	// keeps one from starting another divide or square root for that many
	// cycles and delays no other FP work. Zero keeps the default, in which a
	// divide occupies its FP pipe for one cycle.
	FPDivide          int `json:"fp_divide,omitempty"`
	FPDivideOccupancy int `json:"fp_divide_occupancy,omitempty"`
	// FPLoadExtra is added to the latency of a load into an FP register that
	// the cache supplies.
	FPLoadExtra int `json:"fp_load_extra,omitempty"`
}

func (l Latencies) valid() bool {
	return l.Multiply >= 0 && l.MultiplyAdd >= 0 && l.FPMultiplyAdd >= 0 && l.DivideBase >= 0 && l.DivideBitsPerCycle >= 0 && l.FPArithmetic >= 0 &&
		l.FPMultiply >= 0 && l.FPAccumulate >= 0 && l.FPDivide >= 0 && l.FPDivideOccupancy >= 0 && l.FPLoadExtra >= 0 &&
		// An early-ending divider needs its fixed part.
		(l.DivideBitsPerCycle == 0 || l.DivideBase > 0)
}

// fusedMultiplyAdd reports the scalar fused multiply-add family.
func fusedMultiplyAdd(op arm64asm.Op) bool {
	switch op {
	case arm64asm.FMADD, arm64asm.FMSUB, arm64asm.FNMADD, arm64asm.FNMSUB:
		return true
	}
	return false
}

func integerDivide(op arm64asm.Op) bool { return op == arm64asm.SDIV || op == arm64asm.UDIV }

// executeLatency is the result latency of an instruction that is neither a
// load nor a store. An integer divide with a configured divider is timed by
// divideLatency instead.
func (p *middleware) executeLatency(inst arm64asm.Inst) uint64 {
	l := p.comp.Spec.Latencies
	override := 0
	switch inst.Op {
	case arm64asm.MADD, arm64asm.MSUB:
		override = l.MultiplyAdd
		if override == 0 {
			override = l.Multiply
		}
	case arm64asm.MUL, arm64asm.UMULL, arm64asm.SMULL, arm64asm.SMADDL, arm64asm.UMADDL:
		override = l.Multiply
	case arm64asm.FADD, arm64asm.FSUB, arm64asm.FCMP, arm64asm.FCMPE, arm64asm.FCSEL, arm64asm.FNEG, arm64asm.FABS:
		override = l.FPArithmetic
	case arm64asm.FMUL, arm64asm.FNMUL:
		override = l.FPMultiply
	case arm64asm.FMADD, arm64asm.FMSUB, arm64asm.FNMADD, arm64asm.FNMSUB:
		override = l.FPMultiplyAdd
	case arm64asm.FDIV:
		override = l.FPDivide
	}
	if override > 0 {
		return uint64(override)
	}
	return uint64(instinfo.ExecuteLatency(inst.Op))
}

// divideLatency is the latency Latencies gives the integer divide the backend
// is about to execute, or zero when inst is not one or the default applies. It
// reads the operands, so it must be called before the instruction executes: the
// destination may be one of them, or the zero register.
func (p *middleware) divideLatency(inst arm64asm.Inst) uint64 {
	l := p.comp.Spec.Latencies
	if !integerDivide(inst.Op) || l.DivideBase == 0 {
		return 0
	}
	latency := l.DivideBase
	if l.DivideBitsPerCycle > 0 {
		latency += (p.quotientBits(inst) + l.DivideBitsPerCycle - 1) / l.DivideBitsPerCycle
	}
	return uint64(latency)
}

// quotientBits is the number of significant bits, at least one, of the
// magnitude of the quotient of the divide's operands. Division by zero gives
// the quotient zero, as the instruction does.
func (p *middleware) quotientBits(inst arm64asm.Inst) int {
	registers := p.comp.Resources.backend().CPURegisters()
	operand := func(arg arm64asm.Arg) (value uint64, word bool) {
		register, _ := arg.(arm64asm.Reg)
		if index := instinfo.GprIdx(register); index >= 0 && index < 31 {
			value = registers.GPR[index]
		}
		return value, register >= arm64asm.W0 && register <= arm64asm.WZR
	}
	dividend, word := operand(inst.Args[1])
	divisor, _ := operand(inst.Args[2])
	if word {
		dividend, divisor = uint64(uint32(dividend)), uint64(uint32(divisor))
		if inst.Op == arm64asm.SDIV {
			dividend, divisor = uint64(magnitude(int64(int32(dividend)))), uint64(magnitude(int64(int32(divisor))))
		}
	} else if inst.Op == arm64asm.SDIV {
		dividend, divisor = magnitude(int64(dividend)), magnitude(int64(divisor))
	}
	if divisor == 0 {
		return 1
	}
	return max(1, bits.Len64(dividend/divisor))
}

// magnitude is |v| as an unsigned value; the most negative value keeps its
// magnitude, which does not fit the signed type.
func magnitude(v int64) uint64 {
	if v < 0 {
		return -uint64(v)
	}
	return uint64(v)
}

// occupancy is how long an instruction keeps its unit from issuing another;
// it is independent of result latency. divide is divideLatency's result.
func (p *middleware) occupancy(inst arm64asm.Inst, divide uint64) uint64 {
	switch {
	case divide > 0:
		return divide
	case inst.Op == arm64asm.FDIV && p.comp.Spec.Latencies.FPDivideOccupancy > 0:
		return uint64(p.comp.Spec.Latencies.FPDivideOccupancy)
	}
	return unitOccupancy(inst.Op)
}

// accumulateSlack is how many cycles before its addend is ready a fused
// multiply-add may issue, when the addend's producer can forward it late;
// addend is then that register, and -1 otherwise.
func (p *middleware) accumulateSlack(inst arm64asm.Inst) (addend int, slack uint64) {
	accumulate := p.comp.Spec.Latencies.FPAccumulate
	if accumulate == 0 || !fusedMultiplyAdd(inst.Op) {
		return -1, 0
	}
	addend = instinfo.ArgFP(inst.Args[3])
	latency := int(p.executeLatency(inst))
	if addend < 0 || accumulate >= latency || !p.comp.State.FpLateForward[addend] {
		return -1, 0
	}
	return addend, uint64(latency - accumulate)
}
