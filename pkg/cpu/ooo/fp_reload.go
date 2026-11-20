package ooo

import (
	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"github.com/sarchlab/poros/pkg/emu"
	"golang.org/x/arch/arm64/arm64asm"
)

// Effective recovery duration after a sustained series of partial-address conflicts.
// The hardware measurements bound the duration but do not identify a physical counter.
const conservativeReloadWindow = 5_575_360

// The M5 model compares the low 14 address bits and records one refill per
// double in that span. This empirical comparator is independent of VM page size.
const reloadAliasSpan = 16 * 1024
const refillSlotBytes = 8

type fpReloadState struct {
	PooledBits                                     uint32
	PooledSeen                                     bool
	TransientThrough                               uint64
	InputSets                                      [64]sourceSetReads
	CompetingReadThrough, CompetingFeedbackAddress uint64
	PageAliasChain                                 uint64
	// One-based program position of the load that observed the page-offset conflict.
	PageAliasLoad                      uint64
	Shared                             readSetHistory
	PreviousChainLast, FeedbackAddress uint64
	SetPressure                        []sourceReadSetHistory
	LastFeedback                       uint64
	Refills                            []observedFPRefill
	Streams                            []observedInputStream
	ConflictEpoch                      uint64
	HoldThrough                        uint64
	PartialHistory                     [64]partialLoadHistory
	LateReload                         observedLateReload

	FeedbackFP          [32][2]feedbackOrigin
	OpenLines           map[uint64]uint64
	LineHistory         []activeLine
	NewLinesInReduction uint8
	TwoRecentLines      bool
}

type feedbackOrigin struct {
	Address uint64
	Valid   bool
	Direct  bool
}

// Input-set observations retire with the guest read in the reorder buffer.
type sourceSetReads struct {
	PreviousFeedbackAddress        uint64
	PreviousConflict, PreviousLate bool
	PC                             uint64
	Last                           []sourceSetRead
}
type sourceSetRead struct {
	FeedbackAddress uint64
	Line, Position  uint64
	Late            bool
}

func (s *State) observeSourceSet(pc, addr uint64, sets, lineBytes int, late bool) {
	h := &s.FPReload.InputSets[(pc>>2)%uint64(len(s.FPReload.InputSets))]
	if len(h.Last) != sets {
		*h = sourceSetReads{PC: pc, Last: make([]sourceSetRead, sets)}
	} else if h.PC != pc {
		clear(h.Last)
		*h = sourceSetReads{PC: pc, Last: h.Last}
	}
	line := addr / uint64(lineBytes)
	slot := &h.Last[line%uint64(sets)]
	position := s.InstDone + uint64(s.RobCount) + 1
	feedback := s.FPReload.FeedbackAddress
	conflict := slot.Position > s.InstDone && slot.Line != line && slot.FeedbackAddress == feedback
	lateRead := slot.Late || late
	if conflict && h.PreviousConflict && h.PreviousFeedbackAddress == feedback && (lateRead || h.PreviousLate) && s.FPReload.LastFeedback > s.InstDone {
		s.FPReload.CompetingReadThrough, s.FPReload.CompetingFeedbackAddress = position, feedback
	}
	h.PreviousConflict, h.PreviousLate, h.PreviousFeedbackAddress = conflict, lateRead, feedback
	*slot = sourceSetRead{Line: line, Position: position, Late: late, FeedbackAddress: feedback}
}

type partialLoadHistory struct {
	SameSectorConfidence uint8
	RelativeOffset       uint8
	PC                   uint64
	Prior                uint8
	Counter              [2]uint8
	LastMatchEpoch       uint64
	Consecutive          uint8
}

