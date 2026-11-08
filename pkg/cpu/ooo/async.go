package ooo

import (
	"fmt"
	"math"

	"github.com/sarchlab/akita/v5/mem/memprotocol"
	"github.com/sarchlab/akita/v5/sim/messaging"
	"github.com/sarchlab/akita/v5/sim/modeling/ticking"
	"github.com/sarchlab/akita/v5/sim/timing"
	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

// AsyncSpec describes the response-driven CPU. Unlike Comp's analytical path,
// it cannot reserve a load's completion before the memory response arrives.
type AsyncSpec struct {
	Freq             timing.Freq          `json:"freq"`
	FetchWidth       int                  `json:"fetch_width"`
	RetireWidth      int                  `json:"retire_width"`
	ROBCapacity      int                  `json:"rob_capacity"`
	NumINT           int                  `json:"num_int"`
	NumFP            int                  `json:"num_fp"`
	NumLDST          int                  `json:"num_ldst"`
	NumStores        int                  `json:"num_stores"`
	BranchPenalty    int                  `json:"branch_penalty"`
	FpPhysLimit      int                  `json:"fp_phys_limit"`
	StoreBufferSize  int                  `json:"store_buffer_size"`
	BranchPrediction bool                 `json:"branch_prediction"`
	Predictor        PredictorSpec        `json:"predictor"`
	Bottom           messaging.RemotePort `json:"bottom"`
	LineBytes        uint64               `json:"line_bytes"`
}

func DefaultAsyncSpec() AsyncSpec {
	return AsyncSpec{
		Freq: 3490 * timing.MHz, FetchWidth: 8, RetireWidth: 8, ROBCapacity: 256,
		NumINT: 6, NumFP: 4, NumLDST: 4, NumStores: 2, BranchPenalty: 17, FpPhysLimit: 72, StoreBufferSize: 56,
		BranchPrediction: true, Predictor: PredictorSpec{BimodalSize: 4096, TaggedTableSize: 2048, BTBSize: 2048, BTBWays: 4, RASSize: 32, TaggedHistLen: 32},
		Bottom: "L1.Top", LineBytes: 64,
	}
}

func (v AsyncSpec) Validate() error {
	if math.IsNaN(float64(v.Freq)) || math.IsInf(float64(v.Freq), 0) || math.Trunc(float64(v.Freq)) != float64(v.Freq) {
		return fmt.Errorf("CPU frequency must be finite integer Hz")
	}
	if v.FetchWidth < 1 || v.FetchWidth > 64 || v.RetireWidth < 1 || v.RetireWidth > 64 || v.ROBCapacity < 1 || v.ROBCapacity > 4096 || v.NumINT < 1 || v.NumINT > 64 || v.NumFP < 1 || v.NumFP > 64 || v.NumLDST < 1 || v.NumLDST > 64 || v.NumStores < 1 || v.NumStores > 64 || v.FpPhysLimit < 1 || v.FpPhysLimit > 4096 || v.StoreBufferSize < 1 || v.StoreBufferSize > 4096 || v.BranchPenalty < 0 || v.BranchPenalty > 10000 || v.Bottom == "" || v.LineBytes < 16 || v.LineBytes > 4096 || v.LineBytes&(v.LineBytes-1) != 0 || v.Freq < timing.MHz || v.Freq > 1000*timing.GHz {
		return fmt.Errorf("invalid async CPU configuration")
	}
	if v.BranchPrediction && (v.Predictor.BimodalSize < 1 || v.Predictor.BimodalSize > 1<<20 || v.Predictor.TaggedTableSize < 1 || v.Predictor.TaggedTableSize > 1<<20 || v.Predictor.BTBSize < 1 || v.Predictor.BTBSize > 1<<20 || v.Predictor.BTBWays < 1 || v.Predictor.BTBWays > 64 || v.Predictor.BTBSize%v.Predictor.BTBWays != 0 || v.Predictor.RASSize < 1 || v.Predictor.RASSize > 4096 || v.Predictor.TaggedHistLen < 1 || v.Predictor.TaggedHistLen > 63) {
		return fmt.Errorf("invalid async branch predictor")
	}
	return nil
}

type AsyncPorts struct {
	Memory messaging.Port `akita:"role=github.com/sarchlab/akita/v5/mem/memprotocol.requester"`
}
type AsyncMiddlewares struct{ Pipeline *asyncMiddleware }
type AsyncComp = ticking.Component[AsyncSpec, AsyncState, Resources, AsyncPorts, AsyncMiddlewares]

// A dependency names a producer generation, not just a physical ROB slot.
// Base selects the address-update result of a pre/post-indexed instruction.
type dependency struct {
	Tag  uint64
	Base bool
}
type asyncEntry struct {
	Tag                    uint64
	Op                     arm64asm.Op
	Unit                   unitType
	Deps, AddrDeps         []dependency
	Accesses               []emu.MemAccess
	Issued                 bool
	Complete, BaseComplete uint64
	AddressReady           uint64
	BaseDest               bool
	FPDests                int
	Pending                int
	Sent                   int
	ForwardEligible        bool
	Mispredict             bool
	Branch                 bool
}

type asyncStore struct {
	Tag           uint64
	Accesses      []emu.MemAccess
	AddrDeps      []dependency
	Ready         uint64
	AddressReady  uint64
	Retired       bool
	Sent, Pending int
	Eligible      bool
}
type pendingAccess struct {
	Tag       uint64
	Store     bool
	StoreTags []uint64
}

type MemoryCounters struct {
	Reads             uint64 `json:"reads"`
	Writes            uint64 `json:"writes"`
	StoreAccesses     uint64 `json:"store_accesses"`
	CoalescedAccesses uint64 `json:"coalesced_accesses"`
	Responses         uint64 `json:"responses"`
	ForwardedLoads    uint64 `json:"forwarded_loads"`
	MaxOutstanding    uint64 `json:"max_outstanding"`
	SendBlockedCycles uint64 `json:"send_blocked_cycles"`
}

type AsyncState struct {
	Cycles, InstDone          uint64
	Done                      bool
	Cancelled                 bool
	Err                       string
	Rob                       []asyncEntry
	Waiting                   []uint64
	Head, Count               int
	NextTag                   uint64
	GPR, FP                   [32]dependency
	Flags                     dependency
	FpInFlight                int
	Stores                    []asyncStore
	Pending                   map[uint64]pendingAccess
	IntBusy, FpBusy, LdstBusy []uint64
	FrontendReady             uint64
	BlockedBranch             uint64
	Predictor                 PredictorState
	Memory                    MemoryCounters
}

func (s *AsyncState) Initialize(c *AsyncComp) {
	v := c.Spec
	if (c.Resources.Emu == nil) == (c.Resources.Execution == nil) || c.Resources.Cache != nil {
		panic("async CPU: exactly one functional backend and no analytical cache required")
	}
	if c.Resources.Emu != nil && !c.Resources.Emu.Mem.TrackAccesses {
		panic("async CPU: memory access tracking required")
	}
	if err := v.Validate(); err != nil {
		panic(err)
	}
	s.Rob = make([]asyncEntry, v.ROBCapacity)
	s.NextTag = 1
	s.IntBusy = make([]uint64, v.NumINT)
	s.FpBusy = make([]uint64, v.NumFP)
	s.LdstBusy = make([]uint64, v.NumLDST)
	s.Pending = make(map[uint64]pendingAccess)
	if v.BranchPrediction {
		(&predictor{PredictorState: &s.Predictor, spec: v.Predictor}).init()
	}
}
func (m *AsyncMiddlewares) Initialize(c *AsyncComp) { m.Pipeline = &asyncMiddleware{c: c} }
func (s *AsyncState) CPI() float64 {
	if s.InstDone == 0 {
		return 0
	}
	return float64(s.Cycles) / float64(s.InstDone)
}

type asyncMiddleware struct{ c *AsyncComp }

func (p *asyncMiddleware) Handle(_ timing.Event) bool {
	s := &p.c.State
	if s.Done {
		return false
	}
	if !s.Cancelled && p.c.Resources.Context != nil {
		select {
		case <-p.c.Resources.Context.Done():
			if backend, ok := p.c.Resources.backend().(interface{ CancelIfRequested() bool }); ok {
				backend.CancelIfRequested()
			}
			s.Err = p.c.Resources.Context.Err().Error()
			s.Cancelled = true
		default:
		}
	}
	s.Cycles++
	p.responses()
	if s.Cancelled {
		// Stop admission and issue, but keep consuming already-sent requests
		// so bounded response buffers cannot deadlock the memory hierarchy.
		s.Done = len(s.Pending) == 0
		return !s.Done
	}
	p.retire()
	p.drainStores()
	p.issue()
	p.dispatch()
	if s.Count == 0 && len(s.Stores) == 0 && len(s.Pending) == 0 && (p.c.Resources.backend().Stopped() || s.Err != "") {
		s.Done = true
	}
	return !s.Done
}

func (p *asyncMiddleware) entry(tag uint64) *asyncEntry {
	if tag == 0 {
		return nil
	}
	e := &p.c.State.Rob[(tag-1)%uint64(len(p.c.State.Rob))]
	if e.Tag != tag {
		return nil
	}
	return e
}
func (p *asyncMiddleware) depsReady(ds []dependency) bool {
	for _, d := range ds {
		e := p.entry(d.Tag)
		if e == nil {
			continue
		}
		at := e.Complete
		if d.Base {
			at = e.BaseComplete
		}
		if at == 0 || at > p.c.State.Cycles {
			return false
		}
	}
	return true
}
func (p *asyncMiddleware) retire() {
	s := &p.c.State
	for n := 0; n < p.c.Spec.RetireWidth && s.Count > 0; n++ {
		e := &s.Rob[s.Head]
		if e.Complete == 0 || e.Complete > s.Cycles {
			return
		}
		for i := range s.Stores {
			if s.Stores[i].Tag == e.Tag {
				s.Stores[i].Retired = true
				break
			}
		}
		s.FpInFlight -= e.FPDests
		s.InstDone++
		s.Count--
		s.Head = (s.Head + 1) % len(s.Rob)
		// Retired producer tags are now ready; later generations must not
		// mistake this entry's old dependency lists for a live producer.
		e.Tag = 0
		e.Deps = nil
		e.AddrDeps = nil
		e.Accesses = nil
	}
}
func (p *asyncMiddleware) responses() {
	s := &p.c.State
	port := p.c.Ports.Memory
	for {
		msg, ok := port.RetrieveIncoming()
		if !ok {
			return
		}
		r, ok := s.Pending[msg.RspTo]
		if !ok {
			panic("async CPU: response without pending request")
		}
		if r.Store {
			if _, ok := msg.Payload.(memprotocol.WriteDoneRsp); !ok {
				panic("async CPU: invalid write response")
			}
			for _, tag := range r.StoreTags {
				found := false
				for i := range s.Stores {
					if s.Stores[i].Tag == tag {
						s.Stores[i].Pending--
						found = true
						break
					}
				}
				if !found {
					panic("async CPU: response to removed store")
				}
			}
		} else {
			if _, ok := msg.Payload.(memprotocol.DataReadyRsp); !ok {
				panic("async CPU: invalid read response")
			}
			e := p.entry(r.Tag)
			if e == nil {
				panic("async CPU: response to retired load")
			}
			e.Pending--
			if e.Pending == 0 && e.Sent == len(e.Accesses) {
				e.Complete = s.Cycles
			}
		}
		delete(s.Pending, msg.RspTo)
		s.Memory.Responses++
	}
}

func (p *asyncMiddleware) send(tag uint64, a emu.MemAccess) bool {
	port := p.c.Ports.Memory
	s := &p.c.State
	if !port.CanSend() {
		s.Memory.SendBlockedCycles++
		return false
	}
	msg := messaging.Msg{ID: p.c.NewID(), Src: port.AsRemote(), Dst: p.c.Spec.Bottom, TrafficBytes: 16}
	if a.IsWrite {
		panic("async CPU: stores must drain through their buffer")
	}
	msg.Payload = memprotocol.ReadReq{Address: a.Addr, AccessByteSize: uint64(a.Size), PID: 1}
	s.Memory.Reads++
	port.Send(msg)
	s.Pending[msg.ID] = pendingAccess{Tag: tag, Store: a.IsWrite}
	if uint64(len(s.Pending)) > s.Memory.MaxOutstanding {
		s.Memory.MaxOutstanding = uint64(len(s.Pending))
	}
	return true
}
func (p *asyncMiddleware) drainStores() {
	s := &p.c.State
	n := 0
	for n < len(s.Stores) && s.Stores[n].Retired && s.Stores[n].Sent == len(s.Stores[n].Accesses) && s.Stores[n].Pending == 0 {
		n++
	}
	if n > 0 {
		copy(s.Stores, s.Stores[n:])
		s.Stores = s.Stores[:len(s.Stores)-n]
	}
	// Combine only consecutive retired accesses to the same line. Every
	// contributing instruction keeps its buffer entry until the shared ack.
	for {
		group := p.storeGroup()
		if len(group.Parts) == 0 {
			return
		}
		if !p.c.Ports.Memory.CanSend() {
			s.Memory.SendBlockedCycles++
			return
		}
		msg := messaging.Msg{ID: p.c.NewID(), Src: p.c.Ports.Memory.AsRemote(), Dst: p.c.Spec.Bottom, TrafficBytes: 16 + len(group.Mask), Payload: memprotocol.WriteReq{Address: group.Address, Data: make([]byte, len(group.Mask)), DirtyMask: group.Mask, PID: 1}}
		pending := pendingAccess{Store: true}
		accesses := 0
		for _, part := range group.Parts {
			st := &s.Stores[part.Index]
			st.Sent += part.Count
			st.Pending++
			pending.StoreTags = append(pending.StoreTags, st.Tag)
			accesses += part.Count
		}
		p.c.Ports.Memory.Send(msg)
		s.Pending[msg.ID] = pending
		s.Memory.Writes++
		s.Memory.StoreAccesses += uint64(accesses)
		s.Memory.CoalescedAccesses += uint64(accesses - 1)
		if uint64(len(s.Pending)) > s.Memory.MaxOutstanding {
			s.Memory.MaxOutstanding = uint64(len(s.Pending))
		}
	}
}

type storePart struct{ Index, Count int }
type storeGroup struct {
	Address uint64
	Mask    []bool
	Parts   []storePart
}

func (p *asyncMiddleware) storeGroup() storeGroup {
	var group storeGroup
	lineBytes := p.c.Spec.LineBytes
	var line uint64
	// A group uses full line-relative byte enables internally, then trims
	// untouched leading/trailing bytes. Gaps remain masked, never written.
	var mask []bool
	stop := false
	for i := range p.c.State.Stores {
		st := &p.c.State.Stores[i]
		if !st.Retired {
			break
		}
		count := 0
		for _, a := range st.Accesses[st.Sent:] {
			base := a.Addr &^ (lineBytes - 1)
			if mask == nil {
				line = base
				mask = make([]bool, lineBytes)
			}
			if base != line {
				stop = true
				break
			}
			for j := 0; j < a.Size; j++ {
				mask[a.Addr-line+uint64(j)] = true
			}
			count++
		}
		if count > 0 {
			group.Parts = append(group.Parts, storePart{i, count})
		}
		if stop {
			break
		}
	}
	if len(group.Parts) > 0 {
		lo, hi := 0, len(mask)
		for !mask[lo] {
			lo++
		}
		for !mask[hi-1] {
			hi--
		}
		group.Address = line + uint64(lo)
		group.Mask = mask[lo:hi]
	}
	return group
}

func (p *asyncMiddleware) freeUnit(u unitType, op arm64asm.Op) bool {
	s := &p.c.State
	var units []uint64
	switch u {
	case unitINT:
		units = s.IntBusy
	case unitFP:
		units = s.FpBusy
	default:
		units = s.LdstBusy
	}
	for i := range units {
		if units[i] <= s.Cycles {
			units[i] = s.Cycles + unitOccupancy(op)
			return true
		}
	}
	return false
}

// loadOrder does not use functionally known addresses until the older store's
// address dependencies have completed. Partial overlap waits for that store to
// drain; full coverage can forward, with the youngest overlapping store winning.
func (p *asyncMiddleware) loadOrder(e *asyncEntry) (ready, forward bool, latency uint64) {
	all := len(e.Accesses) > 0
	anyCovered := false
	var conventionalReady uint64
	exact := len(e.Accesses) == 1 && e.ForwardEligible
	for _, a := range e.Accesses {
		var youngest *asyncStore
		for i := range p.c.State.Stores {
			s := &p.c.State.Stores[i]
			if s.Tag >= e.Tag {
				break
			}
			if s.AddressReady == 0 || s.AddressReady > p.c.State.Cycles {
				return false, false, 0
			}
			for _, b := range s.Accesses {
				if a.Addr < b.Addr+uint64(b.Size) && b.Addr < a.Addr+uint64(a.Size) {
					youngest = s
					break
				}
			}
		}
		if youngest == nil {
			all = false
			exact = false
			continue
		}
		if youngest.Ready == 0 || youngest.Ready > p.c.State.Cycles {
			return false, false, 0
		}
		if youngest.Ready+3 > conventionalReady {
			conventionalReady = youngest.Ready + 3
		}
		covered := false
		for _, b := range youngest.Accesses {
			if b.Addr <= a.Addr && b.Addr+uint64(b.Size) >= a.Addr+uint64(a.Size) {
				covered = true
				if b.Addr != a.Addr || b.Size != a.Size {
					exact = false
				}
			}
		}
		if !covered {
			return false, false, 0
		}
		anyCovered = true
		if !youngest.Eligible {
			exact = false
		}
	}
	if !all {
		// A split/pair access is one completion group. Until per-part
		// forwarding is modeled, overlapping parts wait for the older store
		// to drain rather than reading a stale timing-cache line.
		if anyCovered {
			return false, false, 0
		}
		return true, false, 0
	}
	if exact {
		return true, true, exactGPRForwardLatency
	}
	if conventionalReady > p.c.State.Cycles {
		return false, false, 0
	}
	return true, true, conventionalForwardLatency
}

func (p *asyncMiddleware) issue() {
	s := &p.c.State
	stores := 0
	waiting := s.Waiting[:0]
	for _, tag := range s.Waiting {
		e := p.entry(tag)
		if e == nil {
			continue
		}
		p.issueEntry(e, &stores)
		if !e.Issued || (instinfo.IsLoad(e.Op) && e.Sent < len(e.Accesses) && e.Complete == 0) {
			waiting = append(waiting, tag)
		}
	}
	s.Waiting = waiting
}
func (p *asyncMiddleware) issueEntry(e *asyncEntry, stores *int) {
	s := &p.c.State

	if e.Issued {
		if instinfo.IsLoad(e.Op) && e.Complete == 0 {
			for e.Sent < len(e.Accesses) {
				if !p.send(e.Tag, e.Accesses[e.Sent]) {
					break
				}
				e.Sent++
				e.Pending++
			}
		}
		return
	}
	load, store := instinfo.IsLoad(e.Op), instinfo.IsStore(e.Op)
	// Address generation uses an actual LDST slot independently of
	// store-data readiness. Base writeback cannot bypass that resource.
	if store && e.AddressReady == 0 && p.depsReady(e.AddrDeps) && p.freeUnit(unitLDST, e.Op) {
		e.AddressReady = s.Cycles + 1
		if e.BaseDest {
			e.BaseComplete = e.AddressReady
		}
		for i := range s.Stores {
			if s.Stores[i].Tag == e.Tag {
				s.Stores[i].AddressReady = e.AddressReady
				break
			}
		}
	}
	if !p.depsReady(e.Deps) || (store && (e.AddressReady == 0 || e.AddressReady > s.Cycles)) {
		return
	}
	if store && *stores >= p.c.Spec.NumStores {
		return
	}
	forward, latency := false, uint64(instinfo.ExecuteLatency(e.Op))
	if load {
		var ready bool
		ready, forward, latency = p.loadOrder(e)
		if !ready {
			return
		}
		if !forward && !p.c.Ports.Memory.CanSend() {
			return
		}
	}
	if !store && e.Op != arm64asm.NOP && !p.freeUnit(e.Unit, e.Op) {
		return
	}
	e.Issued = true
	if load && !forward && len(e.Accesses) > 0 {
		for e.Sent < len(e.Accesses) {
			if !p.send(e.Tag, e.Accesses[e.Sent]) {
				break
			}
			e.Sent++
			e.Pending++
		}
	} else {
		if store {
			latency = 3
			*stores++
			for i := range s.Stores {
				if s.Stores[i].Tag == e.Tag {
					s.Stores[i].Ready = s.Cycles
					break
				}
			}
		}
		if forward {
			s.Memory.ForwardedLoads++
		}
		if latency == 0 {
			latency = 1
		}
		e.Complete = s.Cycles + latency
	}
	if e.BaseDest && e.BaseComplete == 0 {
		e.BaseComplete = s.Cycles + 1
	}
	if e.Mispredict {
		s.FrontendReady = s.Cycles + uint64(p.c.Spec.BranchPenalty)
		s.BlockedBranch = 0
	}

}

func splitAccesses(accesses []emu.MemAccess, line uint64) []emu.MemAccess {
	var result []emu.MemAccess
	for _, a := range accesses {
		for a.Size > 0 {
			n := int(line - a.Addr%line)
			if n > a.Size {
				n = a.Size
			}
			result = append(result, emu.MemAccess{Addr: a.Addr, Size: n, IsWrite: a.IsWrite})
			a.Addr += uint64(n)
			a.Size -= n
		}
	}
	return result
}

func (p *asyncMiddleware) dispatch() {
	s := &p.c.State
	spec := p.c.Spec
	r := p.c.Resources
	if s.Err != "" || s.BlockedBranch != 0 || s.Cycles < s.FrontendReady {
		return
	}
	for n := 0; n < spec.FetchWidth && s.Count < len(s.Rob) && s.FpInFlight < spec.FpPhysLimit && !r.backend().Stopped(); n++ {
		pc := r.backend().CPURegisters().PC
		inst, gprs, fps, err := r.peekSources(pc)
		if err != nil {
			if s.Count > 0 || len(s.Stores) > 0 {
				return
			}
			_, _, err = r.backend().StepDetailed()
			s.Err = fmt.Sprintf("fetch: %v", err)
			return
		}
		barrier := inst.Op == arm64asm.SVC || inst.Op == arm64asm.DMB || inst.Op == arm64asm.DSB || inst.Op == arm64asm.ISB
		if barrier && (s.Count > 0 || len(s.Stores) > 0 || len(s.Pending) > 0) {
			return
		}
		if instinfo.IsStore(inst.Op) && len(s.Stores) >= spec.StoreBufferSize {
			return
		}
		tag := s.NextTag
		e := asyncEntry{Tag: tag, Op: inst.Op, Unit: classifyUnit(inst), ForwardEligible: directGPRTransfer(inst)}
		add := func(d dependency) {
			if p.entry(d.Tag) != nil {
				e.Deps = append(e.Deps, d)
			}
		}
		for _, i := range gprs {
			add(s.GPR[i])
		}
		for _, i := range fps {
			add(s.FP[i])
		}
		if instinfo.UsesFlags(inst.Op) {
			add(s.Flags)
		}
		for _, a := range inst.Args {
			instinfo.ExtractMemSrcGPRs(a, func(i int) {
				if i >= 0 && i < 31 && p.entry(s.GPR[i].Tag) != nil {
					e.AddrDeps = append(e.AddrDeps, s.GPR[i])
				}
			})
		}
		_, branched, err := r.backend().StepDetailed()
		if err != nil {
			s.Err = fmt.Sprintf("execute: %v", err)
			return
		}
		e.Accesses = splitAccesses(r.backend().MemoryAccesses(), spec.LineBytes)
		// Capture dependencies before renaming destinations, including both
		// results of pair loads and base-register address writeback.
		rename := func(g, f int) {
			if g >= 0 && g < 31 {
				s.GPR[g] = dependency{Tag: tag}
			}
			if f >= 0 {
				s.FP[f] = dependency{Tag: tag}
				e.FPDests++
			}
		}
		g, f := instinfo.DestRegs(inst)
		rename(g, f)
		g2, f2 := instinfo.DestRegs2(inst)
		rename(g2, f2)
		if base := instinfo.BaseWritebackGPR(inst); base >= 0 && base < 31 && base != g && base != g2 {
			s.GPR[base] = dependency{Tag: tag, Base: true}
			e.BaseDest = true
		}
		if instinfo.SetsFlags(inst.Op) {
			s.Flags = dependency{Tag: tag}
		}
		if instinfo.IsStore(inst.Op) {
			s.Stores = append(s.Stores, asyncStore{Tag: tag, Accesses: e.Accesses, AddrDeps: e.AddrDeps, Eligible: e.ForwardEligible})
		}
		if instinfo.IsBranch(inst.Op) && spec.BranchPrediction {
			pred := predictor{PredictorState: &s.Predictor, spec: spec.Predictor}
			call, ret := inst.Op == arm64asm.BL || inst.Op == arm64asm.BLR, inst.Op == arm64asm.RET
			taken, target := pred.Predict(pc, call, ret)
			actual := r.backend().CPURegisters().PC
			pred.Update(pc, branched, actual, taken, call, ret)
			e.Mispredict = taken != branched || (taken && target != actual)
		} else if branched {
			e.Mispredict = true
		}
		s.Rob[(tag-1)%uint64(len(s.Rob))] = e
		s.Count++
		s.NextTag++
		s.Waiting = append(s.Waiting, tag)
		s.FpInFlight += e.FPDests
		if e.Mispredict {
			s.BlockedBranch = tag
			return
		}
		if branched || barrier {
			return
		}
	}
}
