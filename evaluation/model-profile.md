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
address/data issue cycle. Store data completion remains separate from allocation
capacity and drain timing (see the timed-fill section below). Register-offset addressing, pairs, byte/halfword transfers, unscaled
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
references, benchmark inputs, frequency and cache defaults were unchanged by
that forwarding correction; the later timed-fill defaults are described below.

The forwarding rule itself adds no configurable default.
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

## Timed fills, fill transport and stream prefetch

OoO loads and stores use `mem.Cache.Access`; the in-order model and `Lookup`
retain their untimed behavior. Each timed line carries its fill arrival cycle.
Loads cannot complete before the supplying fill arrives, including hits on
pending lines and the adjacent 64 B half of a 128 B L2 line. The OoO pipeline
probes residency without mutation, selects the execution unit and MSHR, and
only then commits the access at its final issue cycle. A demand miss holds its
MSHR until the actual modeled load completion. Prefetches do not occupy demand
MSHRs; the stream table and distance bound their lookahead.

Timed fills use two finite transports: every 64 B L1 fill occupies 2 cycles on
the L2-to-L1 transport (`FillBytesPerCycle` = 32), and every 128 B DRAM fill
occupies 8 cycles on the DRAM-to-L2 transport (`DRAMBytesPerCycle` = 16).
Non-overlapping reservations retain earlier gaps for out-of-order issue times.
Adjacent occupied intervals are coalesced to avoid scanning a growing packed
backlog. History is retired at the actual pipeline-cycle watermark, never at
a future request's issue cycle. A transfer cannot start before its request,
even when a configured latency is shorter than the transfer occupancy.

Store data completion remains three cycles, with unchanged exact-GPR forwarding
eligibility. Separately, each store instruction holds one allocation slot until
the later of data completion and its allocation-fill readiness. A paired store
holds its slot until both accesses are ready. The allocation pool uses the
existing `StoreBufferSize` (56) as a capacity assumption, not a measured M2
structure. A full pool backpressures dispatch before functional store execution.
Normal program termination and serialized process syscalls wait for these
allocations to drain, so a terminal store stream cannot escape its outstanding
cost. Stores use the same allocation transports and adjacent-line prefetch as
loads, but do not train stream prefetching. An odd-half store miss can therefore
also fetch the adjacent L2 line. Dirty writeback traffic, coherence and a full
DRAM controller are not modeled.

The sequential stream model trains on three same-direction demand L1 misses,
then advances by at most four L2 lines per trigger, no farther than 48 lines
ahead of demand. Sixteen entries are replaced LRU. `PrefetchPageBytes` is an
optional boundary and is off by default. Disabling it avoids one modeled
exposed-miss penalty per page; it does not establish the physical M2 page-crossing
policy. Tags, replacement and stream training still evolve in caller/dispatch
order, even for non-monotonic issue times. This is the legacy cache-residency
approximation, not chronological memory scheduling: a later-time access visited
first can replace a line needed by an earlier-time access visited afterwards.
Arrival checks prevent the latter from consuming a pending fill early; they do
not correct residency or replacement order.

The 32/16 B-per-cycle rates are effective model choices inferred from a local,
uncommitted strided-read diagnostic. Its observed times were converted using the
model's assumed 3.49 GHz, yielding about 2.0 and 3.45 assumed cycles per 64 B line
for the smaller and 64 MiB working sets. These are not measured hardware cycles,
physical bus widths, or independently reproducible committed microbenchmark
results. The 48-line distance is a latency-throughput inference; degree, training
length and stream count are modeling hypotheses informed by general stream
prefetch behavior, not measured Apple structures. A scalar read plateau can be
core-bound, so it does not by itself determine memory bandwidth or prefetch
page behavior. Existing committed application/microbenchmark references remain
unchanged, and corrected full application accuracy must be evaluated separately.

Multicore OoO runs share L2 tags, fill readiness and one DRAM-to-L2 transport.
Each core has a private L2-to-L1 transport and stream table. All cores use the
same cycle domain; shared transport history is retired at the minimum actual
cycle of attached, still-running clients. A stopped core deregisters without
cancelling its already-booked transfers. All clients must attach before timed
access/advancement. The model remains a single-thread-calibrated, unvalidated
multicore extension without coherence or chronological cross-core residency.
Cache objects are live resources, not part of the component JSON checkpoint.
