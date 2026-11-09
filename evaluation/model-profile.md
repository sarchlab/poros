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

OoO loads use `mem.Cache.Access` and stores use its explicit `Store` admission API; the in-order model and `Lookup`
retain their untimed behavior. Each timed line carries its fill arrival cycle.
Loads cannot complete before the supplying fill arrives, including hits on
pending lines and the adjacent 64 B half of a 128 B L2 line. The OoO pipeline
probes residency without mutation, selects the execution unit and MSHR, and
only then commits the access at its final issue cycle. A demand miss holds its
MSHR until the actual modeled load completion. Prefetches do not occupy demand
MSHRs; the stream table and distance bound their lookahead. A hit on a pending
L1 fill waits for that arrival without reserving another demand MSHR. This is a
merged-request approximation; the ROB bounds the number of waiting loads.

Timed fills use two finite transports: every 64 B L1 fill occupies 2 cycles on
the L2-to-L1 transport (`FillBytesPerCycle` = 32), and every 128 B DRAM fill
occupies 8 cycles on the DRAM-to-L2 transport (`DRAMBytesPerCycle` = 16).
Non-overlapping reservations retain earlier gaps for out-of-order issue times.
Adjacent occupied intervals are coalesced to avoid scanning a growing packed
backlog. History is retired at the actual pipeline-cycle watermark, never at
a future request's issue cycle. A transfer cannot start before its request,
even when a configured latency is shorter than the transfer occupancy.

Store address/data availability is the execution issue cycle; the three-cycle
CPU store completion is separate execution/retirement bookkeeping. Exact GPR
forwarding still uses the youngest store's issue/data readiness plus its one-cycle
bypass latency. Each instruction holds a finite `StoreBufferSize` slot until
its execution completes and the cache accepts all its bytes. A pair cannot free
its instruction slot until both accesses are accepted.

`Cache.Store` returns **acceptance** and **allocation drain** separately. A miss
acquires one of `WriteBufferEntries` pending line slots; subsequent stores to
that same still-resident allocation share it. The fill retains its actual
arrival and full read-for-ownership traffic: partial stores never eliminate
unwritten-byte fetches. A generation check prevents merging into an evicted
allocation. Resident L1 stores retain ordinary L1 completion timing for acceptance,
while a miss or merge is accepted as soon as its pending-line slot is available.
Thus a default hit holds the instruction slot for five cycles, whereas an
immediately accepted miss holds it through the three-cycle CPU completion. This
intentional asymmetry preserves hit timing; it is not a measured hardware rule. A full
stage delays admission and holds CPU instruction slots, backpressuring dispatch.
The default 56 line entries reuses the existing store-capacity scale as an
explicit model assumption; it is not a measured M2 combining-buffer size.

Forwarding still owns exact per-store address/width/readiness records, separate
from cache allocation ownership. Only a fully covered older record is compacted;
its conventional ordering and drain constraints survive on the newest record.
Partial shadows and pairs retain their byte ranges. The record pool has a hard
internal bound derived from instruction slots and pending line bytes, and stalls
dispatch when full. Program exit, serialized syscalls and fetch-fault completion
wait for instruction slots, pending write allocations and dirty eviction traffic.

Dirty L1 evictions transfer a full L1 line to L2, allocating/fetching any missing
L2 bytes. Dirty L2 evictions transfer a full L2 line to memory. Each eviction
stage has 56 entries (the same explicit `WriteBufferEntries` setting), and queue
admission delays replacement readiness when full. Its timestamps retain future
reservations; dirty bytes cannot transfer before their producing data or fill
arrives. L1 evictions share the existing 32 B/cycle L1/L2 transport with fills;
L2 evictions share the existing 16 B/cycle DRAM transport. These are conservative
shared-direction bandwidth assumptions, not measured bidirectional bus widths.
Private L1 eviction slots are private; L2 eviction slots and DRAM transport are
shared across attached cores. The initiating cache owns final-drain bookkeeping.
An L1 writeback raises readiness for the entire 128 B L2 line until its transfer
completes, so an L1 miss reading the clean 64 B half also waits. This conservative
line-granular dependency is a model assumption, not a measured hardware rule.
Coherence and a full DRAM controller remain unmodeled.

`Access(isWrite=true)` records an already-admitted transaction and its dirty
state, preserving its data-readiness return contract. Callers requiring finite
write admission use `Store`. Zero `WriteBufferEntries` disables the new stage
and dirty-eviction accounting for legacy configurations. `Lookup` remains the
legacy synchronous path. `Lookup` and `Prefetch` bypass dirty-eviction accounting;
when used with timed stores, they are limited to pre-run warming and must not be
interleaved with timed writes. Neither API provides chronological cache replacement.
Timed hit/miss counters count each covered L1 line segment, so a cross-line access
can increment them more than once; the previous first-line-only path did not.

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

