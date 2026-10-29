package emu

import "golang.org/x/arch/arm64/arm64asm"

// Execution is a live architectural execution resource, shared with a timing
// component. Peek must not change architectural state or recorded data accesses. StepDetailed executes
// the instruction at CPURegisters().PC; failed instructions do not retire.
// An error terminates the driver after older work drains, even if Stopped is false.
// MemoryAccesses contains only the last successful instruction's data accesses.
// Implementations must remain owned by one driver for their entire run.
type Execution interface {
	CPURegisters() *Registers
	Peek(uint64) (arm64asm.Inst, error)
	StepDetailed() (arm64asm.Inst, bool, error)
	MemoryAccesses() []MemAccess
	Stopped() bool
}

func (em *Emulator) CPURegisters() *Registers    { return em.Regs }
func (em *Emulator) MemoryAccesses() []MemAccess { return em.Mem.Accesses }
func (em *Emulator) Stopped() bool               { return em.Halted }
func (em *Emulator) Peek(pc uint64) (arm64asm.Inst, error) {
	var b [4]byte
	for i := range b {
		b[i] = em.Mem.readByteRaw(pc + uint64(i))
	}
	return arm64asm.Decode(b[:])
}
