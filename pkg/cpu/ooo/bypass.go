package ooo

import (
	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

// BypassState times the exact GPR store-to-load path as a prediction, with
// Spec.BypassRecovery. The prediction pairs a load with the youngest in-flight
// store that names the same address operand: the same base register and
// immediate displacement, and the same width. Nothing tracks whether the base
// register changed in between, so the named store may have written another
// address, or an unnamed younger store may supply the load. The value the
// named store wrote is then delivered in place of the load's. Only a
// different value exposes the error, and the load's PC then goes without the
// bypass for BypassRecovery executions: a store that supplies it does so in
// Spec.PlainForwardLatency cycles from its data.
//
// The evidence is on the M5 Super core. A same-address increment through a
// second base register takes 7.02 cycles per update against 1.67 through one
// register. The Nussinov kernel with its destination table preset takes 1.8
// cycles per inner iteration when all destinations hold one value and 7.1 when
// consecutive destinations differ, with identical instructions, addresses and
// source values, and 1.4 without the inner store. See
// evaluation/model-profile.md.
type BypassState struct {
	// Distrust maps a load's PC to the executions left without the bypass.
	Distrust map[uint64]int `json:",omitempty"`
}

// addressOperand names the address of an immediate-offset GPR LDR or STR
// without writeback by its base register and displacement.
type addressOperand struct {
	Base  arm64asm.RegSP
	Imm   int32
	Valid bool
}

func namedAddress(inst arm64asm.Inst) addressOperand {
	if !directGPRTransfer(inst) {
		return addressOperand{}
	}
	address := inst.Args[1].(arm64asm.MemImmediate)
	// LDR/STR (immediate, unsigned offset) scale a 12-bit field by the width.
	return addressOperand{Base: address.Base, Imm: int32(inst.Enc>>10&0xfff) << (inst.Enc >> 30), Valid: true}
}

// transferValue is the value an eligible load read or an eligible store
// wrote, after the instruction executed.
func (p *middleware) transferValue(inst arm64asm.Inst, size int) uint64 {
	reg := inst.Args[0].(arm64asm.Reg)
	index := int(reg - arm64asm.X0)
	if reg >= arm64asm.W0 && reg <= arm64asm.W30 {
		index = int(reg - arm64asm.W0)
	}
	value := p.comp.Resources.backend().CPURegisters().ReadX(index)
	if size == 4 {
		value = uint64(uint32(value))
	}
	return value
}

// bypassLoad applies the prediction to a load that forwardLoad timed.
func (p *middleware) bypassLoad(inst arm64asm.Inst, pc uint64, accesses []emu.MemAccess, issueCycle uint64, timed loadForwarding) loadForwarding {
	operand := namedAddress(inst)
	if !operand.Valid || len(accesses) != 1 || (accesses[0].Size != 4 && accesses[0].Size != 8) {
		return timed
	}
	acc := accesses[0]
	supplier := p.youngestOverlap(acc)
	var named *storeBufferEntry
	for i := range p.comp.State.StoreBuffer {
		sb := &p.comp.State.StoreBuffer[i]
		if sb.Operand == operand && sb.Size == acc.Size && sb.CompleteCycle > issueCycle {
			named = sb
		}
	}
	state := &p.comp.State.Bypass
	distrusted := state.Distrust[pc] > 0
	if distrusted {
		if state.Distrust[pc]--; state.Distrust[pc] == 0 {
			delete(state.Distrust, pc)
		}
	} else if named != nil && named != supplier && named.Value != p.transferValue(inst, acc.Size) {
		if state.Distrust == nil {
			state.Distrust = map[uint64]int{}
		}
		state.Distrust[pc] = p.comp.Spec.BypassRecovery
	}
	exact := timed.forwarded && timed.latency == exactGPRForwardLatency
	if exact && (distrusted || supplier != named) {
		return loadForwarding{issueCycle: issueCycle, dataReady: supplier.ReadyCycle, latency: uint64(p.comp.Spec.PlainForwardLatency), forwarded: true}
	}
	return timed
}
