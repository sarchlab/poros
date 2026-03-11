# M2Sim

M2Sim is a cycle-accurate, execution-driven Apple M2 CPU simulator built on the [Akita](https://github.com/sarchlab/akita) discrete-event simulation framework in Go. It models the M2's Avalanche (P-core) out-of-order superscalar pipeline with multi-core support. It runs 34 benchmarks across two suites — [PolyBench/C](https://web.cse.ohio-state.edu/~pouchet.2/software/polybench/) (30 numerical kernels) and [PARSEC](https://parsec.cs.princeton.edu/) (4 parallel workloads) — with high accuracy.

## Accuracy Results

- **Average error: 14.6%** (target: ≤20%)
- **Maximum error: 42.5%** on `swaptions` (target: ≤50%)
- All 34 benchmarks pass the ≤50% error threshold

Error is calculated as `|sim_time - native_time| / min(sim_time, native_time)`.

### PolyBench/C Results

| Benchmark | Error |
|-----------|------:|
| atax | 12.0% |
| mvt | 6.5% |
| gemm | 18.0% |
| 2mm | 5.4% |
| 3mm | 4.1% |
| bicg | 8.0% |
| gesummv | 3.4% |
| gemver | 19.3% |
| doitgen | 15.4% |
| trisolv | 28.8% |
| durbin | 24.5% |
| jacobi1d | 26.7% |
| syrk | 13.3% |
| syr2k | 15.4% |
| trmm | 1.9% |
| symm | 2.0% |
| cholesky | 3.0% |
| lu | 2.2% |
| floydwarshall | 1.6% |
| covariance | 3.2% |
| correlation | 2.9% |
| gramschmidt | 2.0% |
| ludcmp | 36.0% |
| nussinov | 3.1% |
| adi | 21.0% |
| fdtd2d | 29.0% |
| heat3d | 26.4% |
| jacobi2d | 19.7% |
| seidel2d | 26.5% |
| deriche | 10.9% |

### PARSEC Results

| Benchmark | Description | Error |
|-----------|-------------|------:|
| blackscholes | Option pricing via Black-Scholes PDE | 17.1% |
| canneal | Simulated annealing for chip routing | 7.6% |
| streamcluster | Online clustering of streaming data | 36.0% |
| swaptions | Interest-rate derivative pricing (HJM) | 42.5% |

## Build and Run

```bash
go mod tidy          # Download dependencies
go build ./cmd/m2sim # Build simulator
go test ./pkg/cpu    # Run all tests
```

### CLI Usage

```bash
# List all 34 benchmarks
m2sim list

# Run a single benchmark
m2sim run atax
m2sim run swaptions

# Run all benchmarks (full comparison table)
m2sim run --all

# Run a specific suite
m2sim run --suite polybench
m2sim run --suite parsec

# Multi-core mode (2 cores with shared L2)
m2sim run gemm --cores 2
m2sim run --suite parsec --cores 4
m2sim run --all --cores 2

# Quick evaluation (6 benchmarks, ~2 min)
m2sim eval
```

### Testing

```bash
# Full comparison table (all 34 benchmarks, ~25 min)
go test -v -run TestOoO_ComparisonTable ./pkg/cpu -timeout 30m

# Quick evaluation (6 benchmarks, ~2 min)
go test -v -run TestOoO_QuickEval ./pkg/cpu -timeout 10m

# Individual benchmark
go test -v -run TestOoO_PolyBenchAtax ./pkg/cpu
```

## Architecture

### ARM64 Functional Emulator

- Interprets 93+ ARM64 instructions (integer, FP, SIMD, branches, loads/stores)
- In-order functional execution, separate from the timing model
- Flat memory model for statically linked binaries
- Function-level emulation (no OS/syscall layer needed)

### Out-of-Order Superscalar Pipeline

- 3-wide fetch, 8-wide retire, 256-entry ROB
- 6 INT + 4 FP/SIMD + 4 LD/ST execution units
- Register readiness tracking (GPR + FP)
- Physical register file pressure modeling (72 FP physical registers)
- Address-based load-store disambiguation with 56-entry store buffer
- 12 MSHRs for outstanding L1 misses

### TAGE-like Branch Predictor

- Bimodal base: 4096 entries, 2-bit saturating counters
- Tagged table: 2048 entries, 3-bit counters
- BTB: 2048 entries, 4-way set-associative
- RAS: 32-entry circular buffer
- 17-cycle misprediction penalty
- Accuracy: 93–99.5% across the PolyBench suite

### Cache Hierarchy

- L1D: 128 KB, 8-way, 64B lines, 5-cycle latency
- L2: 16 MB, 16-way, 128B lines, 14-cycle latency
- DRAM: 335-cycle latency
- LRU replacement policy

### Multi-Core Support

- `MultiCorePipeline` wraps N independent `OoOPipeline` instances
- Per-core private L1 caches (128 KB each)
- Shared L2 cache (16 MB, 16-way) across all cores
- Configurable core count via `--cores` flag
- All cores driven by a single Akita `SerialEngine` (deterministic simulation)

## Dependencies

- Go 1.25+
- [github.com/sarchlab/akita/v4](https://github.com/sarchlab/akita) — Discrete-event simulation framework
- [golang.org/x/arch](https://pkg.go.dev/golang.org/x/arch) — ARM64 instruction decoder

## Project Structure

```
pkg/cpu/       — OoO pipeline, multi-core pipeline, branch predictor, instruction latencies
pkg/emu/       — ARM64 functional emulator
pkg/mem/       — Cache hierarchy (L1, shared L2, DRAM)
pkg/benchmark/ — Benchmark definitions and kernel registry
pkg/loader/    — Mach-O binary loader
polybench/     — PolyBench/C benchmark suite and native timing data
cmd/m2sim/     — Main simulator entry point (CLI)
cmd/disasm/    — ARM64 disassembler tool
```
