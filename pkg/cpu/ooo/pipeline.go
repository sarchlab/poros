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
	// ScalarFPReloadPrediction models M5 scalar FP feedback and ordering under competing reads.
	// With general store-dependence prediction off, an eligible load learns the
	// static store PC only when the youngest overlapping store supplies exact feedback.
	ScalarFPReloadPrediction bool        `json:"scalar_fp_reload_prediction,omitempty"`
	Freq                     timing.Freq `json:"freq"`
	FetchWidth               int         `json:"fetch_width"`
	RetireWidth              int         `json:"retire_width"`
	ROBCapacity              int         `json:"rob_capacity"`
	NumINT                   int         `json:"num_int"`
	// NumSelectFusion is the number of integer pipes that can execute a fused
	// compare/select pair. They are the last NumINT pipes, a subset shared with
	// ordinary integer work. Zero disables the fusion.
	NumSelectFusion int `json:"num_select_fusion"`
	NumFP           int `json:"num_fp"`
	// NumFPDiv is the number of FP pipes eligible for scalar divide/sqrt.
	// These are a subset of NumFP, so ordinary arithmetic shares their capacity.
	NumFPDiv int `json:"num_fp_div"`
	// L1ReadBankPeriod is the byte distance over which eight-byte L1 read
	// groups repeat. Zero disables read-port conflicts.
	L1ReadBankPeriod int `json:"l1_read_bank_period,omitempty"`
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
	// FetchTakenBranches is how many correctly predicted taken branches one
	// fetch group may continue across. Zero ends the group at every one.
	FetchTakenBranches int `json:"fetch_taken_branches,omitempty"`
	// FetchGroupLimit, when positive, separates fetch from rename: fetch
	// delivers at most one group per cycle, a window of this many instruction
	// slots from the group's first instruction, and FetchWidth counts rename
	// slots, which may take instructions of several fetched groups in one
	// cycle. A correctly predicted taken branch ends the group unless its
	// target lies ahead inside the window; FetchTakenBranches is not used.
	// Zero keeps one combined fetch/rename group of FetchWidth.
	FetchGroupLimit int `json:"fetch_group_limit,omitempty"`
	// FetchLead, with FetchGroupLimit, is how many fetched groups wait for
	// rename: fetch delivers a group no earlier than the cycle after the group
	// FetchLead+1 places before it began rename, so a stalled rename stage
	// resumes with at most FetchLead+1 groups to take in one cycle.
	FetchLead int `json:"fetch_lead,omitempty"`
	// FetchRepeatMin counts consecutive correctly predicted groups that end at
	// the same instruction and transfer back to their first instruction.
	// FetchRepeatExitDelay delays the following fetch when such a group
	// changes its ending or successor. Both zero disables repeat timing.
	FetchRepeatMin       int `json:"fetch_repeat_min,omitempty"`
	FetchRepeatExitDelay int `json:"fetch_repeat_exit_delay,omitempty"`
	// ConditionalBranches, when positive, is how many conditional branches
	// (B.cond, CBZ, CBNZ, TBZ, TBNZ) rename takes in one cycle.
	ConditionalBranches int `json:"conditional_branches,omitempty"`
	// CompareBranchFusion fuses, at most once per cycle, a flag-setting integer
	// compare of a register with a register or an immediate and the B.cond
	// immediately after it, or an ADD/SUB of an immediate and a CBZ/CBNZ of
	// its result immediately after it. The pair shares a rename and retirement
	// slot and issues once on two integer pipes.
	CompareBranchFusion bool `json:"compare_branch_fusion,omitempty"`
	// FlagBranchDependency makes an unfused B.cond wait for NZCV when it
	// issues. Fetch can follow a correct prediction before the flags exist.
	FlagBranchDependency bool `json:"flag_branch_dependency,omitempty"`
	// StoreDependencePrediction makes a load that last overlapped an in-flight
	// store wait for the youngest instance of that static store, even when this
	// instance writes other bytes; see storedep.go.
	StoreDependencePrediction bool `json:"store_dependence_prediction,omitempty"`
	// StoreQueueInOrder makes stores leave the store buffer in program order:
	// a store keeps its slot until every older store has released its own, so
	// a store that waits for the cache holds back the stores behind it. False
	// releases each slot on its own, once the store has executed and the
	// cache has accepted its bytes.
	StoreQueueInOrder bool `json:"store_queue_in_order,omitempty"`
	// Latencies replaces the timing of instruction classes; see latencies.go.
	Latencies Latencies `json:"latencies,omitzero"`
	// NumIntDiv, when positive, is the number of integer dividers: a divide
	// keeps one from starting another divide until it completes and delays no
	// other integer work. Zero keeps the default, in which a divide occupies
	// any integer pipe for its latency.
	NumIntDiv int `json:"num_int_div,omitempty"`
	// StoreForwardLatency, when positive, gives a load that in-flight stores
	// supply a single forwarding path: the value of an access is ready this
	// many cycles after both the load can issue and the data of the youngest
	// store that holds all its bytes has issued, whatever the instruction
	// forms and the register loaded. Zero keeps the two paths of forwardLoad.
	StoreForwardLatency int `json:"store_forward_latency,omitempty"`
	// AdjacentFPForwardLatency times an 8-byte immediate-offset FP store and
	// load of the same address in consecutive dispatch positions. It starts
	// when the load address and the supplying store data are both ready.
	// StoreForwardLatency, if enabled, retains precedence.
	AdjacentFPForwardLatency int `json:"adjacent_fp_forward_latency,omitempty"`
	// FPIssueQueue is the number of FP operations and FP store data that can
	// wait between dispatch and issue; see issuequeue.go. Zero is unlimited.
	FPIssueQueue int `json:"fp_issue_queue,omitempty"`
	// MultiplyIssueQueue is the number of integer multiplies that can wait
	// between dispatch and issue; see issuequeue.go. Zero is unlimited.
	MultiplyIssueQueue int `json:"multiply_issue_queue,omitempty"`
	// StoreDataQueue bounds immediate-offset scalar 32/64-bit stores waiting
	// for their source register. The address and cache retain their own timing.
	StoreDataQueue int `json:"store_data_queue,omitempty"`
	// RegisterOffsetStoreData includes 64-bit STR X/D with a register offset
	// in that same store-data queue.
	RegisterOffsetStoreData bool `json:"register_offset_store_data,omitempty"`
	// BypassRecovery, when positive, qualifies the exact GPR store-to-load
	// path by address operand and by the load's value history; see bypass.go.
	// A load that reads another value after BypassValueRun equal ones goes
	// without the path for BypassRecovery executions. PlainForwardLatency is
	// the latency from a store's data to a load the path does not serve.
	BypassRecovery      int `json:"bypass_recovery,omitempty"`
	BypassValueRun      int `json:"bypass_value_run,omitempty"`
	PlainForwardLatency int `json:"plain_forward_latency,omitempty"`
}

