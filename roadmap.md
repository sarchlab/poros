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
**Status:** Not started
**Budget:** 4 cycles
**Scope:**
- Go module setup with Akita v4 dependency
- PolyBench/C 4.2.1 cloned and compiled natively on M2
- Collect native reference timings for all 30 benchmarks (MINI + SMALL datasets)
- Basic project directory structure
- ARM64 binary loading (parse Mach-O, extract code/data sections)
- Trivial Akita "hello world" component that compiles and runs
- Disassemble a simple ARM64 binary using `golang.org/x/arch/arm64/arm64asm`

### M2: ARM64 Functional Emulator (Core Subset)
**Status:** Not started
**Budget:** 8 cycles
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
**Status:** Not started
**Budget:** 6 cycles
**Scope:**
- Akita TickingComponent modeling a simplified in-order pipeline:
  - Fetch (1 instruction/cycle initially)
  - Decode
  - Execute (with correct latencies per instruction class)
  - Memory access
  - Writeback
- Integrate with functional emulator (timing model drives functional execution)
- Instruction latency tables based on M2 Avalanche measurements
- Basic stall handling (data hazards via pipeline interlocks)
- Run PolyBench MINI and compare estimated cycles vs real cycles
- **Target: get within 5x of real execution time** (we'll refine later)

### M4: Cache Hierarchy
**Status:** Not started
**Budget:** 6 cycles
**Scope:**
- L1 Data Cache: 128KB, 8-way, 64B lines, 3-cycle hit latency
- L2 Cache: 16MB, 128B lines, ~18-cycle latency (use Akita mem/cache package)
- DRAM: ~96ns latency (use Akita mem/idealmemcontroller)
- L1 Instruction Cache: 192KB (model as ideal initially)
- Memory access pipeline integrated with timing model
- Cache miss stall modeling
- **Target: memory-bound benchmarks should show realistic slowdown**

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

## Total Estimated Budget: ~48 cycles

## Lessons Learned
(To be updated as project progresses)

## Key Risks
1. **ARM64 emulator complexity**: Even a subset of ARM64 is large. Compiler may emit unexpected instructions.
2. **Accuracy vs complexity tradeoff**: Full OoO modeling is hard. May need to iterate on what level of detail is needed for 20% accuracy.
3. **Akita learning curve**: No existing CPU simulator on Akita — we're pioneering.
4. **Simulation speed**: Cycle-accurate simulation of LARGE dataset benchmarks may be too slow. May need to use MINI/SMALL datasets.
5. **Calibration difficulty**: Many microarchitectural parameters are estimates from reverse engineering.
