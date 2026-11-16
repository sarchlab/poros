# Grace residual investigation

Status: **the cause of the two new failures is established and a change to
the stride follower is proposed. It has not been compared in full.** Every
number of a changed model below is one run of one input, and the inputs were
chosen, not sampled. The proposal makes three inputs worse that were within
14%, one of them to below 0.8 of the reference; they are listed under
"Worse".

Evidence is labelled **retained** (the completed comparison; nothing rerun),
**run** (one run of this work, at the measured input unless said otherwise)
or **reading** (an inference).

## Baseline (retained)

This work starts at `ecfd0a56ed22e6b9dfe18b3b6ceab344731a41a9`, the merge of
[pull request 84](https://github.com/sarchlab/poros/pull/84). Its tree equals
that pull request's head, `1a52dde`. The three-device comparison was measured
at `a061e9221027f884173104be69f6e7844b77a145` (1,656 points, 2,520
executions, certificate SHA-256
`27efcd04c06526a732d47effaf579da1ba605c43f86459310b210c2de8e9696a`). The two
revisions differ in four files, none of which the simulator executes
(`evaluation/grace-deriche-investigation.md`, `evaluation/model-profile.md`,
`scripts/accuracy_site.py`, `scripts/capacity_microbenchmark_test.py`). The
`a061e92` records are therefore the baseline of this work, under their own
revision and binary identity.

Error is |predicted − reference| / min(predicted, reference). Recomputed here
from the published rows:

| Qualified applications | Mean | Median | Maximum | Above 50% | Model / reference below 0.8 |
|---|---:|---:|---:|---|---:|
| Grace, 237 | 11.65% | 7.88% | 77.55% | symm-5, syrk-6 | 9 |
| M1, 240 | 9.18% | 5.65% | 45.79% | none | 13 |
| M2, 246 | 15.82% | 14.41% | 49.79% | none | 17 |

Grace has no native measurements of the diagnostic microbenchmarks; only the
GH200 application times are references. The nine baseline points below 0.8
are syr2k-3 (0.679), syr2k-1 (0.705), atax-1 to atax-6 (0.735 to 0.751) and
ludcmp-5 (0.798); only ludcmp-5 is new with pull request 84.

## What moved symm-5, syrk-6 and mvt

Pull request 84 made two changes together: a request issued before its line
left L2 finds it in L2 (A), and the stride follower prefetches no farther
ahead of a miss than `PrefetchStrideBytes`, one 64 KiB page (B). "Parent" is
`82e9983`, the comparison before it.

**B alone moved them (run).** A build of the baseline with each change behind
a switch reproduces the retained records to the cycle (symm-5 and syrk-6 run
by the coordinator on other hosts, mvt-3200 locally; with no switch set the
build reproduces the retained deriche-1 record on both platforms):

| Point | A off, B on | Baseline (retained) | A on, B off | Parent (retained) |
|---|---:|---:|---:|---:|
| symm-5 | 295,767,448 | 295,767,448 | 199,828,835 | 199,828,835 |
| syrk-6 | 175,221,185 | 175,221,185 | 132,550,036 | 132,550,036 |
| mvt-3200 | not run | 218,176,607 | 134,730,469 | 134,730,469 |

A's counter of answered requests is zero in the baseline records of symm-5,
symm-6, syrk-4, syrk-6, ludcmp-5 and the six mvt inputs (retained), and the
switch build counts no prefetch withheld by A in its runs of symm-5 and
syrk-6 with A on (run).

**How (run, reading).** The follower sees misses, and its step is the
distance between two of them. A column walk whose lines are mostly in L1
misses only where a row starts a new line. In the runs, 432,405 of symm-5's
trained streams have a step of 36,544 bytes with loads hitting L1 4,568 bytes
apart in between (eight rows of 571 doubles), and 197,780 of syrk-6's have
24,768 bytes with hits 3,096 apart (eight rows of 387). mvt's column walk
misses at every access, 24,576 to 29,696 bytes apart. One page is one such
step at symm-5 and two at syrk-6 and mvt. In the run that takes the
baseline's cycles at symm-5 (A off), 9.5 million of the 11.2 million stride
prefetches are issued one step ahead, and all but 0.02 million of the 10.8
million that are read are still arriving then, 94 cycles late on average; a
line returns from behind L2 in 138 cycles. The parent's 48 steps were 384
rows of the same walks: in the run that takes its cycles (B off), 17.6
million of 29.2 million stride prefetches leave L2 unread.

The baseline's `FillWaitCycles` rose with this (symm-5: 505 M to 1,228 M).
That counter sums, over requests, the readiness beyond the nominal latency of
the level that answered; waits that overlap are each counted, so it is not a
count of stalled cycles.

**What the GH200 times say (retained, reading).** Its cycles per instruction
at the assumed clock are 0.167 to 0.180 over the six syrk inputs, whose
matrix grows from 0.75 to 1.6 MB past the 1 MiB L2: the walk's lines cost it
nothing visible. mvt's whole run takes it 14.3 to 14.5 cycles per matrix
element at 3,200, 3,456 and 3,712 rows, 15.1 at 3,328, and 18.3 and 18.8 at
3,072 and 3,584. The walk's lines fall into 16, 8, 2 and 4 of the 256 L1
sets at those sizes.

## The change

Both rules replace B in `pkg/mem/stride.go`; A is untouched. Neither has a
parameter of its own, and no value was searched. They apply to the Grace
profile only: M2 and M1 have no stride follower.

1. **The reach is `PrefetchDistance` (48) accesses of the walk, not 48
   misses and not a page.** The follower is told of loads that find their
   line in L1. One that lies a whole fraction of a stream's step beyond its
   last miss, and at least a line beyond it, is an access of the walk, and
   the smallest such fraction is the walk's stride. A walk whose every
   access misses is prefetched 48 steps ahead as in the parent; one that
   misses every eighth row six steps, 48 rows.
2. **No farther than the lines ahead have room in their L1 sets.** The
   follower prefetches no line that would be the fifth of the walk's lines
   ahead in one four-way set: 8 steps for strides of 8,192 and 24,576 bytes,
   16 for 28,672, 32 for 26,624, no bound for strides that reach 16 sets or
   more. The Neoverse V2's load-side prefetcher fills L1; the model's fills
   L2 only, and the bound stands for what an L1 set can hold.

What is assumed. That the distance is 48 is carried from the sequential
prefetcher, whose 48 are lines; a distance in accesses is the unit a stride
prefetcher works in, and the GH200 times do not give its value. The follower
has no instruction address, so a load of something else at a matching
address would shorten a walk's reach. For rule 2 the evidence is three
inputs: the two mvt sizes above and deriche-6, the only Deriche input with an
8,192-byte column step, which costs the GH200 about eight cycles more per
column step than deriche-5. Why the core loses that much there is not known;
a line displaced from L1 would still be in L2, and L2's sets are as few for
these strides.

Other reach rules were run and not taken: the reach the walk covers in 138
cycles at its average speed (symm-5 1.245, syrk-6 1.467, mvt-3200 1.000,
symm-1 0.918), and the same without rule 2 (mvt-3072 0.786 against 0.964
with it). A third, that reach at the walk's fastest pace, was run on a
reduced input only.

## Results so far (run; one each; not a comparison)

Cycles in millions, model / reference and error. The first twelve rows were
run by the coordinator on other hosts with the switch build set to the two
rules; the last four locally with the product code. The product code takes
the switch build's steps: mvt-3200 gives 133,400,170 cycles in both. Outputs
and instruction counts equal the retained ones in every run.

| Grace point | Parent (retained) | Baseline (retained) | With the change |
|---|---:|---:|---:|
| symm-5 | 199.83, 1.094 (9.37%) | 295.77, 1.619 (61.88%) | 187.87, 1.028 (2.83%) |
| syrk-6 | 132.55, 1.343 (34.31%) | 175.22, 1.775 (77.55%) | 129.87, 1.316 (31.59%) |
| mvt-3200 | 134.73, 0.916 (9.17%) | 218.18, 1.483 (48.33%) | 133.40, 0.907 (10.26%) |
| mvt-3072 | 123.01, 0.714 (40.02%) | 200.23, 1.163 (16.25%) | 164.99, 0.958 (4.39%) |
| mvt-3584 | 167.66, 0.694 (44.06%) | 274.35, 1.136 (13.58%) | 193.82, 0.802 (24.62%) |
| symm-2 | 97.25, 1.114 (11.39%) | 116.29, 1.332 (33.19%) | 85.01, 0.974 (2.70%) |
| symm-6 | 178.16, 0.747 (33.92%) | 252.39, 1.058 (5.78%) | 182.20, 0.764 (30.95%) |
| syrk-4 | 104.43, 1.534 (53.37%) | 95.00, 1.395 (39.52%) | 89.89, 1.320 (32.02%) |
| syrk-5 | 96.35, 1.182 (18.24%) | 109.67, 1.346 (34.59%) | 96.68, 1.186 (18.65%) |
| syr2k-3 | 134.23, 0.682 (46.72%) | 133.66, 0.679 (47.34%) | 134.24, 0.682 (46.71%) |
| ludcmp-5 | 102.34, 0.816 (22.49%) | 100.06, 0.798 (25.28%) | 100.03, 0.798 (25.32%) |
| deriche-6 | 293.31, 2.819 (181.91%) | 91.44, 0.879 (13.78%) | 91.44, 0.879 (13.78%) |
| symm-1 | 64.66, 0.869 (15.09%) | 72.89, 0.980 (2.10%) | 61.82, 0.831 (20.38%) |
| syrk-1 | 42.92, 1.360 (35.99%) | 35.42, 1.122 (12.22%) | 35.42, 1.122 (12.23%) |
| syrk-2 | 56.08, 1.425 (42.51%) | 44.94, 1.142 (14.20%) | 45.01, 1.144 (14.38%) |
| M2, M1 deriche-2 | | 60,339,580 and 59,587,667 cycles | the same cycles |

Not run: mvt at 3,328, 3,456 and 3,712 rows, the five other Deriche inputs,
and everything else. Walks whose every access misses are prefetched as in
the parent, so the other Deriche inputs are expected near their parent
values (0.937 to 1.061, against 0.956 to 1.061 in the baseline); that is a
reading.

## Worse, or below the reference

Against the baseline, in the runs above:

- **symm-6: 5.78% to 30.95%, model / reference 1.058 to 0.764, newly below
  0.8.** It is where the parent had it (0.747).
- **symm-1: 2.10% to 20.38%, 0.980 to 0.831.** Below the parent (0.869).
- **mvt-3584: 13.58% to 24.62%, 1.136 to 0.802.** Rule 2 gives this walk 16
  steps, which hides more than the GH200 does.
- mvt-3200 changes side: 1.483 to 0.907 (10.26%). mvt-3072: 1.163 to 0.958.
- ludcmp-5 stays below 0.8: 0.7982 to 0.7980.
- syrk-2: 14.20% to 14.38%; syrk-1: 12.22% to 12.23%.

**symm (reading).** The GH200's whole run costs 3.4 to 3.7 cycles per inner
trip at the four symm inputs that do not thrash L1. By their row lengths the
column walks miss every eighth row at inputs 2 and 5 and every fourth row of
every other column at 1 and 6. With the change the model takes 3.3 and 3.6
cycles at the first two and 3.0 and 2.9 at the others. The baseline had
inputs 1 and 6 right with their lines arriving late, and inputs 2 and 5 33%
and 62% slow for the same reason. So the baseline's agreement at symm-6 was
bought with the lateness that this change removes, and what symm costs the
GH200 that the model lacks is not identified.

## Not addressed

- **syrk-6 is still 32% slow, syrk-4 32%.** The follower takes up every walk
  anew, three misses before its first prefetch; syrk-6 has 0.35 million
  demand requests behind L2 in its run. syrk-4's walk also conflicts in L1
  at its last rows (0.6 misses per row in a set model of the published
  geometry), which the GH200's time does not show.
- **ludcmp.** The baseline model runs inputs 2 to 6 at 0.156 to 0.159 cycles
  per instruction, the GH200 at 0.194 to 0.197. The stride follower issues
  0.05 million prefetches at ludcmp-5 with the change, and the time does not
  move. The parent's 0.816 there came with 1.1 million more prefetches; the
  difference of 20% is not a prefetch effect.
- syr2k-1, syr2k-3 and atax stay below 0.8 as in the baseline.

## Limits

- No Grace machine is accessible. The GH200 times are a collaborator's
  measurements on Linux with 64 KiB pages; they do not establish a physical
  prefetch distance, an L2 index function or a latency behind L2.
- The model's caches index by line number modulo the number of sets. That is
  an assumption of the model, not a measured property.
- Fifteen Grace inputs and one input each of M2 and M1 were run with the
  change. Whether the 237 keep their median, and whether another input
  crosses 50% or 0.8, is not known until the comparison is repeated.
- References, workload binaries, input lists, quality classes and the
  completed comparison are not changed by this work.
