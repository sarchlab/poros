package ooo

import "github.com/sarchlab/poros/pkg/emu"

// StoreDependenceState predicts memory dependences by static instruction, as
// store-set predictors do. Each load remembers the static store that supplied
// or ordered it on its previous execution; each static store remembers its
// youngest dispatched instance. A predicted load waits for that instance even
// when the instance writes other bytes, so a run of loads that each depend on
// the previous instance of one store cannot overlap the next run, which reuses
// the same instructions at another address. Only the false dependence adds
// time: a load the predicted instance does supply keeps the forwarding timing.
//
// The evidence is M1 doitgen: its memory-carried accumulations run at the
// single-chain rate of the M1 fp_same and mem_fadd probes (11 cycles per
// update) for chains of 8 to 51 updates instead of overlapping consecutive
// chains; see evaluation/model-profile.md.
type StoreDependenceState struct {
	// LoadStore maps a load instruction's PC to the PC of the store it last
	// overlapped.
	LoadStore map[uint64]uint64
	// Youngest maps a store instruction's PC to its youngest dispatched instance.
	Youngest map[uint64]StoreInstance
}

// StoreInstance is the byte range one dynamic store wrote and its completion.
type StoreInstance struct {
	Addr, End     uint64
	CompleteCycle uint64
}

// predictedStoreWait returns when a load at pc may issue under the
// prediction: no earlier than issueCycle, and no earlier than the completion
// of the predicted store's youngest instance when that instance does not
// overlap any of the load's accesses.
func (p *middleware) predictedStoreWait(pc uint64, accesses []emu.MemAccess, issueCycle uint64) uint64 {
	s := &p.comp.State.StoreDependence
	store, ok := s.LoadStore[pc]
	if !ok {
		return issueCycle
	}
	instance, ok := s.Youngest[store]
	if !ok || instance.CompleteCycle <= issueCycle {
		return issueCycle
	}
	for _, acc := range accesses {
		if acc.Addr < instance.End && instance.Addr < acc.Addr+uint64(acc.Size) {
			return issueCycle // a true dependence; forwarding times it
		}
	}
	return instance.CompleteCycle
}

// trainStoreDependence records the youngest in-flight store the load at pc
// overlaps, or forgets the load's prediction when it overlaps none.
func (p *middleware) trainStoreDependence(pc uint64, accesses []emu.MemAccess) {
	s := &p.comp.State.StoreDependence
	youngest := -1
	for i, sb := range p.comp.State.StoreBuffer {
		for _, acc := range accesses {
			if acc.Addr < sb.Addr+uint64(sb.Size) && sb.Addr < acc.Addr+uint64(acc.Size) {
				youngest = i
			}
		}
	}
	if youngest < 0 {
		delete(s.LoadStore, pc)
		return
	}
	if s.LoadStore == nil {
		s.LoadStore = map[uint64]uint64{}
	}
	s.LoadStore[pc] = p.comp.State.StoreBuffer[youngest].PC
}

// recordStoreInstance makes a dispatched store the youngest instance of pc.
func (p *middleware) recordStoreInstance(pc uint64, accesses []emu.MemAccess, completeCycle uint64) {
	if len(accesses) == 0 {
		return
	}
	instance := StoreInstance{Addr: accesses[0].Addr, End: accesses[0].Addr + uint64(accesses[0].Size), CompleteCycle: completeCycle}
	for _, acc := range accesses[1:] {
		instance.Addr = min(instance.Addr, acc.Addr)
		instance.End = max(instance.End, acc.Addr+uint64(acc.Size))
	}
	s := &p.comp.State.StoreDependence
	if s.Youngest == nil {
		s.Youngest = map[uint64]StoreInstance{}
	}
	s.Youngest[pc] = instance
}
