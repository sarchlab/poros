# M2Sim Roadmap

## Project Goal
Build a cycle-accurate, execution-driven Apple M2 CPU simulator on Akita (Go) that runs the full PolyBench/C suite with ≤20% average time estimation error (max 50% per benchmark).

## Architecture Overview

The simulator consists of:
1. **Functional Emulator**: ARM64 instruction decoder + interpreter (subset sufficient for PolyBench)
2. **Timing Model**: Akita-based cycle-accurate pipeline, OoO engine, memory hierarchy, branch predictor
3. **Benchmark Harness**: PolyBench compilation, native reference timing, comparison framework

## Milestones

### M1: Project Scaffolding + Polybench Baselines
**Status:** COMPLETE (cycle 1)
**Budget:** 4 cycles | **Actual:** 1 cycle
**Scope:**
- Go module setup with Akita v4 dependency
- PolyBench/C 4.2.1 cloned and compiled natively on M2
- Collect native reference timings for all 30 benchmarks (MINI + SMALL datasets)
- Basic project directory structure
- ARM64 binary loading (parse Mach-O, extract code/data sections)
- Trivial Akita "hello world" component that compiles and runs
- Disassemble a simple ARM64 binary using `golang.org/x/arch/arm64/arm64asm`

### M2: ARM64 Functional Emulator (Core Subset)
**Status:** COMPLETE (3 cycles)
**Budget:** 8 cycles | **Actual:** 3 cycles
**Scope:**
- ARM64 instruction decoder covering PolyBench instruction mix:
  - Integer arithmetic (ADD, SUB, MUL, SDIV, UDIV, MADD, shifts, bitfield)
  - Logic (AND, ORR, EOR, BIC, MOV, MVN)
  - Load/Store (LDR, STR, LDP, STP — scalar and FP/SIMD)
  - Branches (B, BL, BR, BLR, RET, B.cond, CBZ, CBNZ, TBZ, TBNZ)
  - FP arithmetic (FADD, FSUB, FMUL, FDIV, FMADD, FMSUB, FCMP, FCSEL)
  - SIMD basics (needed if compiler auto-vectorizes)
  - System (SVC for exit, NOP)
- Register file: 31 GPRs + SP + PC + NZCV flags + 32 FP/SIMD registers
- Flat memory model (no virtual memory needed for statically linked binaries)
- Can execute a simple PolyBench benchmark (e.g., `mvt` with MINI dataset) and produce correct output
- Test harness: compare emulator output vs native output

### M3: Basic In-Order Pipeline Timing Model
**Status:** COMPLETE (2 cycles)
**Budget:** 6 cycles | **Actual:** 2 cycles
**Scope:**
- 5-stage in-order pipeline as Akita TickingComponent (Fetch/Decode/Execute/Memory/Writeback)
- Register scoreboard for data hazard detection (stalls on RAW dependencies)
- Always-not-taken branch policy with flush penalty on taken branches
- Instruction latency table covering all 93+ emulator opcodes
- Fixed 4-cycle load latency, 1-cycle store
- Pipeline-emulator integration via StepDetailed()
- 3 PolyBench kernels (atax, mvt, gemm) running through pipeline with correct output
- Comparison table: atax=2.6x, mvt=4.8x, gemm=7.2x (all within 10x target)

