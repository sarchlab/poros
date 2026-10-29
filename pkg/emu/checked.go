package emu

import (
	"errors"
	"fmt"

	"golang.org/x/arch/arm64/arm64asm"
)

// CheckedMemory is the process execution boundary. Each operation must validate
// the complete range before transferring bytes; failed writes must be atomic.
// Fetch enforces execute permission independently of Read. Implementations must
// not call back into the executor, modify the register file, or retain slices;
// transfer buffers are ephemeral and may be reused by the next instruction.
type CheckedMemory interface {
	Read(uint64, []byte) error
	Write(uint64, []byte) error
	Fetch(uint64, []byte) error
}

// StepResult exposes the decoded instruction and control-flow event. Syscall
// leaves PC/registers unchanged for the process owner to handle and commit.
type StepResult struct {
	Instruction arm64asm.Inst
	// Branched is true when committed PC differs from prior PC+4, matching
	// StepDetailed. A branch targeting its own fallthrough reports false.
	Branched bool
	Syscall  bool
}

// UnsupportedInstructionError identifies a decoded operation without semantics.
type UnsupportedInstructionError struct{ Instruction arm64asm.Inst }

func (e *UnsupportedInstructionError) Error() string {
	return fmt.Sprintf("unimplemented instruction: %s", e.Instruction)
}

type memoryAccessError struct{ error }

func (e *memoryAccessError) Unwrap() error { return e.error }

// StepError distinguishes decode, unsupported and memory faults at a guest PC.
type StepError struct {
	PC   uint64
	Kind string
	Err  error
}

func (e *StepError) Error() string { return fmt.Sprintf("guest %s at PC=%#x: %v", e.Kind, e.PC, e.Err) }
func (e *StepError) Unwrap() error { return e.Err }

// StepChecked executes one instruction using the existing ISA semantics. A
// fault leaves registers (including writeback, FP and flags) unchanged. Stores
// transfer their entire instruction span through one atomic Write. No legacy
// function hooks, synthetic return address, or implicit guest-memory allocation is involved.
func StepChecked(regs *Registers, memory CheckedMemory) (StepResult, error) {
	var result StepResult
	fail := func(kind string, err error) (StepResult, error) { return result, &StepError{regs.PC, kind, err} }
	if regs.PC%4 != 0 {
		return fail("memory_fault", fmt.Errorf("unaligned instruction address"))
	}
	var bytes [4]byte
	if err := memory.Fetch(regs.PC, bytes[:]); err != nil {
		return fail("memory_fault", err)
	}
	inst, err := arm64asm.Decode(bytes[:])
	if err != nil {
		return fail("unsupported_instruction", err)
	}
	result.Instruction = inst
	if inst.Op == arm64asm.SVC {
		if bytes != [4]byte{1, 0, 0, 0xd4} {
			return fail("unsupported_instruction", fmt.Errorf("only SVC #0 is supported: %s", inst))
		}
		result.Syscall = true
		return result, nil
	}
	// Scalar semantics do not implement SIMD arrangements or element operands.
	// Reject these before handlers that assume scalar register forms.
	for _, arg := range inst.Args {
		switch a := arg.(type) {
		case arm64asm.RegisterWithArrangement, arm64asm.RegisterWithArrangementAndIndex:
			return fail("unsupported_instruction", fmt.Errorf("SIMD operand in %s", inst))
		case arm64asm.Reg:
			if a >= arm64asm.B0 && !checkedFPRegisterSupported(inst.Op, a) {
				return fail("unsupported_instruction", fmt.Errorf("unsupported scalar/SIMD register form: %s", inst))
			}
		}
	}
	if inst.Op == arm64asm.HINT {
		return fail("unsupported_instruction", fmt.Errorf("unsupported hint: %s", inst))
	}
	// Scalar FP/integer conversions also have AdvSIMD scalar-register and
	// fixed-point encodings. The legacy handlers only implement unscaled GPR
	// <-> S/D forms; reject other forms instead of their silent fallthrough.
	switch inst.Op {
	case arm64asm.SCVTF, arm64asm.UCVTF, arm64asm.FCVTZS, arm64asm.FCVTZU:
		dst, dok := inst.Args[0].(arm64asm.Reg)
		src, sok := inst.Args[1].(arm64asm.Reg)
		isInt := func(r arm64asm.Reg) bool { return xRegIndex(r) >= 0 || wRegIndex(r) >= 0 }
		isFP := func(r arm64asm.Reg) bool { return dRegIndex(r) >= 0 || sRegIndex(r) >= 0 }
		valid := dok && sok && inst.Args[2] == nil
		if inst.Op == arm64asm.SCVTF || inst.Op == arm64asm.UCVTF {
			valid = valid && isFP(dst) && isInt(src)
		} else {
			valid = valid && isInt(dst) && isFP(src)
		}
		if !valid {
			return fail("unsupported_instruction", &UnsupportedInstructionError{Instruction: inst})
		}
	}
	next := *regs
	ex := executor{regs: &next, checked: memory}
	branched, err := ex.execute(inst)
	if err != nil {
		var access *memoryAccessError
		if errors.As(err, &access) {
			return fail("memory_fault", err)
		}
		var unsupported *UnsupportedInstructionError
		if errors.As(err, &unsupported) {
			return fail("unsupported_instruction", err)
		}
		return fail("execution_fault", err)
	}
	if !branched {
		next.PC += 4
	}
	result.Branched = next.PC != regs.PC+4
	*regs = next
	return result, nil
}

// AdvSIMD scalar integer forms (for example ADD Dd,Dn,Dm) use plain
// registers, not arrangement operands. Only explicitly implemented FP and
// memory operations may reach handlers with an FP/SIMD register operand.
func checkedFPRegisterSupported(op arm64asm.Op, r arm64asm.Reg) bool {
	switch op {
	case arm64asm.LDR, arm64asm.LDUR, arm64asm.LDP, arm64asm.STR, arm64asm.STUR, arm64asm.STP:
		return sRegIndex(r) >= 0 || dRegIndex(r) >= 0 || qRegIndex(r) >= 0
	case arm64asm.FADD, arm64asm.FSUB, arm64asm.FMUL, arm64asm.FDIV,
		arm64asm.FMADD, arm64asm.FMSUB, arm64asm.FSQRT, arm64asm.FNEG,
		arm64asm.FABS, arm64asm.FNMUL, arm64asm.FNMADD, arm64asm.MOVI,
		arm64asm.FMOV, arm64asm.FCMP, arm64asm.FCCMP, arm64asm.FMAXNM,
		arm64asm.FMINNM, arm64asm.FCSEL, arm64asm.SCVTF, arm64asm.UCVTF,
		arm64asm.FCVT, arm64asm.FCVTZS, arm64asm.FCVTZU:
		return sRegIndex(r) >= 0 || dRegIndex(r) >= 0
	default:
		return false
	}
}
