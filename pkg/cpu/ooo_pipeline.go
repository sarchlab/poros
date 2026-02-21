package cpu

import (
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/m2sim3/pkg/emu"
	"github.com/sarchlab/m2sim3/pkg/mem"
	"golang.org/x/arch/arm64/arm64asm"
)

type storeBufferEntry struct {
	addr          uint64
	size          int
	completeCycle uint64
}

type unitType int

const (
	unitINT  unitType = iota
	unitFP
	unitLDST
)

// OoOPipeline is an out-of-order superscalar timing model.
//
// Instructions are executed in program order via StepDetailed(), but the
// timing model tracks OoO scheduling to compute accurate cycle counts.
type OoOPipeline struct {
	*sim.TickingComponent

	Emu        *emu.Emulator
	Cache      *mem.Cache          // optional, nil = fixed latency
	BranchPred *BranchPredictor    // nil = always-not-taken (backward compatible)

	// Configuration (set defaults in constructor).
	FetchWidth    int // 6
	RetireWidth   int // 8
	ROBCapacity   int // 256
	NumINT        int // 6
	NumFP         int // 4
	NumLDST       int // 4
	BranchPenalty int // 15
	LoadLatency   int // 4 (used when Cache is nil)

	// Register readiness: cycle when value is available (0 = available now).
	gprReady   [32]uint64
	fpReady    [32]uint64
	flagsReady uint64 // NZCV condition flags readiness

	// Execution unit availability: next cycle each unit can accept new work.
	intUnits  []uint64 // len = NumINT
	fpUnits   []uint64 // len = NumFP
	ldstUnits []uint64 // len = NumLDST

	// ROB (circular buffer).
	rob      []uint64 // completeCycle per entry
	robHead  int
	robTail  int
	robCount int

	// Store buffer for address-based load-store disambiguation.
	storeBuffer     []storeBufferEntry
	storeBufferSize int // default 56 (Apple M2 estimate)

	// MSHR (Miss Status Holding Registers) — limits outstanding L1 misses.
	mshrBusyUntil []uint64 // len = mshrLimit; each entry = cycle when MSHR frees
	mshrLimit     int      // default 12 (Apple M2 estimate: 12-16)

	// Frontend.
	frontendReady uint64 // cycle when frontend can fetch next batch

	// Statistics.
	Cycles   uint64
	InstDone uint64

	// State.
	done bool
	Err  error
}

// NewOoOPipeline creates an OoO superscalar pipeline wrapping the given emulator.
func NewOoOPipeline(
	name string,
	engine sim.Engine,
	freq sim.Freq,
	emulator *emu.Emulator,
) *OoOPipeline {
	emulator.Mem.TrackAccesses = true
	p := &OoOPipeline{
		Emu:             emulator,
		FetchWidth:      3,
		RetireWidth:     8,
		ROBCapacity:     256,
		NumINT:          6,
		NumFP:           4,
		NumLDST:         4,
		BranchPenalty:   17,
		LoadLatency:     5,
		storeBufferSize: 56,
		mshrLimit:       12,
	}
	p.intUnits = make([]uint64, p.NumINT)
	p.fpUnits = make([]uint64, p.NumFP)
	p.ldstUnits = make([]uint64, p.NumLDST)
	p.rob = make([]uint64, p.ROBCapacity)
	p.storeBuffer = make([]storeBufferEntry, 0, p.storeBufferSize)
	p.mshrBusyUntil = make([]uint64, p.mshrLimit)
	p.BranchPred = NewBranchPredictor()
	p.TickingComponent = sim.NewTickingComponent(name, engine, freq, p)
	return p
}

// CPI returns cycles per instruction.
func (p *OoOPipeline) CPI() float64 {
	if p.InstDone == 0 {
		return 0
	}
	return float64(p.Cycles) / float64(p.InstDone)
}

// Tick advances the pipeline by one cycle.
func (p *OoOPipeline) Tick() bool {
	if p.done {
		return false
	}
	p.Cycles++

	// Phase 1: Retire completed instructions from ROB head.
	p.retire()

	// Phase 2: Fetch, decode, dispatch, issue new instructions.
	p.fetchAndProcess()

	// Check termination: ROB drained and emulator halted.
	if p.robCount == 0 && p.Emu.Halted {
		p.done = true
	}

	return !p.done
}

// retire walks the ROB from head, retiring entries where
// completeCycle <= currentCycle, up to RetireWidth per cycle.
func (p *OoOPipeline) retire() {
	retired := 0
	for retired < p.RetireWidth && p.robCount > 0 {
		if p.rob[p.robHead] > p.Cycles {
			break // head entry not yet complete
		}
		p.InstDone++
		p.robHead = (p.robHead + 1) % p.ROBCapacity
		p.robCount--
		retired++
	}
}

