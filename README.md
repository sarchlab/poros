# Poros

Poros is an execution-driven AArch64 CPU simulator built on the [Akita](https://github.com/sarchlab/akita) discrete-event simulation framework in Go. It models an Apple M2-inspired in-order and Avalanche-style out-of-order pipeline, with independent multicore process experiments. All 41 existing compute kernels now run as complete Linux ELF programs, alongside integer, memory and branch microbenchmarks: **44 named workloads**. The PARSEC names denote simplified compute kernels, not full applications. Hardware accuracy remains unverified.

The [complete ELF workload contract](tests/benchmarks/README.md) defines inputs,
outputs, provenance and correctness checks. The [user-time accuracy report](evaluation/usertime/README.md)
contains all 44 workloads at six larger timing points each (264 cases), with
separate preserved small correctness inputs and native macOS Mach-O hardware
references with audited ELF-matched computation. Measurement coverage and model
predictions do not establish hardware accuracy.

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
# List all 44 complete ELF workloads
./poros list

# Run a single benchmark
./poros run atax
./poros run swaptions

# Run all 44 workloads (JSON execution results)
./poros run --all

# Run a specific suite
./poros run --suite polybench
./poros run --suite parsec

# Multi-core mode (2 cores with shared L2)
./poros run --cores 2 gemm
./poros run --suite parsec --cores 4
./poros run --all --cores 2

# Run all 44 workloads at input point 3
./poros eval
```

### Testing

```bash
# Complete ELF matrix: 40 new kernels, six inputs, three execution modes
go test -v -run '^TestBenchmarkELF$' ./pkg/elfsim -timeout 45m

# Short regression: every workload name plus component and runtime checks
go test -short ./...

# ATAX at all six inputs in all three execution modes
go test -v -run '^TestBenchmarkELF$/^atax$' ./pkg/elfsim -timeout 10m
```

## Architecture

### ARM64 Functional Emulator

- Interprets 93+ ARM64 instructions (integer, FP, SIMD, branches, loads/stores)
- In-order functional execution, separate from the timing model
- Static AArch64 Linux ELF loading with checked mappings and an initial process stack
- Bounded Linux user-mode runtime with captured write/exit syscalls; unsupported instructions and syscalls report errors
- Complete guest startup, input initialization, computation, output and termination

### Out-of-Order Superscalar Pipeline

- 8-wide fetch, 8-wide retire, 256-entry ROB
- 6 INT + 4 FP/SIMD + 4 LD/ST execution units
- Register readiness tracking (GPR + FP)
- Speculative FP destination pressure (360-entry model limit; independently bounded by the 256-entry ROB)
- Address-based load-store disambiguation with 56-entry store buffer
- 12 MSHRs for outstanding L1 misses

### TAGE-like Branch Predictor

- Bimodal base: 4096 entries, 2-bit saturating counters
- Tagged table: 2048 entries, 3-bit counters
- BTB: 2048 entries, 4-way set-associative
- RAS: 32-entry circular buffer
- 17-cycle misprediction penalty
- Historical branch prediction accuracy: 93–99.5% across the pre-migration PolyBench suite

### Cache Hierarchy

- L1D: 128 KB, 8-way, 64B lines, 5-cycle latency
- L2: 16 MB, 16-way, 128B lines, 14-cycle latency
- DRAM: 335-cycle latency
- LRU replacement policy
- OoO uses timed fills, finite L2/DRAM transports and trained stream prefetch;
  in-order uses the legacy untimed cache path. See the
  [timing model and its limitations](evaluation/model-profile.md#timed-fills-fill-transport-and-stream-prefetch).

### Multi-Core Support

- `MultiCorePipeline` wraps N independent `ooo.Comp` instances
- Per-core private L1 caches (128 KB each)
- Shared L2 cache (16 MB, 16-way) across all cores
- Configurable core count via `--cores` flag, running independent copies of the same ELF process
- No guest-thread scheduling or validated physical address-space isolation
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
internal/benchmarks/ — ELF inventory, provenance and complete-output validation
tests/benchmarks/ — Forty complete guest programs and six-point inputs
pkg/loader/    — Mach-O extraction/disassembly and static AArch64 ELF image loading
pkg/linuxuser/ — Checked static Linux AArch64 startup, execution and captured syscalls
polybench/     — PolyBench/C benchmark suite and native timing data
parsec/        — PARSEC benchmark kernels (C source, .o files, native timing)
scripts/       — Build and run scripts (compile_all.sh, run_simulation.sh)
cmd/poros/     — Main simulator entry point (CLI)
cmd/disasm/    — ARM64 disassembler tool
```

## Akita v5 integration

Poros pins **Akita v5.0.0** and requires **Go 1.27.0 or newer**. The migration
preserved the then-existing M2 timing parameters, functional emulator, synchronous
cache model, pipeline semantics and stored native measurements. Subsequent OoO
changes add analytic fill timing as described above; the Akita migration itself
did not introduce new CPU models, asynchronous memory, or CPU–GPU offload.

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
image, err := os.ReadFile("tests/benchmarks/gemm.elf")
if err != nil { panic(err) }
p, err := linuxuser.Parse(image, linuxuser.Options{Args: []string{"gemm", "7", "9", "11"}})
if err != nil { panic(err) }
execution, err := p.NewExecution(context.Background(), linuxuser.RunOptions{})
if err != nil { panic(err) }
defer execution.Close()
spec := ooo.DefaultSpec()
cache := mem.NewCache(mem.DefaultCacheConfig())
core := modeling.MakeBuilder[*ooo.Comp](spec).
    WithSimulation(simulation).
    WithResources(ooo.Resources{Execution: execution, Cache: cache}).
    Build("CPU")
if err := simulation.Initialize(); err != nil { panic(err) }
core.TickLater()
if err := simulation.Run(); err != nil { panic(err) }
// Results are core.State.Cycles, core.State.InstDone, and core.State.CPI().
// A functional execution error is reported in core.State.Err (empty on success).
```

The imports are `context`, `os`, `github.com/sarchlab/poros/pkg/linuxuser`,
`github.com/sarchlab/akita/v5/sim`, `.../sim/modeling`, and Poros's
`pkg/cpu/ooo` and `pkg/mem`. Initialize once after all components have been built,
and terminate afterwards. `elfsim.Run` owns this lifecycle for normal callers.
Each OoO run owns one timed cache client: the core advances and finalizes the
supplied cache. Warm it with `Lookup` before starting, and construct a new cache
for a second timed run. Untimed `Lookup` remains available after finalization.
Multicore assembly accepts distinct execution resources, private L1 caches and a
modeled shared L2; process deadlines remain owned and closed by its caller.
These are independent process copies, not Linux threads or validated physical
address-space isolation. Recording remains off by default.

Akita v5 schedules using integer picoseconds: at 3.49 GHz its period is truncated
to **286 ps**. The CLI continues to report simulated seconds as `cycles / 3.49e9`,
exactly as before; it does not substitute the quantized engine clock. This preserves
the cycle-based comparison with stored native times.

The component State is serializable, including in-flight fetched instruction
encodings and branch-predictor tables. **Whole-system checkpoint/resume is not
supported by this migration:** functional emulator registers/memory/hooks and the
cache hierarchy remain external live Resources and are not saved by component
checkpoints. A restored State with missing/incompatible allocation slots is
rejected explicitly. Restoring pipeline State alone cannot resume a workload.

`go test ./... -timeout 45m` requires all six new input points in all three
execution modes. `go test -short ./...` covers every name at a nontrivial small
point. Complete native/QEMU differential checks replace obsolete whole-kernel
instruction-array goldens. Isolated ISA, cache, pipeline and Akita lifecycle tests
remain, including branch-prediction variants and multicore termination. All three
component packages check that Akita's static inspector agrees with runtime schemas.

## Evaluation and calibration

See [the M0 evaluation contract](evaluation/README.md) for the versioned workload
inventory and regression CI. [Whole-program user-time evaluation](evaluation/usertime/README.md)
runs MVT and three other workloads as complete Linux ELF programs at six points each. Historical
regression data and provisional native measurements are different evidence
classes; neither alone establishes a validated CPU model.
