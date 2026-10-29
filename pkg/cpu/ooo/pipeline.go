// Package ooo models the out-of-order superscalar CPU timing pipeline.
package ooo

import (
	"github.com/sarchlab/akita/v5/sim/modeling/ticking"
	"github.com/sarchlab/akita/v5/sim/timing"
	instinfo "github.com/sarchlab/poros/pkg/cpu/internal/inst"
	"github.com/sarchlab/poros/pkg/emu"
	"github.com/sarchlab/poros/pkg/mem"
	"golang.org/x/arch/arm64/arm64asm"
)

// HookCycles configures the cycle cost of a functional hook.
type HookCycles struct {
	Address uint64 `json:"address"`
	Cycles  int    `json:"cycles"`
}

// Spec contains immutable structural and timing parameters.
type Spec struct {
	Freq             timing.Freq   `json:"freq"`
	FetchWidth       int           `json:"fetch_width"`
	RetireWidth      int           `json:"retire_width"`
	ROBCapacity      int           `json:"rob_capacity"`
	NumINT           int           `json:"num_int"`
	NumFP            int           `json:"num_fp"`
	NumLDST          int           `json:"num_ldst"`
	BranchPenalty    int           `json:"branch_penalty"`
	LoadLatency      int           `json:"load_latency"`
	FpPhysLimit      int           `json:"fp_phys_limit"`
	StoreBufferSize  int           `json:"store_buffer_size"`
	NumMSHREntries   int           `json:"num_mshr_entries"`
	BranchPrediction bool          `json:"branch_prediction"`
	Predictor        PredictorSpec `json:"predictor"`
	FuncHookCycles   []HookCycles  `json:"func_hook_cycles"`
}

func DefaultSpec() Spec {
	return Spec{
		Freq: 3490 * timing.MHz, FetchWidth: 3, RetireWidth: 8, ROBCapacity: 256,
		NumINT: 6, NumFP: 4, NumLDST: 4, BranchPenalty: 17, LoadLatency: 5,
		FpPhysLimit: 72, StoreBufferSize: 56, NumMSHREntries: 12, BranchPrediction: true,
		Predictor: PredictorSpec{BimodalSize: 4096, TaggedTableSize: 2048, BTBSize: 2048, BTBWays: 4, RASSize: 32, TaggedHistLen: 32},
	}
}

// Resources holds the unchanged functional backend and optional synchronous cache.
// They are live objects and are not included in component checkpoints.
type Resources struct {
	Emu       *emu.Emulator
	Execution emu.Execution
	Cache     *mem.Cache
}
type Ports struct{}
type Middlewares struct{ Pipeline *middleware }
type Comp = ticking.Component[Spec, State, Resources, Ports, Middlewares]

// State owns mutable scheduling, queues, predictor tables, and counters.
type State struct {
	Serializing                  bool
	GprReady, FpReady            [32]uint64
	FlagsReady                   uint64
	IntUnits, FpUnits, LdstUnits []uint64
	Rob                          []uint64
	RobFPDests                   []int
	RobHead, RobTail, RobCount   int
	FpInFlight                   int
	StoreBuffer                  []storeBufferEntry
	MshrBusyUntil                []uint64
	FrontendReady                uint64
	Predictor                    PredictorState
	Cycles, InstDone             uint64
	Done                         bool
	Err                          string
}
type storeBufferEntry struct {
	Addr          uint64
	Size          int
	CompleteCycle uint64
}
type unitType int

const (
	unitINT unitType = iota
	unitFP
	unitLDST
)

