package cpu

import (
	"unsafe"

	"golang.org/x/arch/arm64/arm64asm"
)

// Register index extraction for scoreboard tracking.
// GPR indices 0-30 map to X0-X30. Index 31 = XZR/WZR (not tracked).
// FP indices 0-31 map to D0-D31 / S0-S31.

func gprIdx(r arm64asm.Reg) int {
	if r >= arm64asm.X0 && r <= arm64asm.XZR {
		return int(r - arm64asm.X0)
	}
	if r >= arm64asm.W0 && r <= arm64asm.WZR {
		return int(r - arm64asm.W0)
	}
	return -1
}

func fpIdx(r arm64asm.Reg) int {
	if r >= arm64asm.D0 && r <= arm64asm.D31 {
		return int(r - arm64asm.D0)
	}
	if r >= arm64asm.S0 && r <= arm64asm.S31 {
		return int(r - arm64asm.S0)
	}
	return -1
}

func argGPR(arg arm64asm.Arg) int {
	if r, ok := arg.(arm64asm.Reg); ok {
		return gprIdx(r)
	}
	return -1
}

func argFP(arg arm64asm.Arg) int {
	if r, ok := arg.(arm64asm.Reg); ok {
		return fpIdx(r)
	}
	return -1
}

// destRegs returns the GPR and FP register indices written by inst.
// Returns -1 if no register of that type is written.
func destRegs(inst arm64asm.Inst) (gpr, fp int) {
	gpr, fp = -1, -1
	op := inst.Op

	// Stores don't write to a data register destination.
	if IsStore(op) {
		return
	}

	// Compare/test instructions only set flags.
	switch op {
	case arm64asm.CMP, arm64asm.CMN, arm64asm.TST,
		arm64asm.CCMP, arm64asm.FCMP:
		return
	}

	// Branches: BL/BLR write X30 (LR).
	if IsBranch(op) {
		if op == arm64asm.BL || op == arm64asm.BLR {
			gpr = 30
		}
		return
	}

	// NOP/HINT write nothing.
	if op == arm64asm.NOP || op == arm64asm.HINT {
		return
	}

	if inst.Args[0] == nil {
		return
	}

	r, ok := inst.Args[0].(arm64asm.Reg)
	if !ok {
		return
	}

	if idx := gprIdx(r); idx >= 0 && idx < 31 {
		gpr = idx
	}
	if idx := fpIdx(r); idx >= 0 {
		fp = idx
	}

	// LDP writes two GPRs (Args[0] and Args[1]).
	// We only track the first here; the second is handled by destRegs2.
	return
}

// destRegs2 returns the second GPR destination for pair instructions (LDP).
func destRegs2(inst arm64asm.Inst) int {
	if inst.Op != arm64asm.LDP && inst.Op != arm64asm.LDPSW {
		return -1
	}
	if inst.Args[1] == nil {
		return -1
	}
	if r, ok := inst.Args[1].(arm64asm.Reg); ok {
		if idx := gprIdx(r); idx >= 0 && idx < 31 {
			return idx
		}
	}
	return -1
}

// memImmLayout matches arm64asm.MemImmediate internal layout.
type memImmLayout struct {
	Base uint16
	Mode uint8
	_    uint8
	Imm  int32
}

func memImmBase(mi arm64asm.MemImmediate) arm64asm.Reg {
	s := *(*memImmLayout)(unsafe.Pointer(&mi))
	return arm64asm.Reg(s.Base)
}

// regExtLayout matches arm64asm.RegExtshiftAmount internal layout.
type regExtLayout struct {
	Reg      arm64asm.Reg
	ExtShift uint8
	Amount   uint8
	ShowZero bool
}

func regExtReg(rea arm64asm.RegExtshiftAmount) arm64asm.Reg {
	s := *(*regExtLayout)(unsafe.Pointer(&rea))
	return s.Reg
}

// sourceGPRs returns all GPR indices read by the instruction.
func sourceGPRs(inst arm64asm.Inst) []int {
	var srcs []int
	add := func(idx int) {
		if idx >= 0 && idx < 31 {
			srcs = append(srcs, idx)
		}
	}

	op := inst.Op

	// For stores: Args[0] is the value, Args[1] is the address.
	if IsStore(op) {
		add(argGPR(inst.Args[0]))
		extractMemSrcGPRs(inst.Args[1], add)
		// STP also stores Args[1] as a second value, address in Args[2].
		if op == arm64asm.STP {
			add(argGPR(inst.Args[1]))
			extractMemSrcGPRs(inst.Args[2], add)
		}
		return srcs
	}

	// For loads: source is the memory operand (base/index registers).
	if IsLoad(op) {
		// LDP: Args[0] and Args[1] are destinations, Args[2] is memory.
		if op == arm64asm.LDP || op == arm64asm.LDPSW {
			extractMemSrcGPRs(inst.Args[2], add)
		} else {
			extractMemSrcGPRs(inst.Args[1], add)
		}
		return srcs
	}

	// For branches that read registers.
	if IsBranch(op) {
		switch op {
		case arm64asm.BR, arm64asm.BLR, arm64asm.RET:
			if inst.Args[0] != nil {
				add(argGPR(inst.Args[0]))
			} else if op == arm64asm.RET {
				add(30) // default: X30
			}
		case arm64asm.CBZ, arm64asm.CBNZ:
			add(argGPR(inst.Args[0]))
		case arm64asm.TBZ, arm64asm.TBNZ:
			add(argGPR(inst.Args[0]))
		}
		return srcs
	}

	// General case: Args[1..] are sources.
	for i := 1; i < len(inst.Args); i++ {
		if inst.Args[i] == nil {
			break
		}
		extractArgGPRs(inst.Args[i], add)
	}

	return srcs
}

// sourceFPs returns all FP register indices read by the instruction.
func sourceFPs(inst arm64asm.Inst) []int {
	var srcs []int
	add := func(idx int) {
		if idx >= 0 {
			srcs = append(srcs, idx)
		}
	}

	if IsStore(inst.Op) {
		add(argFP(inst.Args[0]))
		return srcs
	}

	for i := 1; i < len(inst.Args); i++ {
		if inst.Args[i] == nil {
			break
		}
		add(argFP(inst.Args[i]))
	}
	return srcs
}

func extractMemSrcGPRs(arg arm64asm.Arg, add func(int)) {
	if arg == nil {
		return
	}
	switch m := arg.(type) {
	case arm64asm.MemImmediate:
		r := memImmBase(m)
		add(gprIdx(r))
	case arm64asm.MemExtend:
		add(gprIdx(arm64asm.Reg(m.Base)))
		add(gprIdx(m.Index))
	}
}

func extractArgGPRs(arg arm64asm.Arg, add func(int)) {
	switch a := arg.(type) {
	case arm64asm.Reg:
		add(gprIdx(a))
	case arm64asm.RegSP:
		// SP — not tracked in scoreboard
	case arm64asm.MemImmediate:
		r := memImmBase(a)
		add(gprIdx(r))
	case arm64asm.MemExtend:
		add(gprIdx(arm64asm.Reg(a.Base)))
		add(gprIdx(a.Index))
	case arm64asm.RegExtshiftAmount:
		r := regExtReg(a)
		add(gprIdx(r))
	}
}
