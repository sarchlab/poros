// Package ooo models the out-of-order superscalar CPU timing pipeline.
package ooo

import (
	"fmt"

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
	Freq        timing.Freq `json:"freq"`
	FetchWidth  int         `json:"fetch_width"`
	RetireWidth int         `json:"retire_width"`
	ROBCapacity int         `json:"rob_capacity"`
	NumINT      int         `json:"num_int"`
	NumFP       int         `json:"num_fp"`
	// NumFPDiv is the number of FP pipes eligible for scalar divide/sqrt.
	// These are a subset of NumFP, so ordinary arithmetic shares their capacity.
	NumFPDiv int `json:"num_fp_div"`
	// NumLDST counts physical memory pipes. NumLoad and NumStore are eligible
	// subsets, not extra pipes: the first NumLoad accept loads and the last
	// NumStore accept stores, sharing the overlapping reservation calendars.
	// Both zero preserves legacy fully shared pipes; otherwise both must be
	// positive, no greater than NumLDST, and together cover all physical pipes.
	NumLDST       int `json:"num_ldst"`
	NumLoad       int `json:"num_load"`
	NumStore      int `json:"num_store"`
	BranchPenalty int `json:"branch_penalty"`
	LoadLatency   int `json:"load_latency"`
	// FpPhysLimit bounds speculative destinations, not total physical registers.
	// At least two are required so a supported paired FP load can dispatch.
	FpPhysLimit      int           `json:"fp_phys_limit"`
	StoreBufferSize  int           `json:"store_buffer_size"`
	NumMSHREntries   int           `json:"num_mshr_entries"`
	BranchPrediction bool          `json:"branch_prediction"`
	Predictor        PredictorSpec `json:"predictor"`
	FuncHookCycles   []HookCycles  `json:"func_hook_cycles"`
}

func DefaultSpec() Spec {
	return Spec{
		Freq: 3490 * timing.MHz, FetchWidth: 8, RetireWidth: 8, ROBCapacity: 256,
		NumINT: 6, NumFP: 4, NumFPDiv: 1, NumLDST: 4, NumLoad: 3, NumStore: 2, BranchPenalty: 17, LoadLatency: 5,
		FpPhysLimit: 360, StoreBufferSize: 56, NumMSHREntries: 12, BranchPrediction: true,
		Predictor: PredictorSpec{BimodalSize: 4096, TaggedTableSize: 2048, BTBSize: 2048, BTBWays: 4, RASSize: 32, TaggedHistLen: 32},
	}
}

// Resources selects exactly one execution policy. Emu preserves legacy kernel
// hooks and termination behavior. Execution drives a checked process: faults
// drain older work, and SVC serializes the pipeline. Cache is optional.
// These live objects are not included in component checkpoints. A Cache is a
// single-run timed client: this pipeline advances its clock and finalizes it on
// termination. Warm it with Lookup before starting; use a new Cache for another
// timed run. Finalization does not prohibit untimed Lookup access.
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
	IntUnits, FpUnits, LdstUnits [][]UnitReservation
	Rob                          []uint64
	RobFPDests                   []int
	RobHead, RobTail, RobCount   int
	FpInFlight                   int
	StoreBuffer                  []storeBufferEntry
	// One finite instruction slot per store until the cache accepts its bytes.
	// Pending write allocations are separately bounded and owned by Cache.
	StoreDrainUntil  []uint64
	MshrBusyUntil    []uint64
	FrontendReady    uint64
	Predictor        PredictorState
	Cycles, InstDone uint64
	Done             bool
	Err              string
}
type storeBufferEntry struct {
	Addr               uint64
	Size               int
	CompleteCycle      uint64
	PriorCompleteCycle uint64 // ordering inherited from fully shadowed older stores
	DrainCycle         uint64 // allocation completion, independent of forwarding data
	// ReadyCycle conservatively records when store address and data can issue.
	// Missing fields in an older serialized state leave FastForwardEligible false, which
	// preserves the conventional forwarding path for those entries.
	ReadyCycle          uint64
	FastForwardEligible bool
}
type unitType int

