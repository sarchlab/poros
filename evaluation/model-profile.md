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
register tracking retain their existing behavior. The load/store eligibility
model below is independent of this frontend limit; branch redirect behavior
remains separate calibration work.

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

The later [recurrence probes](microbenchmarks/recurrence/README.md) count
cycles on an M2 performance core and show where this hypothesis stops. The
one-cycle path is granted by address and width, but hardware distinguishes the
base register: the same load/increment/store chain costs 1.75 cycles per
update through one base register and 6.47 through a second register holding
the same address (model: 2.0 for both). Within the same-base class the added
cost is not one number. It is about zero when a reload follows its store
directly in a four-instruction update, eight instructions later, across a taken
branch, or in the Nussinov loop; 0.75 in the three-instruction update the
forwarding probes use; and two to four and a half cycles for two interleaved
chains and for a memory-carried maximum, whether or not the loaded register is
also the one stored. A narrower load of a wider store costs 5.47 cycles and an
FP chain 9.1, against the model's conventional 8. These rows do not yet give a rule that
reproduces all of them, so the forwarding model is unchanged; they are the
recorded gaps for a separate calibration.

## Compare/select fusion

An eligible compare and the conditional select that immediately follows it
execute as one operation. Eligibility follows the measured forms. The compare
is a 32-bit `CMP` of two plain registers, or of a register and an unshifted
immediate of at most 15; the select is a 32-bit `CSEL`; both instructions lie
in one 64-byte line; and together they read at most two different registers,
the inputs of one two-source operation. A register compare therefore fuses
with a select between its two operands, in either order and into any
destination, and an immediate compare with a select between the compared
register and one other. A select between registers that were not compared,
64-bit pairs, shifted or extended compares, larger immediates, `CMN`, `CSINC`
and the other conditional-select forms, and the zero register or stack pointer
as an operand keep separate timing. The pair issues once, when every register
either instruction reads is ready, occupies one integer pipe for one cycle,
and delivers the flags and the selected register together one cycle later.
Both instructions still execute, hold their own ROB entries and retire, and a
later flag reader is an ordinary consumer of the pair's flags.

`NumSelectFusion` (default **1**) is the number of integer pipes that can
execute a fused pair: the last `NumSelectFusion` of the `NumINT` pipes, a
subset rather than extra pipes. Ordinary integer work fills pipes from the
first and reaches a fusion pipe only when the others are busy at that time, so
a stream of pairs is bounded by one per cycle per fusion pipe. Zero disables
the fusion and reproduces the previous timing exactly; a negative count or one
above `NumINT` is rejected. Omitted in an older complete Spec, the field is
zero.

A pair is admitted whole. With one fetch slot or one ROB entry left, the
compare waits for the next cycle rather than issuing unfused, and a
configuration that can never hold two in a cycle runs unfused. The model's own
cycle boundaries are therefore never a fusion barrier. Whether hardware decode
groups are one was not measured; only the 64-byte line was.

The [recurrence probes](microbenchmarks/recurrence/README.md) support this on a
pinned M2 performance core with counted user-mode cycles (the one-cycle ADD
chain reads 1.0000 and the three-cycle FADD chain 3.0000 cycles per update).
The adjacent W-register pair costs 1.07 cycles per update and the separated
pair 2.01, in two layouts; with every fourth pair straddling a line it costs
1.33. It still costs 1.07 with the select operands in the other order and with
a destination that is neither compared register, and so does a compare with
immediate 15 followed by a select between the compared register and one other.
A select between two registers that were not compared costs 2.01 after either
compare form, as does the same recurrence on X registers or with `CSINC`.
An independent stream of adjacent pairs issues at exactly 1.000 cycle per pair
where the separated stream takes 0.70: the saving is latency on a
one-per-cycle resource, which is why the pair has its own restricted pipe
rather than any integer pipe. The Nussinov inner-loop replica costs 1.39
cycles per trip with the pair adjacent and 2.67 with one instruction between.

Limits of the evidence and of the model:

