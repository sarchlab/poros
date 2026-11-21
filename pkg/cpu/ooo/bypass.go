package ooo

import (
	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

// BypassState qualifies the exact GPR store-to-load path of forwardLoad, with
// Spec.BypassRecovery. Two conditions apply to a load the path would serve.
//
// The store must name the address as the load does: the same base register,
// immediate displacement and width. A store through another register
// supplies the load in Spec.PlainForwardLatency cycles from its data.
//
// The load's PC must not have broken a run of equal values. Each PC remembers
// the value it last read and for how many consecutive executions. Reading
// another value after Spec.BypassValueRun equal ones takes the path away from
// that PC for its next BypassRecovery executions; a store then supplies it
// with the plain forward latency as well. A load whose value changes at
// every execution never builds a run and keeps the path.
//
// The evidence is on the M5 Super core. A same-address increment takes 1.67
// cycles per update through one base register and 7.02 through a second. A
// running maximum that reloads one value for 16 to 256 updates and then
// another takes about eight cycles per update instead of two, also when the
// address never changes and when a store through the same register precedes
// every run. See evaluation/model-profile.md.
type BypassState struct {
	// Loads holds the value history of each eligible load, by PC.
	Loads map[uint64]valueHistory `json:",omitempty"`
}

// valueHistory is what one load PC last read.
type valueHistory struct {
	Value uint64
	// Run counts the consecutive executions that read Value, up to
	// Spec.BypassValueRun.
	Run int
	// Distrust is the number of executions left without the bypass.
	Distrust int `json:",omitempty"`
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

// loadedValue is the value an eligible load read, after it executed.
func (p *middleware) loadedValue(inst arm64asm.Inst, size int) uint64 {
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

// bypassLoad records the value of an eligible load and decides whether the
// exact path that forwardLoad chose still serves it.
func (p *middleware) bypassLoad(inst arm64asm.Inst, pc uint64, accesses []emu.MemAccess, issueCycle uint64, timed loadForwarding) loadForwarding {
	operand := namedAddress(inst)
	if !operand.Valid || len(accesses) != 1 || (accesses[0].Size != 4 && accesses[0].Size != 8) {
		return timed
	}
	state := &p.comp.State.Bypass
	if state.Loads == nil {
		state.Loads = map[uint64]valueHistory{}
	}
	history := state.Loads[pc]
	value := p.loadedValue(inst, accesses[0].Size)
	distrusted := history.Distrust > 0
	if distrusted {
		history.Distrust--
	} else if history.Run >= p.comp.Spec.BypassValueRun && value != history.Value {
		history.Distrust = p.comp.Spec.BypassRecovery
	}
	if history.Run > 0 && value == history.Value {
		history.Run = min(history.Run+1, p.comp.Spec.BypassValueRun)
	} else {
		history.Value, history.Run = value, 1
	}
	state.Loads[pc] = history
	if timed.forwarded && timed.latency == exactGPRForwardLatency {
		if supplier := p.youngestOverlap(accesses[0]); distrusted || supplier.Operand != operand {
			return loadForwarding{issueCycle: issueCycle, dataReady: supplier.ReadyCycle, latency: uint64(p.comp.Spec.PlainForwardLatency), forwarded: true}
		}
	}
	return timed
}