const (
	unitINT unitType = iota
	unitFP
	unitFPDiv
	unitLoad
	unitStore
)

func (s *State) Initialize(c *Comp) {
	if (c.Resources.Emu == nil) == (c.Resources.Execution == nil) {
		panic("ooo: exactly one emulator or execution resource is required")
	}
	if c.Resources.Emu != nil && !c.Resources.Emu.Mem.TrackAccesses {
		panic("ooo: enable emulator memory access tracking before Build")
	}
	v := c.Spec
	if v.FpPhysLimit < 2 {
		panic("ooo: FP destination capacity must be at least 2 for paired loads")
	}
	if v.FetchWidth < 1 || v.RetireWidth < 1 || v.ROBCapacity < 1 || v.NumINT < 1 || v.NumFP < 1 || v.NumFPDiv < 1 || v.NumFPDiv > v.NumFP || v.NumLDST < 1 || v.LoadLatency < 1 || v.StoreBufferSize < 1 || v.NumMSHREntries < 1 || v.BranchPenalty < 0 {
		panic("ooo: invalid pipeline configuration")
	}
	if v.NumLoad != 0 || v.NumStore != 0 {
		if v.NumLoad < 1 || v.NumStore < 1 || v.NumLoad > v.NumLDST || v.NumStore > v.NumLDST || v.NumLoad < v.NumLDST-v.NumStore {
			panic("ooo: load/store eligibility must cover the physical memory pipes")
		}
	}
	s.IntUnits = make([][]UnitReservation, v.NumINT)
	s.FpUnits = make([][]UnitReservation, v.NumFP)
	s.LdstUnits = make([][]UnitReservation, v.NumLDST)
	s.Rob = make([]uint64, v.ROBCapacity)
	s.RobFPDests = make([]int, v.ROBCapacity)
	s.StoreBuffer = make([]storeBufferEntry, 0, v.StoreBufferSize)
	s.StoreDrainUntil = make([]uint64, v.StoreBufferSize)
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

// hookCycles is an immutable lookup derived from Spec.
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
		if p.comp.Resources.Cache != nil {
			p.comp.Resources.Cache.FinishTiming()
		}
		return false
	}
	if len(p.comp.State.StoreDrainUntil) != p.comp.Spec.StoreBufferSize {
		panic("ooo: missing or incompatible store-allocation state; pipeline-only checkpoint restore is unsupported")
	}
	p.comp.State.Cycles++
	p.pruneUnitReservations()
	if p.comp.Resources.Cache != nil {
		p.comp.Resources.Cache.Advance(p.comp.State.Cycles)
	}

	// Phase 1: Retire completed instructions from ROB head.
	p.retire()

	// Phase 2: Fetch, decode, dispatch, issue new instructions.
	p.fetchAndProcess()

	// Drain both retired stores and the ROB before normal termination.
	if p.comp.State.RobCount == 0 && p.storesDrained() && (p.comp.Resources.backend().Stopped() || p.comp.State.Err != "") {
		p.comp.State.Done = true
	}

	if p.comp.State.Done && p.comp.Resources.Cache != nil {
		p.comp.Resources.Cache.FinishTiming()
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
	p.pruneStoreRecords()
	if p.comp.State.Err != "" {
		return
	}
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
	for fetched < p.comp.Spec.FetchWidth && p.comp.State.RobCount < p.comp.Spec.ROBCapacity && !p.comp.Resources.backend().Stopped() {
		// 1. Peek at instruction at emulator's current PC.
		pc := p.comp.Resources.backend().CPURegisters().PC
		inst, gprs, fps, err := p.comp.Resources.peekSources(pc)
		if err != nil {
			if p.comp.Resources.processMode() {
				// Resolve a checked fetch fault only after older instructions
				// and their outstanding store allocations have drained.
				if p.comp.State.RobCount != 0 || !p.storesDrained() {
					return
				}
				_, _, err = p.comp.Resources.backend().StepDetailed()
				if err != nil {
					p.comp.State.Err = fmt.Sprintf("fetch: %v", err)
				}
				p.comp.State.Done = true
			}
			return
		}
		if p.comp.Resources.processMode() && inst.Op == arm64asm.SVC && (p.comp.State.RobCount != 0 || !p.storesDrained()) {
			return
		}

		// Admit the actual destination count, including both halves of LDP.
		// A full FP window does not prevent integer or store-only progress.
		_, fpFirst := instinfo.DestRegs(inst)
		_, fpSecond := instinfo.DestRegs2(inst)
		neededFP := 0
		if fpFirst >= 0 {
			neededFP++
		}
		if fpSecond >= 0 {
			neededFP++
		}
		if p.comp.State.FpInFlight+neededFP > p.comp.Spec.FpPhysLimit {
			return
		}

		// Allocate store capacity at actual dispatch, before functional effects
		// or future cache reservations. Stores retain their instruction slot until
		// cache acceptance; a full pool backpressures the frontend.
		storeSlot := -1
		if instinfo.IsStore(inst.Op) {
			storeSlot = p.availableStoreSlot()
			if storeSlot < 0 || len(p.comp.State.StoreBuffer)+2 > p.forwardingCapacity() {
				return
			}
		}

		// 2. Compute source operand readiness.
		var srcReady uint64
		for _, gpr := range gprs {
			if p.comp.State.GprReady[gpr] > srcReady {
				srcReady = p.comp.State.GprReady[gpr]
			}
		}
		for _, fp := range fps {
			if p.comp.State.FpReady[fp] > srcReady {
				srcReady = p.comp.State.FpReady[fp]
			}
		}

		// Flags (NZCV) dependency.
		if instinfo.UsesFlags(inst.Op) && p.comp.State.FlagsReady > srcReady {
			srcReady = p.comp.State.FlagsReady
		}

		// 3. Classify instruction and find execution unit.
		ut := classifyUnit(inst)
		dispatchCycle := p.comp.State.Cycles
		issueCycle := dispatchCycle
		if srcReady > issueCycle {
			issueCycle = srcReady
		}

		// NOP is eliminated before execution, while retaining frontend, ROB
		// and retirement accounting. Other HINT instructions are not assumed
		// to have the same resource behavior.
		usesUnit := inst.Op != arm64asm.NOP
		if instinfo.IsLoad(inst.Op) {
			// Forwarding eligibility depends on when a load could issue.
			// Other instructions need only the final resource selection below.
			_, issueCycle = p.findUnit(ut, issueCycle, unitOccupancy(inst.Op))
		}

		// 4. Execute functionally (in program order).
		_, branched, execErr := p.comp.Resources.backend().StepDetailed()
		if execErr != nil {
			p.comp.State.Err = fmt.Sprintf("execute: %v", execErr)
			if !p.comp.Resources.processMode() {
				p.comp.State.Done = true
			}
			return
		}

		// 5. Address-based load-store disambiguation (after StepDetailed
		//    so actual addresses are available in p.comp.Resources.backend().MemoryAccesses()).
		forwarding := loadForwarding{issueCycle: issueCycle}
		if instinfo.IsLoad(inst.Op) {
			forwarding = p.forwardLoad(p.comp.Resources.backend().MemoryAccesses(), issueCycle, directGPRTransfer(inst))
			issueCycle = forwarding.issueCycle
		}

		// 6. Probe residency without side effects, choose MSHR/unit issue, then
		// commit cache requests at that final time. Cache fills cannot start
		// while the supplying instruction is still waiting for a resource.
		miss := !forwarding.forwarded && instinfo.IsLoad(inst.Op) && p.loadMissesL1()
		mshr := -1
		if usesUnit {
			issueCycle, mshr = p.reserveIssue(ut, issueCycle, unitOccupancy(inst.Op), miss)
		}
		var latency, dataReady, storeDrain uint64
		if instinfo.IsStore(inst.Op) {
			latency = 3
			dataReady, storeDrain = p.storeTiming(issueCycle)
		} else if forwarding.forwarded {
			latency = forwarding.latency
		} else {
			latency, dataReady = p.instrLatencyWithPC(inst, pc, issueCycle)
		}
		completeCycle := issueCycle + latency
		if !instinfo.IsStore(inst.Op) && dataReady > completeCycle {
			completeCycle = dataReady
		}
		if forwarding.forwarded && forwarding.dataReady+latency > completeCycle {
			completeCycle = forwarding.dataReady + latency
		}
		if mshr >= 0 {
			p.comp.State.MshrBusyUntil[mshr] = completeCycle
		}
		if storeSlot >= 0 {
			drain := completeCycle
			if dataReady > drain {
				drain = dataReady
			}
			p.comp.State.StoreDrainUntil[storeSlot] = drain
		}

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
			eligible := directGPRTransfer(inst)
			for _, acc := range p.comp.Resources.backend().MemoryAccesses() {
				entry := storeBufferEntry{
					Addr:                acc.Addr,
					Size:                acc.Size,
					CompleteCycle:       completeCycle,
					DrainCycle:          max(storeDrain, p.comp.State.StoreDrainUntil[storeSlot]),
					ReadyCycle:          issueCycle,
					FastForwardEligible: eligible,
				}

				p.appendStoreRecord(entry)
			}
		}

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
				break
			}
		} else if branched {
			// Fallback for non-branch PC changes or nil predictor.
			p.comp.State.FrontendReady = issueCycle + uint64(p.comp.Spec.BranchPenalty)
			break
		}

		fetched++
		if p.comp.Resources.processMode() && inst.Op == arm64asm.SVC {
			p.comp.State.Serializing = true
			return
		}
	}
}