- Eligibility is the measured forms and two stated extensions, not the
  compiler's fusion tuning set. Measured: the register compare with a select
  of its operands in both orders, into a compared or a third register, and
  immediate 15 with the compared register as destination and second select
  operand. Extended without a measurement: immediates 0 to 14, because the
  compiler's code-layout rule treats unshifted immediates up to 15 as one
  class and 15 is its largest member; and, for the immediate form, the
  operand order and destination freedom measured on the register form. In the
  workload binaries the extensions apply to one `CMP Wn, #0` pair each in
  facesim and fluidanimate, which changes one cycle of a facesim run and none
  of fluidanimate.
- Immediate 16 and the shifted immediate were probed only with a select of two
  other registers, which alone prevents the saving. They, `CMN` and
  zero-register operands stay unfused because nothing measured supports
  fusing them.
- Hardware loses about one cycle in fifteen pairs in the dense chain and one in
  nine in the tight loop, consistent with other work sharing the single pipe.
  The model's ordinary work avoids the fusion pipe while another is free, so it
  reads 1.00 where hardware reads 1.07 and 1.11, and 1.25 against 1.33 for the
  straddling layout.
- Four memory-carried maximum probes move further from hardware: the model
  falls from 3.0 to 2.0 cycles per update against a measured 5.47 (two probes)
  and 3.79 (two probes). Hardware does save time on the adjacent pair there
  (separating it adds 0.69 cycles in the dense loop); the gap is the store-to-load cost described
  above, which the model does not have.
- The Nussinov loop still costs the model two cycles per trip, because a
  fetch group ends at the taken branch; hardware runs its eleven instructions
  at eight per cycle. That frontend difference is separate work.
- One host and session; the in-order model is unchanged.

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
state JSON is not a compatible checkpoint. Poros does not expose a saved
timing-state restore CLI. The source revision distinguishes this scheduling
change; no claim of complete memory timing accuracy follows from it.

`NumLDST` counts physical memory pipes. The M2 default declares **four** pipes,
of which `NumLoad = 3` accept loads and `NumStore = 2` accept stores. The first
three calendars serve loads; the last two serve stores, so their overlap is
one genuinely shared pipe. Loads prefer the two load-only pipes and stores
prefer the store-only pipe when issue times tie. An earlier available shared
pipe is still used immediately. Thus pure load/store issue rates are bounded
by three/two instructions per cycle and mixed traffic by four in total,
including future reservations. Paired memory instructions retain their existing
one-pipe issue occupancy; this change does not introduce micro-op decomposition.

Custom configurations may set other capacities: each eligible count must be
positive, at most `NumLDST`, and their sum must cover all physical pipes.
Disjoint and multiple-shared-pipe layouts follow the same prefix/suffix rule.
`NumLoad = NumStore = 0` explicitly selects the legacy fully shared layout,
including one- and two-pipe configurations. A single zero is invalid; reducing
`DefaultSpec().NumLDST` also requires choosing valid eligible counts or setting
both to zero. There is no silent clamping. Omitted fields in older complete Spec
literals/JSON retain the legacy layout; newly serialized configurations retain
the zero fields instead of rewriting them. Rebuild timing checkpoints when
changing a Spec; the existing framework includes Spec in checkpoint identity.

The committed [throughput](microbenchmarks/throughput/README.md) load/store
controls and [memory](microbenchmarks/memory/README.md) L1-resident write
controls support this finite eligibility correction. They do not identify every
M2 memory operation's routing or establish overall application accuracy.

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

The M2 default sets `DisableAdjacentL1Prefetch = true`: a timed L1 miss transfers
only the requested L1 line. Fetching a 128 B L2 line does not also install its
unused 64 B half into L1. A later demand for that half performs its own L2
access/transfer, waits for pending L2 data, and consumes a demand MSHR if needed.
This applies to loads and write allocations, including each segment of a
cross-line access. It avoids speculative adjacent traffic, cache pollution and
dirty evictions without increasing either transport's bandwidth. The trained
directional L2 stream prefetcher remains enabled and retains real arrival times.

`DisableAdjacentL1Prefetch = false` retains the prior timed next-line policy;
the negative boolean deliberately makes a missing field in an older complete
cache configuration preserve that behavior. Untimed `Lookup` still warms the
next L1 line on a miss regardless of this timed policy, and explicit `Prefetch`
is unchanged. Runtime configuration and `model-profile.json` expose the policy.
Archived reports retain their original absent fields and exact source hashes;
the validators compare each row with its revision's declaration rather than
inserting new defaults into historical evidence.