// A stable offset with matching address bit 6 predicts contention with the
// pending scalar store that produced the sampled reload.
func (h *partialLoadHistory) observeSector(addr uint64, prev observedLateReload) bool {
	offset := uint8((addr - prev.Addr) & 127)
	if offset != h.RelativeOffset {
		h.SameSectorConfidence = 0
		h.RelativeOffset = offset
	}
	if prev.Valid && addr != prev.Addr && (addr^prev.Addr)&64 == 0 {
		wait := h.SameSectorConfidence >= 2
		h.SameSectorConfidence = min(h.SameSectorConfidence+1, 3)
		return wait
	}
	if h.SameSectorConfidence > 0 {
		h.SameSectorConfidence--
	}
	return false
}

// Predict from the previous match outcome before updating this source's
// confidence. Count at most one match per pending-reload observation.
func (h *partialLoadHistory) observePartialMatch(match, twoRecentLines bool, reloadEpoch uint64) bool {
	old := h.Prior
	expected := h.Counter[old] >= 2
	if match && twoRecentLines {
		epoch := reloadEpoch + 1
		if h.LastMatchEpoch != epoch {
			if h.LastMatchEpoch+1 != epoch {
				h.Consecutive = 0
			}
			h.Consecutive = min(h.Consecutive+1, 32)
			h.LastMatchEpoch = epoch
		}
	}
	if match {
		h.Counter[old] = min(h.Counter[old]+1, 3)
		h.Prior = 1
	} else {
		if h.Counter[old] > 0 {
			h.Counter[old]--
		}
		h.Prior = 0
	}
	return expected
}

func scalarFPAddress(inst arm64asm.Inst, op arm64asm.Op) bool {
	if scalarFPOffset(inst, op) {
		return true
	}
	r, ok := inst.Args[0].(arm64asm.Reg)
	_, indexed := inst.Args[1].(arm64asm.MemExtend)
	return inst.Op == op && ok && r >= arm64asm.D0 && r <= arm64asm.D31 && indexed
}

type observedLateReload struct {
	Addr, Ready uint64
	Valid       bool
}

// Track distinct input lines still in flight between successive true feedback reloads.
func (p *middleware) observeFPLoadLines(inst arm64asm.Inst, accesses []emu.MemAccess) {
	if inst.Op != arm64asm.LDR || len(accesses) != 1 || accesses[0].Size != 8 || p.comp.Resources.Cache == nil {
		return
	}
	reg, ok := inst.Args[0].(arm64asm.Reg)
	if !ok || reg < arm64asm.D0 || reg > arm64asm.D31 {
		return
	}
	a := accesses[0]
	state := &p.comp.State
	st := p.youngestOverlap(a)
	if st == nil {
		line := a.Addr / uint64(p.comp.Resources.Cache.Config().L1LineSize)
		if !state.observeActiveLine(line) {
			state.FPReload.NewLinesInReduction = min(state.FPReload.NewLinesInReduction+1, 2)
		}
	} else if st.Feedback && st.Addr == a.Addr && st.Size == 8 {
		state.FPReload.TwoRecentLines = state.FPReload.NewLinesInReduction >= 2
		state.FPReload.NewLinesInReduction = 0
	}
}