// classifyUnit determines which execution unit type an instruction needs.
func classifyUnit(instruction arm64asm.Inst) unitType {
	op := instruction.Op
	if (op == arm64asm.ORR || op == arm64asm.MOVI) && instinfo.ArgFP(instruction.Args[0]) >= 0 {
		return unitFP
	}
	if instinfo.IsLoad(op) {
		return unitLoad
	}
	if instinfo.IsStore(op) {
		return unitStore
	}
	switch op {
	case arm64asm.FDIV, arm64asm.FSQRT:
		return unitFPDiv
	case arm64asm.FADD, arm64asm.FSUB, arm64asm.FMUL,
		arm64asm.FMADD, arm64asm.FMSUB,
		arm64asm.FNMUL, arm64asm.FNMADD, arm64asm.FNMSUB,
		arm64asm.FCMP, arm64asm.FCMPE, arm64asm.FMOV, arm64asm.FCSEL,
		arm64asm.SCVTF, arm64asm.UCVTF,
		arm64asm.FCVTZS, arm64asm.FCVTZU,
		arm64asm.FNEG, arm64asm.FABS:
		return unitFP
	default:
		return unitINT
	}
}

// instrLatencyWithPC computes the execution latency for an instruction,
// using the PC to check for hooked BL instructions.
func (p *middleware) instrLatencyWithPC(inst arm64asm.Inst, pc uint64, issueCycle uint64) (latency uint64, dataReady uint64) {
	// Check for hooked BL instruction.
	if inst.Op == arm64asm.BL && p.hookCycles != nil {
		if pcrel, ok := inst.Args[0].(arm64asm.PCRel); ok {
			target := uint64(int64(pc) + int64(pcrel))
			if cycles, ok := p.hookCycles[target]; ok {
				return uint64(cycles), 0
			}
		}
	}
	return p.instrLatency(inst, issueCycle)
}

