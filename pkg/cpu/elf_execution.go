package cpu

import (
	"github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/linuxuser"
	"golang.org/x/arch/arm64/arm64asm"
)

// NewELFTimingExecution gives one checked process a per-core lookup table for
// the static dependencies used by the OoO scoreboard.
func NewELFTimingExecution(process *linuxuser.Execution) emu.Execution {
	if process == nil {
		return nil
	}
	return &elfTimingExecution{Execution: process}
}

type elfTimingExecution struct {
	*linuxuser.Execution
	sources [256]struct {
		word  uint32
		valid bool
		gprs  []int
		fps   []int
	}
}

func (x *elfTimingExecution) PeekSources(pc uint64) (arm64asm.Inst, []int, []int, error) {
	instruction, err := x.Peek(pc)
	if err != nil {
		return instruction, nil, nil, err
	}
	// Linux user Peek decodes the currently executable bytes. Even when code
	// at pc is replaced, the complete encoding identifies its register sources.
	word := instruction.Enc
	slot := &x.sources[(word*0x9e3779b9)>>24]
	if !slot.valid || slot.word != word {
		slot.gprs = inst.SourceGPRs(instruction)
		slot.fps = inst.SourceFPs(instruction)
		slot.word = word
		slot.valid = true
	}
	return instruction, slot.gprs, slot.fps, nil
}
