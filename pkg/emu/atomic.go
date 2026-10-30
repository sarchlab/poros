package emu

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/arch/arm64/arm64asm"
	"strings"
)

// OpCAS is in Poros's reserved 0xff00..0xffff extension range. x/arch v0.24
// has no FEAT_LSE decoder. Never assign an unrelated base opcode to an RMW.
const (
	OpCAS   arm64asm.Op = 0xff00
	OpSWP   arm64asm.Op = 0xff01
	OpLDADD arm64asm.Op = 0xff04
)

func IsAtomicOperation(op arm64asm.Op) bool { return op == OpCAS || op == OpSWP || op == OpLDADD }

// AtomicAlignmentPolicy describes effective hardware/OS alignment behavior.
// It covers atomic RMW and acquire/release single-register accesses.
// The default is natural alignment; Darwin may supply a measured 16-byte policy.
type AtomicAlignmentPolicy uint8

const (
	AtomicNaturalAlignment AtomicAlignmentPolicy = iota
	AtomicWithin16Bytes
)

// AtomicMemory performs an indivisible single-thread compare/exchange. It must
// check read AND write permissions even on mismatch. Architectural alignment
// policy is checked by the executor before entering the memory operation.
// Errors leave guest memory and old unchanged; buffers must not be retained.
type AtomicMemory interface {
	CompareExchange(address uint64, expected, replacement, old []byte) (bool, error)
	Exchange(address uint64, replacement, old []byte) error
	FetchAdd(address uint64, increment, old []byte) error
}

func decodeAtomic(word uint32) (arm64asm.Inst, bool) {
	// CAS, SWP and LDADD ordering variants, 32/64-bit only.
	// Byte/halfword forms, CASP and other LSE operations are unsupported.
	op := OpCAS
	if word&0xbf20fc00 == 0xb8208000 {
		op = OpSWP
	} else if word&0xbf20fc00 == 0xb8200000 {
		op = OpLDADD
	} else if word&0xbfa07c00 != 0x88a07c00 {
		return arm64asm.Inst{}, false
	}
	base := arm64asm.W0
	if word>>30 == 3 {
		base = arm64asm.X0
	}
	return arm64asm.Inst{Op: op, Enc: word, Args: arm64asm.Args{
		base + arm64asm.Reg((word>>16)&31), base + arm64asm.Reg(word&31),
		arm64asm.MemImmediate{Base: arm64asm.RegSP(arm64asm.X0 + arm64asm.Reg((word>>5)&31)), Mode: arm64asm.AddrOffset},
	}}, true
}

// InstructionString preserves extension mnemonics in diagnostics and traces.
func InstructionString(inst arm64asm.Inst) string {
	if inst.Op == OpSB {
		return "SB"
	}
	if inst.Op == arm64asm.MSR && (inst.Enc == 0xd503405f || inst.Enc == 0xd503415f) {
		return fmt.Sprintf("MSR DIT, #%d", (inst.Enc>>8)&1)
	}
	if inst.Op == OpLDAPR {
		return "LDAPR " + inst.Args[0].String() + ", " + inst.Args[1].String()
	}
	if IsPointerStrip(inst.Op) {
		name := "XPACI"
		if inst.Op == OpXPACD {
			name = "XPACD"
		}
		return name + " " + inst.Args[0].String()
	}
	if !IsAtomicOperation(inst.Op) {
		return inst.String()
	}
	name := "CAS"
	acquire, release := uint32(1<<22), uint32(1<<15)
	if inst.Op == OpSWP || inst.Op == OpLDADD {
		name = "SWP"
		if inst.Op == OpLDADD {
			name = "LDADD"
		}
		acquire = 1 << 23
		release = 1 << 22
	}
	if inst.Enc&acquire != 0 {
		name += "A"
	}
	if inst.Enc&release != 0 {
		name += "L"
	}
	var args []string
	for _, a := range inst.Args {
		if a == nil {
			break
		}
		args = append(args, a.String())
	}
	return name + " " + strings.Join(args, ", ")
}
func (e *executor) execAtomic(inst arm64asm.Inst) error {
	memory, ok := e.checked.(AtomicMemory)
	if !ok {
		return &UnsupportedInstructionError{Instruction: inst}
	}
	word := inst.Enc
	size := 4
	if word>>30 == 3 {
		size = 8
	}
	rs, rt, rn := int((word>>16)&31), int(word&31), int((word>>5)&31)
	addr := e.regs.ReadX(rn)
	if rn == 31 {
		addr = e.regs.SP
	}
	if err := e.checkAtomicAlignment(addr, size); err != nil {
		return err
	}
	var expected, replacement, old [8]byte
	binary.LittleEndian.PutUint64(expected[:], e.regs.ReadX(rs))
	binary.LittleEndian.PutUint64(replacement[:], e.regs.ReadX(rt))
	var err error
	if inst.Op == OpSWP {
		err = memory.Exchange(addr, expected[:size], old[:size])
		rs = rt
	} else if inst.Op == OpLDADD {
		err = memory.FetchAdd(addr, expected[:size], old[:size])
		rs = rt
	} else {
		_, err = memory.CompareExchange(addr, expected[:size], replacement[:size], old[:size])
	}
	if err != nil {
		return &memoryAccessError{err}
	}
	if e.regs.Exclusive.Policy == ExclusiveM2Local64 {
		e.regs.ClearExclusive()
	} // LSE clears the measured M2 local monitor even on comparison mismatch.
	value := binary.LittleEndian.Uint64(old[:])
	e.regs.WriteX(rs, value)
	return nil
}

func (e *executor) checkAtomicAlignment(addr uint64, size int) error {
	switch e.regs.AtomicAlignment {
	case AtomicNaturalAlignment:
		if addr%uint64(size) != 0 {
			return &memoryAccessError{fmt.Errorf("unaligned atomic address %#x", addr)}
		}
	case AtomicWithin16Bytes:
		if addr%16+uint64(size) > 16 {
			return &memoryAccessError{fmt.Errorf("atomic access crosses 16-byte granule")}
		}
	default:
		return fmt.Errorf("unsupported atomic alignment policy")
	}
	return nil
}
