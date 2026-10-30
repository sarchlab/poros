package inst

import (
	"github.com/sarchlab/poros/pkg/emu"
	"unsafe"

	"golang.org/x/arch/arm64/arm64asm"
)

// Register index extraction for scoreboard tracking.
// GPR indices 0-30 map to X0-X30. Index 31 = XZR/WZR (not tracked).
// FP indices 0-31 map to D0-D31 / S0-S31.

func GprIdx(r arm64asm.Reg) int {
	if r >= arm64asm.X0 && r <= arm64asm.XZR {
		return int(r - arm64asm.X0)
	}
	if r >= arm64asm.W0 && r <= arm64asm.WZR {
		return int(r - arm64asm.W0)
	}
	return -1
}

func FpIdx(r arm64asm.Reg) int {
	if r >= arm64asm.B0 && r <= arm64asm.B31 {
		return int(r - arm64asm.B0)
	}
	if r >= arm64asm.H0 && r <= arm64asm.H31 {
		return int(r - arm64asm.H0)
	}
	if r >= arm64asm.D0 && r <= arm64asm.D31 {
		return int(r - arm64asm.D0)
	}
	if r >= arm64asm.S0 && r <= arm64asm.S31 {
		return int(r - arm64asm.S0)
	}
	// Q and V registers alias the same physical FP register file.
	if r >= arm64asm.Q0 && r <= arm64asm.Q31 {
		return int(r - arm64asm.Q0)
	}
	if r >= arm64asm.V0 && r <= arm64asm.V31 {
		return int(r - arm64asm.V0)
	}
	return -1
}

func ArgGPR(arg arm64asm.Arg) int {
	switch r := arg.(type) {
	case arm64asm.Reg:
		return GprIdx(r)
	case arm64asm.RegSP:
		// RegSP is an operand class: values 0..30 are ordinary GPRs.
		// Actual SP/WSP stays outside the existing GPR scoreboard.
		if idx := GprIdx(arm64asm.Reg(r)); idx >= 0 && idx < 31 {
			return idx
		}
	}
	return -1
}

// rwaLayout matches arm64asm.RegisterWithArrangement internal layout.
type rwaLayout struct {
	r   uint16
	a   uint8
	cnt uint8
}

// rwaiLayout matches arm64asm.RegisterWithArrangementAndIndex internal layout.
type rwaiLayout struct {
	r     uint16
	a     uint8
	index uint8
	cnt   uint8
}

func ArgFP(arg arm64asm.Arg) int {
	switch a := arg.(type) {
	case arm64asm.Reg:
		return FpIdx(a)
	case arm64asm.RegisterWithArrangement:
		s := *(*rwaLayout)(unsafe.Pointer(&a))
		return FpIdx(arm64asm.Reg(s.r))
	case arm64asm.RegisterWithArrangementAndIndex:
		s := *(*rwaiLayout)(unsafe.Pointer(&a))
		return FpIdx(arm64asm.Reg(s.r))
	}
	return -1
}

// DestRegs returns the GPR and FP register indices written by inst.
// Returns -1 if no register of that type is written.
func DestRegs(inst arm64asm.Inst) (gpr, fp int) {
	gpr, fp = -1, -1
	op := inst.Op
	if op == emu.OpCAS {
		gpr = ArgGPR(inst.Args[0])
		if gpr == 31 {
			gpr = -1
		}
		return gpr, -1
	}
	if op == emu.OpSWP || op == emu.OpLDADD {
		gpr = ArgGPR(inst.Args[1])
		if gpr == 31 {
			gpr = -1
		}
		return gpr, -1
	}

	if emu.IsExclusiveStore(op) {
		gpr = ArgGPR(inst.Args[0])
		if gpr == 31 {
			gpr = -1
		}
		return gpr, -1
	}
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

	// Extract destination register from Args[0], handling both
	// plain Reg and SIMD RegisterWithArrangement.
	fp = ArgFP(inst.Args[0])
	if idx := ArgGPR(inst.Args[0]); idx >= 0 && idx < 31 {
		gpr = idx
	}

	// LDP writes two registers (Args[0] and Args[1]).
	// We only track the first here; the second is handled by DestRegs2.
	return
}

