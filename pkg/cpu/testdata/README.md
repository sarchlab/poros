# Akita v4 migration baseline

`akita-v4-baseline.jsonl` was captured from Poros commit
`2a8abd5071f2774842255729f9d098dd477c9a6c` using Akita v4.9.0, before any
migration edits. It covers all 41 entries returned by `benchmark.All()`, each on
one and two independent cores sharing the existing L2 tag model. The frequency
is 3490 MHz, default CPU/cache configuration, cache warm-up uses each benchmark's
Regions, and function-hook cycle costs come from the registry.

Each row records per-core retired instruction and cycle counts. `Digest` is
SHA-256 over, in order:

1. `encoding/json.Marshal(em.Regs)` (all GPR, FP/SIMD, PC, SP and NZCV values).
2. Raw bytes from each benchmark Regions entry, in listed order and increasing
   address order, after execution.
3. `em.InstCount` encoded as an eight-byte little-endian integer.

This captures architectural output, including initialized/modified benchmark
buffers, without depending on JSON map iteration or heap addresses. It is a
migration regression baseline, not fresh hardware validation. `TestV4Baseline`
asserts each emulator halted and compares every value. Do not regenerate the
fixture from the migrated model to make a regression pass.

`pipeline-v4-baseline.jsonl` was captured from a separate `git archive` of the
same pre-migration commit. It records exact instruction/cycle counts and the same
architectural digest for raytrace, blackscholes, atax, and gemm in the in-order
and OoO-without-branch-prediction modes. `TestV4PipelineVariants` enforces these
values in eight automated cases. Separately, the migration validation runs the
full CLI 41-kernel report and compares its output byte-for-byte with the frozen
v4 report; that CLI comparison is not performed by TestV4PipelineVariants.

## Host-math portability

These digests originate from Go1.27 darwin/arm64. The Black-Scholes kernel invokes
host math.Exp/Log, whose low-order floating-point bits differ on AMD64. On
non-ARM64 hosts, that audited workload retains exact historical instruction/cycle
checks, but its whole-state digest is compared with same-host functional execution.
A separate mathematical test checks all1024 registry option results, linking to
pipeline results through the full digest comparison. Test logs and evaluation
reports identify this alternative explicitly. Other workloads retain frozen
digests. The JSONL fixtures themselves have not changed.