// instrLatency computes the execution latency for an instruction.
// dataReady is the absolute cycle at which the cache can deliver the data
// (zero when the cache model is not timed or not present). Loads use the timed
// cache path only after final execution-resource selection. Stores instead use
// storeTiming, which owns their separate cache admission/drain contract.
func (p *middleware) instrLatency(inst arm64asm.Inst, issueCycle uint64) (latency uint64, dataReady uint64) {
	if instinfo.IsLoad(inst.Op) {
		if p.comp.Resources.Cache != nil {
			accesses := p.comp.Resources.backend().MemoryAccesses()
			if len(accesses) == 0 {
				return uint64(p.comp.Spec.LoadLatency), 0
			}
			worst := mem.HitL1
			for _, acc := range accesses {
				level, ready := p.comp.Resources.Cache.Access(acc.Addr, acc.Size, acc.IsWrite, issueCycle)
				if level > worst {
					worst = level
				}
				if ready > dataReady {
					dataReady = ready
				}
			}
			cfg := p.comp.Resources.Cache.Config()
			switch worst {
			case mem.HitL2:
				return uint64(cfg.L2Latency), dataReady
			case mem.HitDRAM:
				return uint64(cfg.DRAMLatency), dataReady
			default:
				return uint64(cfg.L1Latency), dataReady
			}
		}
		return uint64(p.comp.Spec.LoadLatency), 0
	}
	return uint64(instinfo.ExecuteLatency(inst.Op)), 0
}