The policy is an effective calibration choice, not a claim that M2 never
prefetches adjacent data. Controlled post-PR72 diagnostics found that removing
unconditional adjacent requests reduced the Syr2K-3 conflict penalty and
16-chain L2 chase error while preserving a usable 256 KiB copy control. Doubling
fill bandwidth instead severely overpredicted the copy speed. Those bounded
comparisons motivate removing unsupported requests, not a measured replacement
hardware prefetch algorithm. Broader streaming, adjacent-size and application
regressions, including new underpredictions, remain part of full evaluation.

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

## Memory-side cache and translation reach

Two mechanisms on the timed memory path. Their evidence is three native
captures on one M2 performance core (counted user-mode cycles; per-hop counter
ticks in the third), described with their fixed reading rules, failed
predictions and limits in
[the MVT 3072 investigation](mvt-outlier-investigation.md). Neither uses an
application reference time. The values below are effective: they reproduce
what was counted in one regime and are not properties read out of the part.

**Memory-side cache.** The model keeps a record of the lines most recently
evicted from L2, in the order they left: the last `SLCSize / L2LineSize`
(**8 MiB / 128 B = 65,536**) victims. It has no sets or ways. A timed L2 miss
on a line still in the record arrives `SLCLatency` (**138**) cycles after the
request instead of `DRAMLatency`, reports `HitSLC`, and the line returns to L2
and leaves the record; taking a line out does not let the others stay longer.
A clean victim's 128 bytes travel to the record on a transport of their own,
as wide as the fill transport (`DRAMBytesPerCycle`), one line after another;
the line can be supplied only once that move has ended (its return transfer
on the fill transport starts no earlier), and the fill that displaced it is
not delayed. A dirty victim's write is charged on the fill
transport as before and the line has left L2 when that transfer ends. A line
evicted before its own fill arrived cannot be supplied before that data
existed. The return uses the existing fill transport and the existing demand
MSHR; stream prefetches and writeback allocations take the same path. With a
shared L2 the record and its transport are shared. `SLCSize = 0` removes all
of it, so an older complete cache configuration without these fields keeps its
timing. The untimed `Lookup`/`Prefetch` paths neither fill nor consult it.

- 138 cycles is what a hop of a dependent chase costs on the M2 when the
  chased lines conflict in L2: 138.2 to 138.4 at three shapes in the first
  capture. The third capture timed hops one at a time with a 24 MHz counter
  (one tick is 146 cycles). At one shape 99% of hops take one or two ticks,
  under 1% are as fast as an L2 hit and none takes three. At the other two
  shapes 84 to 90% take one or two ticks, 6 to 14% none and 1 to 5% three or
  more. Read afterwards as three kinds of hop (not one of that capture's fixed
  rules): 74 to 99% intermediate, 1 to 22% as fast as an L2 hit, 0 to 6% as
  slow as memory, and the intermediate cost that reproduces the separately
  counted chases is 132 to 140 cycles at all three shapes. The model makes
  every such hop 138.
- 8 MiB is the published size of the M1's system-level cache, used because
  nothing measured here gives a size: the conflicting walks need about
  0.3 MiB, and the attempt to measure a size by reuse distance failed (see
  the limits below). MVT 3072 differs by 0.02% with a 1 MiB record.
- That the record holds victims only, and that a line leaves it when it
  returns to L2, follows the published description of the M1's cache as
  exclusive of the CPU caches. Nothing measured here tests that choice.
- The separate transport for clean victims is a modelling choice bounded by a
  measurement: if victims shared the fill transport at its present rate, a
  conflicting walk could not go below 16 cycles per access; the M2 runs it at
  11.5.
- In the conflicting rows the independent walk costs 11.5 cycles per access,
  one twelfth of the chase, which twelve overlapping returns of 138 cycles
  also give. The twelve demand MSHRs are unchanged; this is a consistency, not
  a count of a hardware resource.

