# Simulation model declaration

`model-profile.json` declares the default OoO timing configuration used by the
accuracy harness. The Go test `TestDeclaredModelProfileMatchesRuntimeDefaults`
checks it against the actual CPU and cache defaults. Keep the declaration in
canonical JSON (sorted keys, two-space indentation, final newline).

This declaration is independent of frozen workload manifests and native reference
measurements. A calibration change updates the simulation declaration, without
rewriting the inputs or the hardware observations. Plans and reports bind head
and base profiles separately to their Git revisions; every execution must match
its role's profile. Before publication, both declarations are read from the exact
commits, so a self-consistent but fabricated base profile cannot pass. Archived
reports carry the profiles and source hashes and remain readable offline.

Revisions before this declaration use their recorded
`tests/usertime/performance.json` profile (or the earlier `manifest.json`). Historical reports keep their original
frozen-profile validation. The native input manifests and both native reference
files are unchanged by the frontend/dependency correction.

## Response-driven memory backend

The default single-core OoO model uses `ooo.AsyncComp`, Akita write-back L1/L2,
and `simplebankedmemory`. `poros elf --mode ooo --memory-backend analytical`
retains the prior model. In-order and independent-copy multicore execution remain
analytical and identify that backend in their results. An explicit Akita request
to the multicore API is rejected, rather than silently falling back.

Functional instruction execution remains in program order to determine the correct
instruction stream, addresses and values. Timing dispatch snapshots producer tags
before destination renaming. Unknown load completions remain unresolved until a
real memory response; independent instructions can issue meanwhile. Timing-only
cache payloads never overwrite architectural registers or memory. Address knowledge
from functional execution does not permit a younger load to bypass an older
store whose address-generation dependencies/unit are still unresolved.

Stores generate addresses through LDST units, wait separately for data and the
scalar-store issue limit, retire in order, then drain in the background. Consecutive
retired writes to one L1 line may coalesce with byte enables. Each contributing
instruction retains its buffer entry until the combined response returns. Forwarding
still uses the youngest overlapping original store; partial or mixed pair coverage
waits conservatively for the store to drain. SVC, barriers and process exit drain
CPU memory operations; they do not flush all dirty cache lines to backing memory.
Cancellation stops new work while consuming already-sent responses so a small
response port cannot deadlock the hierarchy. A cancelled run is not a valid
accuracy sample.

The cache domain uses four internal ticks per CPU cycle at the M2 profile. This
is an abstract timing resolution, **not a measured M2 cache frequency**. Akita v5.0.0
advances the directory stage `NumReqPerCycle` times per component tick; this profile
sets it to one and expresses throughput through the internal timebase and queues.
Effective latencies must be measured end to end, not inferred from one pipeline
parameter. The simplified backing service has its own declared frequency and
pipeline depth, separating approximately 100 ns service latency from its nominal
64 GB/s line-service limit. This one-bank abstraction is not a claim about physical
M2 DRAM bank count or topology. Native calibration and full application acceptance
remain separate checks. Parameter calibration/full-suite comparison is pending.

At other requested CPU frequencies the default domains scale proportionally,
except that internal ticks are capped at the engine's one-picosecond resolution.
Akita truncates periods to integer picoseconds. Results retain the established
`CPU cycles / assumed CPU Hz` convention; all domain frequencies are in the profile.

`--timing-profile FILE.json` accepts a complete alternative declaration for bounded
calibration. Its CPU frequency must match `--frequency-hz`. The runner validates
geometry, capacity, routing, clock and pipeline bounds before building components;
unknown JSON fields are rejected. Every result reports the effective configuration.
Cache hit/miss counters from the old analytical cache are unavailable in the new
path (`cache_metrics_available: false`); zero placeholders must not be interpreted
as no misses. `memory_requests` reports actual CPU reads, coalesced writes,
responses, outstanding traffic and forwarding instead.

No detailed DRAM commands, timed prefetcher, address translation/TLB, coherence,
or calibrated M2 system-level cache are added here. Both cache replacement and
virtual-address indexing still need independent validation. The measured Mach-O
references are unchanged, microbenchmarks remain separate from application
accuracy, and `hardware accuracy unverified` remains the appropriate status.

## Analytical backend history

