package inst

import (
	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

// An execution resource may return the static register dependencies alongside
// the instruction it fetched. Other resources can construct synthetic Inst
// values, so extract their operands directly rather than relying on Enc.
type sourcePeeker interface {
	PeekSources(pc uint64) (arm64asm.Inst, []int, []int, error)
}

func PeekSources(backend emu.Execution, pc uint64) (arm64asm.Inst, []int, []int, error) {
	if p, ok := backend.(sourcePeeker); ok {
		return p.PeekSources(pc)
	}
	inst, err := backend.Peek(pc)
	if err != nil {
		return inst, nil, nil, err
	}
	return inst, SourceGPRs(inst), SourceFPs(inst), nil
}