func DefaultSpec() Spec {
	return Spec{
		Freq: 3490 * timing.MHz, FetchWidth: 8, RetireWidth: 8, ROBCapacity: 256,
		NumINT: 6, NumSelectFusion: 1, NumFP: 4, NumFPDiv: 1, NumLDST: 4, NumLoad: 3, NumStore: 2, BranchPenalty: 17, LoadLatency: 5,
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
	FPReload                     fpReloadState `json:",omitzero"`
	Serializing                  bool
	GprReady, FpReady            [32]uint64
	FAddWakeup                   faddWakeupState `json:",omitzero"`
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
	StoreDependence  StoreDependenceState
	Bypass           BypassState `json:",omitzero"`
	Cycles, InstDone uint64
	Done             bool
	Err              string
	// With StoreQueueInOrder: the cycle the youngest store releases its slot.
	StoreRelease uint64
	// With Latencies.FPAccumulate: whether an FP multiply or fused
	// multiply-add produced the register, so it can be forwarded late.
	FpLateForward [32]bool
	// Dividers that are resources of their own (Spec.NumIntDiv,
	// Latencies.FPDivideOccupancy); empty otherwise.
	IntDividers, FpDividers [][]UnitReservation
	// With Spec.FPIssueQueue: for each entry in use, the first cycle in which
	// dispatch can use it again (issuequeue.go).
	FpQueue []uint64
	// With Spec.MultiplyIssueQueue: the same for its entries.
	MultiplyQueue []uint64
	// With Spec.StoreDataQueue: cycles when store data slots become free.
	StoreDataQueue []uint64 `json:",omitempty"`
	// With Spec.FetchGroupLimit: the cycle the current fetch group was
	// fetched in, how many instructions it holds so far, and whether the next
	// instruction starts another group.
	FetchGroupCycle uint64 `json:",omitempty"`
	FetchGroupStart uint64 `json:",omitempty"`
	FetchGroupSize  int    `json:",omitempty"`
	FetchGroupEnded bool   `json:",omitempty"`
	// A run of correctly predicted self-transfers, saturated at FetchRepeatMin.
	FetchRepeatStart uint64 `json:",omitempty"`
	FetchRepeatEnd   uint64 `json:",omitempty"`
	FetchRepeatCount int    `json:",omitempty"`
	// The cycles in which the last FetchLead+1 groups began rename, by group
	// number modulo FetchLead+1, and the number of groups fetched.
	FetchGroupRenamed []uint64 `json:",omitempty"`
	FetchGroups       uint64   `json:",omitempty"`
	// With Spec.CompareBranchFusion: which ROB entries hold a branch fused
	// with the compare before it.
	RobFused []bool `json:",omitempty"`
	// L1ReadIssues holds the aligned addresses served by L1 read ports at
	// each still-relevant issue cycle. Only the banked profile populates it.
	L1ReadIssues map[uint64][]uint64 `json:",omitempty"`
}
type storeBufferEntry struct {
	Feedback           bool   `json:",omitempty"`
	PC                 uint64 // static store, for dependence prediction
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
	// With Spec.BypassRecovery: the address operand of an eligible store.
	Operand addressOperand `json:",omitzero"`
	// A qualifying FP store records the one dispatch position at which its
	// consumer would be adjacent. Zero also covers older saved states.
	NextFPReload uint64 `json:",omitempty"`
	// One-based dispatch position of a scalar FP store, for feedback timing.
	ScalarStorePosition uint64 `json:",omitempty"`
}
type unitType int

const (
	unitINT unitType = iota
	unitIntDiv
	unitSelectFusion
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
	if v.FetchTakenBranches < 0 || v.FetchGroupLimit < 0 || v.FetchLead < 0 || v.ConditionalBranches < 0 {
		panic("ooo: invalid fetch taken-branch count, group limit, lead or branch limit")
	}
	s.L1ReadIssues = nil
	if v.L1ReadBankPeriod < 0 || v.L1ReadBankPeriod%8 != 0 {
		panic("ooo: L1 read bank period must be a nonnegative multiple of eight bytes")
	}
	if v.FetchRepeatMin < 0 || v.FetchRepeatExitDelay < 0 ||
		(v.FetchRepeatMin > 0) != (v.FetchRepeatExitDelay > 0) ||
		(v.FetchRepeatMin > 0 && v.FetchGroupLimit == 0) {
		panic("ooo: fetch repetition requires a group, positive threshold and exit delay")
	}
	s.FetchRepeatStart, s.FetchRepeatEnd, s.FetchRepeatCount = 0, 0, 0
	s.FAddWakeup = faddWakeupState{}
	if v.CompareBranchFusion {
		s.RobFused = make([]bool, v.ROBCapacity)
	}
	if v.FetchGroupLimit > 0 {
		s.FetchGroupRenamed = make([]uint64, v.FetchLead+1)
		s.FetchGroupEnded = true
	}
	if v.NumSelectFusion < 0 || v.NumSelectFusion > v.NumINT {
		panic("ooo: compare/select fusion pipes must be a subset of the integer pipes")
	}
	if v.RegisterOffsetStoreData && v.StoreDataQueue == 0 {
		panic("ooo: register-offset store data requires a store-data queue")
	}
	if v.BypassRecovery < 0 || (v.BypassRecovery > 0) != (v.PlainForwardLatency > 0) || (v.BypassRecovery > 0) != (v.BypassValueRun > 0) {
		panic("ooo: bypass recovery, value run and plain forward latency must be set together")
	}
	if !v.Latencies.valid() || v.NumIntDiv < 0 || v.StoreForwardLatency < 0 || v.AdjacentFPForwardLatency < 0 || v.FPIssueQueue < 0 || v.MultiplyIssueQueue < 0 || v.StoreDataQueue < 0 {
		panic("ooo: invalid latency configuration")
	}
	if v.NumIntDiv > 0 {
		s.IntDividers = make([][]UnitReservation, v.NumIntDiv)
	}
	if v.Latencies.FPDivideOccupancy > 0 {
		s.FpDividers = make([][]UnitReservation, v.NumFPDiv)
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
	if len(p.comp.State.L1ReadIssues) > 0 {
		delete(p.comp.State.L1ReadIssues, p.comp.State.Cycles)
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
	for p.comp.State.RobCount > 0 {
		// A branch fused with its compare retires in the compare's slot.
		fused := len(p.comp.State.RobFused) > 0 && p.comp.State.RobFused[p.comp.State.RobHead]
		if !fused && retired >= p.comp.Spec.RetireWidth {
			break
		}
		if p.comp.State.Rob[p.comp.State.RobHead] > p.comp.State.Cycles {
			break // head entry not yet complete
		}
		p.comp.State.FpInFlight -= p.comp.State.RobFPDests[p.comp.State.RobHead]
		p.comp.State.InstDone++
		p.comp.State.RobHead = (p.comp.State.RobHead + 1) % p.comp.Spec.ROBCapacity
		p.comp.State.RobCount--
		if !fused {
			retired++
		}
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
	takenLeft := p.comp.Spec.FetchTakenBranches
	grouped := p.comp.Spec.FetchGroupLimit > 0
	var pair fusedSelect
	// The conditional branch completing a compare/branch pair, if any, and
	// whether this cycle has fused such a pair already.
	var branchPair fusedBranch
	fusedThisCycle := false
	conditionals := 0
	for p.comp.State.RobCount < p.comp.Spec.ROBCapacity && !p.comp.Resources.backend().Stopped() {
		// 1. Peek at instruction at emulator's current PC.
		pc := p.comp.Resources.backend().CPURegisters().PC
		fused := branchPair
		branchFused := fused.pending && fused.pc == pc
		branchPair.pending = false
		if fetched >= p.comp.Spec.FetchWidth && !branchFused {
			return
		}
		if grouped {
			if p.comp.State.FetchGroupEnded {
				p.startFetchGroup(pc)
			}
			if p.comp.State.FetchGroupCycle > p.comp.State.Cycles {
				return // the next group is fetched in a later cycle
			}
		}
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

		// A fused compare/select pair is admitted whole: the compare waits
		// for the next cycle when its select would not fit in this one.
		selects := pair.pending && pair.pc == pc
		pair.pending = false
		var partner []int
		compares := false
		if !selects {
			partner, compares = p.fusedPartner(inst, pc)
		}
		if compares && (fetched+2 > p.comp.Spec.FetchWidth || p.comp.State.RobCount+2 > p.comp.Spec.ROBCapacity) {
			return
		}
		// A compare that fuses with the branch after it brings that branch
		// into this cycle; the pair waits whole for room under the
		// conditional-branch limit. Only a pair this cycle can deliver whole
		// fuses, and only a producer's form reads the next instruction.
		fuses := false
		if p.comp.Spec.CompareBranchFusion && !fusedThisCycle && !branchFused && !compares && p.deliversBranchPair(inst, pc) {
			next, err := p.comp.Resources.backend().Peek(pc + 4)
			fuses = err == nil && fusesWithBranch(inst, next)
		}
		if limit := p.comp.Spec.ConditionalBranches; limit > 0 && conditionals >= limit &&
			(fuses || (!branchFused && countsAsConditional(inst))) {
			return
		}

		// Admit the actual destination count, including both halves of LDP.
		// A full FP window does not prevent integer or store-only progress.
		gpr, fp := instinfo.DestRegs(inst)
		_, fpSecond := instinfo.DestRegs2(inst)
		neededFP := 0
		if fp >= 0 {
			neededFP++
		}
		if fpSecond >= 0 {
			neededFP++
		}
		if p.comp.State.FpInFlight+neededFP > p.comp.Spec.FpPhysLimit {
			return
		}
		queued := p.comp.Spec.FPIssueQueue > 0 && queuesFP(inst, fps)
		if queued && p.queueFull(&p.comp.State.FpQueue, p.comp.Spec.FPIssueQueue) {
			return
		}
		multiplies := p.comp.Spec.MultiplyIssueQueue > 0 && queuesMultiply(inst.Op)
		if multiplies && p.queueFull(&p.comp.State.MultiplyQueue, p.comp.Spec.MultiplyIssueQueue) {
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
			if p.comp.Spec.StoreDataQueue > 0 {
				if ready, tracked := p.scalarStoreDataReady(inst); tracked {
					if p.queueFull(&p.comp.State.StoreDataQueue, p.comp.Spec.StoreDataQueue) {
						return
					}
					p.holdEntry(&p.comp.State.StoreDataQueue, ready)
				}
			}
		}

		// 2. Compute source operand readiness.
		var srcReady uint64
		for _, gpr := range gprs {
			if p.comp.State.GprReady[gpr] > srcReady {
				srcReady = p.comp.State.GprReady[gpr]
			}
		}
		// The addend of a fused multiply-add is its last FP source; a producer
		// that forwards late lets the instruction issue before it is ready.
		addend, slack := p.accumulateSlack(inst)
		wakeup := &p.comp.State.FAddWakeup
		wakeupEnabled := p.comp.Spec.Latencies.FAddWakeupSpan > 0
		doubleAdd := wakeupEnabled && scalarFAdd64(inst)
		var fraction uint64
		for i, fp := range fps {
			ready := p.comp.State.FpReady[fp]
			if fp == addend && i == len(fps)-1 {
				ready -= min(ready, slack)
			}
			var remainder uint64
			if doubleAdd {
				ready, remainder = p.faddOperandReady(wakeup, fp, ready)
			}
			if ready > srcReady || (ready == srcReady && remainder > fraction) {
				srcReady, fraction = ready, remainder
			}
		}
		// A fused pair issues when every register either half reads is ready.
		for _, gpr := range partner {
			if p.comp.State.GprReady[gpr] > srcReady {
				srcReady = p.comp.State.GprReady[gpr]
			}
		}

		// Flags (NZCV) dependency.
		readsFlags := instinfo.UsesFlags(inst.Op) ||
			(p.comp.Spec.FlagBranchDependency && !branchFused && conditionalBranch(inst))
		if readsFlags && p.comp.State.FlagsReady > srcReady {
			srcReady = p.comp.State.FlagsReady
		}

		// 3. Classify instruction and find execution unit.
		ut := classifyUnit(inst)
		if compares {
			ut = unitSelectFusion
		}
		dispatchCycle := p.comp.State.Cycles
		issueCycle := dispatchCycle
		if srcReady > issueCycle {
			issueCycle = srcReady
		}

		// NOP is eliminated before execution, while retaining frontend, ROB
		// and retirement accounting. Other HINT instructions are not assumed
		// to have the same resource behavior.
		usesUnit := inst.Op != arm64asm.NOP && !branchFused
		if selects {
			// The pair issued once, with its compare. The select reserves
			// nothing and completes with the flags the pair produced.
			issueCycle, usesUnit = pair.complete-1, false
		}
		if branchFused {
			// The pair issued once, with its compare, which also reserved
			// the branch's pipe: the branch resolves in that cycle.
			issueCycle = fused.issue
		}
		if instinfo.IsLoad(inst.Op) {
			// Forwarding eligibility depends on when a load could issue.
			// Other instructions need only the final resource selection below.
			_, issueCycle = p.findUnit(ut, issueCycle, unitOccupancy(inst.Op))
		}
		// A configured divider is timed by the operands, read before they change.
		divide := p.divideLatency(inst)

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
		// With StoreForwardLatency: stores supply accesses of the load, and the
		// data of the last of them issues at storeData.
		var storeSupplied bool
		var storeData, conflictReturn uint64
		// The accesses of a load that the cache supplies: every one of them,
		// unless stores supply some.
		var cached []emu.MemAccess
		if instinfo.IsLoad(inst.Op) {
			accesses := p.comp.Resources.backend().MemoryAccesses()
			scalarReload := p.comp.Spec.ScalarFPReloadPrediction && len(accesses) == 1 && accesses[0].Size == 8 && scalarFPAddress(inst, arm64asm.LDR)
			if p.comp.Spec.ScalarFPReloadPrediction {
				p.observeFPLoadLines(inst, accesses)
			}
			if scalarReload {
				issueCycle, conflictReturn = p.partialReloadWait(pc, accesses[0], issueCycle)
			}
			if p.comp.Spec.StoreDependencePrediction || scalarReload {
				// When the entire load comes from a newer ready store, its producer is known.
				// The older predicted store cannot contribute bytes to that load.
				if !scalarReload || !p.newerReadyProducer(pc, accesses[0], issueCycle) {
					issueCycle = p.predictedStoreWait(pc, accesses, issueCycle)
				}
			}
			if storeData, cached, storeSupplied = p.suppliedLoad(accesses); storeSupplied {
				// A load that stores supply only in part reads the rest from the cache.
				forwarding = loadForwarding{issueCycle: issueCycle, latency: uint64(p.comp.Spec.StoreForwardLatency), forwarded: len(cached) == 0}
			} else if adjacent, ok := p.forwardAdjacentFP(inst, accesses, issueCycle); ok {
				forwarding = adjacent
			} else {
				forwarding = p.forwardLoad(accesses, issueCycle, directGPRTransfer(inst))
				if forwarding.forwarded && scalarReload {
					forwarding = p.trainAndForwardFeedback(accesses[0], issueCycle, forwarding)
				}
				if p.comp.Spec.BypassRecovery > 0 {
					forwarding = p.bypassLoad(inst, pc, accesses, issueCycle, forwarding)
				}
			}
			issueCycle = forwarding.issueCycle
			if p.comp.Spec.StoreDependencePrediction {
				p.trainStoreDependence(pc, accesses)
			} else if scalarReload {
				st := p.youngestOverlap(accesses[0])
				if st != nil && st.Feedback && st.Addr == accesses[0].Addr && st.Size == 8 {
					p.trainStoreDependence(pc, accesses)
				} else {
					delete(p.comp.State.StoreDependence.LoadStore, pc)
				}
			}
		}

		// An indexed load produces its updated base on a load pipe. Store
		// overlap may have delayed issue since the initial forwarding probe.
		base := instinfo.BaseWritebackGPR(inst)
		writesBase := base >= 0 && base < 31 && base != gpr
		indexedLoad := ut == unitLoad && writesBase
		occupancy := p.occupancy(inst, divide)
		addressCycle := issueCycle
		if indexedLoad {
			addressCycle = p.reserveIssue(ut, issueCycle, occupancy)
		}
		issueCycle = addressCycle

		// 6. A cache request waits for translation and (on an L1 miss) a miss
		// entry. An indexed load uses its address slot for an immediate access;
		// a deferred access returns to the same pool of load pipes to issue.
		mshr := -1
		if ut == unitLoad && !forwarding.forwarded {
			issueCycle = p.translated(cached, addressCycle)
			if p.loadMissesL1(cached) {
				mshr, issueCycle = p.acquireMSHR(issueCycle)
			}
		}
		if fuses {
			// A compare/branch pair issues once, on an integer pipe for each
			// half: fusion saves the branch's rename and retire slots only.
			issueCycle = p.reserveIssuePair(ut, issueCycle, occupancy)
		} else if usesUnit && (!indexedLoad || issueCycle != addressCycle) {
			issueCycle = p.reserveIssue(ut, issueCycle, occupancy)
			if p.comp.Spec.L1ReadBankPeriod > 0 && ut == unitLoad && !forwarding.forwarded && !indexedLoad {
				issueCycle = p.replayL1Read(inst, cached, issueCycle)
			}
		}
		if queued {
			p.holdEntry(&p.comp.State.FpQueue, issueCycle)
		}
		if multiplies {
			p.holdEntry(&p.comp.State.MultiplyQueue, issueCycle)
		}
		var latency, dataReady, storeDrain uint64
		if instinfo.IsStore(inst.Op) {
			latency = 3
			dataReady, storeDrain = p.storeTiming(issueCycle)
		} else if forwarding.forwarded {
			latency = forwarding.latency
		} else if divide > 0 {
			latency = divide
		} else {
			latency, dataReady = p.instrLatencyWithPC(inst, pc, cached, issueCycle)
			if p.comp.Spec.ScalarFPReloadPrediction && p.comp.Resources.Cache != nil && len(cached) == 1 && cached[0].Size == 8 &&
				scalarFPAddress(inst, arm64asm.LDR) {
				cfg := p.comp.Resources.Cache.Config()
				late := dataReady > issueCycle+uint64(cfg.L1Latency)
				p.comp.State.observeSourceSet(pc, cached[0].Addr, cfg.L1Size/(cfg.L1LineSize*cfg.L1Ways), cfg.L1LineSize, late)
				p.comp.State.noteFPRefill(pc, cached[0].Addr, issueCycle, dataReady, late)
			}
			if extra := uint64(p.comp.Spec.Latencies.FPLoadExtra); ut == unitLoad && (fp >= 0 || fpSecond >= 0) {
				// What the cache supplies reaches an FP register later.
				latency += extra
				if dataReady > 0 {
					dataReady += extra
				}
			}
		}
		completeCycle := max(issueCycle+latency, conflictReturn)
		if !instinfo.IsStore(inst.Op) && dataReady > completeCycle {
			completeCycle = dataReady
		}
		if forwarding.forwarded && forwarding.dataReady+latency > completeCycle {
			completeCycle = forwarding.dataReady + latency
		}
		if storeSupplied {
			// The forwarding path starts when the load and the store's data
			// have both issued, also when the cache supplies other accesses.
			completeCycle = max(completeCycle, max(issueCycle, storeData)+uint64(p.comp.Spec.StoreForwardLatency))
		}
		if mshr >= 0 {
			p.comp.State.MshrBusyUntil[mshr] = completeCycle
		}
		if branchFused {
			// Both halves of the pair complete together.
			completeCycle = fused.complete
		}
		if compares {
			pair = fusedSelect{pc: pc + 4, complete: completeCycle, pending: true}
		}
		if storeSlot >= 0 {
			drain := completeCycle
			if dataReady > drain {
				drain = dataReady
			}
			if p.comp.Spec.StoreQueueInOrder {
				// Stores leave the queue in program order.
				drain = max(drain, p.comp.State.StoreRelease)
				p.comp.State.StoreRelease = drain
			}
			p.comp.State.StoreDrainUntil[storeSlot] = drain
		}

		// 7. Update destination register readiness.
		if gpr >= 0 && gpr < 31 {
			p.comp.State.GprReady[gpr] = completeCycle
		}
		if fp >= 0 {
			p.comp.State.FpReady[fp] = completeCycle
			if p.comp.Spec.ScalarFPReloadPrediction {
				p.comp.State.FPReload.FeedbackFP[fp] = p.feedbackOrigins(inst)
			}
			if wakeupEnabled {
				wakeup.Producers[fp] = faddProducer{}
				if doubleAdd {
					// A later dispatch or pipe reservation already hid any
					// fractional delay inherited from this operand.
					if issueCycle > srcReady {
						fraction = 0
					}
					wakeup.Producers[fp] = faddProducer{Position: wakeup.Position, Fraction: fraction, Valid: true}
				}
			}
			p.comp.State.FpLateForward[fp] = inst.Op == arm64asm.FMUL || inst.Op == arm64asm.FNMUL || fusedMultiplyAdd(inst.Op)
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
				if p.comp.Spec.ScalarFPReloadPrediction {
					p.comp.State.FPReload.FeedbackFP[fp2] = [2]feedbackOrigin{}
				}
				if wakeupEnabled {
					wakeup.Producers[fp2] = faddProducer{}
				}
				p.comp.State.FpLateForward[fp2] = false
				fpDests++
			}
		}

		// 7b. Track base register writeback for pre/post-indexed addressing.
		// The base writeback (e.g., X22 += 8 in "str D1, [X22], #8") is
		// available after address generation (1 cycle), independent of the
		// instruction's data latency.
		if writesBase {
			if instinfo.IsStore(inst.Op) {
				// For stores, base writeback depends only on the base
				// register, not on the store data operand.
				baseReady := p.comp.State.GprReady[base]
				if dispatchCycle > baseReady {
					baseReady = dispatchCycle
				}
				p.comp.State.GprReady[base] = baseReady + 1
			} else {
				p.comp.State.GprReady[base] = addressCycle + 1
			}
		}

		// Update NZCV flags readiness.
		if instinfo.SetsFlags(inst.Op) {
			p.comp.State.FlagsReady = completeCycle
		}

		// 8. For stores: add to store buffer.
		if instinfo.IsStore(inst.Op) {
			eligible := directGPRTransfer(inst)
			if p.comp.Spec.StoreDependencePrediction || (p.comp.Spec.ScalarFPReloadPrediction && scalarFPAddress(inst, arm64asm.STR)) {
				p.recordStoreInstance(pc, p.comp.Resources.backend().MemoryAccesses(), completeCycle)
			}
			accesses := p.comp.Resources.backend().MemoryAccesses()
			var nextFPReload uint64
			if p.comp.Spec.AdjacentFPForwardLatency > 0 && len(accesses) == 1 && accesses[0].Size == 8 && scalarFPOffset(inst, arm64asm.STR) {
				nextFPReload = p.comp.State.InstDone + uint64(p.comp.State.RobCount) + 1
			}
			var operand addressOperand
			if p.comp.Spec.BypassRecovery > 0 && eligible && len(accesses) == 1 {
				operand = namedAddress(inst)
			}
			for _, acc := range accesses {
				var feedback bool
				var scalarStorePosition uint64
				if p.comp.Spec.ScalarFPReloadPrediction && len(accesses) == 1 && acc.Size == 8 && scalarFPAddress(inst, arm64asm.STR) {
					scalarStorePosition = p.comp.State.InstDone + uint64(p.comp.State.RobCount) + 1
					for _, origin := range p.comp.State.FPReload.FeedbackFP[instinfo.ArgFP(inst.Args[0])] {
						feedback = feedback || (origin.Valid && origin.Address == acc.Addr)
					}
				}
				entry := storeBufferEntry{
					Feedback:            feedback,
					ScalarStorePosition: scalarStorePosition,
					Operand:             operand,
					NextFPReload:        nextFPReload,
					PC:                  pc,
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
		if len(p.comp.State.RobFused) > 0 {
			p.comp.State.RobFused[p.comp.State.RobTail] = branchFused
		}
		p.comp.State.FpInFlight += fpDests
		p.comp.State.RobTail = (p.comp.State.RobTail + 1) % p.comp.Spec.ROBCapacity
		p.comp.State.RobCount++

		// A fused branch shares its compare's rename slot.
		slot := 1
		if branchFused {
			slot = 0
		}
		if grouped {
			if p.comp.State.FetchGroupSize == 0 {
				// The group began rename now; it leaves the fetch buffer.
				renamed := p.comp.State.FetchGroupRenamed
				renamed[(p.comp.State.FetchGroups-1)%uint64(len(renamed))] = p.comp.State.Cycles
			}
			p.comp.State.FetchGroupSize++
		}

		if wakeupEnabled {
			p.advanceFAddWakeup(wakeup, inst, branched)
		}

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
				p.redirect(issueCycle + uint64(p.comp.Spec.BranchPenalty))
				break
			}
			// Correct prediction, taken: BTB provides the target in the
			// same cycle, so the redirect is effectively free on M2
			// (loop stream detector / micro-op cache eliminates the
			// redirect bubble for well-predicted branches). The group ends
			// here unless it may still continue at the target.
			if !grouped {
				if actualTaken && takenLeft == 0 {
					p.comp.State.FrontendReady = p.comp.State.Cycles + 1 // 0-cycle redirect
					fetched += slot
					break
				}
				if actualTaken {
					takenLeft--
				}
			}
		} else if branched {
			// Fallback for non-branch PC changes or nil predictor.
			p.redirect(issueCycle + uint64(p.comp.Spec.BranchPenalty))
			break
		}
		if grouped && !p.continuesFetchGroup(pc, p.comp.Resources.backend().CPURegisters().PC) {
			// The next instruction is behind this one or beyond the window:
			// fetch delivers it in a later group. Rename continues with
			// whatever fetch has already delivered.
			p.comp.State.FetchGroupEnded = true
			if p.comp.Spec.FetchRepeatMin > 0 {
				p.finishRepeatedGroup(pc, p.comp.Resources.backend().CPURegisters().PC)
			}
		}

		fetched += slot
		if countsAsConditional(inst) {
			conditionals++
		}
		if fuses {
			branchPair = fusedBranch{pc: pc + 4, issue: issueCycle, complete: completeCycle, pending: true}
			fusedThisCycle = true
		}
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
	case arm64asm.SDIV, arm64asm.UDIV:
		return unitIntDiv
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
// using the PC to check for hooked BL instructions. cached is as in
// instrLatency.
func (p *middleware) instrLatencyWithPC(inst arm64asm.Inst, pc uint64, cached []emu.MemAccess, issueCycle uint64) (latency uint64, dataReady uint64) {
	// Check for hooked BL instruction.
	if inst.Op == arm64asm.BL && p.hookCycles != nil {
		if pcrel, ok := inst.Args[0].(arm64asm.PCRel); ok {
			target := uint64(int64(pc) + int64(pcrel))
			if cycles, ok := p.hookCycles[target]; ok {
				return uint64(cycles), 0
			}
		}
	}
	return p.instrLatency(inst, cached, issueCycle)
}

// instrLatency computes the execution latency for an instruction.
// dataReady is the absolute cycle at which the cache can deliver the data
// (zero when the cache model is not timed or not present). Loads use the timed
// cache path only after final execution-resource selection, for the accesses
// in cached: those the cache supplies. Stores instead use storeTiming, which
// owns their separate cache admission/drain contract.
func (p *middleware) instrLatency(inst arm64asm.Inst, cached []emu.MemAccess, issueCycle uint64) (latency uint64, dataReady uint64) {
	if instinfo.IsLoad(inst.Op) {
		if p.comp.Resources.Cache != nil {
			if len(cached) == 0 {
				return uint64(p.comp.Spec.LoadLatency), 0
			}
			worst := mem.HitL1
			for _, acc := range cached {
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
			case mem.HitSLC:
				return uint64(cfg.SLCLatency), dataReady
			case mem.HitDRAM:
				return uint64(cfg.DRAMLatency), dataReady
			default:
				return uint64(cfg.L1Latency), dataReady
			}
		}
		return uint64(p.comp.Spec.LoadLatency), 0
	}
	return p.executeLatency(inst), 0
}

// translated returns when the address translations of a load's
// cache-supplied accesses are available to an access whose address is known
// at issueCycle. Without a cache, or with translation timing disabled, that is
// issueCycle.
func (p *middleware) translated(cached []emu.MemAccess, issueCycle uint64) uint64 {
	cache := p.comp.Resources.Cache
	if cache == nil {
		return issueCycle
	}
	ready := issueCycle
	for _, acc := range cached {
		ready = max(ready, cache.Translate(acc.Addr, acc.Size, issueCycle))
	}
	return ready
}

// loadMissesL1 only classifies current dispatch-order cache residency of a
// load's cache-supplied accesses.
func (p *middleware) loadMissesL1(cached []emu.MemAccess) bool {
	if p.comp.Resources.Cache == nil {
		return false
	}
	lineSize := p.comp.Resources.Cache.Config().L1LineSize
	for _, acc := range cached {
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