The descriptions below document the retained analytical backend and the previous
frontend/forwarding fixes. They are not the response-driven memory protocol above.

The default frontend admission limit is eight instructions per cycle. This is a
modeling choice supported by the native NOP and mixed-resource throughput probes;
it does not assert an eight-wide backend issue limit. NOP still uses frontend,
ROB and retirement capacity, but no integer execution slot. Ordinary registers
represented by `arm64asm.RegSP` participate in the scoreboard; actual SP and zero
register tracking retain their existing behavior. Load/store sharing and branch
redirect behavior are separate calibration work.

The comparison operand dependency correction also affects the shared in-order
scoreboard and cached decode metadata, not only the OoO pipeline. Validation of
these shared helpers must cover both pipelines; the ordinary full regression
includes the in-order ELF workload cases.

## Store-to-load timing hypothesis

The analytical OoO model has a one-cycle effective forwarding path for scalar
32/64-bit GPR `LDR`/`STR` with immediate-offset addressing and no writeback on
both producer and consumer. Its single access must exactly match the youngest
overlapping, still-in-flight eligible store in address and width. The load
result waits for both its own address/unit readiness and the store's recorded
address/data issue cycle. Store completion and buffer-capacity accounting remain
unchanged. Register-offset addressing, pairs, byte/halfword transfers, unscaled
`LDUR`/`STUR`, pre/post-index writeback, FP/SIMD, zero-register transfers, partial
overlap, mismatched widths and completed stores retain conventional timing.

The [native dependency probes](microbenchmarks/forwarding/README.md) measure
value-varying loops. Immediate 32/64-bit GPR chains add about 0.21–0.23 ns/update
over matched register chains, while register-offset producer or consumer forms
and FP chains have substantially larger incremental cost. The native dependency
chains measured zero displacement with a general-register base. The model also
allows nonzero scaled immediate offsets and SP bases as an unmeasured extension
of the same immediate-offset instruction class; these individual forms were not
established by the probes. One cycle is an approximate model cost, not a measured
physical bypass latency or evidence of M2 memory renaming. Core/frequency were
unobserved; the same instruction class
can behave differently in longer application dependency graphs. The unchanged
FP path can still contain compensating errors. Full application accuracy remains
a separate check. Existing application and instruction-throughput hardware
references, benchmark inputs, frequency and cache defaults remain unchanged.

This change adds no configurable default, so `model-profile.json` is unchanged.
Hardcoded timing/dependency behavior is identified by the exact source revision
and binary hashes already bound into each plan and report; equal configuration
JSON does not imply equal simulator behavior. The source revision must therefore
be retained when comparing or reusing results. Old serialized store-buffer
entries without the new readiness/classification fields retain the conventional
path; newly produced states round-trip those fields.

## Execution-unit reservations

OoO execution units record occupied half-open intervals rather than a single
busy-until timestamp. A future reservation leaves its earlier gaps available to
ready independent instructions. Each instruction reserves its whole throughput
occupancy; its result latency remains unchanged. After store forwarding and
MSHR availability determine the earliest issue time, the pipeline selects an
execution-unit gap and commits the unit/MSHR allocation once. MSHRs retain their
existing busy-until policy. Expired unit reservations are pruned at the actual
pipeline cycle, never at an instruction's future operand-ready time.
A separate finite issue-queue capacity is not modeled; the existing ROB and FP
destination limits still bound admitted work. These reservation lists do not
claim a physical M2 issue-queue size.

These mutable interval lists belong to component `State` and survive JSON state
round trips. The old experimental numeric `IntUnits`, `FpUnits`, and `LdstUnits`
arrays are replaced by per-unit arrays of `{Start, End}` intervals; old numeric
state JSON is not a compatible checkpoint. Configuration and model-profile
parameters are unchanged, and Poros does not expose a saved timing-state restore
CLI. The source revision distinguishes this scheduling change. Cache fill and
prefetch behavior remain separate work; no claim of complete memory timing
accuracy follows from this correction.

The [scheduling probes](microbenchmarks/scheduling/README.md) contrast dependent
MADD/FMADD chains with added independent work. They support mechanism-level
checks, while the full application comparison must disclose regressions as well
as improvements. Fewer modeled cycles alone do not establish better accuracy.
