# Akita v4 migration baseline

`akita-v4-baseline.jsonl` was captured from Poros commit
`2a8abd5071f2774842255729f9d098dd477c9a6c` using Akita v4.9.0, before any
migration edits. The two Facesim rows were subsequently replaced with validated
finite M0 inputs and output digests, as documented below. It originally covered all 41 entries
returned by `benchmark.All()`, each on
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
fixture from the migrated model to make a regression pass. The Facesim exception
was an explicit correction of invalid input, independently checked before its
two output digests were replaced.

`pipeline-v4-baseline.jsonl` was captured from a separate `git archive` of the
same pre-migration commit. It records exact instruction/cycle counts and the same
architectural digest for raytrace, blackscholes, atax, and gemm in the in-order
and OoO-without-branch-prediction modes. `TestV4PipelineVariants` enforces these
values in eight automated cases. Separately, the migration validation runs the
historical full CLI 41-kernel report and compares its output byte-for-byte with the frozen
v4 report; that CLI comparison is not performed by TestV4PipelineVariants.

## Host-math portability

These digests originate from Go1.27 darwin/arm64. The Black-Scholes and Bodytrack
kernels invoke host math.Exp/Log, whose low-order floating-point bits differ on
AMD64. On non-ARM64 hosts, only those audited workloads retain exact historical
instruction/cycle checks while comparing whole-state digests with same-host
functional execution. Separate mathematical tests check all 1024 option prices
and all 64 particle likelihoods on actual registry inputs. Full-state equality
links these functional outputs to pipeline results. Logs and evaluation reports
identify the alternatives and require the corresponding numerical tests. All
other workloads retain frozen digests. The Facesim digests and the seven dependency-corrected cycle rows
below are explicit exceptions; other fields retain their historical values.

## Corrected Facesim exception to historical provenance

The two Facesim rows in `akita-v4-baseline.jsonl` now describe the corrected M0
input, not the v4 checkout. The previous mesh produced NaN forces and was not a
valid numerical regression oracle. After independent all-60-force mathematical
and native C checks, only these two digests were replaced with
`5f67d9c4ff061f9c2fb1daab144f368d1c4d8d573f4b19187e776150d13ebd71`.
Instructions (8166) and cycles (5392) per core did not change. At that correction, the other 80 rows
and all eight pipeline-variant rows were unchanged. `TestRegistryFacesimNumericalReference`
is required by both short and full evaluation reports; missing, skipped or failed
reference execution rejects acceptance. There is no Facesim host-math exception.

## MVT ELF migration

MVT now runs as a complete ELF program in the 24-case user-time suite. Its two
legacy CPU rows and one initial-state fingerprint were removed; no other rows
changed during that migration. The remaining registry has 40 kernels (29 PolyBench and 11 PARSEC),
80 one/two-core baseline cases plus eight pipeline variants. Git history retains
the superseded MVT fixtures for rollback.

## Corrected partial-register dependencies

Mach-O bring-up exposed missing old-destination dependencies in MOVK and BFI.
Both preserve bits of the previous destination. Owned native MOVK vectors and
focused delayed-load producer tests establish the dependency without fitting a
hardware latency. Source classification now also handles BFM/BFXIL consistently.

Three pipeline fixture cycle fields changed: raytrace/inorder 1533→1559,
raytrace/no-bp 299→301, and blackscholes/inorder 643266→643308. Isolating MOVK
accounts for +18, +2, +42 respectively; BFI adds 8 to raytrace/inorder; BFM changes
none of these eight cases. Four default-OoO rows also change solely from MOVK:
swaptions/1 97307→97308; swaptions/2 [97307,97183]→[97308,97183];
fluidanimate/1 16230→16231; fluidanimate/2 [16230,16230]→[16231,16231].
Isolating BFI or BFM leaves these four rows unchanged. Workload inputs, instruction counts, architectural
digests and CPU/cache parameters are unchanged. Strict comparisons remain.
Historical cycle values remain in Git history rather than a second golden set.
