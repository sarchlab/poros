# Simulation model declaration

`model-profile.json` declares the default OoO timing configuration used by the
accuracy harness. The Go test `TestDeclaredModelProfileMatchesRuntimeDefaults`
checks it against the actual CPU and cache defaults. Keep the declaration in
canonical JSON (sorted keys, two-space indentation, final newline).

This is the Apple M2 profile, the simulator's `m2` target. The other
[modeled devices](README.md#modeled-devices) name `m2` as their parameter source
and are validated against this same declaration until they have parameters of
their own; `TestDeclaredModelProfileCoversTargetsCarryingIt` checks each of them.
A device with parameters of its own declares them in
`model-profile-<device>.json`; at present only `m1` does (see
[Apple M1 profile](#apple-m1-profile)).

## Apple M1 profile

`model-profile-m1.json` is the `m1` target: the M2 profile with seven
differences. Four are established by measurements on the M1 Linux host used
for the m1-7031aa3 campaign; the last three implement a store rule fitted to
probe timings on that host, which is an interpretation of them and not a
measured structure. `TestM1DiffersFromM2OnlyInMeasuredParameters` keeps
every other core, in-order and cache parameter equal to M2's. The three new
core fields and the new cache field are zero in the M2 profile and are omitted
from its declaration, so M2 timing and its declared configuration are
unchanged.

- `freq` 3,203,500,000 Hz: one-cycle ADD and three-cycle FADD chains on the M1
  host give 3.2028 to 3.2037 GHz, and counted user cycles of the integer
  control match the model's cycles (48.0 M). The whole-program workload
  contract keeps 3.49 GHz as its assumed clock, so an M1 run must convert its
  cases at this profile's clock (`scripts/devices.py at_profile_clock`).
- `fetch_taken_branches` 1: a fetch group continues across one correctly
  predicted taken branch. The M1 NOP and mixed-resource probes of 34 to 514
  instructions per trip run at N/8 cycles per trip, where ending every group at
  the loop branch gives ceil(N/8). M1 runs jacobi1d-1 (a 10-instruction loop) at
  45.8 M counted user cycles, 1.27 cycles per trip; the frozen rule gives 72.2 M
  and this rule 45.3 M.
- `store_dependence_prediction` true (`pkg/cpu/ooo/storedep.go`): a load that
  last overlapped an in-flight store waits for the youngest instance of that
  static store even when the instance writes other bytes. doitgen accumulates
  `sum[p]` in memory over `s`, then starts the next `p` with the same
  instructions. M1 counted 11.30, 11.14, 11.07 and 11.04 cycles per trip for
  chains of 8, 16, 32 and 51 trips, the single-chain rate of the M1 `mem_fadd`
  and `fp_same` probes (11.0); this rule gives 11.36, 11.16, 11.08 and 11.07,
  and overlapping consecutive chains (the M2 profile) gives 3.49 to 6.35. The
  same pattern makes 2mm, 3mm, trmm, cholesky, lu, correlation and covariance
  11 to 27 % slower in the model.
- `L2Latency` 18: the M1 pointer chases over 1 MiB (1, 2, 4, 8 and 16 chains)
  and 256 KiB (one chain) take 0.92 to 1.05 of the model's time with 18 cycles,
  0.79 to 0.82 with M2's 14; streaming probes are unchanged by it.
- `store_queue_in_order` true, `store_buffer_size` 140 and
  `InterleavedStoreRunsWait` true: in the model, a store of a run of
  contiguous stores that continues across single stores elsewhere waits until
  its line has arrived, in a store queue that stores leave in program order.
  The rule and the 140 are fitted to the M1 store probes; see
  [Store runs](#store-runs-apple-m1).

Compare/select fusion is kept although M1's register-only select probes
(`sel32_*_adj`) take two cycles: M1 runs the memory-carried Nussinov
recurrence (`nuss_const`, `nuss_chg`) in the same two cycles as its register
form, which this model reproduces only with the fused pair plus its one-cycle
forwarding path; disabling fusion alone made nussinov 44 % slower than M1.

Known gaps of this profile, each measured on M1 but not modeled: streaming
reads from L2 run at L1 speed in the model (M1 1 MiB read 0.72; the frozen
fetch rule hid this in fdtd2d and vips, and a narrower fill transport would
fix reads only by making L2 writes and copies 1.3 to 2.2 times too slow);
DRAM chase latency is 12 % short (M1 chase1 at 64 to 256 MiB; 385 cycles would
fit chase1 and chase4 but make chase16 1.18); and the gaps listed under
[Store runs](#store-runs-apple-m1).

### Store runs (Apple M1)

Streamcluster's initialization stores `p1[i]` and `p2[i]` in one loop. Once
the two arrays no longer fit the caches M1 needs 4.73 to 5.03 counted cycles
per trip for it (five captures) where the same loop takes 1.63 on L1-resident
arrays, and the model without this rule 2.00. Two probe requests on the M1
host (counted `cycles:u`, CPU 4, one repeat as the difference of two repeat
counts; 27 loop and placement variants over 82 MB, 17 of them also on
L1-resident arrays) show which patterns pay that cost, and raw PMU event
counts on 15 of the rows differ between the patterns.

The timings and the counts are the observations. The store mechanism
described after them is this model's interpretation: the observations
constrain what the core does with a store miss, but they do not show its
detector, its prefetching or its queues. Events are cited by number. Their
names (0xe5 `ST_NT_UOP`, 0xa2 `L1D_CACHE_MISS_ST`, 0xa8
`L1D_CACHE_WRITEBACK`, 0xc4 `ST_MEMORY_ORDER_VIOLATION_NONSPEC`, 0x84
`FLUSH_RESTART_OTHER_NONSPEC`) are labels recalled from the public event
list for this core; what each of them counts was not verified.

Observed, in cycles per 64-byte line stored beyond the same loop's cost on
L1-resident arrays:

- One stream, ascending or descending, alone or with a load of a fixed word
  per trip: nothing (within 0.05). A load stream copied to a store stream:
  0.4. In the one-stream row that has counts, events 0xe5 and 0xa2 each count
  once per store and event 0xa8 173 times in 1.28 M lines.
- One value stored to three or four interleaved streams, whatever their page
  offsets or order, and one stream that leaves gaps: 0.8 to 2.2. In the three
  of these rows that have counts, event 0xe5 counts none of the stores and
  event 0xa8 one per line.
- Two interleaved streams: 12.4 to 13.6 in five captures of the measured
  loop, and 12.0 to 19.5 in seven variants of it (4 KiB instead of 8 KiB
  apart in the page, entering their lines four trips apart, on 16 KiB or
  32 MiB pages, one value or two, store instructions adjacent or 16 bytes
  apart). One stream with a store to a fixed word per trip: 12.8 to 13.3 in
  three captures; with a moving store instead: 10.6. Event 0xe5 counts 32 to
  38 % of the measured loop's stores (one whole stream would be 50 %) and
  87 % of the stream's stores in the row with the fixed store.
- The two-stream loop with its store instructions 20 bytes apart: 8.6, which
  is 3.90 cycles per trip from the medians, 3.89 and 4.02 from the tenth- and
  ninetieth-percentile pairs, against 1.76 on L1-resident arrays. It lies
  between the two groups above, and its spread (27 % and 15 % of the median
  at its two repeat counts) exceeds the 5 % quality bound. Event 0xe5 counts
  23 % of its stores.
- The two-stream loop with the second stream at the first one's page offset:
  3.6, which is 2.5 cycles per trip instead of 4.9, with a spread of 20 % and
  10 %. Event 0xe5 counts 12 % of its stores.
- The two-stream loop retiring one trip per eight cycles: 0.4. On huge pages
  its cost is unchanged, and at 2 x 16 MB it already takes 4.5 cycles per
  trip. Event 0xc4 counts nothing in any row and event 0x84 2 to 12 times
  per 82 MB.

The model's interpretation of these contrasts is that M1 behaves as if it
followed one run of contiguous stores at a time, in program order:

- A store that is part of no run allocates its line in L1, and such misses
  are prefetched: the three- and four-stream rows and the stream with gaps.
- A run that nothing interrupts is streamed without its lines entering L1,
  at no extra cost: the one-stream rows.
- A run that continues across single stores elsewhere is streamed too, but
  its lines cannot complete in one piece and wait for memory: the two-stream
  rows and the stream with one other store per trip.
- A run does not survive two stores elsewhere, nor one store elsewhere at
  its own page offset. Two rows were predicted from this before they were
  measured: the two-stream loop with a third store per trip to one fixed
  word costs 2.02 cycles per trip (1.76 L1-resident) and event 0xe5 counts
  none of its stores, and with the second stream 8 bytes past the first
  one's page offset it costs 2.32 (2.25 to 2.51 between the tenth and
  ninetieth percentiles; this row's spread exceeds the 5 % quality bound).

The interpretation takes the cost of the third case to be bounded lookahead
in program order rather than a stall per line, because the slowly retiring
loop does not pay it, and not translation, because huge pages do not change
it. If only the run's lines wait, a waiting line takes 38.9 cycles where
there is one every 16 stores and 28.3 and 26.0 where there is one every 12:
2.43, 2.36 and 2.17 cycles per store of lookahead. Other mechanisms that
would produce the same timings and counts are not excluded.

The model (`pkg/mem/writes.go`, `storeRun`; `pkg/cpu/ooo`,
`StoreQueueInOrder`):

- `InterleavedStoreRunsWait`: the store path follows one run. A store is
  the bytes one instruction writes, counted once whatever its width or
  alignment and however many accesses or cache lines carry it. A store
  continues the run when it starts where the run ends or ends where it
  starts; the run survives one store elsewhere and ends at a second, or at
  once when that store starts in the run's own line of the page. A store of
  a run that has written at least one line and has continued across a store
  elsewhere within its last line is accepted only when its line has arrived;
  every other store miss keeps the existing early acceptance (56 pending
  lines), which stands in for both the prefetched and the cleanly streamed
  cases.
- `store_queue_in_order`: a store keeps its store-buffer slot until every
  older store has released its own, so the stores of other streams between
  two waiting stores take lookahead too. Under this interpretation the
  measured ratio of the two-stream loop to the one-stream-plus-store loop
  (16 against 12 stores per waiting line) requires it.
- `store_buffer_size` 140 with the unchanged 335-cycle memory latency gives
  2.39 cycles per store of lookahead. Only that ratio follows from the
  timings, so 140 is fitted, not M1's store-queue capacity: two groups of n
  stores, each behind an unfinished 400-cycle divide chain, show no limit on
  stores awaiting retirement (400 cycles per repeat to n = 160, then about
  one cycle per two added stores to 551 at n = 320, and 852 at n = 384, as
  a window of roughly 370 instructions retiring two stores per cycle would
  give). In the model the slots therefore bound only the stores that wait
  for the cache, and a store does not hold its slot for an older instruction
  that is not a store.

With it the model gives 4.63 cycles per trip for the measured loop (M1 4.73
to 5.03), 7.18 for one stream plus a fixed or a moving store (7.08 and 6.50),
8.00 for the slowly retiring loop (8.02), and leaves every fast row at its
former value (1.25 to 4.00 per trip against M1's 1.07 to 3.44). Against the
frozen campaign's native times, streamcluster's inputs then take 1.10 to
1.13 of M1's time where they took 0.66: its distance loop (two
loads, three FP operations and a 32-trip accumulation per point) takes 2.00
cycles per trip in the model and 1.25 on M1 (1.79 against 1.21 on L1-resident
arrays, where an experimental build with a window of 512 instead of 256
instructions gives 1.21), and the two errors no longer cancel.

Limits and gaps:

- The rule is fitted to 27 loop variants and their event counts on one host.
  It does not describe a known circuit: that the core follows one run, what
  it prefetches and what bounds the lookahead are inferred, not observed.
  Whatever M1 does, it does by degrees where the rule has two classes: event
  0xe5 counts none of the stores on the fast rows with two stores elsewhere,
  8 to 14 % on the three rows near the same page offset, 32 to 38 % on the
  measured loop and 23 % and 50 % on two variants of it.
- Every probe store is an aligned eight-byte store. That one instruction is
  one store to the rule whatever it writes (the two registers of a pair, a
  store that crosses a word or a cache line) is the model's choice; how M1
  treats such stores was not measured.
- Two rows measured after the rule was written fall between its classes, both
  with spreads of 15 to 21 %: one stream plus two fixed stores in two other
  lines per trip costs M1 3.41 per trip (2.04 L1-resident) where the model
  gives 2.00, and the two-stream loop with the second stream 64 bytes past
  the first one's page offset costs 3.45 where the model gives 4.54.
- The same-page-offset exception rests on two probe rows whose spread exceeds
  the 5 % quality bound (2.54 and 2.32 per trip, 2.25 to 2.54 over medians
  and tenth and ninetieth percentiles, against 4.67 to 5.26 for the slow
  loop) and on gesummv, whose two interleaved 26 to 42 MB streams lie 2,592
  pages apart and which M1 runs within 4 % of the model. Arrays that a
  program allocates separately in whole pages share their page offset;
  streamcluster's two streams are parts of one array.
- M1's cost varies between variants the rule treats alike: the two-stream loop
  costs 3.9 cycles per trip with its store instructions 20 bytes apart (the
  intermediate row above) and 6.0 with one shared value, against 4.9; the
  model gives 4.63 for all.
- The model's window is 256 instructions where the divide-chain loops suggest
  about 370 on M1: with 256 and 320 stores behind each chain the model needs
  800 and 816 cycles per repeat, M1 486 and 551 (as before this rule; up to
  224 stores the model is within 4 %, where the former 56 slots gave 8 % too
  many cycles at 224).
- The cleanly streamed and the prefetched cases are not modeled separately:
  both keep the existing store-miss path, which is bound by the model's
  16 bytes per cycle of memory transport and is 14 to 41 % slower than M1 on
  eight of those rows (as before this rule).
- One stream plus one store elsewhere per trip costs M1 3.85 cycles per trip
  even on L1-resident arrays (1.5 to 1.6 in the model). Not modeled.
- A prefetch for write 2 KiB ahead of each store halves the cost on M1 (3.7
  per trip); the simulator does not execute PRFM.
- M2 has no rows of this kind; its profile is unchanged.

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
