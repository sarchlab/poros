# M2Sim

M2Sim is a cycle-accurate, execution-driven Apple M2 CPU simulator built on the [Akita](https://github.com/sarchlab/akita) discrete-event simulation framework in Go. It models the M2's Avalanche (P-core) out-of-order superscalar pipeline and runs the full [PolyBench/C](https://web.cse.ohio-state.edu/~pouchet.2/software/polybench/) suite of 30 numerical benchmarks with high accuracy.

## Accuracy Results

- **Average error: 13.1%** (target: ≤20%)
- **Maximum error: 36.0%** on `ludcmp` (target: ≤50%)
- All 30 benchmarks pass the ≤50% error threshold

Error is calculated as `|sim_time - native_time| / min(sim_time, native_time)`.

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

## Build and Run

```bash
go mod tidy          # Download dependencies
go build ./cmd/m2sim # Build simulator
go test ./pkg/cpu    # Run all tests

# Run full comparison table (all 30 benchmarks, ~20 min)
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

## Dependencies

- Go 1.25+
- [github.com/sarchlab/akita/v4](https://github.com/sarchlab/akita) — Discrete-event simulation framework
- [golang.org/x/arch](https://pkg.go.dev/golang.org/x/arch) — ARM64 instruction decoder

## Project Structure

```
pkg/cpu/       — OoO pipeline, branch predictor, instruction latencies
pkg/emu/       — ARM64 functional emulator
pkg/mem/       — Cache hierarchy
pkg/loader/    — Mach-O binary loader
polybench/     — PolyBench/C benchmark suite and native timing data
cmd/m2sim/     — Main simulator entry point
cmd/disasm/    — ARM64 disassembler tool
```