**Translation reach.** `Cache.Translate` gives the cycle at which a data
address is translated. Pages are `TLBPageBytes` (**16 KiB**); the table holds
`TLBSets × TLBWays` (**256 × 12 = 3,072**) pages, indexed by page number and
replaced LRU. A page that is not in the table starts a walk of
`TLBWalkLatency` (**30**) cycles on one of `TLBWalkers` (**4**) walkers;
accesses to a page whose walk is pending wait for that walk. A load that takes
its data from the cache issues its cache access when its translation is
available; a load forwarded from a store does not wait. A store's bytes are
accepted by the cache once translated, which holds its store slot longer but
does not delay forwarding. A walk delays a load's value, not its address: the
base register of a pre- or post-indexed load is updated one cycle after its
address stage reserves a load pipe. The model uses that same slot for an
immediate cache access or a forwarded value. If translation or an MSHR delays
the cache request, the model charges a second issue on an eligible load pipe
when the request can proceed. A walk overlaps a wait for an MSHR. Walker bookings
keep real times and earlier gaps for requests visited out of order, and are
retired only at the pipeline's clock. `TLBPageBytes = 0` disables translation
timing.

- 3,072 entries is the size of this core's second-level TLB in Apple's
  optimization guide (read through a secondary source that quotes it); 256
  sets of 12 is the published reverse-engineering result for the M1. In
  the first capture a walk with no cache conflict costs the same on 16 KiB and
  on huge pages up to 3,072 pages and more beyond; 46% of hops pay at 3,200
  pages where this geometry with LRU predicts 52%.
- 30 cycles and four walkers are effective values from averages of that
  capture: the chase over 4,096 pages costs 29.7 cycles per hop more than over
  2,048 pages, and the MVT-shaped walk over 4,096 pages costs 6.93 cycles per
  access (30 / 6.93 = 4.3; a gather loop over the same pages costs 5.96, which
  would be five). Huge pages change physical placement as well as
  translation, so the attribution is an inference.

Limits:

- One host, Linux. Application references are macOS.
- **Read pressure.** On the M2, reading 16 to 96 MiB of other memory (in
  order or shuffled, once or twice) leaves a recently used 512 KiB target
  mostly as fast as L2 hits: a chase over it costs 20 to 39 cycles per hop
  after 16 to 32 MiB and 60 to 131 after 96 MiB. The model's L2 is exact LRU
  and loses the whole target at 16 MiB: 135 cycles per hop at 16 to 20 MiB
  with the record and 328 from 24 MiB; 328 from 16 MiB without it. The
  prediction that the M2 would show the 138-cycle level by reuse distance
  failed. This gap is in the committed model too and is not addressed.
- **Near the onset of conflict the record makes the model worse.** The M2
  starts losing lines earlier than a modulo index does (between 1,024 and
  1,536 rows at a 24,576-byte stride; the model between 2,048 and 2,560).
  Where the model has only a few misses, charging them a memory access
  happened to cost about what the M2 pays; as 138-cycle returns they cost
  less. Four measured walk rows (2,048 × 24,576 and 1,024 × 16,384 bytes, MVT
  loop and gather loop) move from 0.74–0.94 of the M2 to 0.43–0.51.
- **Not every return is intermediate.** At the largest conflicting shape about
  6% of the M2's hops cost a memory access; the model has none. At 3,072 rows
  the M2 keeps about a fifth of the lines in L2; the model keeps none of them
  there and is 17 to 21% too slow for that chase.
- Only the last translation level is modelled. The first level costs a
  dependent chase about six cycles per hop beyond 256 pages on the M2 and is
  absent here, as are page-table cache misses on very large footprints.
- Physical placement is not modelled: caches are indexed by virtual address.
  On 16 KiB pages the M2 shows partial L2 conflicts that scattered frames
  explain (3,584 rows at 28,672 bytes) and that the model does not have.
- An L2 hit costs a dependent chase 14 cycles in the model and 19 on the M2.
- The record and the translation table change in dispatch order, like the
  cache tags. A request visited after the eviction of its line but issued
  earlier finds the line in the record and waits for the eviction and the
  move: later than hardware that still had the line in L2, never earlier.

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
with at most one FP destination; paired FP loads can still consume it.
`FpPhysLimit` must be at least **2**, the maximum destination count of a supported
single instruction. Smaller custom configurations are rejected at initialization,
because a paired FP load could otherwise wait forever even with an empty ROB. The
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
