# Lightweight memory accuracy: design and acceptance plan

## Status and scope

The candidate-profile interface and optional store-transfer resource are implemented;
calibrated defaults and acceptance results are still pending. The starting revision is
`a1954771adc8a13f84f689c11e2d3229fe67c682` on the memory-accuracy umbrella.
The prior Akita cache replacement experiment is not part of this change. Keep
Poros's existing lightweight CPU/cache path and its functional behavior. Reuse
all existing matched Mach-O references, including the 140 memory cases; do not
resample hardware or change workload inputs, application/microbenchmark report
separation, CI triggers, or the hardware-accuracy qualification.

The first increment should test whether latency calibration and a small,
independent store-throughput constraint improve the observed errors. It should
not require a new CPU implementation, message-based cache hierarchy, or detailed
DRAM controller. The default numeric timing remains the baseline while controlled candidates are measured.

## What the current code actually does

`pkg/mem/cache.go` has set-associative LRU L1/L2 state and fixed hit latencies.
`Cache.Lookup` immediately installs a missed line and calls `Prefetch` to install
the adjacent line without a separate latency or bandwidth charge. It ignores the
access span after its starting address. These are known timing approximations,
not completed-fill or finite-bandwidth semantics.

`pkg/cpu/ooo/pipeline.go` executes instructions functionally in program order and
books timing work into the future. `instrLatency` calls that cache immediately;
`reserveIssue` then finds an execution-unit interval and an MSHR time. MSHRs are
anonymous busy-until slots, not line-indexed transactions. The execution-unit
interval lists preserve gaps so a later-dispatched independent instruction can
issue before an earlier instruction waiting for operands. Stores and loads share
`NumLDST`; there is no separate store-data throughput limit.

Consequently, merely adding `AdvanceTo(futureIssue)` to this cache is unsafe:
future issue times are not monotone in dispatch order. An earlier query after a
future query can observe an incorrectly installed or evicted line. A resident
line's fill timestamp alone cannot reconstruct the overwritten cache history.

## Selected first experiments

1. **Latency-only control.** Use usable dependent pointer-chase points to test
   L1, L2, and beyond-L2 latency candidates. Initial hypotheses are approximately
   3/20/360 CPU cycles rather than the current 5/14/335. These are calibration
   candidates, not measured physical pipeline depths. Native measurements are
   times; conversion uses the explicitly assumed model frequency. Keep cache
   geometry, workload sizes, and references fixed during this comparison.
2. **Store throughput.** Test a distinct scalar-store issue resource alongside
   the existing shared LDST resource. An initial candidate is two stores/cycle
   while retaining four shared LDST slots. The existing write-only result is much
   faster than its reference whereas the L1 copy result is already close; slowing
   all LDST operations would confound these effects. Check actual read/write/copy
   case kinds from the manifest, including pair/vector behavior, before choosing
   the final constraint or its scope.
3. **Bounded follow-up only if evidence requires it.** Compare multi-chain chase
   and larger streaming cases before changing MSHR count or cache capacity. Do not
   increase resource counts just to fit a particular benchmark. A transfer-rate
   reservation could bound traffic, but cannot repair uncharged prefetches or
   establish correct cache hit classification on its own. It is not a committed
   requirement of this first increment.

A new store resource can use the current half-open interval lists. Find a common
free start for the LDST and store resource, advancing the tentative start until
both agree, then commit each interval exactly once. Prune only at the actual CPU
cycle. This preserves earlier gaps and avoids backdating or tentative allocation
rollback. It does not change the fixed store result latency or architectural
store/load values. Tests must establish the appropriate throughput unit for pair
and vector stores rather than silently counting all instructions as one scalar
store.

## Deferred mechanisms and their prerequisite

Completion-delayed fills, finite prefetch transactions, line-specific miss
merging, and eviction ordering remain worthwhile follow-ups. A correct temporal
implementation must first resolve the nonmonotone future-issue contract. Two
possible approaches are to defer memory issue until its actual time with explicit
unresolved dependencies, or to constrain memory admission to chronological order.
The first has CPU scheduling scope and host-speed costs; the second can introduce
artificial head-of-line blocking. Neither is silently adopted here. This PR must
not claim to have fixed early-fill semantics if it only calibrates parameters.

SP dependency tracking is another separately observed inherited CPU limitation;
it is not automatically included in this memory calibration change.

## Tests and evidence

- Keep functional output and architectural instruction counts identical for every
  reused case. Validate source revisions, binary hashes, effective profiles, and
  reference hashes. Compare head and umbrella base on the same inputs.
- Add targeted resource tests for independent stores, mixed load/store throughput,
  future-reservation gaps, operand/MSHR delays followed by joint-resource checks,
  store forwarding, ROB wrap, and unchanged load-only behavior. If a parameter-only
  trial wins, do not add the extra resource merely to satisfy this plan.
- Select only usable native memory references for calibration. Retain noisy points
  in the report with their existing quality labels, but do not fit them. Include
  single-chain latency, multi-chain parallelism, and read/write/copy controls at
  L1, L2, and beyond-L2 working sets. Record all candidates and negative results.
- Run the complete application suite after the final microbenchmark selection:
  264 points, plus all 140 memory cases and the existing other microbenchmarks.
  Keep microbenchmarks outside application S-curves and correlation statistics.
- On the same qualified paired application points, publish absolute `diff/min`
  mean/median, Pearson r, Kendall tau-b, correct-pair share, and error bands. List
  improved and regressed workloads and their covered sizes; aggregate improvement
  cannot conceal a large individual regression.
- Preserve current unit/short tests and required CI. Automatic accuracy runs remain
  governed by the existing post-main-merge policy; this local campaign does not
  change triggers.

## Speed and acceptance decisions

Measure wall time and retired-instruction MIPS on identical host/binary/input
contracts, using paired repeated ordering for representative chase and stream
cases. Separate changes in modeled cycles from host cost per modeled cycle. A
single tiny run or cross-host timing is not speed evidence. The target is to stay
close to the lightweight baseline; a reproducible slowdown above roughly 20% in
representative cases is a profiling/scope-review trigger, not a reason to alter
modeled latency to make the simulator run faster.

Accept a calibrated profile only with disclosed usable-point gains, copy/load
controls, functional equivalence, and full application regression evidence.
There is no predeclared promise that every workload improves or that hardware
accuracy becomes verified. If a small parameter/resource change cannot explain
large stream or parallelism errors, retain that finding and propose the next
mechanism explicitly rather than expanding into another simulator rewrite.

## Candidate interface

`poros elf --mode ooo --timing-profile candidate.json program.elf ...` accepts a
complete `configuration`/`cache_config` object matching `model-profile.json`.
Unknown fields, trailing JSON, invalid geometry/resource bounds, unsupported
functional hooks, and mismatched requested/profile frequency are rejected. A
profile is supported only for single-core OoO; it is never silently ignored for
functional, in-order, or multicore execution. Effective values are emitted in
each result. `configuration.num_stores = 0` disables the extra store resource and
is the initial default/control. Candidate profiles and measurement records live
outside the source tree until numeric selection is justified by the memory suite.
