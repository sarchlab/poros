# Poros

Poros is a cycle-accurate, execution-driven Apple M2 CPU simulator built on the [Akita](https://github.com/sarchlab/akita) discrete-event simulation framework in Go. It models the M2's Avalanche (P-core) out-of-order superscalar pipeline with multi-core support. It runs 41 benchmarks across two suites — [PolyBench/C](https://web.cse.ohio-state.edu/~pouchet.2/software/polybench/) (30 numerical kernels) and [PARSEC](https://parsec.cs.princeton.edu/) (11 parallel workloads) — with high accuracy.

## Accuracy Results

- **Average error: 15.8%** (target: ≤20%)
- **Maximum error: 42.5%** on `swaptions` (target: ≤50%)
- All 41 benchmarks pass the ≤50% error threshold

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

### PARSEC Results (11 benchmarks)

| Benchmark | Description | Error |
|-----------|-------------|------:|
| blackscholes | Option pricing via Black-Scholes PDE | 17.1% |
| bodytrack | Particle filter body tracking (vision) | 20.4% |
| canneal | Simulated annealing for chip routing | 7.6% |
| facesim | FEM tetrahedron force computation | 38.6% |
| ferret | k-NN similarity search | 24.0% |
| fluidanimate | SPH fluid dynamics simulation | 39.3% |
| raytrace | BVH ray-triangle intersection | 1.6% |
| streamcluster | Online clustering of streaming data | 36.0% |
| swaptions | Interest-rate derivative pricing (HJM) | 42.5% |
| vips | 2D image convolution | 10.0% |
| x264 | H.264 motion estimation (integer SAD/SATD) | 19.3% |

**Skipped benchmarks:** `dedup` (pipeline parallelism incompatible with function-level emulation) and `freqmine` (pointer-chasing tree traversal not representable in flat-memory emulator).

## Build and Run

The project has been renamed from M2Sim to Poros. Update Go imports from
`github.com/sarchlab/m2sim3` to `github.com/sarchlab/poros`, and replace
`cmd/m2sim` / `m2sim` with `cmd/poros` / `poros` in build and run commands.
The old module and command paths are no longer provided by this checkout.

```bash
go mod tidy          # Download dependencies
go build ./cmd/poros # Build simulator
go test ./... -timeout 45m # Run all tests (including long regressions)
```

### CLI Usage

```bash
# List all 41 benchmarks
./poros list

# Run a single benchmark
./poros run atax
./poros run swaptions

# Run all benchmarks (full comparison table)
./poros run --all

# Run a specific suite
./poros run --suite polybench
./poros run --suite parsec

# Multi-core mode (2 cores with shared L2)
./poros run --cores 2 gemm
./poros run --suite parsec --cores 4
./poros run --all --cores 2

# Quick evaluation (6 benchmarks, ~2 min)
./poros eval
```

### Testing

```bash
# Full comparison table (all 41 benchmarks, ~10 min)
go test -v -run TestOoO_ComparisonTable ./pkg/cpu -timeout 20m

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

- `MultiCorePipeline` wraps N independent `ooo.Comp` instances
- Per-core private L1 caches (128 KB each)
- Shared L2 cache (16 MB, 16-way) across all cores
- Configurable core count via `--cores` flag
- All cores share one Akita `sim.Simulation` using its default serial engine (deterministic simulation)

## Dependencies

- Go 1.27+
- [github.com/sarchlab/akita/v5](https://github.com/sarchlab/akita) — Discrete-event simulation framework, pinned to v5.0.0
- [golang.org/x/arch](https://pkg.go.dev/golang.org/x/arch) — ARM64 instruction decoder

## Project Structure

```
pkg/cpu/       — Multi-core assembly and regression tests
pkg/cpu/inorder/ — In-order Akita v5 ticking component
pkg/cpu/ooo/     — OoO Akita v5 ticking component and branch predictor
pkg/cpu/internal/inst/ — Shared instruction dependency and latency helpers
pkg/emu/       — ARM64 functional emulator
pkg/mem/       — Cache hierarchy (L1, shared L2, DRAM)
pkg/benchmark/ — Benchmark definitions and kernel registry
pkg/loader/    — Mach-O extraction/disassembly and static AArch64 ELF image loading
pkg/linuxuser/ — Checked Linux process address space and initial ABI state (execution pending)
polybench/     — PolyBench/C benchmark suite and native timing data
parsec/        — PARSEC benchmark kernels (C source, .o files, native timing)
scripts/       — Build and run scripts (compile_all.sh, run_simulation.sh)
cmd/poros/     — Main simulator entry point (CLI)
cmd/disasm/    — ARM64 disassembler tool
```

## Akita v5 integration

Poros pins **Akita v5.0.0** and requires **Go 1.27.0 or newer**. The migration
preserves the M2 timing parameters, functional emulator, synchronous cache model,
benchmark definitions, and stored native baselines. It does not introduce new CPU
models, asynchronous memory, or CPU–GPU offload.

The in-order, OoO, and demo components each use the v5
`Spec` / `State` / `Resources` / `Ports` / `Middlewares` model and declare
`Comp` and `DefaultSpec()`. The demo uses `modeling.None` for Resources.
Configuration is supplied before
Build, runtime pipeline and predictor data lives in State, and middlewares contain
only component references and immutable configuration-derived lookups. The old
`cpu.NewPipeline` / `cpu.NewOoOPipeline` constructors are replaced by the Akita
model builder. For example:

```go
simulation := sim.MakeBuilder().Build()
defer func() {
    if err := simulation.Terminate(); err != nil { panic(err) }
}()
b := benchmark.ByName("raytrace")
spec := ooo.DefaultSpec()
spec.FuncHookCycles = cpu.HookCycleSpec(b.FuncHookCycles)
em := b.Setup()
em.Mem.TrackAccesses = true
cache := mem.NewCache(mem.DefaultCacheConfig())
benchmark.WarmCache(cache, b.Regions)
core := modeling.MakeBuilder[*ooo.Comp](spec).
    WithSimulation(simulation).
    WithResources(ooo.Resources{Emu: em, Cache: cache}).
    Build("CPU")
if err := simulation.Initialize(); err != nil { panic(err) }
core.TickLater()
if err := simulation.Run(); err != nil { panic(err) }
// Results are core.State.Cycles, core.State.InstDone, and core.State.CPI().
// A functional execution error is reported in core.State.Err (empty on success).
```

The imports are `github.com/sarchlab/poros/pkg/benchmark`,
`github.com/sarchlab/akita/v5/sim`, `.../sim/modeling`, and Poros's `pkg/cpu`,
`pkg/cpu/ooo`, and `pkg/mem`. Set memory tracking and warm caches during setup.
Initialize once after all components have been built. Multi-core assembly accepts
the shared `*sim.Simulation` and enables memory-access tracking on each supplied
emulator during setup. Call its `Initialize()` before `MultiCorePipeline.Run()`
and `Terminate()` afterwards. All cores use the same default serial simulation,
with the existing core registration order. Recording remains off by default.

Akita v5 schedules using integer picoseconds: at 3.49 GHz its period is truncated
to **286 ps**. The CLI continues to report simulated seconds as `cycles / 3.49e9`,
exactly as before; it does not substitute the quantized engine clock. This preserves
the cycle-based comparison with stored native times.

The component State is serializable, including in-flight fetched instruction
encodings and branch-predictor tables. **Whole-system checkpoint/resume is not
supported by this migration:** functional emulator registers/memory/hooks and the
synchronous cache hierarchy remain external live Resources and are not saved by
component checkpoints. Restoring pipeline State alone cannot resume a workload.

`go test ./... -timeout 45m` includes the long existing benchmark tests and the
v4 regression fixture. `go test -short ./...` retains representative baseline and
determinism checks while skipping the full sweep. `TestV4Baseline` compares every
registered kernel on one and two cores against pre-migration instruction counts,
cycle counts, register values, and benchmark data-region contents. See
[pkg/cpu/testdata/README.md](pkg/cpu/testdata/README.md) for fixture provenance.
All three component packages also check that Akita's static inspector agrees with
the runtime component schema and defaults.

## Evaluation and calibration

See [the M0 evaluation contract](evaluation/README.md) for the versioned workload
inventory, regression CI, and fresh M2 multi-size timing pilot. Historical
regression data and provisional native measurements are different evidence
classes; neither alone establishes a validated CPU model.