// A learned partial overlap can let the cache read start before the earlier store
// provides its data. The second return value bounds when the result can be used.
func (p *middleware) partialReloadWait(pc uint64, a emu.MemAccess, issueCycle uint64) (uint64, uint64) {
	var conflictReturn uint64
	prev := p.comp.State.FPReload.LateReload
	st := p.youngestOverlap(a)
	match := st == nil && prev.Valid && prev.Ready > issueCycle && a.Addr != prev.Addr && (a.Addr^prev.Addr)&127 == 0
	sharedPressure := false
	if st == nil && p.comp.Resources.Cache != nil {
		cfg := p.comp.Resources.Cache.Config()
		sets := cfg.L1Size / (cfg.L1LineSize * cfg.L1Ways)
		sharedPressure = p.comp.State.FPReload.Shared.observe(a.Addr, sets, cfg.L1LineSize, 128)
		hot := p.comp.State.FPReload.sideReadSetPressure(pc, a.Addr, sets, cfg.L1LineSize)
		if hot && match && p.comp.State.FPReload.TwoRecentLines {
			state := &p.comp.State
			state.FPReload.enterReloadEpoch(state.InstDone + uint64(state.RobCount))
		}
	}
	expected := false
	if st == nil {
		state := &p.comp.State
		index := (pc >> 2) % uint64(len(state.FPReload.PartialHistory))
		h := state.FPReload.PartialHistory[index]
		found := h.PC == pc
		if !found {
			h = partialLoadHistory{PC: pc}
		}
		if h.observeSector(a.Addr, prev) {
			latest := p.youngestOverlap(emu.MemAccess{Addr: prev.Addr, Size: 8})
			if latest != nil && latest.ScalarStorePosition != 0 && latest.Addr == prev.Addr && latest.Size == 8 {
				issueCycle = max(issueCycle, latest.ReadyCycle)
			}
		}
		expected = h.observePartialMatch(match, state.FPReload.TwoRecentLines, state.FPReload.ConflictEpoch)
		if match || found || h.SameSectorConfidence > 0 {
			state.FPReload.PartialHistory[index] = h
		}
	}
	if match && p.comp.State.FPReload.TwoRecentLines {
		state := &p.comp.State
		if !expected && state.FPReload.LastFeedback > state.InstDone && (prev.Addr^a.Addr)&(reloadAliasSpan-1) == 0 {
			state.FPReload.PageAliasChain = prev.Addr
			state.FPReload.PageAliasLoad = state.InstDone + uint64(state.RobCount) + 1
		}

		if sharedPressure && !expected && state.FPReload.PreviousChainLast > state.InstDone {
			state.FPReload.enterReloadEpoch(state.InstDone + uint64(state.RobCount))
		}
		if state.conservativeReloadActive() {
			for j := len(p.comp.State.StoreBuffer) - 1; j >= 0; j-- {
				st := &p.comp.State.StoreBuffer[j]
				if st.Addr == prev.Addr && st.Size == 8 {
					issueCycle = max(issueCycle, st.ReadyCycle)
					break
				}
			}
		}
		state.FPReload.PooledSeen = true
		h := state.FPReload.PartialHistory[(pc>>2)%uint64(len(state.FPReload.PartialHistory))]
		if h.Consecutive == 32 {
			state.FPReload.enterReloadEpoch(state.InstDone + uint64(state.RobCount))
		}
		if !expected || state.conservativeReloadActive() {
			issueCycle = max(issueCycle, prev.Ready)
		} else {
			conflictReturn = prev.Ready
		}
	}
	if st != nil && st.ScalarStorePosition != 0 && st.Addr == a.Addr && st.Size == 8 && st.ReadyCycle > issueCycle {
		state := &p.comp.State

		state.FPReload.PooledBits <<= 1
		if state.FPReload.PooledSeen {
			state.FPReload.PooledBits |= 1
		}
		state.FPReload.PooledSeen = false
		if state.FPReload.PooledBits == ^uint32(0) {
			state.FPReload.TransientThrough = state.InstDone + uint64(state.RobCount) + 1
		}
		state.FPReload.ConflictEpoch++

		p.comp.State.FPReload.LateReload = observedLateReload{Addr: a.Addr, Ready: st.ReadyCycle, Valid: true}
	}
	return issueCycle, conflictReturn
}

func (s *State) conservativeReloadActive() bool {
	return s.InstDone+uint64(s.RobCount) < s.FPReload.HoldThrough || s.FPReload.TransientThrough > s.InstDone
}

