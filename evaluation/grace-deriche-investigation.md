# Grace Deriche outlier investigation

Status: **the cause is established; two model changes are proposed; no
accuracy change is claimed.** Every number of a changed model below is one
local run, not a campaign. The three-device comparison has not been repeated.

Evidence is labelled **retained** (the completed comparison of the parent
revision; nothing rerun), **run** (a local run of this work), or **reading**
(an inference).

## Starting data (retained)

The completed three-device comparison of parent revision
`82e998387a52ad243b2390c7fc788d600a9437e2`, error = |predicted − reference| /
min. Two of the 237 qualified Grace application points exceed 50%:

| Point | GH200 reference (ms) | Model (ms) | Model / reference | Error |
|---|---:|---:|---:|---:|
| deriche-6 | 30.342 | 85.537494 | 2.8191 | 181.9112% |
| syrk-4 | 19.857 | 30.453765 | 1.5337 | 53.3654% |

The other five Deriche inputs are within 4.2 to 6.7% (6.10, 4.18, 6.68, 5.01
and 5.04% for inputs 1 to 5). Before the Grace profile deriche-6 was 28.7035%
and syrk-4 3.0954%. The Grace parameters and their sources are in
[the model profile](model-profile.md#nvidia-grace-profile); the reference
times and their limits are in the
[GH200 measurement record](usertime/grace-native/README.md).

## Workload

`polybench/kernels/kernel_deriche.c` makes two passes along the rows of a
`w × h` image, two along its columns and two sums. The four arrays are static,
12 MiB each and 12 MiB apart in the measured ELF, so element `x` of any two
of them falls into the same set of every cache. A column pass steps `8h`
bytes. deriche-6 is `1536 × 1024`: the only input whose step, 8,192 bytes, is
a power of two. In the Grace model's L2 (1 MiB, eight ways, 2,048 sets of
64-byte lines) such a walk reaches 16 sets.

The compiled backward column loop stores `y2[x]` and then loads `result[x]`;
the forward loop loads `result[x]` first. The store's value depends, through
four FP operations, on the load of the previous trip.

## What the parent model does (retained and run)

- **Retained.** deriche-6 against deriche-5: L2 misses 5.61 M against 1.32 M,
  lines returned by the record of L2 victims 8.20 M against 2.07 M, 2.14
  cycles per instruction against 0.55.
- **Run.** The same binary on `256 × 1024` (not a measured input; the same
  counter rates per column step as deriche-6) takes 37,771,863 cycles. A
  build that records cycles by loop reproduces that count, the instruction
  count and the output. The backward column pass takes 27.52 M cycles for
  262,144 steps, 105 per step; the forward one 3.05 M, 11.7 per step; the row
  passes 9.4 and 10.4.
- **Run.** In the backward pass the load is supplied by the record of L2
  victims 172,966 times of 262,144 and waits 396 cycles on average, where a
  return from the record takes 138. Dispatch stands still for 27.0 M of the
  27.5 M cycles with the FP issue queue full, 23.6 M of them with that load
  at the head of the window.

**Reading.** Each of the 16 sets holds the walk's lines of two arrays and
three prefetched lines (the walk is prefetched 48 steps ahead and returns to
a set every 16): nine lines for eight ways. In the backward loop the store is
visited first. Its allocation replaces the least recently used line of the
set, which is the prefetched line the load is about to read. Cache tags
change in the order instructions are dispatched, but the store's access is
timed at its issue cycle, when its data is ready, hundreds of cycles later.
The record of L2 victims would not supply a line before the cycle it left L2,
so the load, issued long before, waited for the store and then for a 138-cycle
return; and the next trip's store waits for that load. Every trip pays one
return behind the previous one. A core does not delay a load for an eviction
that happens after it. In the forward loop the load touches the line first
and nothing of the kind happens.

## Proposed changes

**A. A request issued before its line left L2 finds it in L2**
(`pkg/mem/cache.go` `notYetEvicted`, `pkg/mem/slc.go`). The record keeps the
cycle a line left L2 and the cycle its data was there. An earlier demand
request is answered from that as an L2 hit, and a stream prefetch of such a
line is not issued. Nothing else changes: no tag is replaced, no transfer is
booked or awaited, and the line stays in the record, so every replacement
decision, writeback and booking is the parent's. Replacement is not replayed:
a line read before its eviction is still the one that was evicted, and its
next request after that cycle pays a return from the record, where a core
might have kept it. No parameter, device, benchmark or address is involved.
It applies to every device that has the record.

A first form of A put such a line back into the tags. Review found that this
could displace a dirty line without waiting for a writeback slot, and the
form above replaced it. Below, "first form" marks the runs that were not
repeated.