// DestRegs2 returns the second destination for pair instructions (LDP).
func DestRegs2(inst arm64asm.Inst) (gpr, fp int) {
	gpr, fp = -1, -1
	if inst.Op != arm64asm.LDNP && inst.Op != arm64asm.LDP && inst.Op != arm64asm.LDPSW {
		return
	}
	if inst.Args[1] == nil {
		return
	}
	if r, ok := inst.Args[1].(arm64asm.Reg); ok {
		if idx := GprIdx(r); idx >= 0 && idx < 31 {
			gpr = idx
		}
		if idx := FpIdx(r); idx >= 0 {
			fp = idx
		}
	}
	return
}

// memImmLayout matches arm64asm.MemImmediate internal layout.
type memImmLayout struct {
	Base uint16
	Mode uint8
	_    uint8
	Imm  int32
}

func MemImmBase(mi arm64asm.MemImmediate) arm64asm.Reg {
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

func RegExtReg(rea arm64asm.RegExtshiftAmount) arm64asm.Reg {
	s := *(*regExtLayout)(unsafe.Pointer(&rea))
	return s.Reg
}

// BaseWritebackGPR returns the GPR index of the base register written back
// by pre/post-indexed load/store instructions (e.g., "ldr d1, [x22], #8"
// writes x22 = x22+8). Returns -1 if no writeback occurs.
func BaseWritebackGPR(inst arm64asm.Inst) int {
	if emu.IsAtomicOperation(inst.Op) {
		return -1
	}
	var memArg arm64asm.Arg
	if IsLoad(inst.Op) {
		if inst.Op == arm64asm.LDNP || inst.Op == arm64asm.LDP || inst.Op == arm64asm.LDPSW {
			memArg = inst.Args[2]
		} else {
			memArg = inst.Args[1]
		}
	} else if IsStore(inst.Op) {
		if inst.Op == arm64asm.STNP || inst.Op == arm64asm.STP {
			memArg = inst.Args[2]
		} else {
			memArg = inst.Args[1]
		}
	}
	if memArg == nil {
		return -1
	}
	mi, ok := memArg.(arm64asm.MemImmediate)
	if !ok {
		return -1
	}
	s := *(*memImmLayout)(unsafe.Pointer(&mi))
	mode := arm64asm.AddrMode(s.Mode)
	if mode != arm64asm.AddrPostIndex && mode != arm64asm.AddrPreIndex {
		return -1
	}
	return GprIdx(arm64asm.Reg(s.Base))
}

// SourceGPRs returns all GPR indices read by the instruction.
func SourceGPRs(inst arm64asm.Inst) []int {
	var srcs []int
	add := func(idx int) {
		if idx >= 0 && idx < 31 {
			srcs = append(srcs, idx)
		}
	}

	op := inst.Op

	if emu.IsExclusiveStore(op) {
		add(ArgGPR(inst.Args[1]))
		ExtractMemSrcGPRs(inst.Args[2], add)
		return srcs
	}
	if emu.IsPointerStrip(op) {
		add(ArgGPR(inst.Args[0]))
		return srcs
	}
	if emu.IsAtomicOperation(op) {
		add(ArgGPR(inst.Args[0]))
		if op == emu.OpCAS {
			add(ArgGPR(inst.Args[1]))
		}
		ExtractMemSrcGPRs(inst.Args[2], add)
		return srcs
	}

	// For stores: Args[0] is the value, Args[1] is the address.
	if IsStore(op) {
		add(ArgGPR(inst.Args[0]))
		ExtractMemSrcGPRs(inst.Args[1], add)
		// STP also stores Args[1] as a second value, address in Args[2].
		if op == arm64asm.STNP || op == arm64asm.STP {
			add(ArgGPR(inst.Args[1]))
			ExtractMemSrcGPRs(inst.Args[2], add)
		}
		return srcs
	}

	// For loads: source is the memory operand (base/index registers).
	if IsLoad(op) {
		// LDP: Args[0] and Args[1] are destinations, Args[2] is memory.
		if op == arm64asm.LDNP || op == arm64asm.LDP || op == arm64asm.LDPSW {
			ExtractMemSrcGPRs(inst.Args[2], add)
		} else {
			ExtractMemSrcGPRs(inst.Args[1], add)
		}
		return srcs
	}

	// For branches that read registers.
	if IsBranch(op) {
		switch op {
		case arm64asm.BR, arm64asm.BLR, arm64asm.RET:
			if inst.Args[0] != nil {
				add(ArgGPR(inst.Args[0]))
			} else if op == arm64asm.RET {
				add(30) // default: X30
			}
		case arm64asm.CBZ, arm64asm.CBNZ:
			add(ArgGPR(inst.Args[0]))
		case arm64asm.TBZ, arm64asm.TBNZ:
			add(ArgGPR(inst.Args[0]))
		}
		return srcs
	}

	// Bitfield insertion and MOVK preserve part of the previous destination.
	switch op {
	case arm64asm.MOVK:
		add(ArgGPR(inst.Args[0]))
	case arm64asm.BFI, arm64asm.BFM, arm64asm.BFXIL:
		bits := uint32(64)
		if r, ok := inst.Args[0].(arm64asm.Reg); ok && r >= arm64asm.W0 && r <= arm64asm.WZR {
			bits = 32
		}
		first, a := inst.Args[2].(arm64asm.Imm)
		second, b := inst.Args[3].(arm64asm.Imm)
		full := bits
		if op == arm64asm.BFM {
			full--
		}
		if !a || !b || first.Imm != 0 || second.Imm != full {
			add(ArgGPR(inst.Args[0]))
		}
	}
	// General case: Args[1..] are sources.
	for i := 1; i < len(inst.Args); i++ {
		if inst.Args[i] == nil {
			break
		}
		ExtractArgGPRs(inst.Args[i], add)
	}

	return srcs
}

// SourceFPs returns all FP register indices read by the instruction.
func SourceFPs(inst arm64asm.Inst) []int {
	var srcs []int
	add := func(idx int) {
		if idx >= 0 {
			srcs = append(srcs, idx)
		}
	}

	if IsStore(inst.Op) {
		add(ArgFP(inst.Args[0]))
		if inst.Op == arm64asm.STNP || inst.Op == arm64asm.STP {
			add(ArgFP(inst.Args[1]))
		}
		return srcs
	}

	// Loads read from memory addresses (GPRs), not FP registers.
	if IsLoad(inst.Op) {
		return srcs
	}

	// Vector BIC immediate preserves all destination bits outside its mask.
	if inst.Op == arm64asm.BIC && inst.Enc&0xbff80c00 == 0x2f000400 {
		add(ArgFP(inst.Args[0]))
	}
	if _, partial := inst.Args[0].(arm64asm.RegisterWithArrangementAndIndex); partial {
		add(ArgFP(inst.Args[0]))
	}
	for i := 1; i < len(inst.Args); i++ {
		if inst.Args[i] == nil {
			break
		}
		add(ArgFP(inst.Args[i]))
	}
	return srcs
}

func ExtractMemSrcGPRs(arg arm64asm.Arg, add func(int)) {
	if arg == nil {
		return
	}
	switch m := arg.(type) {
	case arm64asm.MemImmediate:
		r := MemImmBase(m)
		add(GprIdx(r))
	case arm64asm.MemExtend:
		add(GprIdx(arm64asm.Reg(m.Base)))
		add(GprIdx(m.Index))
	}
}

func ExtractArgGPRs(arg arm64asm.Arg, add func(int)) {
	switch a := arg.(type) {
	case arm64asm.Reg:
		add(GprIdx(a))
	case arm64asm.RegSP:
		add(ArgGPR(a))
	case arm64asm.MemImmediate:
		r := MemImmBase(a)
		add(GprIdx(r))
	case arm64asm.MemExtend:
		add(GprIdx(arm64asm.Reg(a.Base)))
		add(GprIdx(a.Index))
	case arm64asm.RegExtshiftAmount:
		r := RegExtReg(a)
		add(GprIdx(r))
	}
}