// trainAndForwardFeedback observes live feedback stores on the conventional
// forwarding path. Dispatch handles supplied-byte and adjacent forwarding separately.
func (p *middleware) trainAndForwardFeedback(a emu.MemAccess, issue uint64, fallback loadForwarding) loadForwarding {
	st := p.youngestOverlap(a)
	if st == nil || !st.Feedback || st.Addr != a.Addr || st.Size != 8 || st.CompleteCycle <= issue {
		return fallback
	}
	state := &p.comp.State
	position := state.InstDone + uint64(state.RobCount)
	if a.Addr != state.FPReload.FeedbackAddress {
		state.FPReload.PageAliasLoad = 0
		state.FPReload.PreviousChainLast = state.FPReload.LastFeedback
		state.FPReload.FeedbackAddress = a.Addr
	}
	state.FPReload.LastFeedback = position + 1
	// ScalarStorePosition records one plus the dynamic position of the store.
	liveAlias := state.FPReload.PageAliasLoad > state.InstDone && state.FPReload.PageAliasChain == a.Addr
	liveSetConflict := state.FPReload.CompetingReadThrough > state.InstDone && state.FPReload.CompetingFeedbackAddress == a.Addr
	if liveAlias || liveSetConflict || state.conservativeReloadActive() || position < st.ScalarStorePosition || position-st.ScalarStorePosition < 8 {
		return fallback
	}
	return loadForwarding{issueCycle: issue, dataReady: st.ReadyCycle, latency: 1, forwarded: true}
}

// Remember an exact scalar load and one following floating-point add/subtract.
func (p *middleware) feedbackOrigins(inst arm64asm.Inst) [2]feedbackOrigin {
	var origins [2]feedbackOrigin
	if scalarFPAddress(inst, arm64asm.LDR) {
		a := p.comp.Resources.backend().MemoryAccesses()
		if len(a) == 1 && a[0].Size == 8 {
			origins[0] = feedbackOrigin{a[0].Addr, true, true}
		}
	} else if inst.Op == arm64asm.FADD || inst.Op == arm64asm.FSUB {
		r, _ := inst.Args[0].(arm64asm.Reg)
		if r >= arm64asm.D0 && r <= arm64asm.D31 {
			for i := 1; i < 3; i++ {
				if from := instinfo.ArgFP(inst.Args[i]); from >= 0 {
					origin := p.comp.State.FPReload.FeedbackFP[from][0]
					if origin.Direct {
						origin.Direct = false
						origins[i-1] = origin
					}
				}
			}
		}
	}

	return origins
}

func (p *middleware) newerReadyProducer(loadPC uint64, a emu.MemAccess, at uint64) bool {
	state := &p.comp.State
	predictedPC, ok := state.StoreDependence.LoadStore[loadPC]
	if !ok {
		return false
	}
	predicted, ok := state.StoreDependence.Youngest[predictedPC]
	if !ok || predicted.CompleteCycle <= at {
		return false
	}
	for i := len(state.StoreBuffer) - 1; i >= 0; i-- {
		st := state.StoreBuffer[i]
		if st.PC == predictedPC {
			return false
		}
		if st.Addr < a.Addr+uint64(a.Size) && a.Addr < st.Addr+uint64(st.Size) {
			return st.Addr == a.Addr && st.Size == a.Size && st.ReadyCycle <= at
		}
	}
	return false
}

type activeLine struct {
	Line, Position uint64
	Valid          bool
}

// observeActiveLine returns whether an older, not-yet-retired FP read
// already requested this line. The history is bounded by the ROB capacity.
func (s *State) observeActiveLine(line uint64) bool {
	if len(s.FPReload.LineHistory) == 0 {
		s.FPReload.LineHistory = make([]activeLine, len(s.Rob))
		s.FPReload.OpenLines = make(map[uint64]uint64)
	}
	readAt, ok := s.FPReload.OpenLines[line]
	live := ok && readAt >= s.InstDone
	slot := &s.FPReload.LineHistory[s.RobTail]
	if slot.Valid && s.FPReload.OpenLines[slot.Line] == slot.Position {
		delete(s.FPReload.OpenLines, slot.Line)
	}
	position := s.InstDone + uint64(s.RobCount)
	*slot = activeLine{Line: line, Position: position, Valid: true}
	s.FPReload.OpenLines[line] = position
	return live
}

