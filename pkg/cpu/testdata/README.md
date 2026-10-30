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
other workloads retain frozen digests. The Facesim digests and the dependency-corrected cycle rows
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

## Corrected ordinary RegSP operand dependencies

The decoder's `RegSP` operand class includes ordinary X/W0..30, not just SP.
The scoreboard previously discarded their source and destination dependencies
for immediate arithmetic. Delayed-load→ADD→SUB tests now require the true RAW
chain, while an independent ADD control remains ready early. A separate
load→independent ADD overwrite→consumer test proves the replacement destination
must supersede the stale load timestamp. Actual SP remains outside the existing
GPR scoreboard; this change does not equate it with the zero register.

All 88 cases were audited. Seventy-eight retain every value; the following ten
rows change only `Cycles`. Each listed default-OoO value applies per core in both
the one- and two-core rows. Replacing only the register-classification file with
its pre-fix version restores all ten original assertions.

| Workload / model | Old cycles | Corrected cycles | Rows |
|---|---:|---:|---:|
| trisolv / default OoO | 517718 | 517722 | 2 |
| canneal / default OoO | 57396 | 57397 | 2 |
| raytrace / default OoO | 474 | 475 | 2 |
| facesim / default OoO | 5392 | 5335 | 2 |
| raytrace / inorder | 1559 | 1569 | 1 |
| atax / inorder | 10332347 | 10652147 | 1 |

Facesim decreases because publishing the previously missing destination replaces
stale producer readiness: destination-only isolation yields 5014 cycles,
source-only isolation retains 5392, and both corrections yield 5335. The
independent-overwrite test directly covers this false-stall mechanism. These are
logical timing corrections, not hardware latency calibration. All retired
counts, complete architectural digests, workload inputs, initial-state identity
pins, and CPU/cache settings remain unchanged. Only these audited cycle fields
and the corresponding strict fixture SHA pins were updated.

Together with the earlier seven partial-register corrections (one overlapping
raytrace/inorder row), the dependency corrections above changed 16 distinct historical cycle rows.

## Corrected flag-only comparison dependencies

CMP, CMN, TST, CCMP, CCMN, FCMP and FCCMP read their first operand; none
writes a data register. The old generic source scan skipped that operand and
misclassified CCMN/FCCMP as data writers. Delayed-load→CMP→ADC and delayed-FP-load
→FCMP/FCCMP→CSET tests now verify real source readiness, NZCV dependencies and
absence of a spurious destination. ADC's native vectors independently establish
its input-carry dependency and unchanged flags.

A strict 88-case audit, supplemented by the only registry CCMN workload
(raytrace), found the following 15 cycle-only differences. Every other field,
all inputs, initial-state pins, CPU/cache parameters and all 88 assertions are
unchanged. Default-OoO values apply to each core in both one- and two-core rows.

| Workload / model | Old cycles | Corrected cycles | Rows |
|---|---:|---:|---:|
| correlation / default OoO | 1881185 | 1881425 | 2 |
| fluidanimate / default OoO | 16231 | 16596 | 2 |
| raytrace / default OoO | 475 | 483 | 2 |
| ferret / default OoO | 142074 | 142110 | 2 |
| x264 / default OoO | 246145 | 246140 | 2 |
| raytrace / no-bp | 301 | 309 | 1 |
| raytrace / inorder | 1569 | 1571 | 1 |
| blackscholes / inorder | 643308 | 647404 | 1 |
| atax / inorder | 10652147 | 10652537 | 1 |
| gemm / inorder | 11898798 | 11903598 | 1 |

Restoring only the old register classifier restores correlation and raytrace
assertions. For x264, the old classifier and destination-only correction both
retain 246145; source-only correction produces 246140, exactly as the complete
fix. Thus its decrease is caused by the real source dependencies, not removal
of a false FP destination. Changed dependency readiness affects the existing
OoO scheduling model; these total cycles are not a monotonic sum of individual
latencies. No hardware timing fit or parameter adjustment was performed.
Only audited cycle fields and their strict fixture SHA pins were updated.

Including overlaps with the earlier corrections, this PR now changes 23 distinct
historical cycle rows; all non-cycle fields remain unchanged by these dependency fixes.