func (s *State) Initialize(c *Comp) {
	if (c.Resources.Emu == nil) == (c.Resources.Execution == nil) {
		panic("ooo: exactly one emulator or execution resource is required")
	}
	if c.Resources.Emu != nil && !c.Resources.Emu.Mem.TrackAccesses {
		panic("ooo: enable emulator memory access tracking before Build")
	}
	v := c.Spec
	if v.FetchWidth < 1 || v.RetireWidth < 1 || v.ROBCapacity < 1 || v.NumINT < 1 || v.NumFP < 1 || v.NumLDST < 1 || v.LoadLatency < 1 || v.FpPhysLimit < 1 || v.StoreBufferSize < 1 || v.NumMSHREntries < 1 || v.BranchPenalty < 0 {
		panic("ooo: invalid pipeline configuration")
	}
	s.IntUnits = make([]uint64, v.NumINT)
	s.FpUnits = make([]uint64, v.NumFP)
	s.LdstUnits = make([]uint64, v.NumLDST)
	s.Rob = make([]uint64, v.ROBCapacity)
	s.RobFPDests = make([]int, v.ROBCapacity)
	s.StoreBuffer = make([]storeBufferEntry, 0, v.StoreBufferSize)
	s.MshrBusyUntil = make([]uint64, v.NumMSHREntries)
	if v.BranchPrediction {
		(&predictor{PredictorState: &s.Predictor, spec: v.Predictor}).init()
	}
}
func (m *Middlewares) Initialize(c *Comp) {
	p := &middleware{comp: c, hookCycles: make(map[uint64]int, len(c.Spec.FuncHookCycles))}
	for _, h := range c.Spec.FuncHookCycles {
		if h.Cycles < 1 {
			panic("ooo: hook cycles must be positive")
		}
		if _, ok := p.hookCycles[h.Address]; ok {
			panic("ooo: duplicate hook address")
		}
		p.hookCycles[h.Address] = h.Cycles
	}
	m.Pipeline = p
}

// hookCycles is an immutable lookup derived from Spec. All runtime data is in State.
type middleware struct {
	comp       *Comp
	hookCycles map[uint64]int
}

func (p *middleware) predictor() *predictor {
	return &predictor{PredictorState: &p.comp.State.Predictor, spec: p.comp.Spec.Predictor}
}

// CPI returns cycles per instruction.
func (s *State) CPI() float64 {
	if s.InstDone == 0 {
		return 0
	}
	return float64(s.Cycles) / float64(s.InstDone)
}

// Handle advances the pipeline by one cycle.
func (p *middleware) Handle(_ timing.Event) bool {
	if p.comp.State.Done {
		return false
	}
	p.comp.State.Cycles++

	// Phase 1: Retire completed instructions from ROB head.
	p.retire()

	// Phase 2: Fetch, decode, dispatch, issue new instructions.
	p.fetchAndProcess()

	// Check termination: ROB drained and emulator halted.
	if p.comp.State.RobCount == 0 && p.comp.Resources.backend().Stopped() {
		p.comp.State.Done = true
	}

	return !p.comp.State.Done
}

// retire walks the ROB from head, retiring entries where
// completeCycle <= currentCycle, up to RetireWidth per cycle.
func (p *middleware) retire() {
	retired := 0
	for retired < p.comp.Spec.RetireWidth && p.comp.State.RobCount > 0 {
		if p.comp.State.Rob[p.comp.State.RobHead] > p.comp.State.Cycles {
			break // head entry not yet complete
		}
		p.comp.State.FpInFlight -= p.comp.State.RobFPDests[p.comp.State.RobHead]
		p.comp.State.InstDone++
		p.comp.State.RobHead = (p.comp.State.RobHead + 1) % p.comp.Spec.ROBCapacity
		p.comp.State.RobCount--
		retired++
	}
}

