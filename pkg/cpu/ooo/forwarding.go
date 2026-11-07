package ooo

import (
	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

const (
	conventionalForwardLatency = 5
	// This one-cycle exact GPR path is a timing-model hypothesis. It does not
	// identify a physical M2 pipeline stage; see evaluation/model-profile.md.
	exactGPRForwardLatency = 1
)

type loadForwarding struct {
	issueCycle uint64
	dataReady  uint64
	latency    uint64
	forwarded  bool
}

// forwardLoad preserves conventional overlap ordering unless every access can
// take the exact GPR path. A younger overlapping store always shadows an older
// match, including when its width makes it ineligible for the fast path.
func (p *middleware) forwardLoad(accesses []emu.MemAccess, issueCycle uint64, eligibleLoad bool) loadForwarding {
	result := loadForwarding{issueCycle: issueCycle, latency: conventionalForwardLatency, forwarded: len(accesses) > 0}
	allExact := len(accesses) == 1 && eligibleLoad
	var dataReady uint64
	for _, acc := range accesses {
		covered := false
		var youngest *storeBufferEntry
		for i := range p.comp.State.StoreBuffer {
			sb := &p.comp.State.StoreBuffer[i]
			if acc.Addr < sb.Addr+uint64(sb.Size) && sb.Addr < acc.Addr+uint64(acc.Size) {
				youngest = sb
				if sb.CompleteCycle > result.issueCycle {
					result.issueCycle = sb.CompleteCycle
				}
				if sb.Addr <= acc.Addr && acc.Addr+uint64(acc.Size) <= sb.Addr+uint64(sb.Size) {
					covered = true
				}
			}
		}
		result.forwarded = result.forwarded && covered
		if youngest == nil || !youngest.FastForwardEligible || youngest.Addr != acc.Addr ||
			youngest.Size != acc.Size || (acc.Size != 4 && acc.Size != 8) || youngest.CompleteCycle <= issueCycle {
			allExact = false
		} else if youngest.ReadyCycle > dataReady {
			dataReady = youngest.ReadyCycle
		}
	}
	if allExact {
		// Address/unit readiness still controls load issue. Its result waits for
		// the supplying store's data, but not the store's completion delay.
		result.issueCycle = issueCycle
		result.dataReady = dataReady
		result.latency = exactGPRForwardLatency
	}
	return result
}

// directGPRTransfer restricts the fast hypothesis to the measured instruction
// class. Both producer and consumer must use scalar 32/64-bit immediate-offset
// LDR/STR without writeback. Register-offset, pair and unscaled forms retain
// conventional timing; see evaluation/microbenchmarks/forwarding/README.md.
func directGPRTransfer(inst arm64asm.Inst) bool {
	if inst.Op != arm64asm.LDR && inst.Op != arm64asm.STR {
		return false
	}
	reg, ok := inst.Args[0].(arm64asm.Reg)
	if !ok || !((reg >= arm64asm.W0 && reg <= arm64asm.W30) || (reg >= arm64asm.X0 && reg <= arm64asm.X30)) {
		return false
	}
	address, ok := inst.Args[1].(arm64asm.MemImmediate)
	return ok && address.Mode == arm64asm.AddrOffset
}
