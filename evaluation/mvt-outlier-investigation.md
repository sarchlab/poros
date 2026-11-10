# MVT 3072 outlier investigation

Status: **a candidate model change is under assessment; no fix is claimed.**
The cause of the outlier is measured. The proposed change reproduces the
measured regime, and one of its predictions outside that regime failed; both
are reported below. Whether the change becomes the default is decided by
paired application results, which are not complete.

Evidence is labelled **measured** (counted on hardware), **simulated** (this
model), **published** (cited, not re-measured), or **reading** (an inference
from measured costs).

## Starting data

Source: the completed canonical report for head `63abde6` (merged as `a27923d`),
246 qualified application points, mean error 16.86193%, median 14.57811%
(error = |predicted − native| / min). One application point exceeds 50%:

| Point | Native (ms) | Model (ms) | Model / native | Instructions | Model cycles | Native ns per element | Model ns per element |
|---|---:|---:|---:|---:|---:|---:|---:|
| mvt-3072 | 45.337 | 90.404 | 1.994 | 217,196,663 | 315,508,551 | 4.804 | 9.580 |
| mvt-3200 | 33.107 | 29.408 | 0.888 | 235,667,319 | 102,635,472 | 3.233 | 2.872 |
| mvt-3328 | 41.184 | 31.853 | 0.773 | 254,891,639 | 111,166,472 | 3.718 | 2.876 |
| mvt-3456 | 46.508 | 34.346 | 0.739 | 274,869,623 | 119,868,483 | 3.894 | 2.876 |
| mvt-3584 | 52.707 | 36.991 | 0.702 | 295,601,271 | 129,100,239 | 4.103 | 2.880 |
| mvt-3712 | 53.316 | 39.653 | 0.744 | 317,086,583 | 138,388,154 | 3.869 | 2.878 |

The last two columns divide the reported native and predicted times by n², the
number of matrix elements. They are arithmetic on existing data: not new
measurements and not native cycle counts (the native core and frequency are
unobserved). A change that only makes MVT faster would push the five
underpredicted sizes further from their references.

## Workload

`tests/usertime/mvt.c` has three phases over a flat `n × n` array of doubles
with row pitch `8n` bytes: row-major initialisation (one integer remainder per
element), a row-major matrix–vector product, and a transposed product that
walks each column with a stride of `8n` bytes (24,576 bytes at n = 3072, which
is 1.5 pages of 16 KiB and 192 lines of 128 bytes).

## What the model does at n = 3072 (simulated)

- The unmodified tree reproduces the report exactly (315,508,551 cycles,
  217,196,663 instructions, committed output hash); read-only phase snapshots
  leave all three unchanged.
- 83.6% of the cycles are the transposed product: 263,739,495 cycles for
  9,437,184 column accesses, 27.95 each. 99.98% of those accesses miss L2.
  The model indexes its 16 MiB, 16-way L2 by address modulo 8,192 sets; a
  192-line stride reaches 128 sets, which hold 2,048 lines; a column needs
  3,072. Exact LRU evicts every line before its next use, and each miss is
  charged the memory latency: 335 cycles, twelve at a time.
- At the other five sizes a column fits the sets its stride reaches. Only the
  first of every sixteen walks misses, giving 4.56 cycles per access.

## Measurements on an M2

Three captures on one Mac mini M2 under Asahi Linux 6.14.2 (16 KiB pages),
performance core 4 pinned, user-mode cycles and instructions counted with
`perf_event_open`, 12 mirrored blocks each. Every capture was run once by the
coordinator from a frozen binary, with the checks and reading rules written
down beforehand; all outputs equal an independent oracle, every interval ran
on core 4 inside its instruction contract, and the counters were never
multiplexed. A simulated ELF runs the same assembled loops (the loop that
reads the counter after every hop is native only). Native time used: 71.9 s.
The application references are macOS measurements and are unchanged.

### 1. Strided walks (measured)

One column of a matrix is walked repeatedly: with the MVT loop (`walk`,
cycles per access) and as a shuffled dependent pointer chase over the same
lines (`chase`, cycles per hop). Huge pages / 16 KiB pages.