// fetchAndProcess fetches up to FetchWidth instructions, decodes, dispatches,
// and functionally executes each one.
func (p *middleware) fetchAndProcess() {
	if p.comp.State.Serializing {
		if p.comp.State.RobCount != 0 {
			return
		}
		p.comp.State.Serializing = false
	}
	if p.comp.State.Cycles < p.comp.State.FrontendReady {
		return // frontend stalled from branch penalty
	}

	fetched := 0
	for fetched < p.comp.Spec.FetchWidth && p.comp.State.RobCount < p.comp.Spec.ROBCapacity &&
		p.comp.State.FpInFlight < p.comp.Spec.FpPhysLimit && !p.comp.Resources.backend().Stopped() {
		// 1. Peek at instruction at emulator's current PC.
		pc := p.comp.Resources.backend().CPURegisters().PC
		inst, err := p.comp.Resources.backend().Peek(pc)
		if err != nil {
			if p.comp.Resources.Execution != nil {
				// Resolve a checked fetch fault only after older work retires.
				if p.comp.State.RobCount != 0 {
					return
				}
				_, _, err = p.comp.Resources.backend().StepDetailed()
				if err != nil {
					p.comp.State.Err = err.Error()
				}
				p.comp.State.Done = true
			}
			return
		}
		if p.comp.Resources.Execution != nil && inst.Op == arm64asm.SVC && p.comp.State.RobCount != 0 {
			return
		}

		// 2. Compute source operand readiness.
		var srcReady uint64
		for _, gpr := range instinfo.SourceGPRs(inst) {
			if p.comp.State.GprReady[gpr] > srcReady {
				srcReady = p.comp.State.GprReady[gpr]
			}
		}
		for _, fp := range instinfo.SourceFPs(inst) {
			if p.comp.State.FpReady[fp] > srcReady {
				srcReady = p.comp.State.FpReady[fp]
			}
		}

		// Flags (NZCV) dependency.
		if instinfo.UsesFlags(inst.Op) && p.comp.State.FlagsReady > srcReady {
			srcReady = p.comp.State.FlagsReady
		}

		// 3. Classify instruction and find execution unit.
		ut := classifyUnit(inst.Op)
		dispatchCycle := p.comp.State.Cycles
		issueCycle := dispatchCycle
		if srcReady > issueCycle {
			issueCycle = srcReady
		}

		bestUnit, bestCycle := p.findUnit(ut, issueCycle)
		issueCycle = bestCycle

		// 4. Execute functionally (in program order).
		_, branched, execErr := p.comp.Resources.backend().StepDetailed()
		if execErr != nil {
			p.comp.State.Err = execErr.Error()
			if p.comp.Resources.Emu != nil {
				p.comp.State.Done = true
			}
			return
		}

		// 5. Address-based load-store disambiguation (after StepDetailed
		//    so actual addresses are available in p.comp.Resources.backend().MemoryAccesses()).
		forwarded := false
		if instinfo.IsLoad(inst.Op) {
			accesses := p.comp.Resources.backend().MemoryAccesses()
			allForwarded := len(accesses) > 0
			for _, acc := range accesses {
				covered := false
				for _, sb := range p.comp.State.StoreBuffer {
					if acc.Addr < sb.Addr+uint64(sb.Size) &&
						sb.Addr < acc.Addr+uint64(acc.Size) {
						// Address overlap: load must wait for this store.
						if sb.CompleteCycle > issueCycle {
							issueCycle = sb.CompleteCycle
						}
						// Check if the store fully covers this load access.
						if sb.Addr <= acc.Addr &&
							acc.Addr+uint64(acc.Size) <= sb.Addr+uint64(sb.Size) {
							covered = true
						}
					}
				}
				if !covered {
					allForwarded = false
				}
			}
			forwarded = allForwarded
		}

		// 6. Compute execution latency.
		var latency uint64
		if forwarded {
			// Store-to-load forwarding: ~5-cycle forwarding latency (M2 estimate).
			latency = 5
		} else {
			var miss bool
			latency, miss = p.instrLatencyWithPC(inst, pc)
			// MSHR: if load missed L1, acquire an MSHR slot (may stall).
			if miss {
				mshrIdx, mshrCycle := p.acquireMSHR(issueCycle)
				issueCycle = mshrCycle
				p.comp.State.MshrBusyUntil[mshrIdx] = issueCycle + latency
			}
		}
		completeCycle := issueCycle + latency

		// 7. Update destination register readiness.
		gpr, fp := instinfo.DestRegs(inst)
		if gpr >= 0 && gpr < 31 {
			p.comp.State.GprReady[gpr] = completeCycle
		}
		if fp >= 0 {
			p.comp.State.FpReady[fp] = completeCycle
		}
		fpDests := 0
		if fp >= 0 {
			fpDests++
		}
		if gpr2, fp2 := instinfo.DestRegs2(inst); gpr2 >= 0 || fp2 >= 0 {
			if gpr2 >= 0 {
				p.comp.State.GprReady[gpr2] = completeCycle
			}
			if fp2 >= 0 {
				p.comp.State.FpReady[fp2] = completeCycle
				fpDests++
			}
		}

		// 7b. Track base register writeback for pre/post-indexed addressing.
		// The base writeback (e.g., X22 += 8 in "str D1, [X22], #8") is
		// available after address generation (1 cycle), independent of the
		// instruction's data latency.
		if base := instinfo.BaseWritebackGPR(inst); base >= 0 && base < 31 && base != gpr {
			if instinfo.IsStore(inst.Op) {
				// For stores, base writeback depends only on the base
				// register, not on the store data operand.
				baseReady := p.comp.State.GprReady[base]
				if dispatchCycle > baseReady {
					baseReady = dispatchCycle
				}
				p.comp.State.GprReady[base] = baseReady + 1
			} else {
				// For loads, issueCycle already includes base readiness.
				p.comp.State.GprReady[base] = issueCycle + 1
			}
		}

		// Update NZCV flags readiness.
		if instinfo.SetsFlags(inst.Op) {
			p.comp.State.FlagsReady = completeCycle
		}

		// 8. For stores: add to store buffer.
		if instinfo.IsStore(inst.Op) {
			for _, acc := range p.comp.Resources.backend().MemoryAccesses() {
				entry := storeBufferEntry{
					Addr:          acc.Addr,
					Size:          acc.Size,
					CompleteCycle: completeCycle,
				}
				if len(p.comp.State.StoreBuffer) >= p.comp.Spec.StoreBufferSize {
					// Store buffer full — stall frontend until oldest
					// store retires (its completeCycle).
					stallUntil := p.comp.State.StoreBuffer[0].CompleteCycle + 1
					if stallUntil > p.comp.State.FrontendReady {
						p.comp.State.FrontendReady = stallUntil
					}
					p.comp.State.StoreBuffer = p.comp.State.StoreBuffer[1:]
				}
				p.comp.State.StoreBuffer = append(p.comp.State.StoreBuffer, entry)
			}
		}

		// 9. Mark execution unit busy.
		p.markUnitBusy(ut, bestUnit, issueCycle, inst.Op)

		// 10. Add to ROB and track FP register usage.
		p.comp.State.Rob[p.comp.State.RobTail] = completeCycle
		p.comp.State.RobFPDests[p.comp.State.RobTail] = fpDests
		p.comp.State.FpInFlight += fpDests
		p.comp.State.RobTail = (p.comp.State.RobTail + 1) % p.comp.Spec.ROBCapacity
		p.comp.State.RobCount++

		// 11. Handle branch.
		if instinfo.IsBranch(inst.Op) && p.comp.Spec.BranchPrediction {
			isCall := inst.Op == arm64asm.BL || inst.Op == arm64asm.BLR
			isReturn := inst.Op == arm64asm.RET

			// Simulate the prediction that would have happened at fetch time.
			predTaken, predTarget := p.predictor().Predict(pc, isCall, isReturn)

			// Actual outcome from StepDetailed.
			actualTaken := branched
			actualTarget := p.comp.Resources.backend().CPURegisters().PC

			// Update predictor with actual outcome.
			p.predictor().Update(pc, actualTaken, actualTarget, predTaken, isCall, isReturn)

			// Check if prediction was correct.
			mispredicted := (predTaken != actualTaken) ||
				(predTaken && predTarget != actualTarget)

			if mispredicted {
				p.comp.State.FrontendReady = issueCycle + uint64(p.comp.Spec.BranchPenalty)
				break
			}
			// Correct prediction, taken: BTB provides the target in the
			// same cycle, so the redirect is effectively free on M2
			// (loop stream detector / micro-op cache eliminates the
			// redirect bubble for well-predicted branches).
			if actualTaken {
				p.comp.State.FrontendReady = p.comp.State.Cycles + 1 // 0-cycle redirect
				fetched++
				if p.comp.Resources.Execution != nil && inst.Op == arm64asm.SVC {
					return
				}
				break
			}
		} else if branched {
			// Fallback for non-branch PC changes or nil predictor.
			p.comp.State.FrontendReady = issueCycle + uint64(p.comp.Spec.BranchPenalty)
			break
		}

		fetched++
		if p.comp.Resources.Execution != nil && inst.Op == arm64asm.SVC {
			p.comp.State.Serializing = true
			return
		}
	}
}

