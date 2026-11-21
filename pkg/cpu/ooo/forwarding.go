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
				result.issueCycle = max(result.issueCycle, sb.CompleteCycle, sb.PriorCompleteCycle)
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

// A direct transfer is an ordinary LDR or STR of a double-precision
// register with immediate-offset addressing and no base writeback.
func scalarFPOffset(inst arm64asm.Inst, op arm64asm.Op) bool {
	if inst.Op != op {
		return false
	}
	reg, ok := inst.Args[0].(arm64asm.Reg)
	address, immediate := inst.Args[1].(arm64asm.MemImmediate)
	return ok && reg >= arm64asm.D0 && reg <= arm64asm.D31 && immediate && address.Mode == arm64asm.AddrOffset
}

func (p *middleware) forwardAdjacentFP(inst arm64asm.Inst, accesses []emu.MemAccess, issue uint64) (loadForwarding, bool) {
	latency := p.comp.Spec.AdjacentFPForwardLatency
	if latency == 0 || len(accesses) != 1 || accesses[0].Size != 8 || !scalarFPOffset(inst, arm64asm.LDR) {
		return loadForwarding{}, false
	}
	store := p.youngestOverlap(accesses[0])
	position := p.comp.State.InstDone + uint64(p.comp.State.RobCount)
	if store == nil || store.NextFPReload != position || position == 0 ||
		store.Addr != accesses[0].Addr || store.Size != 8 || store.CompleteCycle <= issue {
		return loadForwarding{}, false
	}
	return loadForwarding{issueCycle: issue, dataReady: store.ReadyCycle, latency: uint64(latency), forwarded: true}, true
}

// suppliedLoad is the single forwarding path of Spec.StoreForwardLatency. It
// takes each access on its own: the youngest in-flight store that overlaps it
// must hold all its bytes, or no store may overlap it and the cache supplies
// it. It returns when the data of the supplying stores has issued, the
// accesses left to the cache (none when stores supply every access), and
// whether the path applies. It does not apply when the path is off, when no
// store supplies an access, or when a store overlaps an access in part:
// forwardLoad then decides as before, and the list returned is every access.
func (p *middleware) suppliedLoad(accesses []emu.MemAccess) (ready uint64, cached []emu.MemAccess, ok bool) {
	if p.comp.Spec.StoreForwardLatency == 0 {
		return 0, accesses, false
	}
	supplied := 0
	for _, acc := range accesses {
		youngest := p.youngestOverlap(acc)
		if youngest == nil {
			continue
		}
		if acc.Addr < youngest.Addr || youngest.Addr+uint64(youngest.Size) < acc.Addr+uint64(acc.Size) {
			return 0, accesses, false
		}
		supplied++
		ready = max(ready, youngest.ReadyCycle)
	}
	switch supplied {
	case 0:
		return 0, accesses, false
	case len(accesses):
		return ready, nil, true
	}
	// Stores supply some accesses: a list of the others, for this load alone.
	for _, acc := range accesses {
		if p.youngestOverlap(acc) == nil {
			cached = append(cached, acc)
		}
	}
	return ready, cached, true
}

// youngestOverlap is the youngest in-flight store with a byte of the access.
func (p *middleware) youngestOverlap(acc emu.MemAccess) *storeBufferEntry {
	var youngest *storeBufferEntry
	for i := range p.comp.State.StoreBuffer {
		sb := &p.comp.State.StoreBuffer[i]
		if acc.Addr < sb.Addr+uint64(sb.Size) && sb.Addr < acc.Addr+uint64(acc.Size) {
			youngest = sb
		}
	}
	return youngest
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

// Forwarding records and write allocation ownership have different lifetimes.
// This pool is explicitly capped from instruction-pair and queued-line-byte
// scales; dispatch stalls when full. Fully shadowed records are compacted, but
// records covered only by several partial stores remain until they drain. The
// cap, not a one-owner-per-byte invariant, bounds future-ready record history.
func (p *middleware) forwardingCapacity() int {
	capacity := 2 * p.comp.Spec.StoreBufferSize
	if cache := p.comp.Resources.Cache; cache != nil {
		cfg := cache.Config()
		capacity += cfg.WriteBufferEntries * cfg.L1LineSize
	}
	return capacity
}

func (p *middleware) appendStoreRecord(entry storeBufferEntry) {
	kept := p.comp.State.StoreBuffer[:0]
	for _, old := range p.comp.State.StoreBuffer {
		if entry.Addr <= old.Addr && old.Addr+uint64(old.Size) <= entry.Addr+uint64(entry.Size) {
			// Preserve conservative ordering for partial/FP forwarding. The exact
			// youngest GPR bypass still uses only the latest producer's readiness.
			entry.PriorCompleteCycle = max(entry.PriorCompleteCycle, old.CompleteCycle, old.PriorCompleteCycle)
			entry.DrainCycle = max(entry.DrainCycle, old.DrainCycle, old.CompleteCycle, old.PriorCompleteCycle)
		} else {
			kept = append(kept, old)
		}
	}
	p.comp.State.StoreBuffer = append(kept, entry)
}

// storeTiming hands the current store instruction's bytes to the cache. A
// backend may record one store as several accesses (the registers of a pair;
// the Linux process backend also splits at word boundaries), so accesses that
// continue one another are one store: the cache classifies its whole range
// once, and each access keeps its own translation, line allocations and
// forwarding record.
func (p *middleware) storeTiming(issue uint64) (accepted, drained uint64) {
	cache := p.comp.Resources.Cache
	if cache == nil {
		return
	}
	accesses := p.comp.Resources.backend().MemoryAccesses()
	for len(accesses) > 0 {
		n, end := 1, accesses[0].Addr+uint64(accesses[0].Size)
		for n < len(accesses) && accesses[n].Addr == end {
			end += uint64(accesses[n].Size)
			n++
		}
		class := cache.ClassifyStore(accesses[0].Addr, int(end-accesses[0].Addr))
		for _, acc := range accesses[:n] {
			// The cache accepts the bytes once their address is translated.
			a, d := cache.StorePart(class, acc.Addr, acc.Size, cache.Translate(acc.Addr, acc.Size, issue))
			accepted, drained = max(accepted, a), max(drained, d)
		}
		accesses = accesses[n:]
	}
	return
}
