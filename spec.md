# Project Specification

## What do you want to build?

M2Sim is a cycle-accurate, execution-driven Apple M2 CPU simulator built on the Akita framework.

## How do you consider the project is success?

We need to support simulating the whole PolyBench benchmark suite AND the **whole PARSEC benchmark suite** with 20% average time estimation error. The maximum error of each benchmark execution cannot be greater than 50%. Error is calculated by error = abs(sim-hw)/min(sim,hw).

**Whole PARSEC means:** Support all 13 PARSEC benchmarks. Skip a benchmark only for a strong, documented reason (e.g., fundamental architectural incompatibility like heavy SIMD dependency that cannot be compiled as scalar ARM64).

You run on an Apple M2 machine, so you can run locally to measure.

## Deliverables (from human issue #156)

When done, the project must include:
1. **Source code of all benchmarks** — scalar kernel C source files for every supported PARSEC benchmark
2. **A script to compile all benchmarks** — builds all C kernel sources into ARM64 object files
3. **A script to run benchmarks and collect native execution time** — runs each benchmark natively on M2 hardware, records wall-clock time
4. **A script to run simulation and collect simulator-predicted time** — runs the m2sim simulator on each benchmark, records simulated time

## PARSEC Benchmark Suite

The full PARSEC 3.0 suite has 13 benchmarks:
1. blackscholes — Option pricing (FP-heavy, embarrassingly parallel) ✅ DONE
2. bodytrack — Computer vision body tracking
3. canneal — Simulated annealing for chip routing ✅ DONE
4. dedup — Data deduplication (pipeline parallelism)
5. facesim — Physics-based face simulation
6. ferret — Content-based similarity search (pipeline parallelism)
7. fluidanimate — SPH fluid animation
8. freqmine — Frequent itemset mining
9. raytrace — Path-tracing renderer
10. streamcluster — Online clustering ✅ DONE
11. swaptions — Interest-rate derivative pricing ✅ DONE
12. vips — Image processing
13. x264 — H.264 video encoding

## Multi-Core Requirements (from human issue #119)

- Support multi-core simulation for Apple M2 chips (M2 has up to 8 P-cores + 4 E-cores)
- Introduce the PARSEC benchmark suite for multi-threaded workload evaluation
- All benchmarks (PolyBench and PARSEC) must meet ≤20% average error, ≤50% max error targets

## CLI Tool Requirement (from human issue #126)

- `/cmd/m2sim/main.go` must be a usable command-line tool, not just a placeholder
- The tool should allow users to run benchmarks, view results, and interact with the simulator without writing test code
- Must support running PolyBench and PARSEC benchmarks from the command line
- Should provide clear usage instructions and help text

## Constraints

- Built on Akita v4 framework in Go
- Only ARM64 instruction set (no x86)
- No virtual memory needed for statically linked binaries
- Use MINI/SMALL/MEDIUM dataset sizes to keep simulation time manageable
- Native timing references measured on the same Apple M2 machine
- Function-level emulation approach: extract computational kernels, no need for full binary or pthread support

## Notes

- PolyBench single-core simulation: COMPLETE (avg 13.1%, max 36.0%)
- PARSEC 4-benchmark support: COMPLETE (avg 25.8%, max 42.5%)
- Combined (34 benchmarks): avg 14.6%, max 42.5%
- Multi-core simulation with shared L2 cache: DONE
- M2 P-core (Avalanche): 3.49 GHz, 8-wide OoO, 330-entry ROB
- M2 has coherent shared L2 cache (16MB, shared across P-cores)
- Function-level emulation avoids pthread complexity — extract per-thread kernel body
- BL-intercept hooks for math library functions (exp, log, erfc, pow) already implemented
- Current architecture: OoOPipeline (single-core) + MultiCorePipeline wrapper + shared L2
