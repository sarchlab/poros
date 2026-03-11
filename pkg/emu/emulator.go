package emu

import (
	"fmt"

	"golang.org/x/arch/arm64/arm64asm"
)

// Emulator is an ARM64 functional emulator.
type Emulator struct {
	Regs      *Registers
	Mem       *Memory
	exec      executor
	InstCount uint64
	MaxInst   uint64 // Max instructions before stopping (default 10M)
	haltPC    uint64 // Stop when PC equals this value
	Halted    bool
	FuncHooks map[uint64]func(em *Emulator) // address → Go function hook
}

// NewEmulator creates a new emulator with empty state.
func NewEmulator() *Emulator {
	regs := NewRegisters()
	mem := NewMemory()
	return &Emulator{
		Regs:    regs,
		Mem:     mem,
		exec:    executor{regs: regs, mem: mem},
		MaxInst: 10_000_000,
	}
}

// SetHaltPC sets the PC value that will cause the emulator to stop.
// Typically set to the initial LR value for function-level emulation.
func (em *Emulator) SetHaltPC(pc uint64) {
	em.haltPC = pc
}

// Run executes instructions until a halt condition is met.
// Returns an error if an instruction fails or the instruction limit is reached.
func (em *Emulator) Run() error {
	for em.InstCount < em.MaxInst {
		if em.Regs.PC == em.haltPC && em.InstCount > 0 {
			em.Halted = true
			return nil
		}

		// Fetch
		var buf [4]byte
		for i := range buf {
			buf[i] = em.Mem.ReadU8(em.Regs.PC + uint64(i))
		}

		// Decode
		inst, err := arm64asm.Decode(buf[:])
		if err != nil {
			return fmt.Errorf("decode error at PC=%#x: %w (bytes: %02x %02x %02x %02x)",
				em.Regs.PC, err, buf[0], buf[1], buf[2], buf[3])
		}

		// BL intercept: check if target is a hooked function.
		// The hook simulates the entire call+return sequence. LR is set
		// temporarily for the hook's benefit, then restored to preserve
		// the caller's link register.
		if inst.Op == arm64asm.BL && em.FuncHooks != nil {
			target := blTarget(em.Regs.PC, inst)
			if hook, ok := em.FuncHooks[target]; ok {
				retAddr := em.Regs.PC + 4
				savedLR := em.Regs.ReadX(30)
				em.Regs.WriteX(30, retAddr) // LR = PC + 4 (visible to hook)
				hook(em)
				em.Regs.WriteX(30, savedLR) // restore caller's LR
				em.Regs.PC = retAddr        // PC = return address
				em.InstCount++
				continue
			}
		}

		// Execute
		branched, err := em.exec.execute(inst)
		if err != nil {
			return fmt.Errorf("execute error at PC=%#x (%s): %w",
				em.Regs.PC, inst, err)
		}

		em.InstCount++

		// Advance PC if not branched
		if !branched {
			em.Regs.PC += 4
		}
	}
	return fmt.Errorf("instruction limit reached (%d)", em.MaxInst)
}

// Step executes a single instruction.
func (em *Emulator) Step() error {
	var buf [4]byte
	for i := range buf {
		buf[i] = em.Mem.ReadU8(em.Regs.PC + uint64(i))
	}

	inst, err := arm64asm.Decode(buf[:])
	if err != nil {
		return fmt.Errorf("decode error at PC=%#x: %w", em.Regs.PC, err)
	}

	branched, err := em.exec.execute(inst)
	if err != nil {
		return fmt.Errorf("execute error at PC=%#x (%s): %w", em.Regs.PC, inst, err)
	}

	em.InstCount++
	if !branched {
		em.Regs.PC += 4
	}
	return nil
}

// StepDetailed executes a single instruction and returns the decoded
// instruction and whether the branch was taken (PC changed to something
// other than PC+4). It also checks the halt condition before executing.
func (em *Emulator) StepDetailed() (arm64asm.Inst, bool, error) {
	em.Mem.ClearAccesses()

	if em.Regs.PC == em.haltPC && em.InstCount > 0 {
		em.Halted = true
		return arm64asm.Inst{}, false, nil
	}

	pcBefore := em.Regs.PC

	var buf [4]byte
	for i := range buf {
		buf[i] = em.Mem.readByteRaw(em.Regs.PC + uint64(i))
	}

	inst, err := arm64asm.Decode(buf[:])
	if err != nil {
		return arm64asm.Inst{}, false, fmt.Errorf(
			"decode error at PC=%#x: %w", em.Regs.PC, err)
	}

	// BL intercept: check if target is a hooked function.
	if inst.Op == arm64asm.BL && em.FuncHooks != nil {
		target := blTarget(em.Regs.PC, inst)
		if hook, ok := em.FuncHooks[target]; ok {
			retAddr := em.Regs.PC + 4
			savedLR := em.Regs.ReadX(30)
			em.Regs.WriteX(30, retAddr) // LR = PC + 4 (visible to hook)
			hook(em)
			em.Regs.WriteX(30, savedLR) // restore caller's LR
			em.Regs.PC = retAddr        // PC = return address
			em.InstCount++

			// Check halt after hook execution.
			if em.Regs.PC == em.haltPC && em.InstCount > 0 {
				em.Halted = true
			}

			branched := em.Regs.PC != pcBefore+4
			return inst, branched, nil
		}
	}

	execBranched, err := em.exec.execute(inst)
	if err != nil {
		return inst, false, fmt.Errorf(
			"execute error at PC=%#x (%s): %w", em.Regs.PC, inst, err)
	}

	em.InstCount++
	if !execBranched {
		em.Regs.PC += 4
	}

	// Check halt after execution so the pipeline can see Halted immediately.
	if em.Regs.PC == em.haltPC && em.InstCount > 0 {
		em.Halted = true
	}

	branched := em.Regs.PC != pcBefore+4
	return inst, branched, nil
}

// RegisterFuncHook registers a Go function to be called when a BL to the
// given address is encountered. The hook reads arguments from registers and
// writes results. After the hook executes, PC is set to LR (X30).
func (em *Emulator) RegisterFuncHook(addr uint64, fn func(em *Emulator)) {
	if em.FuncHooks == nil {
		em.FuncHooks = make(map[uint64]func(em *Emulator))
	}
	em.FuncHooks[addr] = fn
}

// blTarget computes the target address for a BL instruction decoded at the
// given PC.
func blTarget(pc uint64, inst arm64asm.Inst) uint64 {
	offset := int64(inst.Args[0].(arm64asm.PCRel))
	return uint64(int64(pc) + offset)
}

// LoadProgram loads program bytes at the specified address.
func (em *Emulator) LoadProgram(addr uint64, code []byte) {
	em.Mem.Write(addr, code)
}