## Scalar FP timing and measured destination capacity

Scalar FMADD/FMSUB/FNMADD/FNMSUB have four-cycle result latency and one-cycle
issue occupancy. The existing M2 scheduling chain directly supports FMADD;
other fused variants are same-family inference. FNMSUB was previously unsupported
and defaulted to one-cycle timing; it now executes product minus accumulator,
with fused binary64 and correctly rounded binary32 round-to-nearest-even results.
The preexisting binary32 FMADD/FMSUB/FNMADD paths still use Go expressions whose
fusion can depend on the host compiler; repairing those older paths is deferred
separate ISA work. This does not add FPCR/exception emulation.

The result-latency table is shared with the in-order model, so these changes
also retime its scalar fused/divide/sqrt instructions. In-order regression tests
cover the shared change; native capacity and divider-throughput calibration here
is for the OoO model. Checked ELF execution rejects vector FDIV/FSQRT forms;
this work does not add or calibrate SIMD divide/sqrt support.

FDIV result latency is 10 cycles; FSQRT is 13. Divide/sqrt use only the first
`NumFPDiv` FP pipes (default one), with occupancies one/two cycles respectively.
Ordinary FP operations share those same reservation calendars and can also use
the other FP pipes. Four generic FP pipes therefore cannot issue four divides
per cycle. `NumFPDiv` must be positive and no larger than `NumFP`; start custom
configurations from `DefaultSpec`, or add this field explicitly when migrating
old complete Spec literals. Old timing checkpoints/configuration fingerprints
must be rebuilt; whole-system cache checkpoint/resume remains unsupported.

The new [capacity/divider suite](microbenchmarks/capacity/README.md) keeps native
ns/iteration primary and records the assumed 3.49 GHz only for model conversion.
Core and instantaneous frequency were unobserved. Nontrivial FDIV uses the
bounded reciprocal recurrence 2/x around 1.5; FSQRT uses sqrt(x+1), converging
to an irrational fixed point. Separate one/four/sixteen-lane controls distinguish
result latency and issue throughput. The FSQRT normal recurrence includes one
FADD per square root, whose latency/port demand must be included in comparisons.
These controls and [direct M2 measurements](https://jia.je/hardware/2026/07/06/apple-m2/)
support effective timing choices, not exact instantaneous hardware cycle counts.

Two integer dependency gates surround an FP-destination sweep. Matching NOP,
changed gate-length and ungated FP controls distinguish dependency, ROB and
port limits. An interleaved ABBA measurement finds an effective transition around
366–370 FP destinations; variable/noisy rows remain in the reference. Both gate
lengths meet the 15% rise and nonoverlapping plateau-band criterion at 370.
An initial exploratory sweep informed the dense confirmation sweep; the quality
and rise criteria were fixed before that second acquisition, not before the entire
sequential study. Post-acquisition `infer_transition` makes the plateau choice
explicit: qualified counts ≤356, their median of medians and maximum P90. It
returns (366,370] for gate64 and (366,368] for gate96; the report shows all paired
ratios and their dispersion, including noisy rows. The model uses a rounded conservative lower-bound **360** based on that
measurement uncertainty, without fitting application error. It counts in-flight
FP destinations, **not total physical registers**: no architectural-register
reservation is subtracted again. The published external FP32 breakpoint is not
substituted for this local FP64 evidence.

The ROB remains **256**. Thus the default FP cap is nonbinding for instructions
with at most one FP destination; paired FP loads can still consume it. The
default simulator cannot reproduce a hardware knee above its own ROB limit.
This known distinct ROB approximation is retained rather than silently changing
another structure or claiming that the native measurement identifies a complete
physical window. Pair admission now checks both destinations before execution;
integer/store progress is allowed when FP destination capacity is exhausted.
New or aggravated application underpredictions must be reported; no artificial
latency or workload-specific compensation is applied.

## Predictor history semantics

`TaggedHistLen` now means the number of most recent branch outcomes used by the
tagged table (1–64, default 32). Every selected bit is XOR-folded into the actual
index width. Previously folding to 32 then indexing 11 bits discarded history.
The tag mixes the full instruction PC with the configured history, preventing
low-address code from receiving all-zero tags. Tags are stored at full 64-bit
width: with default 32-bit history and PCs below 2^34, this is an exact PC/history
context match. Noisy histories can therefore fall back to bimodal more often;
branch-direction hit rates and application effects require paired evaluation.
This is a generic anti-aliasing
model correction, not a reconstruction of Apple's predictor geometry. Outcomes
outside the configured history do not participate. All mutable predictor and
execution-unit state remains in component State and is serialization-tested.