| rows × stride (bytes) | walk | chase | committed model walk / chase |
|---|---:|---:|---:|
| 1,024 × 24,576 | 3.01 / 3.46 | 17.8 / 26.1 | 3.05 / 14.0 |
| 2,048 × 24,576 | 8.68 / 7.10 | 102.7 / 85.7 | 6.68 / 14.3 |
| **3,072 × 24,576** | **10.39 / 11.26** | **125.2 / 141.6** | **27.93 / 334.9** |
| 4,096 × 24,576 | 11.52 / 11.99 | 138.4 / 177.4 | 27.96 / 334.9 |
| 1,536 × 16,384 | 11.52 / 11.05 | 138.2 / 138.3 | 27.93 / 334.8 |
| 2,048 × 16,384 | 11.53 / 11.53 | 138.2 / 142.9 | 27.96 / 334.8 |
| 3,072 × 25,600 | 3.00 / 3.00 | 19.2 / 29.1 | 3.00 / 14.0 |
| 3,200 × 25,600 | 3.00 / 4.22 | 19.2 / 39.0 | 3.00 / 14.0 |
| 4,096 × 25,600 | 3.00 / 6.93 | 19.3 / 54.9 | 3.00 / 14.0 |
| 3,584 × 28,672 | 3.08 / 7.66 | 19.8 / 93.5 | 3.00 / 14.0 |

- At the MVT 3072 shape the M2 pays 10.4 to 11.3 cycles per access and 125 to
  142 per hop. The model charges 27.9 and 335.
- Where the walk conflicts hardest, three shapes cost 138.2 to 138.4 per hop
  on huge pages, against 358 to 375 for the committed random chase over 64 to
  256 MiB.
- With no conflict (25,600 bytes) the walk costs the same on both page sizes
  up to 3,072 pages and more beyond on 16 KiB pages.

### 2. Reuse distance (measured; a prediction that failed)

To test whether lines stay reachable after L2 evicts them, a fixed 512 KiB
target was warmed, N MiB of other memory was read once in address order, and
one shuffled chase over the target was timed. The prediction written down
before the capture was 117 to 159 cycles per hop at 20 to 28 MiB and memory
cost from 34 MiB.

| MiB read in between | M2, huge pages | M2, 16 KiB pages | committed model | candidate |
|---:|---:|---:|---:|---:|
| 0 | 19.0 | 19.0 | 13.7 | 13.7 |
| 8 | 19.2 | 19.4–19.5 | 13.9 | 13.9 |
| 16 | 20.2–20.5 | 20.8–22.0 | 328.5 | 135.4 |
| 20 | 21.8–22.7 | 23.2–25.2 | 328.5 | 135.4 |
| 24 | 25.2–26.2 | 27.4–29.7 | 328.5 | 328.5 |
| 32 | 35.5–36.5 | 37.8–38.5 | 328.5 | 328.5 |
| 48 | 49.0–50.6 | 52.3–53.1 | 328.5 | 328.7 |
| 96 | 74.0–77.3 | 83.4–84.1 | 328.5 | 328.7 |

The prediction failed: 22 to 31 cycles at 20 to 28 MiB, and no plateau
anywhere. Cost rises 0.6 to 0.7 cycles per hop per MiB and is still rising at
96 MiB. At 20 MiB at least 94% of the hops still cost what an L2 hit costs
(taking every other hop at only 138 cycles).
Reading the other memory in shuffled order, or twice, pushes the target out
no faster (third capture). A second chase between two reads resets the clock;
an efficiency-core thread reading its own memory in between changes nothing;
touching the same pages but 1/128 of the lines changes nothing.

So this M2 does not evict recently used lines from L2 in order of age when
other memory is read. The model's exact-LRU L2 does. That is a model gap in
its own right. It is in the committed model, the candidate does not address
it, and no cache size can be read from this curve.

### 3. One hop at a time (measured)

The chase reads the 24 MHz virtual counter after every hop (after an `ISB`,
so the read follows the load). One tick is 146 cycles: an L2 hit plus the
loop's 31 cycles of overhead is a third of a tick, a 138-cycle return about
1.2 ticks, a memory access 2.5 or more. Share of hops by ticks, two huge-page
allocations (36 chases each):

