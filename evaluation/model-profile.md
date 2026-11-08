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

## Timed fills, fill transport and stream prefetch

OoO loads use a timed cache path (`mem.Cache.Access`). Every line installed by
that path carries the cycle at which its fill arrives; a load that hits a line
whose fill is pending completes at the arrival, never earlier, including the
adjacent 64 B half of a 128 B L2 line and lines copied by the untimed store path.
Timed fills use two serial transports: every 64 B L1 fill occupies 2 cycles on
the L2-to-L1 transport (`FillBytesPerCycle` = 32) and every 128 B DRAM fill
8 cycles on the DRAM-to-L2 transport (`DRAMBytesPerCycle` = 16), booked as
non-overlapping intervals so out-of-order issue times can use earlier gaps. The
rates come from a native one-load-per-line strided read on the M2 host (about
2.0 cycles per 64 B line from a 4 MiB buffer, 3.6 from a 64 MiB buffer); they
are effective single-core rates, not bus specifications. A
sequential stream prefetcher in units of L2 lines trains on three consecutive
same-direction demand L1 misses (`PrefetchTrainMisses`), then advances a frontier
by at most `PrefetchDegree` lines per demand L1 miss, at most `PrefetchDistance`
lines ahead of the demand line, into L2 only; `PrefetchStreams` entries are
replaced LRU. An optional `PrefetchPageBytes` boundary stops the frontier; it is
off by default because an exposed miss per 16 KiB page would make a sequential
DRAM stream about 30 % slower than an L2-resident one, while the native scalar
read rate is the same from L2 and DRAM within 4 %. OoO stores allocate their
lines through the same transports (one 128 B DRAM transfer per store-missed
line, the lower bound of an allocation fetch or a writeback) but still never
wait for their line and do not train streams; native `copy` moves each
direction at about half the rate of `read`, which free stores cannot produce.
The in-order model and `Lookup` keep the previous untimed behavior.

Evidence classes: the transport rates are measured on the M2 host as described
above (the DRAM rate is a lower bound, rounded down); the distance is an
inferred lower bound (the native scalar read rate of 57.6-59.8 GB/s from 1 MiB
to 64 MiB times the DRAM latency); the training rule,
degree, stream count and page bound are mechanism-compatible choices shaped
after the only published stream-prefetcher reverse engineering (Intel, not
Apple). None of these is a measured Apple M2 parameter. Demand misses still use
the OoO MSHRs; prefetches do not. Stores and the in-order pipeline do not charge
the transport. This does not claim complete memory timing accuracy.
