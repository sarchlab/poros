package ooo

import (
	"math"

	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

// replayL1Read checks the load-pipe reservation already made for the first
// attempt. A conflict consumes that reservation. The next attempt requires a
// new load pipe once the L1 lookup latency has elapsed.
func (p *middleware) replayL1Read(inst arm64asm.Inst, accesses []emu.MemAccess, issue uint64) uint64 {
	cache := p.comp.Resources.Cache
	if cache == nil || (inst.Op != arm64asm.LDR && inst.Op != arm64asm.LDUR) || len(accesses) != 1 {
		return issue
	}
	if mem, ok := inst.Args[1].(arm64asm.MemImmediate); ok && mem.Mode != arm64asm.AddrOffset {
		return issue
	}
	acc := accesses[0]
	if acc.Size != 8 || acc.Addr%8 != 0 || !cache.L1Ready(acc.Addr, issue) {
		return issue
	}

	lineBytes := uint64(cache.Config().L1LineSize)
	if acc.Addr%lineBytes+8 > lineBytes {
		return issue
	}
	period := uint64(p.comp.Spec.L1ReadBankPeriod)
	latency := uint64(cache.Config().L1Latency)
	issues := p.comp.State.L1ReadIssues
	if issues == nil {
		issues = make(map[uint64][]uint64)
		p.comp.State.L1ReadIssues = issues
	}
	for {
		conflict := false
		for _, other := range issues[issue] {
			if other%period == acc.Addr%period && other/lineBytes != acc.Addr/lineBytes {
				conflict = true
				break
			}
		}
		if !conflict {
			issues[issue] = append(issues[issue], acc.Addr)
			return issue
		}
		if issue >= math.MaxUint64-latency {
			panic("ooo: L1 read replay exceeds the cycle range")
		}
		issue = p.reserveIssue(unitLoad, issue+latency, 1)
	}
}