| chain (rows × stride) | 0 ticks | 1 tick | 2 ticks | 3 or more | plain chase, cycles per hop |
|---|---:|---:|---:|---:|---:|
| 3,072 × 25,600 (no conflict) | 64–65% | 35–36% | 0 | 0 | 19.5 |
| 1,536 × 16,384 | 0.4–0.5% | 78–81% | 19–22% | 0.0% | 132.6–136.1 |
| 4,096 × 24,576 | 5.5–5.7% | 70–72% | 18–19% | 4.3–5.4% | 141.8–143.3 |
| 3,072 × 24,576 | 13.7–13.8% | 66–69% | 16–18% | 1.4–2.2% | 114.0–118.1 |

The rules fixed before the capture named two outcomes, and neither holds at
all three shapes:

- *A mix of L2 hits and memory accesses* (at most 70% of hops slower than an
  L2 hit, at least 8% at three or more ticks) **fails at every shape**: 78 to
  99% of hops are slower than an L2 hit and 0 to 5.4% take three or more
  ticks. The 138 cycles of the first capture are not an average of fast and
  slow hops.
- *One intermediate latency* (at least 90% slower than an L2 hit, 80% at the
  3,072-row shape; at most 3% at three or more ticks) **holds at
  1,536 × 16,384 and misses at the other two**: 4.3 and 5.4% at three or more
  ticks at 4,096 rows, and 78.4 and 78.9% slower than an L2 hit at 3,072 rows.

What the counts establish: in a conflicting walk most hops, two thirds to four
fifths at exactly one tick, take a time that is neither an L2 hit nor a
memory access. What they do not establish: that every such hop does, or what
supplies the line.

Reading (made after the capture, not one of its rules): as three kinds of hop,
the shares are 99% intermediate at 1,536 × 16,384; 85% intermediate, 8 to 9%
as fast as an L2 hit and 6% as slow as memory at 4,096 × 24,576; 74 to 76%,
21 to 22% and 3 to 4% at 3,072 × 24,576. The intermediate cost that then
reproduces each shape's separately counted plain chase is 132 to 140 cycles
at all three.

The same counter on the reuse-distance target after 96 MiB of reading: about
15% of hops are slower than an L2 hit and most of those cost a memory access
(360 to 369 cycles when read in order).

## Diagnosis

- **Measured.** At the MVT 3072 shape the M2 pays about 11 cycles per column
  access where the model charges 28, and about 130 per dependent hop where
  the model charges 335. This is the outlier. MVT 3072 is the one formal
  point whose hot loop is such a walk.
- **Measured.** The M2's hops there are mostly of intermediate cost. They are
  not the model's memory accesses, and they are not a mix of L2 hits and
  memory accesses.
- **Reading.** A line that conflict has just pushed out of L2 usually comes
  back from something closer than memory. Published work on the M1
  (EXAM, arXiv 2504.13385) describes an 8 MB system-level cache that is
  exclusive of the CPU caches and receives L2 victims; nothing here
  identifies the M2's structure.
- **Measured, separate.** With no cache conflict, walks over more than 3,072
  pages cost more on 16 KiB pages than on huge pages. The model has no
  translation cost. This concerns 3200 to 3712, not 3072.
- **Not supported by anything measured:** a different L2 index for 3072, a
  prefetcher covering the walk, an instruction-side cause.

## Candidate change

