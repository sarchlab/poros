# Simulation model declaration

`model-profile.json` declares the default OoO timing configuration used by the
accuracy harness. The Go test `TestDeclaredModelProfileMatchesRuntimeDefaults`
checks it against `elfsim.DefaultTimingProfile`, the single-core ELF OoO
profile. Generic CPU/cache component defaults are separate; in-order and
multicore execution retain those numeric defaults. Keep the declaration in
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

The default OoO model has a one-cycle effective forwarding path for scalar
32/64-bit GPR `LDR`/`STR` with immediate-offset addressing and no writeback on
both producer and consumer. Its single access must exactly match the youngest
overlapping, still-in-flight eligible store in address and width. The load
result waits for both its own address/unit readiness and the store's recorded
address/data issue cycle. Store result latency remains unchanged. Completed
program-order store-buffer prefixes now drain at actual CPU time, and loads
exclude completed entries from forwarding at their queried issue time.
Register-offset addressing, pairs, byte/halfword transfers, unscaled
`LDUR`/`STUR`, pre/post-index writeback, FP/SIMD, zero-register transfers, partial
overlap and mismatched widths retain conventional forwarding timing while
the source store is in flight. Completed stores no longer provide forwarding;
the load consults the cache.

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
references and benchmark inputs remain unchanged. The narrow ELF OoO
calibration below changes the declared L2 latency and store resource; its
frequency and all other numeric parameters remain unchanged.

The forwarding mechanism itself adds no configurable default. Its source-level
correction is distinct from the numeric calibration declared below.
Hardcoded timing/dependency behavior is identified by the exact source revision
and binary hashes already bound into each plan and report; equal configuration
JSON does not imply equal simulator behavior. The source revision must therefore
be retained when comparing or reusing results. Old serialized store-buffer
entries without the new readiness/classification fields retain the conventional
eligibility classification while in flight; newly produced states round-trip
those fields. Completed-entry filtering and actual-time prefix draining also
apply to those older entries.

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
state JSON is not a compatible checkpoint. This reservation mechanism itself
does not change numeric parameters, and Poros does not expose a saved timing-state
restore CLI. The source revision distinguishes this scheduling change. Cache fill and
prefetch behavior remain separate work; no claim of complete memory timing
accuracy follows from this correction.

The [scheduling probes](microbenchmarks/scheduling/README.md) contrast dependent
MADD/FMADD chains with added independent work. They support mechanism-level
checks, while the full application comparison must disclose regressions as well
as improvements. Fewer modeled cycles alone do not establish better accuracy.


## Lightweight calibration candidates

`poros elf --mode ooo --timing-profile FILE.json` accepts a validated complete
CPU/cache profile for bounded calibration. Every canonical top-level, CPU,
predictor, and cache key must be present; null is accepted only for
`configuration.func_hook_cycles`. Explicit valid zero values remain supported. Results emit the effective values;
normal accuracy execution still uses the revision-bound defaults. The optional
`num_stores` resource limits store transfers separately from shared LDST issue.
Zero retains the earlier behavior. Scalar and single Q stores book one cycle;
The supported pair-store instruction STP books two cycles on one lane. Unsupported
STNP and structured ST1–ST4 forms are not added by this timing change. These
abstract transfer counts are not a claim
about measured M2 vector micro-ops. Joint resource search preserves future gaps
and commits shared LDST/store reservations once, without changing store result
latency. The existing immediate cache-fill/free-prefetch approximations remain.
See [the calibration plan](analytical-memory-plan.md) for acceptance and limits.

Completed store-buffer entries no longer provide perpetual forwarding. A load
whose issue time is at or after a store's modeled completion consults the cache
instead; a completed younger overlap still shadows older pending forwarding
sources. Forwarding queries do not delete entries, because issue times can arrive
out of order. Only a completed program-order prefix is pruned at the actual CPU
cycle. This fixes a source-dependent timing bug without changing the numeric
profile: keep exact source revisions when comparing candidates with older runs.

This shared OoO correction also applies to multicore OoO; in-order execution is
unchanged. Custom calibration profiles remain single-core OoO only. Timed
prefetch/read-fill service and write-traffic modeling are deferred; the existing
immediate tag installation and free next-line prefetch remain approximations.

The provisional single-core ELF OoO profile uses `num_stores = 2` and
`L2Latency = 20`; L1 latency 5, DRAM latency 335, MSHRs 12, line geometry,
issue widths, ROB, and assumed frequency are retained. This profile is derived
in `elfsim.DefaultTimingProfile` and emitted by default ELF OoO runs. Generic
component, in-order, and multicore numeric defaults remain unchanged. Full
memory-suite, application, and host-speed validation gates its final adoption.
