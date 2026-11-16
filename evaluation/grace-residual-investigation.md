# Grace residual investigation

Status: **started; nothing run and nothing changed yet.** This note holds the
verified starting data, the hypotheses and the plan. Evidence is labelled
**retained** (the completed comparison; nothing rerun), **run** (a run of
this work; none so far) or **reading** (an inference).

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
GH200 application times are references.

## Residuals (retained)

"Parent" is `82e9983`, the comparison before pull request 84, given for the
direction of the change only. Values are model / reference with the error in
parentheses.

| Grace point | Parent | Baseline |
|---|---:|---:|
| symm-5 | 1.094 (9.37%) | 1.619 (61.88%) |
| syrk-6 | 1.343 (34.31%) | 1.775 (77.55%) |
| mvt-3200 | 0.916 (9.17%) | 1.483 (48.33%) |
| mvt-3328 | 0.864 (15.71%) | 1.407 (40.72%) |
| mvt-3456 | 0.923 (8.34%) | 1.497 (49.68%) |
| mvt-3712 | 0.910 (9.86%) | 1.478 (47.77%) |
| symm-2 | 1.114 (11.39%) | 1.332 (33.19%) |
| syrk-5 | 1.182 (18.24%) | 1.346 (34.59%) |
| ludcmp-5 | 0.816 (22.49%) | 0.798 (25.28%) |
| ludcmp-6 | 0.895 (11.71%) | 0.807 (23.93%) |
| ludcmp-3 | 0.874 (14.37%) | 0.809 (23.62%) |

Gains of pull request 84 that a change here must keep:

| Grace point | Parent | Baseline |
|---|---:|---:|
| deriche-6 | 2.819 (181.91%) | 0.879 (13.78%) |
| syrk-4 | 1.534 (53.37%) | 1.395 (39.52%) |
| mvt-3072 | 0.714 (40.02%) | 1.163 (16.25%) |
| mvt-3584 | 0.694 (44.06%) | 1.136 (13.58%) |
| symm-6 | 0.747 (33.92%) | 1.058 (5.78%) |
| syrk-1, syrk-2 | 1.360, 1.425 | 1.122, 1.142 |

The nine baseline points below 0.8 are syr2k-3 (0.679), syr2k-1 (0.705),
atax-1 to atax-6 (0.735 to 0.751) and ludcmp-5 (0.798). Only ludcmp-5 is new
with pull request 84.

## What the retained counters say

Pull request 84 made two changes together: a request issued before its line
left L2 finds it in L2 (A), and the stride follower prefetches no farther
ahead than `PrefetchStrideBytes` (B).

- **A answered no demand request at these points.** Its counter
  (`L2HitsBeforeEviction`) is zero in the baseline records of all six symm,
  syrk, mvt and ludcmp inputs. It is 1,003 at deriche-6. A also withholds a
  stream prefetch of a line that has not left L2 yet; that is not counted.
- **The misses are the same; the prefetches are fewer and late.** Parent to
  baseline, L1 misses and trained streams are identical at every point below.

| Point | Prefetches (M) | L2 hits on a line still arriving (M) | Fill wait (M cycles) | Cycles (M) |
|---|---:|---:|---:|---:|
| symm-5 | 29.20 → 11.23 | 2.80 → 10.77 | 505 → 1,228 | 199.8 → 295.8 |
| symm-6 | 27.11 → 15.14 | 7.28 → 14.35 | 532 → 1,376 | 178.2 → 252.4 |
| syrk-6 | 12.89 → 3.53 | 0.80 → 3.23 | 180 → 373 | 132.6 → 175.2 |
| mvt-3200 | 11.89 → 11.71 | 8.33 → 10.18 | 3,161 → 4,230 | 134.7 → 218.2 |

- **The GH200 does not show the cost.** Its cycles per instruction at the
  assumed clock are 0.167 to 0.180 over the six syrk inputs (baseline model:
  0.191 to 0.306) and 0.245 and 0.263 at symm-5 and symm-6 (0.397 and 0.279).
  mvt's whole run takes 14.3 to 14.5 cycles per matrix element on the GH200
  at 3,200, 3,456 and 3,712 rows, 15.1 at 3,328, and 18.3 and 18.8 at 3,072
  and 3,584; the parent model takes 13.0 to 13.2 at all six and the baseline
  21.2 to 21.4 at all six.
- **ludcmp is uniformly fast.** The baseline model runs inputs 2 to 6 at 0.156
  to 0.159 cycles per instruction, the GH200 at 0.194 to 0.197. The parent
  was closer mostly at inputs 3 and 6 (0.170 and 0.176; 0.158 and 0.161 at
  inputs 4 and 5), with 2.0 M and 16.4 M prefetches where the baseline has
  0.03 M and 9.4 M.

## Hypotheses (not established)

1. **The new failures come from B, not from A.** The counters above point
   that way. No run separates the two yet, and A's withheld prefetches are
   not counted.
2. **The follower measures a step between misses and its reach is counted in
   that step or bounded by the detection limit; neither follows the walk's
   accesses.** A column walk whose lines are mostly in L1 misses every few
   rows. From the arguments of the inputs, symm-5's misses would be 64 × 571
   = 36,544 bytes apart and syrk-6's 64 × 387 = 24,768; mvt's are 24,576 to
   29,696. Under a 65,536-byte bound these walks are prefetched one or two
   steps ahead, which may be too little to hide a return from behind L2;
   without the bound they were prefetched 48 steps ahead, which overran
   syrk's short walks. The miss spacing has to be read from the address
   trace before this is more than arithmetic.
3. **How much a short reach costs is set by parameters carried from M2:** the
   138-cycle return from the record of L2 victims and the 16-byte-per-cycle
   memory-side transport. Neither is established for Grace.
4. **ludcmp-5 crossed 0.8 because a prefetch cost that should not have been
   there was removed;** what remains is a flat difference of about 20% on
   inputs 2 to 6 that the cache counters do not explain.

## Plan

Local budget: 40 simulator runs, 180 s each at most, 2,700 s together, one
worker. Larger runs are requested from the coordinator.

1. Separate A and B on the failing shapes: the baseline with B alone removed
   and with A alone removed, same binaries otherwise, outputs and instruction
   counts checked against the retained records.
2. Read the walks: addresses and spacing of the misses the follower sees in
   symm, syrk and mvt, the level that supplies each line, and how far ahead
   of its use a prefetch was issued, by loop.
3. Probe the follower in isolation with synthetic walks (step, miss spacing,
   length, restart) to see which of its rules produces each behaviour.
4. Only then decide whether a general rule follows from the evidence. A
   negative finding is a valid result.
5. Controls for any candidate: the gains listed above, the five other
   Deriche inputs, and M1 and M2 at their worst points and Deriche inputs.
6. A full three-device comparison of a frozen candidate is the coordinator's;
   no accuracy is claimed from local runs.

## Limits

- No Grace machine is accessible. The GH200 times are a collaborator's
  measurements on Linux with 64 KiB pages; they do not establish a physical
  prefetch distance, an L2 index function or a latency behind L2.
- The model's L2 indexes by line number modulo the number of sets. That is an
  assumption of the model, not a measured property.
- References, workload binaries, input lists, quality classes and the
  completed comparison are not changed by this work.