Two mechanisms, each switched off by one configuration field
(`SLCSize = 0`, `TLBPageBytes = 0`). Details and the basis of each value are
in [the model profile](model-profile.md#memory-side-cache-and-translation-reach).

**A. Record of recent L2 victims.** An L2 miss on a line that left L2 within
the last 65,536 evictions from L2 (8 MiB of lines) arrives after 138 cycles
instead of 335 and the line returns to L2. The record has no sets. A clean
victim's move occupies a finite transport of its own; a dirty victim's write
is charged as before; a returned line cannot arrive before it left L2, before
its move ended, or before its data existed. Twelve miss entries, the memory
latency, both fill transports, the L2 geometry and its LRU are unchanged.

- 138 cycles: the measured cost of the intermediate hops (first capture
  138.2 to 138.4; third capture 132 to 140 by the plain chases).
- 8 MiB: the published M1 figure. Nothing measured here gives a size; the
  conflicting walks need about 0.3 MiB, and MVT 3072 differs by 0.02% with a
  1 MiB record.
- Bounded effect: it can only make an L2 miss cheaper when that same line
  left L2 within the last 8 MiB of evictions. First-touch misses and
  streaming bandwidth are unchanged.

**B. Translation reach.** 3,072 pages of 16 KiB (256 sets of 12, LRU); a
missing page starts a 30-cycle walk on one of four walkers; a load's cache
access waits for its translation, a forwarded load does not, a store's slot
is held until its translation. 3,072 is the second-level TLB size given for
this core in Apple's optimization guide (read through a secondary source);
30 cycles and four walkers are effective values from the first capture's
averages.

Neither mechanism reads a benchmark name, path, hash or size, and neither
parameter was taken from an application reference.

## Results so far (simulated)

These runs use a diagnostic driver that builds the same model objects as
`poros elf` and can switch either mechanism off. On the unmodified tree it
reproduces the report's MVT 3072 cycles exactly; on the candidate tree with
both mechanisms off it gives the unmodified tree's cycles on a probe ELF
(108,728,296).

Every cycle count in this section was produced by commit `cc29961`. Review
then found three defects, corrected since: a page walk delayed the base
register of a pre- or post-indexed load as well as its value; a line's
return from the record could occupy the fill transport before the line was
available there (only when the return latency is below the transfer time,
which the defaults are not); and translation could loop on the last
addressable page with a one-byte page. The first changes default timing
slightly wherever a walk falls on an indexed load, so the corrected
candidate's cycles will not be exactly those below. They have not been
re-run; the canonical comparison on the corrected candidate replaces them.

**Probes.** All 54 points of the first capture, candidate against committed
model, error = |model − M2| / min, M2 on 16 KiB pages:

| | mean error | points over 50% |
|---|---:|---:|
| committed model | 125.4% | 34 of 54 |
| candidate (A + B) | 54.7% | 17 of 54 |

| rows × stride | M2 walk / chase (16 KiB) | committed | candidate |
|---|---:|---:|---:|
| 3,072 × 24,576 | 11.26 / 141.6 | 27.93 / 334.9 | 11.57 / 138.0 |
| 4,096 × 24,576 | 11.99 / 177.4 | 27.96 / 334.9 | 11.56 / 168.0 |
| 1,536 × 16,384 | 11.05 / 138.3 | 27.93 / 334.8 | 11.51 / 137.9 |
| 3,200 × 25,600 | 4.22 / 39.0 | 3.00 / 14.0 | 3.90 / 27.8 |
| 4,096 × 25,600 | 6.93 / 54.9 | 3.00 / 14.0 | 7.51 / 44.0 |
| 3,584 × 28,672 | 7.66 / 93.5 | 3.00 / 14.0 | 7.52 / 44.0 |

Rows that get worse with the candidate: at the edge of conflict the M2 starts
losing lines earlier than a modulo index does, and the committed model's few
memory accesses there happened to cost about what the M2 pays. With the
record those become 138-cycle returns and four walk rows move from 0.74–0.94
of the M2 to 0.43–0.51 (2,048 × 24,576 and 1,024 × 16,384, MVT loop and
gather loop).

**Application.**

| Point | Native (ms) | Committed model / native | Candidate cycles | Candidate / native | Candidate error |
|---|---:|---:|---:|---:|---:|
| mvt-3072 | 45.337 | 1.994 | 169,438,252 | 1.071 | 7.1% |
| mvt-3200 | 33.107 | 0.888 | 110,791,611 | 0.959 | 4.3% |
| mvt-3328 | 41.184 | 0.773 | 137,880,117 | 0.959 | 4.2% |
| mvt-3456 | 46.508 | 0.739 | 167,237,017 | 1.030 | 3.0% |
| mvt-3584 | 52.707 | 0.702 | 182,945,249 | 0.995 | 0.6% |
| mvt-3712 | 53.316 | 0.744 | 196,250,962 | 1.055 | 5.5% |

Mean error of the six MVT points: 42.3% with the committed model, 4.1% with
the candidate; none above 50% and none below 0.8. Every run's instruction
count, guest output hash and ELF hash equal the committed identities. Per
phase at n = 3072 the transposed product falls from 27.95 to 12.47 cycles per
access (94% of its L2 misses return from the record); at 3200 and 3328 it
rises from 4.56 to 5.34 and 6.96 (0.47 and 0.75 page walks per access).

Each mechanism does one job (three ablation runs so far):

| Run | Cycles | Model / native | Committed |
|---|---:|---:|---:|
| mvt-3072, A only | 169,414,974 | 1.071 | 1.994 |
| mvt-3072, B only | 315,534,177 | 1.994 | 1.994 |
| mvt-3200, A only | 102,300,060 | 0.885 | 0.888 |

The record alone accounts for the whole change at 3072; translation alone
leaves the outlier where it was; the record alone leaves a size that does
not conflict where it was.

Three other application points under the candidate:

| Point | Committed cycles | Candidate cycles | Change |
|---|---:|---:|---:|
| symm-1 | 59,879,435 | 59,879,468 | +33 |
| canneal-1 | 46,043,260 | 46,043,290 | +30 |
| usertime `memory`, 32 MiB | 68,201,316 | 68,201,351 | +35 |

Each ends normally with the committed instruction count and output. symm-1
stays inside L2 and touches 180 pages; the other two are sequential and
exceed L2. All three are unchanged, as expected of workloads that neither
re-use lines L2 has just evicted nor walk more than 3,072 pages. The `memory`
case is a sequential read-modify-write loop; it is not the pointer-chase
microbenchmark and says nothing about the candidate's accuracy on dependent
accesses.

A first attempt at symm-1 is excluded: it was run with a 1 MiB output limit,
stopped at the guest's final write, and is recorded as a failed run, not as
a result.

The six MVT points are the ones the change was designed around, so they show
that it does what it was meant to do, not that it is safe. The remaining
ablations and the control workloads that could regress (SYR2K, Nussinov,
GEMM, Jacobi1d, Streamcluster, Facesim, the other SYMM, Canneal and memory
points, and the memory microbenchmarks with their pointer-chase rows) are
not complete; this section is extended only with completed runs. The paired
canonical comparison of the whole application set is still to be run on the
corrected candidate.

## Limits and open points

- **Read pressure** (capture 2 above): committed model and candidate are both
  4 to 16 times too slow for a chase over recently used lines after 16 to
  96 MiB of other reading. Not addressed.
- **Not every return is intermediate.** About 6% of the M2's hops at the
  largest conflicting shape cost a memory access; the candidate has none. At
  3,072 rows the M2 keeps about a fifth of the lines in L2; the candidate
  keeps none there and its chase is 17 to 21% too slow on huge pages.
- **Conflict onset** is later in the model than on the M2 (rows above).
- **Scattered physical pages** (3,584 × 28,672 on 16 KiB pages) are not
  modelled; the candidate's 3584 result rests on translation alone.
- **Dependent L2 hits** cost 14 cycles in the model and 19 on the M2.
- Size, organisation and identity of whatever supplies the intermediate hops
  are not measured. One host; Linux; the references are macOS.
- The audit's three generic items (flags readiness of conditional branches,
  unclassified FP operations, stack-pointer dependencies) were not found to
  bear on MVT and are untouched.
- Found on the way and left unchanged: the functional emulator executes
  register-variable shifts (`LSL/LSR/ASR/ROR Xd, Xn, Xm`) as shifts by zero.

## Scope and unchanged references

- Benchmark sources, ELF files, native references and published report data
  are unchanged.
- No behaviour keyed on benchmark name, path, hash or size; no reference
  lookup; no threshold tuning; no arbitrary delay.
- Finite resources, real arrival times, issue reordering and eviction and
  writeback dependencies stay in the model; the candidate adds one finite
  transport and one finite set of walkers.
- Microbenchmarks are evaluated separately and are outside the single-outlier
  statement above.
- The probe sources, binaries and raw captures are kept with the
  investigation record, not in this repository; integrating the probes as
  shared microbenchmarks is left open. SHA-256 of the native collectors:
  `cd22d9ca…f8adf0` (strided walks), `38142702…128438c` (reuse distance),
  `13599b46…28e5a0` (per hop).
- A negative finding is a valid result of this work.