// classifyUnit determines which execution unit type an instruction needs.
func classifyUnit(op arm64asm.Op) unitType {
	if instinfo.IsLoad(op) || instinfo.IsStore(op) {
		return unitLDST
	}
	switch op {
	case arm64asm.FADD, arm64asm.FSUB, arm64asm.FMUL,
		arm64asm.FMADD, arm64asm.FMSUB,
		arm64asm.FNMUL, arm64asm.FNMADD,
		arm64asm.FDIV, arm64asm.FSQRT,
		arm64asm.FCMP, arm64asm.FMOV, arm64asm.FCSEL,
		arm64asm.SCVTF, arm64asm.UCVTF,
		arm64asm.FCVTZS, arm64asm.FCVTZU,
		arm64asm.FNEG, arm64asm.FABS:
		return unitFP
	default:
		return unitINT
	}
}

// findUnit scans all units of the matching type and returns the one that
// becomes free earliest (>= earliest). Returns (unitIndex, actualIssueCycle).
func (p *middleware) findUnit(ut unitType, earliest uint64) (int, uint64) {
	var units []uint64
	switch ut {
	case unitINT:
		units = p.comp.State.IntUnits
	case unitFP:
		units = p.comp.State.FpUnits
	case unitLDST:
		units = p.comp.State.LdstUnits
	}

	bestIdx := 0
	bestTime := units[0]
	if earliest > bestTime {
		bestTime = earliest
	}
	for i := 1; i < len(units); i++ {
		t := units[i]
		if earliest > t {
			t = earliest
		}
		if t < bestTime {
			bestIdx = i
			bestTime = t
		}
	}
	return bestIdx, bestTime
}

