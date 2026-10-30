package emu

import (
	"math"

	"golang.org/x/arch/arm64/arm64asm"
)

// Registers holds the complete ARM64 register state.
type Registers struct {
	PointerStrip     PointerStripPolicy    `json:",omitempty"`
	SPAlignmentCheck bool                  `json:",omitempty"`
	AtomicAlignment  AtomicAlignmentPolicy `json:",omitempty"`
	ThreadPointer    ThreadPointerState    `json:",omitzero"`
	Counter          CounterState          `json:",omitzero"`
	PointerAuth      PointerAuthMode       `json:",omitempty"` // explicit process ABI state; zero rejects PAC instructions
	GPR              [31]uint64            // X0-X30
	SP               uint64                // Stack pointer (register 31 in SP context)
	PC               uint64                // Program counter
	N, Z, C, V       bool                  // NZCV condition flags
	FP               [32]uint128           // V0-V31 (128-bit SIMD/FP registers)
}

// uint128 represents a 128-bit value as two 64-bit halves.
type uint128 struct {
	Lo uint64 // bits [63:0]
	Hi uint64 // bits [127:64]
}

// NewRegisters creates a zero-initialized register file.
func NewRegisters() *Registers {
	return &Registers{}
}

// ReadX reads a 64-bit X register. XZR returns 0.
func (r *Registers) ReadX(reg int) uint64 {
	if reg == 31 {
		return 0 // XZR
	}
	return r.GPR[reg]
}

// WriteX writes a 64-bit X register. Writes to XZR (31) are discarded.
func (r *Registers) WriteX(reg int, val uint64) {
	if reg == 31 {
		return // XZR: discard
	}
	r.GPR[reg] = val
}

// ReadW reads a 32-bit W register (lower 32 bits of X register). WZR returns 0.
func (r *Registers) ReadW(reg int) uint32 {
	if reg == 31 {
		return 0 // WZR
	}
	return uint32(r.GPR[reg])
}

// WriteW writes a 32-bit W register, zero-extending to 64 bits.
// Writes to WZR (31) are discarded.
func (r *Registers) WriteW(reg int, val uint32) {
	if reg == 31 {
		return // WZR: discard
	}
	r.GPR[reg] = uint64(val)
}

// ReadD reads a float64 from a D register (lower 64 bits of V register).
func (r *Registers) ReadD(reg int) float64 {
	return math.Float64frombits(r.FP[reg].Lo)
}

// WriteD writes a float64 to a D register, clearing the upper 64 bits.
func (r *Registers) WriteD(reg int, val float64) {
	r.FP[reg] = uint128{Lo: math.Float64bits(val)}
}

// ReadS reads a float32 from an S register (lower 32 bits of V register).
func (r *Registers) ReadS(reg int) float32 {
	return math.Float32frombits(uint32(r.FP[reg].Lo))
}

// WriteS writes a float32 to an S register, clearing the upper 96 bits.
func (r *Registers) WriteS(reg int, val float32) {
	r.FP[reg] = uint128{Lo: uint64(math.Float32bits(val))}
}

// ReadDRaw reads the raw 64 bits of a D register.
func (r *Registers) ReadDRaw(reg int) uint64 {
	return r.FP[reg].Lo
}

// WriteDRaw writes raw 64 bits to a D register.
func (r *Registers) WriteDRaw(reg int, val uint64) {
	r.FP[reg] = uint128{Lo: val}
}

// ReadSRaw reads the raw 32 bits of an S register.
func (r *Registers) ReadSRaw(reg int) uint32 {
	return uint32(r.FP[reg].Lo)
}

// WriteSRaw writes raw 32 bits to an S register.
func (r *Registers) WriteSRaw(reg int, val uint32) {
	r.FP[reg] = uint128{Lo: uint64(val)}
}

// EvalCond evaluates a condition code against the current NZCV flags.
func (r *Registers) EvalCond(c arm64asm.Cond) bool {
	condStr := c.String()
	switch condStr {
	case "EQ":
		return r.Z
	case "NE":
		return !r.Z
	case "CS", "HS":
		return r.C
	case "CC", "LO":
		return !r.C
	case "MI":
		return r.N
	case "PL":
		return !r.N
	case "VS":
		return r.V
	case "VC":
		return !r.V
	case "HI":
		return r.C && !r.Z
	case "LS":
		return !r.C || r.Z
	case "GE":
		return r.N == r.V
	case "LT":
		return r.N != r.V
	case "GT":
		return !r.Z && (r.N == r.V)
	case "LE":
		return r.Z || (r.N != r.V)
	case "AL":
		return true
	default:
		return true
	}
}

// SetNZ sets the N and Z flags based on a 64-bit result.
func (r *Registers) SetNZ64(result uint64) {
	r.N = (result >> 63) != 0
	r.Z = result == 0
}

// SetNZ32 sets the N and Z flags based on a 32-bit result.
func (r *Registers) SetNZ32(result uint32) {
	r.N = (result >> 31) != 0
	r.Z = result == 0
}

// regIndex extracts the register index (0-30) from an arm64asm register.
// For X registers: X0=index 0, X30=index 30, XZR=31.
// For W registers: W0=index 0, W30=index 30, WZR=31.
func xRegIndex(reg arm64asm.Reg) int {
	if reg >= arm64asm.X0 && reg <= arm64asm.XZR {
		return int(reg - arm64asm.X0)
	}
	return -1
}

func wRegIndex(reg arm64asm.Reg) int {
	if reg >= arm64asm.W0 && reg <= arm64asm.WZR {
		return int(reg - arm64asm.W0)
	}
	return -1
}

func dRegIndex(reg arm64asm.Reg) int {
	if reg >= arm64asm.D0 && reg <= arm64asm.D31 {
		return int(reg - arm64asm.D0)
	}
	return -1
}

func sRegIndex(reg arm64asm.Reg) int {
	if reg >= arm64asm.S0 && reg <= arm64asm.S31 {
		return int(reg - arm64asm.S0)
	}
	return -1
}

func qRegIndex(reg arm64asm.Reg) int {
	if reg >= arm64asm.Q0 && reg <= arm64asm.Q31 {
		return int(reg - arm64asm.Q0)
	}
	return -1
}

func vRegIndex(reg arm64asm.Reg) int {
	if reg >= arm64asm.V0 && reg <= arm64asm.V31 {
		return int(reg - arm64asm.V0)
	}
	return -1
}

// ReadQ reads the full 128-bit Q register as (lo, hi).
func (r *Registers) ReadQ(reg int) (uint64, uint64) {
	return r.FP[reg].Lo, r.FP[reg].Hi
}

// WriteQ writes a full 128-bit Q register.
func (r *Registers) WriteQ(reg int, lo, hi uint64) {
	r.FP[reg] = uint128{Lo: lo, Hi: hi}
}