// readSetHistory counts competing lines per L1 set across a rolling window of
// reads. The per-instruction window spans 64 observations; the shared one 128.
type readSetHistory struct {
	Tags                map[uint64]uint8
	Occupancy           []uint8
	Access              []uint64
	OtherLines          []uint8
	Cursor, Filled, Sum int
}

func (h *readSetHistory) observe(addr uint64, sets, lineBytes, window int) bool {
	if h.Tags == nil {
		h.Tags = make(map[uint64]uint8)
		h.Occupancy = make([]uint8, sets)
		h.Access = make([]uint64, window)
		h.OtherLines = make([]uint8, window)
	}
	line := addr / uint64(lineBytes)
	index := line % uint64(sets)
	if h.Filled == len(h.Access) {
		old := h.Access[h.Cursor]
		if h.Tags[old] == 1 {
			delete(h.Tags, old)
			h.Occupancy[old%uint64(sets)]--
		} else {
			h.Tags[old]--
		}
	} else {
		h.Filled++
	}
	other := h.Occupancy[index]
	if h.Tags[line] > 0 {
		other--
	} else {
		h.Occupancy[index]++
	}
	h.Tags[line]++
	h.Sum += int(other) - int(h.OtherLines[h.Cursor])
	h.OtherLines[h.Cursor] = other
	h.Access[h.Cursor] = line
	h.Cursor = (h.Cursor + 1) % len(h.Access)
	return h.Filled == len(h.Access) && h.Sum >= len(h.Access)
}

type sourceReadSetHistory struct {
	PC      uint64
	History readSetHistory
}

func (s *fpReloadState) sideReadSetPressure(pc, addr uint64, sets, lineBytes int) bool {
	if s.SetPressure == nil {
		s.SetPressure = make([]sourceReadSetHistory, 64)
	}
	source := &s.SetPressure[(pc>>2)%uint64(len(s.SetPressure))]
	if source.PC != pc {
		*source = sourceReadSetHistory{PC: pc}
	}
	return source.History.observe(addr, sets, lineBytes, 64)
}

type observedFPRefill struct {
	Addr, PC, Start, End uint64
	Step                 int64
}
type observedInputStream struct {
	PC, Addr   uint64
	Step       int64
	Confidence uint8
}

func (s *fpReloadState) enterReloadEpoch(pos uint64) {
	if pos >= s.HoldThrough {
		s.HoldThrough = pos + conservativeReloadWindow
	}
}

func (s *State) noteFPRefill(pc, addr, issue, ready uint64, late bool) {
	f := &s.FPReload
	if f.Streams == nil {
		f.Streams = make([]observedInputStream, 64)
		f.Refills = make([]observedFPRefill, reloadAliasSpan/refillSlotBytes)
	}
	stream := &f.Streams[(pc/4)%64]
	if stream.PC != pc {
		*stream = observedInputStream{PC: pc, Addr: addr}
		return
	}
	step := int64(addr) - int64(stream.Addr)
	steady := (step == 8 || step == -8) && step == stream.Step
	stream.Addr, stream.Step = addr, step
	if !steady {
		stream.Confidence = 0
		return
	}
	if !late {
		return
	}
	slot := &f.Refills[(addr%reloadAliasSpan)/refillSlotBytes]
	if f.LastFeedback > s.InstDone && step == slot.Step && slot.PC != pc && slot.Addr != addr && issue < slot.End && slot.Start < ready {
		if stream.Confidence < 3 {
			stream.Confidence++
		}
		if stream.Confidence >= 2 {
			f.enterReloadEpoch(s.InstDone + uint64(s.RobCount))
		}
	} else if stream.Confidence > 0 {
		stream.Confidence--
	}
	*slot = observedFPRefill{addr, pc, issue, ready, step}
}