**B. The stride follower prefetches no farther ahead than its stride limit in
bytes** (`pkg/mem/stride.go`). The follower's lookahead was 48 steps, and a
step is the distance between two misses. A walk that finds most of its lines
in L1 misses only every few accesses, so its lookahead was many times the
walk. This is what makes syrk slow, see below. The bound is the existing
`PrefetchStrideBytes` (65,536 on Grace); like that limit it is an assumption
and not a published property. The lookahead had never been declared: its 48
were the sequential prefetcher's distance in lines, reused as a count of
steps of any length. Why a byte bound is needed and why the page was taken
for it is argued in the
[model profile](model-profile.md#nvidia-grace-profile); one value was tried
and none was searched. It has no effect on a device without a stride limit
(M2, M1).

Either change alone removes the deriche-6 outlier: with a shorter lookahead a
set no longer holds more lines than ways, and without the wait the
replacement costs nothing.

## Results so far (run; local, one run each)

Instruction counts and output hashes equal the retained ones in every run.

| deriche-6 (reference 30.342 ms) | Cycles | Model / reference | Error |
|---|---:|---:|---:|
| parent (retained) | 293,308,066 | 2.8191 | 181.91% |
| A | 81,920,607 | 0.7874 | 27.00% |
| B | 91,575,563 | 0.8802 | 13.61% |
| A and B | 91,474,016 | 0.8792 | 13.74% |

The `256 × 1024` input takes 14,138,500 cycles with A against 37,771,863. A
build of A's first form that records cycles by loop took 14,119,956: every
loop before the backward column pass took the parent's cycles exactly, and
that pass fell from 27.52 M to 3.89 M cycles (14.9 per step). With the
prefetch part of A left out the input takes 14,126,430 cycles: here it is the
demand request that matters.

Other measured inputs with A and B, model / reference (error). "Early hits"
are the requests A answered (`L2HitsBeforeEviction`; the first form counted
prefetches and writebacks as well as demand requests):

| Point | Parent (retained) | A and B | Early hits | Form of A run |
|---|---:|---:|---:|---|
| grace deriche-1 | 1.0610 (6.10%) | 1.0609 (6.09%) | 707 | present |
| grace deriche-5 | 0.9520 (5.04%) | 0.9621 (3.94%) | 18 | first |
| grace syrk-1 | 1.3599 (35.99%) | 1.1222 (12.22%) | 0 | both |
| grace syrk-4 | 1.5337 (53.37%) | 1.3952 (39.52%) | 0 | first |
| grace mvt-3072 | 0.7142 (40.02%) | 1.1625 (16.25%) | 0 | first |
| m2 deriche-6 | 1.0068 (0.68%) | 1.0075 (0.75%) | 1 | present |
| m1 deriche-6 | 1.0998 (9.98%) | 1.1008 (10.08%) | 1 | present |

Where the first form counted no early hit, no request of any kind came before
an eviction and the two forms of A take the same steps: syrk-1 was run with
both and has the same cycle count. With the first form of A alone, grace
syrk-1, mvt-3072, gemver-1 and trmm-1 and m2 and m1 mvt-3072 had no early hit
and the retained cycle count, to the cycle.

On M2 and M1, deriche-6 now takes 0.07% and 0.08% more cycles than the parent;
with the first form it took 0.06% fewer. 8,192 stream prefetches asked for a
line that had not left L2 yet and were not issued, and the demand requests for
those lines came after the eviction and paid the return. M1 deriche-6 moves
from 9.98% to 10.08%.

## Syrk (retained, run and reading)

syrk's inner loop reads `A[j*m+k]` down a column, `8m` bytes apart, and most
of those reads find their line in L1: with an odd `m`, four of the six
inputs, its misses are eight rows apart. The follower took that distance as
its step and prefetched up to 48 of them: hundreds of kilobytes beyond a walk
of at most 516 rows that restarts for every column. Five of the six inputs
have 3.3 to 12.9 million
prefetches and are 18 to 53% slow; the sixth, whose `8m` is a whole number of
lines, has 0.39 million and is 8% slow (retained). syrk-1 takes 42.92 M
cycles as measured, 36.70 M with the follower switched off and 35.42 M with
change B (run); the GH200's time is 31.56 M cycles at the assumed clock.

syrk-4 stays the largest syrk error with B, 39.5%. Its 11.1 M L1 misses are
2.6 times what its size predicts and do not change with B; this second
component is not explained here.

## Open

- **No campaign.** These are single local runs of 12 of the 1,656 points.
  Whether A and B keep the Grace median and the other 235 points where they
  are, and whether any point crosses 50% or falls below 0.8, is not known.
  mvt-3072 changes sign with B, from 29% fast to 16% slow.
- **deriche-6 is now too fast** (0.88; 0.79 with A alone). The GH200 pays
  about eight cycles more per column step at this input than at deriche-5;
  the model pays about five with both changes and two with A alone. The
  conflict costs the hardware something the model does not have.
- **syrk-4** is above 30% in every variant.
- The record's 138-cycle latency and its unlimited associativity are M2's
  and are not established for Grace; with B more of the Grace results depend
  on them.
- M2 and M1 change wherever A applies: 0.07% and 0.08% more cycles at
  deriche-6, which both already over-predict, none at mvt-3072. Other points
  were not run.
- A answers a request that comes before an eviction and leaves the eviction
  as it was decided in dispatch order. A line that was read just before it
  was replaced is fetched again afterwards, which costs the model a return
  the hardware may not pay.

## Scope and unchanged references

- References, workload binaries, the completed comparison and its published
  report data are unchanged.
- No behaviour is keyed on a benchmark name, path, hash, size or address, and
  no number was chosen by search. B reuses a limit the profile already had.
- A negative finding is a valid result of this work.