// loadMissesL1 only classifies current dispatch-order cache residency.
func (p *middleware) loadMissesL1() bool {
	if p.comp.Resources.Cache == nil {
		return false
	}
	lineSize := p.comp.Resources.Cache.Config().L1LineSize
	for _, acc := range p.comp.Resources.backend().MemoryAccesses() {
		for addr, remaining := acc.Addr, acc.Size; remaining > 0; {
			if p.comp.Resources.Cache.Probe(addr) != mem.HitL1 {
				return true
			}
			n := min(remaining, lineSize-int(addr%uint64(lineSize)))
			addr += uint64(n)
			remaining -= n
		}
	}
	return false
}

// Keep every still-pending forwarding record, including old future-ready
// stores while younger independent stores drain. Cache combining can outlive
// instruction slots; dispatch enforces the separate forwardingCapacity bound.
func (p *middleware) pruneStoreRecords() {
	kept := p.comp.State.StoreBuffer[:0]
	for _, entry := range p.comp.State.StoreBuffer {
		until := entry.DrainCycle
		if entry.CompleteCycle > until {
			until = entry.CompleteCycle
		}
		if until > p.comp.State.Cycles {
			kept = append(kept, entry)
		}
	}
	p.comp.State.StoreBuffer = kept
}

func (p *middleware) availableStoreSlot() int {
	for i, until := range p.comp.State.StoreDrainUntil {
		if until <= p.comp.State.Cycles {
			return i
		}
	}
	return -1
}

func (p *middleware) storesDrained() bool {
	if c := p.comp.Resources.Cache; c != nil && c.DrainUntil() > p.comp.State.Cycles {
		return false
	}
	for _, until := range p.comp.State.StoreDrainUntil {
		if until > p.comp.State.Cycles {
			return false
		}
	}
	return true
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

// An execution resource may return the static register dependencies alongside
// the instruction it fetched. Other resources can construct synthetic Inst
// values, so extract their operands directly rather than relying on Enc.
type sourcePeeker interface {
	PeekSources(pc uint64) (arm64asm.Inst, []int, []int, error)
}

func (r Resources) peekSources(pc uint64) (arm64asm.Inst, []int, []int, error) {
	backend := r.backend()
	if p, ok := backend.(sourcePeeker); ok {
		return p.PeekSources(pc)
	}
	inst, err := backend.Peek(pc)
	if err != nil {
		return inst, nil, nil, err
	}
	return inst, instinfo.SourceGPRs(inst), instinfo.SourceFPs(inst), nil
}

func (r Resources) backend() emu.Execution {
	if r.Execution != nil {
		return r.Execution
	}
	return r.Emu
}

// processMode enables process boundary/fault policy independently of backend state.
func (r Resources) processMode() bool { return r.Execution != nil }
