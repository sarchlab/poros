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
**Status:** Not started
**Budget:** 5 cycles
**Scope:**
- Add memory access tracking to the emulator (record address, size, read/write for each load/store instruction)
- Simple L1D cache model: 128KB, 8-way set-associative, 64B lines, LRU replacement
- L2 cache model: 16MB, 16-way, 128B lines, LRU replacement
- Hit/miss latency: L1 hit=3 cycles, L1 miss + L2 hit=18 cycles, L2 miss (DRAM)=335 cycles (~96ns at 3.49GHz)
- L1 Instruction Cache: model as ideal (always hit, 1 cycle) — instruction working set is small
- Pipeline Memory stage uses cache model to determine actual latency per access
- Fix scoreboard flush bug (decrement without matching increment on taken branches)
- Extract 3+ additional PolyBench kernels to increase test coverage
- Run all kernels through pipeline and report comparison table
- **Target: memory-bound benchmarks should show realistic slowdown vs compute-bound**
- **Note: MINI datasets likely fit in L1, so cache effect may be small; this builds infrastructure for larger datasets later**

### M5: Superscalar + Out-of-Order Execution
**Status:** Not started
**Budget:** 8 cycles
**Scope:**
- Widen pipeline: 8-wide fetch/decode, up to 14-wide issue
- Execution unit modeling: 6 INT + 4 LD/ST + 4 FP/SIMD ports
- Instruction scheduling / reservation stations
- ROB (~300 entries)
- Register renaming
- Load/store queue
- Instruction elimination (MOV, NOP)
- **Target: get within 2x of real execution time**

### M6: Branch Predictor
**Status:** Not started
**Budget:** 4 cycles
**Scope:**
- TAGE-like predictor (simplified: 4-6 tables, ~16K total entries)
- BTB (~2K entries, 4-way)
- Branch misprediction recovery (~13 cycle penalty)
- Return address stack
- **Target: control-flow-heavy benchmarks (floyd-warshall, nussinov) should improve significantly**

### M7: Calibration + Full Benchmark Suite
**Status:** Not started
**Budget:** 8 cycles
**Scope:**
- Run all 30 PolyBench benchmarks through simulator
- Compare vs native timing
- Identify systematic errors (too fast? too slow?)
- Tune parameters: cache latencies, pipeline widths, branch predictor size, memory bandwidth
- Add missing instruction support as needed
- Correctness validation: compare emulator output vs native for all benchmarks
- **Target: ≤20% average error, no benchmark >50% error**

### M8: Polish + Edge Cases
**Status:** Not started
**Budget:** 4 cycles
**Scope:**
- Fix remaining outlier benchmarks
- Code cleanup and documentation
- Final validation pass
- Performance optimization of the simulator itself (simulation speed)

## Total Estimated Budget: ~42 cycles (revised down — team has been faster than expected)

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

## Key Risks
1. **ARM64 emulator complexity**: Even a subset of ARM64 is large. Compiler may emit unexpected instructions.
2. **Accuracy vs complexity tradeoff**: Full OoO modeling is hard. May need to iterate on what level of detail is needed for 20% accuracy.
3. **Akita learning curve**: No existing CPU simulator on Akita — we're pioneering.
4. **Simulation speed**: Cycle-accurate simulation of LARGE dataset benchmarks may be too slow. May need to use MINI/SMALL datasets.
5. **Calibration difficulty**: Many microarchitectural parameters are estimates from reverse engineering.