// markUnitBusy marks an execution unit as occupied.
// Most units are pipelined (1-cycle throughput). DIV/SQRT are partially
// pipelined: the unit is busy for throughput cycles (< latency).
func (p *middleware) markUnitBusy(ut unitType, idx int, issueCycle uint64, op arm64asm.Op) {
	busyUntil := issueCycle + 1 // pipelined: 1-cycle throughput
	switch op {
	case arm64asm.SDIV, arm64asm.UDIV:
		busyUntil = issueCycle + uint64(instinfo.ExecuteLatency(op)) // non-pipelined
	case arm64asm.FDIV:
		busyUntil = issueCycle + 10 // partially pipelined: 10cy throughput, 22cy latency
	case arm64asm.FSQRT:
		busyUntil = issueCycle + 6 // partially pipelined: 6cy throughput, 12cy latency
	}
	switch ut {
	case unitINT:
		p.comp.State.IntUnits[idx] = busyUntil
	case unitFP:
		p.comp.State.FpUnits[idx] = busyUntil
	case unitLDST:
		p.comp.State.LdstUnits[idx] = busyUntil
	}
}

// instrLatencyWithPC computes the execution latency for an instruction,
// using the PC to check for hooked BL instructions.
func (p *middleware) instrLatencyWithPC(inst arm64asm.Inst, pc uint64) (latency uint64, l1Miss bool) {
	// Check for hooked BL instruction.
	if inst.Op == arm64asm.BL && p.hookCycles != nil {
		if pcrel, ok := inst.Args[0].(arm64asm.PCRel); ok {
			target := uint64(int64(pc) + int64(pcrel))
			if cycles, ok := p.hookCycles[target]; ok {
				return uint64(cycles), false
			}
		}
	}
	return p.instrLatency(inst)
}

// instrLatency computes the execution latency for an instruction.
// For loads, l1Miss indicates whether the load missed L1 (needs MSHR).
func (p *middleware) instrLatency(inst arm64asm.Inst) (latency uint64, l1Miss bool) {
	if instinfo.IsLoad(inst.Op) {
		if p.comp.Resources.Cache != nil {
			accesses := p.comp.Resources.backend().MemoryAccesses()
			if len(accesses) == 0 {
				return uint64(p.comp.Spec.LoadLatency), false
			}
			worst := mem.HitL1
			for _, acc := range accesses {
				level := p.comp.Resources.Cache.Lookup(acc.Addr, acc.Size, acc.IsWrite)
				if level > worst {
					worst = level
				}
			}
			cfg := p.comp.Resources.Cache.Config()
			switch worst {
			case mem.HitL2:
				return uint64(cfg.L2Latency), true
			case mem.HitDRAM:
				return uint64(cfg.DRAMLatency), true
			default:
				return uint64(cfg.L1Latency), false
			}
		}
		return uint64(p.comp.Spec.LoadLatency), false
	}
	if instinfo.IsStore(inst.Op) {
		if p.comp.Resources.Cache != nil {
			for _, acc := range p.comp.Resources.backend().MemoryAccesses() {
				p.comp.Resources.Cache.Lookup(acc.Addr, acc.Size, acc.IsWrite)
			}
		}
		return 3, false
	}
	return uint64(instinfo.ExecuteLatency(inst.Op)), false
}

// acquireMSHR finds the earliest-available MSHR slot. If all MSHRs are busy
// past issueCycle, the load is delayed. Returns (mshrIndex, adjustedIssueCycle).
func (p *middleware) acquireMSHR(issueCycle uint64) (int, uint64) {
	bestIdx := 0
	bestTime := p.comp.State.MshrBusyUntil[0]
	for i := 1; i < len(p.comp.State.MshrBusyUntil); i++ {
		if p.comp.State.MshrBusyUntil[i] < bestTime {
			bestIdx = i
			bestTime = p.comp.State.MshrBusyUntil[i]
		}
	}
	if bestTime > issueCycle {
		return bestIdx, bestTime
	}
	return bestIdx, issueCycle
}

func (r Resources) backend() emu.Execution {
	if r.Execution != nil {
		return r.Execution
	}
	return r.Emu
}