### M4: Cache Hierarchy + Memory Access Tracking
**Status:** COMPLETE (2 cycles)
**Budget:** 5 cycles | **Actual:** 2 cycles
**Scope:**
- Memory access tracking in emulator (`Memory.Accesses` + `TrackAccesses` flag)
- L1D cache: 128KB/8-way/64B lines, LRU replacement
- L2 cache: 16MB/16-way/128B lines, LRU replacement
- Hit/miss latency: L1=3, L2=18, DRAM=335 cycles
- Pipeline Memory stage uses cache `Lookup()` for variable latency
- Scoreboard flush bug fixed (don't decrement for un-issued decode instructions)
- FP STP source tracking fixed (check both Args[0] and Args[1])
- Instruction fetch cache pollution fixed (use `readByteRaw` for fetch, not `ReadU8`)
- 3 new kernels extracted: 2mm, 3mm, bicg (total: 6 kernels)
- L1 hit rates 97.7-99.6% (MINI datasets fit in cache as expected)
- CPI: atax=4.75, mvt=4.91, gemm=3.46, 2mm=3.44, 3mm=3.23, bicg=5.51

### M5: Superscalar + Out-of-Order Execution
**Status:** COMPLETE (2 cycles)
**Budget:** 8 cycles | **Actual:** 2 cycles
**Scope:**
- Replace 5-stage in-order pipeline with OoO superscalar timing model
- **Key design**: In-order functional execution (emulator runs in program order via `StepDetailed()`), OoO timing model tracks when instructions *would* issue/complete
- Frontend: 8-wide fetch, 8-wide decode/rename
- Backend: Reservation stations → execution units → ROB retire
- Execution units: 6 INT + 4 LD/ST + 4 FP/SIMD ports (M2 Avalanche core)
- ROB: 330 entries, in-order retirement
- Register readiness tracking (gprReady[32]/fpReady[32]) for ILP extraction
- Conservative load-store ordering (loads wait for all prior stores)
- Cache-integrated load latency
- All 6 existing kernels produce correct output
- **Results: OoO CPI 1.47-1.95 (target 1.0-2.0 ✓), speedup 1.53-1.99x over in-order**

### M6: Branch Predictor
**Status:** COMPLETE (3 cycles)
**Budget:** 4 cycles | **Actual:** 3 cycles
**Scope:**
- TAGE-like predictor: bimodal base (4096 entries, 2-bit counters) + tagged table (2048 entries, 3-bit counters)
- BTB: 2048 entries, 4-way set-associative
- RAS: 32-entry circular buffer for call/return prediction
- Global history register (uint64 shift register)
- 13-cycle misprediction penalty, integrated into OoO pipeline
- **Results:** BP accuracy 93.1-97.5%, CPI improvement up to 31.1% (3mm), 12.6% (2mm), 9.6% (mvt)
- Current sim/native ratios: atax=2.2x, mvt=4.4x, gemm=4.0x, 2mm=2.4x, 3mm=2.0x, bicg=2.9x

### M7: Full Benchmark Coverage
**Status:** COMPLETE (2 cycles)
**Budget:** 8 cycles | **Actual:** 2 cycles
**Scope:**
- All 30 PolyBench benchmarks extracted as Go test functions
- 4 new ARM64 instructions: FCVT, SXTL, MOVI.2D, STP/LDP Q-register
- All 30 produce correct output (maxRelErr ≤ 3.96e-16)
- SMALL dataset sizes for ComparisonTable timing comparison
- ComparisonTable runs all 30 benchmarks (~19 min)
- Sim/native ratios range 0.4x-8.5x (need calibration)
- BP accuracy 94.4%-99.5% across all 30 benchmarks

### M8: Calibration + Accuracy Tuning
**Status:** DEADLINE MISSED (8 cycles used, target not met)
**Budget:** 8 cycles | **Actual:** 8 cycles (incomplete)
**Progress:**
- Address-based load-store disambiguation ✓ (store buffer)
- Store-to-load forwarding ✓
- Native time re-validation (7-run medians) ✓ (trisolv, jacobi2d, fdtd2d, syrk fixed)
- Parameter tuning: ROB 330→256, FetchWidth 8→3, BranchPenalty 13→17, various latencies ✓
- Error metrics in ComparisonTable + QuickEval (6-benchmark fast test) ✓
- Next-line prefetcher, MSHR limits, taken-branch redirect penalty ✓
- **Final results: Avg error 46.6% (target ≤20%), Max error 236.1% (target ≤50%)**
- 10/30 benchmarks ≤20% error, 21/30 ≤50% error
- Improved from ~57.8% to 46.6% avg error over 8 cycles

**Post-mortem — Why M8 missed deadline:**
1. **Compensating parameter tuning instead of structural fixes**: FetchWidth was reduced from 8→3 (unrealistic) to globally slow down the sim, creating artificial IPC ceiling of 3.0. Real M2 achieves IPC 4+ on some benchmarks.
2. **No per-benchmark root cause analysis**: The 5 worst outliers (>100% error) contribute ~50% of total error but weren't individually investigated until late.
3. **Slow feedback loop**: Full ComparisonTable takes 20 min. QuickEval added late.
4. **Structural model gaps**: Pipeline has no frontend depth (fetch→issue is instant), L1 latency too high (5cy vs real 3-4cy), FetchWidth too low (3 vs real 6-8).

### M8.1: Pipeline Structural Fixes — Fix Top Outliers
**Status:** NEXT
**Budget:** 4 cycles
**Target:** Max error ≤ 120%, Avg error ≤ 40%
**Scope:**
- **Root cause analysis** for 5 worst benchmarks: jacobi2d (236%), trisolv (156%), nussinov (109%), fdtd2d (100%), seidel2d (95%)
- **Fix FetchWidth**: Increase from 3 to 6 (realistic M2 value). The current value caps IPC at 3.0, preventing the sim from achieving realistic throughput on high-ILP benchmarks like jacobi2d (real IPC ~4.0) and nussinov (real IPC ~4.0)
- **Fix L1 latency**: Reduce from 5cy to 3-4cy (real M2 value). This directly reduces FADD chain critical path in stencils.
- **Add compensating mechanism**: FetchWidth increase will make sim-too-fast benchmarks worse unless compensated. Options: add frontend pipeline depth (decode→issue latency), increase taken-branch redirect cost, or add decode bandwidth limit separate from FetchWidth.
- **Native timing for small benchmarks**: Re-measure sub-50µs benchmarks (trisolv 24µs, jacobi1d 14µs, gesummv 39µs, atax 41µs) with 101+ runs
- Do NOT use global parameter tuning as the primary approach

### M8.2: Average Error Reduction — Target ≤20% avg, ≤50% max
**Status:** Not started
**Budget:** 4 cycles
**Target:** Avg error ≤ 20%, Max error ≤ 50% (project success criteria)
**Scope:**
- Fix remaining >50% error benchmarks
- Improve 20-50% error benchmarks toward ≤20%
- Final parameter calibration after structural fixes are stable
- May need per-benchmark investigation for stubborn outliers

### M9: Polish + Final Validation
**Status:** Not started
**Budget:** 4 cycles
**Scope:**
- Fix remaining outlier benchmarks if M8.2 doesn't fully converge
- Code cleanup
- Final validation pass
- Performance optimization of the simulator itself (simulation speed)

## Total Estimated Budget: ~55 cycles | **Spent so far: ~23 cycles (M1:1 + M2:3 + M3:2 + M4:2 + M5:2 + M6:3 + M7:2 + M8:8)**

## Lessons Learned

### M1
- Completed in 1 cycle (budgeted 4) — team was effective with parallel workers
- macOS lacks `timeout` command — use `perl -e 'alarm shift; exec @ARGV'` as workaround
- MINI/SMALL datasets produce microsecond-range timings — noise is significant even with median of 3 runs
- PolyBench timing output goes to stdout (not stderr)
- Akita v4 API: `sim.NewTickingComponent(name, engine, freq, ticker)` + `Tick() bool` pattern
- Fat/universal Mach-O handling needed (e.g., /usr/bin/* are often universal)

### Instruction Analysis (for M2)
- Disassembled 10 PolyBench benchmarks: ~60 unique ARM64 instructions identified
- Compiler auto-vectorizes some kernels (mvt uses 40x FMLA + 40x LD1R)
- **Recommendation: compile with `-fno-vectorize -fno-slp-vectorize`** to eliminate SIMD for initial emulator, reducing scope significantly
- Top instructions by frequency: MOV, ADD, STP, LDR, STR, LDP, B, BL, FADD, FDIV, FMADD, FMLA
- Key FP instructions needed: FADD, FSUB, FMUL, FDIV, FMADD, FMSUB, FSQRT, SCVTF, UCVTF

### M2
- Completed in 3 cycles (budgeted 8) — Kai built full emulator + e2e test efficiently
- 53+ ARM64 instructions implemented, 100 tests, kernel_atax e2e passing
- Function-level emulation approach works well — no need for full binary/syscall support
- arm64asm library is solid for decoding; some aliases (CMP→SUBS, MOV→ORR) need careful handling
- UBFIZ and MOVI needed beyond initial instruction list — expect more such gaps in future kernels
- PolyBench recompiled with -fno-vectorize: 7 benchmarks spot-checked, zero SIMD

### M3
- Completed in 2 cycles (budgeted 6) — Kai built pipeline, Nora extracted additional kernels in parallel
- 5-stage pipeline with scoreboard, branch flush, multi-cycle execute all working
- CPI ~3.1-3.2 for PolyBench kernels — realistic for in-order with 4-cycle loads and FP latencies
- Sim/native ratios (2.6x-7.2x) show simulator overestimates time, expected since M2 is OoO superscalar
- Minor bugs found by Apollo verification:
  - Scoreboard flush bug: `clearScoreboard()` decrements for un-issued decode instruction after taken branch (timing accuracy only, not correctness)
  - Store latency not configurable (hardcoded 1 cycle)
  - Latency table is a function, not configurable struct/map
  - FP STP source tracking incomplete in regs.go
- `unsafe.Pointer` usage in regs.go for accessing arm64asm internals — fragile across library versions
- Key insight: For M4 cache integration, need to add memory access tracking to the emulator since it currently accesses `em.Mem` directly in executor functions with no visibility from the pipeline

### M4
- Completed in 2 cycles (budgeted 5) — parallel workers effective (Leo: cache model, Kai: integration + bug fixes, Nora: kernel extraction)
- Cache model is synchronous (no Akita ports) — simple `Lookup()` call, correct design choice for in-order pipeline
- Instruction fetch was polluting D-cache: fixed by using raw byte read for fetch stage
- MINI dataset CPI: 3.2-5.5x — overestimation dominated by in-order pipeline, not cache misses
- L1 hit rates 97.7-99.6% confirm MINI datasets fit in cache — cache model mainly matters for larger datasets
- Key insight for M5: the 3-5x overestimation gap is almost entirely due to in-order single-issue vs real M2's OoO superscalar. M5 (OoO) is the critical accuracy milestone
- Architecture note: `StepDetailed()` returns (inst, branched, err) and the pipeline calls it in Execute stage. For OoO, the emulator still executes in program order but the timing model tracks overlapped execution

### M5
- Completed in 2 cycles (budgeted 8) — Kai built OoO pipeline, Nora wrote comprehensive tests
- OoO CPI: atax=1.80, mvt=1.95, gemm=1.58, 2mm=1.62, 3mm=1.73, bicg=1.47 (all within 1.0-2.0 target)
- Speedup over in-order: 1.53x-1.99x — realistic for OoO over in-order with MINI datasets
- Design: instructions still execute in program order via StepDetailed(), timing model tracks overlapped execution
- Simplified register renaming (readiness tracking per arch reg) works well enough
- Conservative memory ordering (loads wait for all stores) — correct but pessimistic
- 142 total tests, all passing
- Minor gaps: no explicit ROB-full test, bicg native time in test off from timing.csv
- Sim/native ratios still up to 4.9x — branch predictor (M6) and calibration (M7) needed

### M6
- Completed in 3 cycles (budgeted 4) — Kai built predictor in 1 cycle, Nora wrote tests in 2 cycles
- TAGE-like predictor with bimodal + tagged table works well for PolyBench loops
- BP accuracy 93-97.5% — high because PolyBench has very regular control flow
- BP improvement varies: 31% for 3mm (many branches), ~0% for atax/gemm/bicg (few mispredictions)
- taggedTag() uses only PC, not GHR — minor deviation from true TAGE, doesn't matter for these benchmarks
- Sim/native ratios still 2.0-4.4x — remaining gap is primarily from: (1) conservative memory ordering, (2) parameter calibration, (3) MINI dataset timing noise
- 2 unit tests are trivially passable by always-taken predictor — tagged table weakly tested in isolation
- **Key insight for M7**: The 2-4x gap won't close from microarchitectural features alone. Need to expand to all 30 benchmarks and systematically calibrate parameters.

### M7
- Completed in 2 cycles (budgeted 8) — parallel worker batches very effective (8 workers across 2 cycles)
- 24 new benchmarks + SMALL dataset migration done in parallel batches of 3 benchmarks each
- 4 new instructions needed: FCVT (float precision convert), SXTL (sign extend long), MOVI.2D (vector zero), STP/LDP Q-register (128-bit pair load/store)
- Sim/native ratios wildly inconsistent: 0.4x (bicg) to 8.5x (gemm). Some benchmarks *underestimate* time.
- Native times are hardcoded constants (from M1 measurements), not measured at runtime. Many are 15-500 microseconds — likely noisy.
- **Key insight for M8**: Two separate problems: (1) simulator timing model accuracy (memory ordering, parameters), (2) native reference time accuracy (measurement noise at microsecond scale). Must fix both.
- ComparisonTable takes ~19 minutes for all 30 benchmarks at SMALL — need -timeout 30m flag
- Individual tests use MINI for fast correctness checks, ComparisonTable uses SMALL for timing comparison — good separation of concerns

### M8 (partial)
- Used 8 cycles (budgeted 8) — milestone NOT met. Reduced avg error from ~57.8% to 46.6% and max from ~300% to 236%
- **Biggest win**: Taken-branch redirect penalty reduced avg error by 10.5 percentage points in one change
- **Biggest misfire**: FetchWidth 8→3 was a compensation hack that caps IPC at 3.0. Real M2 achieves IPC 4+. This helped sim-too-fast benchmarks but fundamentally limited accuracy for sim-too-slow ones.
- **Prefetcher and L2 latency reduction had zero impact** because cache warm-up pre-populates L1 (97-100% hit rates)
- Parameter tuning hit diminishing returns after cycle 4-5. Remaining errors require structural model changes, not parameter knob turning.
- **Root cause of remaining error**: (1) FetchWidth=3 caps cross-iteration parallelism for stencils, (2) L1 latency 5cy is too high (real M2 is 3-4cy), (3) No frontend pipeline depth makes sim systematically too fast, compensated by artificially low FetchWidth
- **Key error decomposition**: 5 benchmarks >100% error contribute 50% of total error. 9 benchmarks >50% error contribute 67%. Must do per-benchmark root cause analysis, not global tuning.
- Current pipeline params: FetchWidth=3, RetireWidth=8, ROB=256, NumINT=6, NumFP=4, NumLDST=3, BranchPenalty=17, L1=5cy, L2=14cy, DRAM=335cy, StoreBuffer=56, MSHR=12
- **Process lesson**: QuickEval (130s, 6 benchmarks) was much more useful for iteration than full ComparisonTable (20min, 30 benchmarks). Should have been built earlier.
- **Process lesson**: Sequential tune→measure→tune loop was slow. Need parallel investigation of specific outliers.

## Key Risks
1. **ARM64 emulator complexity**: Even a subset of ARM64 is large. Compiler may emit unexpected instructions.
2. **Accuracy vs complexity tradeoff**: Full OoO modeling is hard. May need to iterate on what level of detail is needed for 20% accuracy.
3. **Akita learning curve**: No existing CPU simulator on Akita — we're pioneering.
4. **Simulation speed**: Cycle-accurate simulation of LARGE dataset benchmarks may be too slow. May need to use MINI/SMALL datasets.
5. **Calibration difficulty**: Many microarchitectural parameters are estimates from reverse engineering.