// fetchAndProcess fetches up to FetchWidth instructions, decodes, dispatches,
// and functionally executes each one.
func (p *OoOPipeline) fetchAndProcess() {
	if p.Cycles < p.frontendReady {
		return // frontend stalled from branch penalty
	}

	fetched := 0
	for fetched < p.FetchWidth && p.robCount < p.ROBCapacity && !p.Emu.Halted {
		// 1. Peek at instruction at emulator's current PC.
		pc := p.Emu.Regs.PC
		var buf [4]byte
		for i := range buf {
			buf[i] = p.Emu.Mem.ReadU8(pc + uint64(i))
		}
		inst, err := arm64asm.Decode(buf[:])
		if err != nil {
			break // end of program or garbage
		}

		// 2. Compute source operand readiness.
		var srcReady uint64
		for _, gpr := range sourceGPRs(inst) {
			if p.gprReady[gpr] > srcReady {
				srcReady = p.gprReady[gpr]
			}
		}
		for _, fp := range sourceFPs(inst) {
			if p.fpReady[fp] > srcReady {
				srcReady = p.fpReady[fp]
			}
		}

		// Flags (NZCV) dependency.
		if UsesFlags(inst.Op) && p.flagsReady > srcReady {
			srcReady = p.flagsReady
		}

		// 3. Classify instruction and find execution unit.
		ut := classifyUnit(inst.Op)
		dispatchCycle := p.Cycles
		issueCycle := dispatchCycle
		if srcReady > issueCycle {
			issueCycle = srcReady
		}

		bestUnit, bestCycle := p.findUnit(ut, issueCycle)
		issueCycle = bestCycle

		// 4. Execute functionally (in program order).
		_, branched, execErr := p.Emu.StepDetailed()
		if execErr != nil {
			p.Err = execErr
			p.done = true
			return
		}

		// 5. Address-based load-store disambiguation (after StepDetailed
		//    so actual addresses are available in p.Emu.Mem.Accesses).
		forwarded := false
		if IsLoad(inst.Op) {
			accesses := p.Emu.Mem.Accesses
			allForwarded := len(accesses) > 0
			for _, acc := range accesses {
				covered := false
				for _, sb := range p.storeBuffer {
					if acc.Addr < sb.addr+uint64(sb.size) &&
						sb.addr < acc.Addr+uint64(acc.Size) {
						// Address overlap: load must wait for this store.
						if sb.completeCycle > issueCycle {
							issueCycle = sb.completeCycle
						}
						// Check if the store fully covers this load access.
						if sb.addr <= acc.Addr &&
							acc.Addr+uint64(acc.Size) <= sb.addr+uint64(sb.size) {
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
			latency, miss = p.instrLatency(inst)
			// MSHR: if load missed L1, acquire an MSHR slot (may stall).
			if miss {
				mshrIdx, mshrCycle := p.acquireMSHR(issueCycle)
				issueCycle = mshrCycle
				p.mshrBusyUntil[mshrIdx] = issueCycle + latency
			}
		}
		completeCycle := issueCycle + latency

		// 7. Update destination register readiness.
		gpr, fp := destRegs(inst)
		if gpr >= 0 && gpr < 31 {
			p.gprReady[gpr] = completeCycle
		}
		if fp >= 0 {
			p.fpReady[fp] = completeCycle
		}
		if gpr2, fp2 := destRegs2(inst); gpr2 >= 0 || fp2 >= 0 {
			if gpr2 >= 0 {
				p.gprReady[gpr2] = completeCycle
			}
			if fp2 >= 0 {
				p.fpReady[fp2] = completeCycle
			}
		}

		// Update NZCV flags readiness.
		if SetsFlags(inst.Op) {
			p.flagsReady = completeCycle
		}

		// 8. For stores: add to store buffer.
		if IsStore(inst.Op) {
			for _, acc := range p.Emu.Mem.Accesses {
				entry := storeBufferEntry{
					addr:          acc.Addr,
					size:          acc.Size,
					completeCycle: completeCycle,
				}
				if len(p.storeBuffer) >= p.storeBufferSize {
					// Store buffer full — stall frontend until oldest
					// store retires (its completeCycle).
					stallUntil := p.storeBuffer[0].completeCycle + 1
					if stallUntil > p.frontendReady {
						p.frontendReady = stallUntil
					}
					p.storeBuffer = p.storeBuffer[1:]
				}
				p.storeBuffer = append(p.storeBuffer, entry)
			}
		}

		// 9. Mark execution unit busy.
		p.markUnitBusy(ut, bestUnit, issueCycle, inst.Op)

		// 10. Add to ROB.
		p.rob[p.robTail] = completeCycle
		p.robTail = (p.robTail + 1) % p.ROBCapacity
		p.robCount++

		// 11. Handle branch.
		if IsBranch(inst.Op) && p.BranchPred != nil {
			isCall := inst.Op == arm64asm.BL || inst.Op == arm64asm.BLR
			isReturn := inst.Op == arm64asm.RET

			// Simulate the prediction that would have happened at fetch time.
			predTaken, predTarget := p.BranchPred.Predict(pc, isCall, isReturn)

			// Actual outcome from StepDetailed.
			actualTaken := branched
			actualTarget := p.Emu.Regs.PC

			// Update predictor with actual outcome.
			p.BranchPred.Update(pc, actualTaken, actualTarget, predTaken, isCall, isReturn)

			// Check if prediction was correct.
			mispredicted := (predTaken != actualTaken) ||
				(predTaken && predTarget != actualTarget)

			if mispredicted {
				p.frontendReady = issueCycle + uint64(p.BranchPenalty)
				break
			}
			// Correct prediction: taken branch still costs ~1 cycle for
			// BTB redirect of the instruction fetch unit.
			if actualTaken {
				p.frontendReady = p.Cycles + 2 // 1-cycle redirect bubble
				fetched++
				break
			}
		} else if branched {
			// Fallback for non-branch PC changes or nil predictor.
			p.frontendReady = issueCycle + uint64(p.BranchPenalty)
			break
		}

		fetched++
	}
}

// classifyUnit determines which execution unit type an instruction needs.
func classifyUnit(op arm64asm.Op) unitType {
	if IsLoad(op) || IsStore(op) {
		return unitLDST
	}
	switch op {
	case arm64asm.FADD, arm64asm.FSUB, arm64asm.FMUL,
		arm64asm.FMADD, arm64asm.FMSUB,
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
func (p *OoOPipeline) findUnit(ut unitType, earliest uint64) (int, uint64) {
	var units []uint64
	switch ut {
	case unitINT:
		units = p.intUnits
	case unitFP:
		units = p.fpUnits
	case unitLDST:
		units = p.ldstUnits
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
// Most units are pipelined (1-cycle throughput). DIV/SQRT are non-pipelined.
func (p *OoOPipeline) markUnitBusy(ut unitType, idx int, issueCycle uint64, op arm64asm.Op) {
	busyUntil := issueCycle + 1 // pipelined: 1-cycle throughput
	if op == arm64asm.SDIV || op == arm64asm.UDIV ||
		op == arm64asm.FDIV || op == arm64asm.FSQRT {
		busyUntil = issueCycle + uint64(ExecuteLatency(op)) // non-pipelined
	}
	switch ut {
	case unitINT:
		p.intUnits[idx] = busyUntil
	case unitFP:
		p.fpUnits[idx] = busyUntil
	case unitLDST:
		p.ldstUnits[idx] = busyUntil
	}
}

// instrLatency computes the execution latency for an instruction.
// For loads, l1Miss indicates whether the load missed L1 (needs MSHR).
func (p *OoOPipeline) instrLatency(inst arm64asm.Inst) (latency uint64, l1Miss bool) {
	if IsLoad(inst.Op) {
		if p.Cache != nil {
			accesses := p.Emu.Mem.Accesses
			if len(accesses) == 0 {
				return uint64(p.LoadLatency), false
			}
			worst := mem.HitL1
			for _, acc := range accesses {
				level := p.Cache.Lookup(acc.Addr, acc.Size, acc.IsWrite)
				if level > worst {
					worst = level
				}
			}
			cfg := p.Cache.Config()
			switch worst {
			case mem.HitL2:
				return uint64(cfg.L2Latency), true
			case mem.HitDRAM:
				return uint64(cfg.DRAMLatency), true
			default:
				return uint64(cfg.L1Latency), false
			}
		}
		return uint64(p.LoadLatency), false
	}
	if IsStore(inst.Op) {
		if p.Cache != nil {
			for _, acc := range p.Emu.Mem.Accesses {
				p.Cache.Lookup(acc.Addr, acc.Size, acc.IsWrite)
			}
		}
		return 3, false
	}
	return uint64(ExecuteLatency(inst.Op)), false
}

// acquireMSHR finds the earliest-available MSHR slot. If all MSHRs are busy
// past issueCycle, the load is delayed. Returns (mshrIndex, adjustedIssueCycle).
func (p *OoOPipeline) acquireMSHR(issueCycle uint64) (int, uint64) {
	bestIdx := 0
	bestTime := p.mshrBusyUntil[0]
	for i := 1; i < len(p.mshrBusyUntil); i++ {
		if p.mshrBusyUntil[i] < bestTime {
			bestIdx = i
			bestTime = p.mshrBusyUntil[i]
		}
	}
	if bestTime > issueCycle {
		return bestIdx, bestTime
	}
	return bestIdx, issueCycle
}
