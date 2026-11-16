# Residual accuracy, round 2

The M1 and M2 target profiles now bound the number of integer multiplies
waiting to issue at **43 and 38**, respectively. With the existing 256-entry
reorder window and 360 FP physical registers, the simulated cycles per iteration
for the 64-filler capacity probe agree within 3% of the native cycles counted
on each core (section 5). In the certified three-device comparison,
all 792 whole-program cycle counts matched the baseline and 12 Apple capacity
microbenchmarks improved (section 6).
The larger M1 window tested alongside this change raised fdtd2d-1 error from
42.17% to 46.46% and failed its predeclared acceptance rule (section 5).
M2's own probes show
it fetching across a taken branch where its profile does not; what that rule
alone would do to the M2 applications was not run. A larger reorder window
was run at three bracket sizes. At 512, on eight measured M1 inputs, it
brings five over-predicted ones to within 10% of their references, leaves
vips-1 where it was, takes fdtd2d-6 from 45.79% to 49.35% and takes ludcmp-6
from 0.801 to 0.743 of its reference; at 630 it takes M2 doitgen-1 to 64%.
No size for it is accepted. Counted cycles on both cores then showed that
neither stops at 256 instructions, and that work which waits bounds the
overlap of the probe's multiply chains on both and of doitgen's chains on
M2. The source of the latter limit remains unresolved. What was measured, what the
runs showed and what is missing is below.

Evidence is labelled **retained** (the certified `b6de5ca` comparison and the
committed native references; nothing rerun), **run** (one local run of this
work each, 40 in all), **remote run** (one run each by the coordinator,
eight at window 512, twelve for the combined candidate and four for the
subsequent queue-only test, all audited), **counted** (four native captures of user-mode
cycles made for this work, section 5), **reading** (arithmetic on retained
or counted numbers) or
**hypothesis** (an explanation that the available evidence does not test).

## Starting point (retained; nothing rerun)

This work starts at `5ba7137486c31c0b16d93e7195706544b8563fca`, the merge of
[pull request 85](https://github.com/sarchlab/poros/pull/85). The
three-device comparison was measured at
`b6de5ca5d9f4fa2ade4053ce52765fcbc350a008` (1,656 points, 2,520 execution
records, certificate SHA-256
`15358deb6284a4d2f5204fd1c7419badb16f99caeec3af0b20fb0de96b4f5819`). The two
revisions differ in three files, all documents
(`evaluation/grace-deriche-investigation.md`,
`evaluation/grace-residual-investigation.md`, `evaluation/model-profile.md`),
so the `b6de5ca` records are the baseline of this work under their own
revision and binary identity. References, inputs, quality classes and
exclusions are unchanged.

Error is |predicted − reference| / min(predicted, reference). Recomputed
here from the retained rows:

| Qualified applications | Mean | Median | Maximum | Within 50% | Model / reference below 0.8 |
|---|---:|---:|---:|---:|---:|
| Grace, 237 | 10.61% | 7.87% | 46.71% | 237 | 10 |
| M1, 240 | 9.18% | 5.65% | 45.79% | 240 | 13 |
| M2, 246 | 15.82% | 14.41% | 49.79% | 246 | 17 |

The acceptance limits (each device's median at most 20%, every qualified
application within 50%) are met at the baseline and must stay met.

## Largest residuals at the baseline (retained)

Model / reference, error. "Low" means the model is faster than the hardware.

| Device | Inputs | Model / reference | Error |
|---|---|---:|---:|
| Grace | syr2k-3, syr2k-1 | 0.682, 0.712 (low) | 46.71%, 40.48% |
| Grace | atax-1 to atax-6 | 0.735 to 0.751 (low) | 33.21% to 36.13% |
| Grace | syrk-4, syrk-6 | 1.320, 1.316 (high) | 32.02%, 31.59% |
| Grace | symm-6, symm-1, mvt-3584 | 0.764, 0.831, 0.802 (low) | 30.95%, 20.38%, 24.62% |
| M1 | fdtd2d-1 to fdtd2d-6 | 0.686 to 0.712 (low) | 40.44% to 45.79% |
| M1 | vips-1 to vips-6 | 0.704 to 0.724 (low) | 38.06% to 42.06% |
| M1 | ferret-1 to ferret-6 | 1.354 to 1.356 (high) | 35.44% to 35.62% |
| M2 | jacobi1d-1 to jacobi1d-6 | 1.458 to 1.498 (high) | 45.83% to 49.79% |
| M2 | streamcluster-1 to streamcluster-6 | 0.673 to 0.723 (low) | 38.23% to 48.49% |
| M2 | gemm-1 to gemm-6 | 1.422 to 1.472 (high) | 42.24% to 47.15% |
| M2 | nussinov-1, nussinov-3 to nussinov-6 | 1.325 to 1.400 (high) | 32.45% to 39.95% |

## 1. M2 fetch groups (retained; measured on M2)

The M2 profile ends a fetch group at every taken branch; the M1 profile
continues across one. The committed M2 throughput probes are M2 measurements
of exactly this (cycles per trip at the suite's 3.49 GHz):

| Probe, instructions per trip | M2 native | M2 model | M1 model |
|---|---:|---:|---:|
| nop_32, 34 | 4.250 | 5.00 (+17.6%) | 4.25 |
| nop_128, 130 | 16.23 | 17.00 (+4.7%) | 16.25 |
| nop_256, 258 | 32.28 | 33.00 (+2.2%) | 32.25 |
| nop_512, 514 | 64.13 | 65.00 (+1.4%) | 64.25 |
| mixed_16, 194 | 24.19 | 25.00 (+3.4%) | 24.25 |

The register form of the Nussinov loop (eleven instructions) counts 1.392
cycles per trip on M2 and 2.018 in the M2 model. In these probes M2 runs N
instructions per trip in N/8 cycles, as M1 does. That is what was measured;
it is a statement about these loops, not about a fetch mechanism.

**Effect on the applications: not isolated (hypothesis).** jacobi1d (a
ten-instruction loop), gemm, syrk and floydwarshall are 22 to 50% slow on M2,
and the rule is a candidate reason. No M2 run with only that rule changed
was made. The one estimate at hand is a counterfactual that does not isolate
it: the M1 model's retained cycles against M2's references. The M1 model
differs from the M2 model by the fetch rule, the store-run rule, the
dependence rule and the L2 latency together, and it ran a binary with a
different layout. By that estimate jacobi1d would be 0.91 to 0.94, gemm 0.91
to 0.94, syrk 0.91 to 0.93 and floydwarshall 0.79 to 0.83, and fdtd2d 0.66
to 0.70 and vips 0.65 to 0.67 (0.88 to 0.93 and 0.82 to 0.84 today), with
vips-1, vips-2, vips-3 and fdtd2d-3 past 50%. Which of the four differences
moves which kernel it does not say. fdtd2d and vips are M1's largest
residuals with all four in place (section 4).

## 2. The reorder window (run; exploratory)

The model's window is 256 instructions on every device. The committed M1
profile note says its divide-chain probes suggest about 370 and that a
window of 512 reproduces M1's L1-resident distance loop; a unit test keeps
the 256 explicit. Runs used an experimental build that reproduces the
certified records with no option set (M2 and M1 trisolv-1, to the cycle) and
changes only the window when asked; the FP-destination limit stayed at 360.

Measured inputs, model / reference (one run each; output and instruction
count equal to the certified record in every run):

| Input | Window 256 (retained) | 370 | 512 | 630 |
|---|---:|---:|---:|---:|
| M1 swaptions-1 | 1.270 | 1.139 | 0.986 | 0.882 |
| M1 deriche-1 | 1.047 | | 1.031 | |
| M1 nussinov-1 | 1.025 | | 1.023 | |
| M1 doitgen-1 | 0.994 | | | 0.993 |
| M1 2mm-1 | 0.992 | | 0.992 | |
| M1 gesummv-1 | 0.988 | | 0.988 | |
| M1 gramschmidt-1 | 0.987 | | 0.974 | |
| M1 durbin-1 | 0.985 | | 0.978 | |
| M1 canneal-1 | 0.981 | | 0.981 | |
| M1 trisolv-1 | 0.924 | | 0.924 | |
| **M2 doitgen-1** | 0.884 | | | **0.610** |
| M2 integer-16000000 | 0.885 | | | 0.885 |

Reduced inputs of M1 kernels (not measured inputs: no reference, so these
are cycle ratios against a run of the same input with window 256, with equal
output and instruction count, and say nothing about the measured sizes):
ferret 0.78 at 370 and 0.68 at 512 and 630; bodytrack 0.74 at 512 and 630;
raytrace 0.80 and blackscholes 0.86 at 630; ludcmp 0.96 at 512 and 630;
syr2k 0.99 and x264 1.00 at 512; cholesky 1.00 at 630.

Eight measured M1 inputs at window 512 (remote runs by the coordinator with
the same experimental source, each audited by the coordinator against the
canonical output and instruction checks; parity with the certified M1
trisolv-1 record was its precondition for them). They were chosen as the
worst residuals and as the measured sizes of kernels run reduced above.
Model / reference and error:

| Input | Window 256 (retained) | Window 512 |
|---|---:|---:|
| M1 ferret-1 | 1.354 (35.44%) | 0.914 (9.40%) |
| M1 bodytrack-1 | 1.323 (32.28%) | 0.982 (1.85%) |
| M1 raytrace-3 | 1.214 (21.44%) | 0.977 (2.35%) |
| M1 swaptions-6 | 1.169 (16.90%) | 0.991 (0.95%) |
| M1 streamcluster-1 | 1.132 (13.20%) | 1.043 (4.28%) |
| M1 vips-1 | 0.704 (42.06%) | 0.704 (42.06%) |
| **M1 fdtd2d-6** | 0.686 (45.79%) | **0.670 (49.35%)** |
| **M1 ludcmp-6** | 0.801 (24.82%) | **0.743 (34.56%)** |

What this shows:

- **In the model, M1's over-predicted FP kernels are bound by the window.**
  That is a property of the model shown by these runs; that the M1's own
  window is what the model lacks there is a hypothesis.
  ferret, bodytrack, raytrace, blackscholes and swaptions are 13 to 36% slow
  on M1 today. At 512 the four inputs of them that were run at measured
  size (ferret-1, bodytrack-1, raytrace-3, swaptions-6) end 1 to 9% below
  their references: all four change side. streamcluster-1, 13.2% slow
  today, ends 4.3% above its reference.
- **At 512 two measured M1 inputs that are already too fast get faster.**
  fdtd2d-6, M1's largest residual, loses 2.4% of its cycles and goes from
  45.79% to 49.35%, 0.65 points from the 50% limit. ludcmp-6 loses 7.2% and
  goes from 0.801 to 0.743 of its reference, newly below 0.8; the reduced
  ludcmp input above had lost 4%. vips-1 does not move. Of the first inputs
  run locally, gramschmidt-1 and durbin-1, already below their references,
  go about 1% lower; doitgen-1, 2mm-1 and five others move by 1.5% or
  less. Over these eight remote inputs alone five errors fall, two rise and
  one stays; they are selected inputs and say nothing about M1's 240.
- **The same change breaks the limit on M2 as its profile stands.** The M2
  model has no dependence rule; with 630 its consecutive doitgen chains
  overlap further and doitgen-1 goes from 13% to 64% error. A register
  multiply chain (the integer control) does not move, as it must not.
- **M2's doitgen references and chain length (reading, then hypothesis).**
  doitgen accumulates in memory over a chain of 51 to 74 trips and then
  starts the next chain. By the references M1 spends 11.07 to 11.14 cycles
  per inner trip at all six chain lengths, which the M1 model's dependence
  rule reproduces (11.05 to 11.07). M2 spends 7.18 cycles per trip at 51
  trips rising to 8.44 at 74 (at the profile's 3.49 GHz). The M2 model takes
  6.35 to 7.21. One description that fits M2's six values within 0.04 is
  11.4 cycles per trip with consecutive chains overlapping by a fixed 19
  trips; a queue of about 38 waiting FP operations is one structure that
  would give that. Both are hypotheses: six points of one kernel over a
  narrow range of lengths, at an assumed clock, fit other descriptions too,
  and nothing here identifies a structure.
- **The value is not established.** 370, 512 and 630 bracket the M1 kernels
  (swaptions-1 is slow at 370 and fast at 630), which is not evidence for a
  value. The committed M2 capacity probes do not give one either. Between two
  64-multiply gates the M2 takes 303 to 367 cycles per iteration with 64 to
  366 FP destinations (one row 392) and 306 to 340 with 256 to 640 NOPs, and
  426 to 496 from 368 FP destinations on (at 3.49 GHz). The certified model
  takes 194 to 209 up to 192 fillers and 385 to 433 from 256: it overlaps the
  gates fully where the M2 does not, and serializes them where the M2 does
  not. With a window of 512 the model reads 214.0 at 352 FP destinations and
  388.0 at 384 (run, through the suite's own validator), where the M2
  measured 344.9 and 453.3 and the certified model gives 397.0 and 401.0. So
  the FP-destination limit then puts the step where the M2 has it, but the
  level below the step goes from 15% high to 38% low. The M2's plateau lies
  between the model's two states at every filler count, so a window alone
  does not describe this probe. At this stage, a finite integer issue queue
  was one untested explanation for what held the second gate back, and M1 had
  no committed capacity measurement. Section 5 reports the subsequent counted
  probes on both cores and the test of a bound on waiting integer multiplies.

## 3. The clock of the M2 application references (reading; its cause a hypothesis)

The integer control is a register multiply-add chain: three cycles per
iteration in the model, whatever the caches, predictor or window, and M1
counted the same. A line through its six retained references gives 3.2009
GHz and 0.035 ms of fixed time on M1, whose profile clock is 3.2035 GHz, and
**3.2001 GHz and 0.61 ms on M2, whose profile clock is 3.49 GHz** (M2
residuals up to 0.19 ms). The M2 throughput probes fit 3.49 GHz (table
above); the application references were captured separately, as short child
processes that include start-up.

That is arithmetic on six references of one control. The frequency of the
M2 core during the application captures was not observed, and the line
could also come from time that is not clock rate at all. If the references
did run at that rate, converting the retained M2 cycles at 3.2001 GHz would
take the M2 median from 14.41% to 9.18% and put 15 inputs past 50%
(jacobi1d and gemm at 1.55 to 1.63), and part of M2's general
under-prediction would be the conversion clock and not the model. Nothing
is changed for it here.

The counted captures of section 5 give a second reading, and it does not
agree on the clock. doitgen-1 and doitgen-6 count 24.51 and 138.27 million
cycles on the M2 under Linux; their references are 7.822 and 41.056 ms. As
time = cycles / clock + fixed, the pair gives **3.42 GHz and 0.66 ms**. Both
readings have a fixed time of about 0.6 ms per process, which the simulated
program does not contain and which is 8% of the shortest M2 reference and 3%
of the median one; they differ on the clock (3.20 against 3.42 GHz), each
from one kernel. The committed M2 capacity timings, taken separately, are
1.10 to 1.32 times the counted cycles at 3.49 GHz, with spreads up to 33%.
The clock of the M2 references is not established.

## 4. fdtd2d and vips on M1 (not identified)

In the M1 model vips runs at the fetch width (0.125 cycles per instruction)
and fdtd2d close to it (0.134; a window of 512 takes another 2.4% off
fdtd2d-6, section 2). M1 runs them at 0.175 and 0.191. Five candidate causes were
considered. The available evidence speaks against each, but excludes none:
it is indirect, two items rest on M2 probes that say nothing certain about
M1, and no M1 probe isolates either kernel.

- FP divide throughput in the input loops. Against: an M2 probe starts a
  divide every 1.13 cycles with sixteen lanes. M1 was not measured.
- First touch of large arrays. Against: atax, gesummv and mvt touch more and
  are within 13% on M1.
- Arrays at equal page offsets. Against: jacobi1d has them and M1 runs its
  ten-instruction loop in 1.27 cycles per trip.
- Updating in place. Against: gemm does and is within 3% on M1.
- The cost of lines from L2. For: the committed M2 stream probe reads at
  3.74 to 3.88 cycles per 64-byte line from 256 KiB to 64 MiB against 2.84 in
  L1, where the model takes 3.00 up to 16 MiB (2.67 with M1's fetch rule),
  and the M1 note records the same shortfall on M1. Against it being the
  whole cause: fdtd2d needs seven new lines per 272 instructions and vips
  one per 736 by their sources, and M1's memory control is no slower at
  256 KiB and 1 MiB (0.152 cycles per instruction by its references) than
  at 8 and 64 KiB (0.163). A one-stream probe does not show what several
  streams with stores cost.

What costs these two kernels a third of their time on M1 is not known.

## Not addressed

- **Grace.** No run. A hypothesis for the next round: if the model's kernel
  time is right, the input loops of atax, gesummv and bicg lack 2.9, 2.1 and
  2.6 cycles per integer divide on the GH200. The model ends a divide early
  by the size of the quotient; both that and ending by the size of the
  dividend reproduce the published ranges. Not tested.
- **M2 streamcluster and nussinov.** By the counterfactual of section 1,
  which changes four things at once, the M1 model's cycles give
  streamcluster 1.13 to 1.21 against M2's references; M2 has no store
  probes of its own. The memory-carried Nussinov loop counts 1.392 cycles
  per trip on M2 and 2.018 in the model. The M1 model, with the fetch rule
  among its differences, takes the M2 model's cycles on the nussinov
  inputs, so that rule is not expected to close it by itself.

## 5. Counted cycles on both cores (counted)

Four diagnostic captures were made for this work by the coordinator, once
each: the committed capacity probes and doitgen at six chain lengths, on one
performance core of an M1 and of an M2 under Linux, in user-mode counted
cycles and instructions (2,290 process records, none multiplexed, every
sample kept).
They are diagnostics: they replace no reference and enter no accuracy
figure. One capacity row (372 destinations, long gate) cannot run on
16 KiB pages; one M2 row (366, short gate) is noisy and is kept.

Capacity probe, cycles per iteration, 64-multiply gates (the 96-multiply
rows read the same way, 192 cycles higher). The certified model gives both
targets the same values within half a cycle:

| Fillers between the gates | M1 counted | M2 counted | Certified model |
|---|---:|---:|---:|
| 64 FP destinations | 257.0 | 270.3 | 194.0 |
| 128 | 257.0 | 270.3 | 200.5 |
| 256 | 269.7 | 283.5 | 385.0 |
| 352 | 282.8 | 296.7 | 397.0 |
| 360 | 284.2 | 298.7 | 398.0 |
| 368 | 285.4 | 370.3 | 399.0 |
| 384 | 287.8 | 402.0 | 401.0 |
| 416 | 402.3 | 409.5 | 405.0 |
| 640 NOPs | 279.7 | 293.8 | 433.0 |
| 448 FP destinations, no gates | 112.0 | 112.0 | 112.0 |

- **What the model has right.** The serial level, the gate latency and the
  four FP pipes: above the rise and without gates the model is within 2% of
  both cores, in counted cycles.
- **The rise.** M1 is on the low branch at 384 FP destinations and on the
  high one at 416, at both gate lengths; no row lies between. M2 rises
  from 362 to 384, unevenly (spread up to 32% at 366). NOPs do not produce
  it at 640 on either core.
- **Neither core stops at 256 instructions.** At 384 destinations the M1's
  second gate starts when 32 multiplies of the first have not run: at least
  417 instructions are in flight. The same reading gives about 390 on M2 at
  360. The model's 256 puts every row from 256 fillers to the rise on the
  serial level, 24 to 43% above M1 and 20 to 36% above M2.
- **Below the rise the gates overlap only in part.** Both cores sit at six
  cycles per multiply of one gate less a constant, 127.0 cycles on M1 and
  113.7 on M2, at both gate lengths: the second gate starts when 42.7 (M1)
  or 38.2 (M2) multiplies of the first are still waiting. The certified baseline overlaps
  the gates completely there and is 14 to 37% fast up to 192 fillers. From
  about 160 FP destinations, or 300 NOPs, the level rises by about 0.14
  cycles per destination and 0.07 per NOP on both cores.

doitgen, cycles per inner trip (a long input less a short one of the same
chain length):

| Trips per chain | 8 | 16 | 32 | 51 | 74 | 120 |
|---|---:|---:|---:|---:|---:|---:|
| M1 counted | 11.30 | 11.14 | 11.07 | 11.04 | 11.04 | 11.02 |
| M2 counted | 2.67 | 3.56 | 5.27 | 6.44 | 8.14 | 9.62 |

- **M1's chains do not overlap** at any length, as its store rule has it.
  The certified M1 model is at 1.002 and 1.001 of the counted cycles of
  doitgen-1 and doitgen-6 (the 51- and 74-trip inputs).
- **M2's chains overlap by a bounded amount of waiting work.** From 51 trips
  a chain costs 11.97 cycles per trip less 282 cycles: 23.6 trips of
  overlap. At 16 and 32 trips a new chain starts every 57 and 169 cycles;
  304 and 323 instructions then lie between the oldest unfinished trip and
  the new chain, while 19.5 and 21.8 trips have yet to run. So the M2 holds
  more than 256 instructions here too, and what bounds the overlap is the
  work that waits, not the distance (reading of the counted times, with
  11.97 cycles per trip and in-order retirement). The model has only the
  distance. In counted cycles the certified M2 model is at 0.985 of
  doitgen-1 and 0.885 of doitgen-6.
- **Which work waits is not identified.** A trip has a load that waits for
  the previous store, an add and that store's data; one loop cannot tell
  them apart, and M1 shows no such bound because its chains never overlap.

What follows for the model:

- A reorder window is the wrong kind of bound for both shapes at once, on
  both cores. 256 stands in for limits on waiting work that the model does
  not have, which is why a larger window alone corrects M1's over-predicted
  FP kernels and breaks M2 doitgen (section 2).
- **A candidate was tested and rejected (remote run).** For M1: a window of
  416 and 384 FP destinations, the two lower bounds above, with a bound of
  43 on multiplies that wait between dispatch and issue; for M2 that bound
  alone, at 38. The values came from the capacity rows and from no
  application, and the rule for rejecting them was fixed before the twelve
  runs. The other parameters in the M1 combination changed alongside the
  multiply bound, so its effect required a separate test.

  | Run | Certified model | Candidate | Counted or rule |
  |---|---:|---:|---:|
  | M1 capacity, 64 FP destinations | 194.0 | 261.0 | 257.0 |
  | M1 capacity, 384 | 401.0 | 301.0 | 287.8 |
  | M1 capacity, 416 | 405.0 | 389.0 | 402.3 |
  | M2 capacity, 64 (bound only) | 194.0 | 276.0 | 270.3 |
  | M1 doitgen-1, cycles | 42,086,698 | 42,048,233 | within 0.5%: kept |
  | **M1 fdtd2d-1, cycles** | 40,652,073 | **39,461,037** | **at least 39,676,000: failed** |
  | M1 ludcmp-1, cycles | 44,838,580 | 42,678,971 | reported |
  | M2 doitgen-1, cycles (bound only) | 24,142,751 | 24,142,751 | equal: kept |

  The model follows the counted rows on both sides of the rise, within 1.6
  to 4.6%, so the values reproduce these rows. But the window part
  again takes time off the two M1 kernels that are already too fast:
  fdtd2d-1 loses 2.9% (error 42.17% to 46.46%) and ludcmp-1 4.8% (7.65% to
  13.09%). By the rule the combination is rejected; 416 and 384 are not
  adjusted to pass. The M1 window stays 256 until what slows fdtd2d and
  ludcmp on the M1 is in the model.
- **The multiply bound alone (remote run).** Four M1 runs held the window
  at 256 and FP registers at 360 and set only the queue bound to 43. At 1,024
  and 2,048 iterations, the capacity probe took exactly the same 268,664 and
  535,928 cycles as the combined candidate, or 261 cycles per iteration
  against 257.01 counted. M1 fdtd2d-1 and doitgen-1 took 40,652,073 and
  42,086,698 cycles, exactly their certified baseline counts. Their existing
  underpredictions therefore kept their depth. On M2 the preceding experiment
  already used the queue bound of 38 alone, with the same source and binary.
  Its capacity result (276 versus 270.28 counted) and baseline-matching
  doitgen-1 result (24,142,751 cycles) are reused here. These exploratory
  results motivated the target values used in the certified comparison below.
- Nothing is proposed for M2's window: a value for the waiting-work bound
  that M2 doitgen shows would rest on one loop.
- fdtd2d and vips (section 4) stream through arrays far larger than L1;
  these captures are L1-resident and say nothing about them.

## 6. Certified three-device comparison

The full run measured the target builders at
`5c8ed4a683a2e3ba2808e904b63dd0608e0b1e83`.
All 1,656 points passed validation across 2,520 raw execution records
(certificate SHA-256
`fe49ea6d3de81e3f1c8456fa368dac1b69e4b365cc4de97d62a181d771704796`).
The later revision `1f29f28` changed a profile test and a Go comment.

All 792 whole-program cycle counts matched the certified baseline. Across the
723 qualified applications, every error stayed the same. There were no new
or deeper underpredictions and no model/reference ratio crossed below 0.8.

| Device | Qualified applications | Mean error | Median error | Maximum error |
|---|---:|---:|---:|---:|
| Grace | 237 | 10.606877% | 7.873813% | 46.707675% |
| M1 | 240 | 9.183616% | 5.651829% | 45.790342% |
| M2 | 246 | 15.816498% | 14.413331% | 49.791762% |

Six of the 73 capacity microbenchmarks improved on each Apple core. They use
64- or 96-multiply gates with 64, 128 or 192 FP fillers between gates. All
other microbenchmark cycle counts matched the baseline, including every Grace
point. Across all 73 capacity cases per Apple core, the error changed as follows.

| Device | Baseline mean | Queue-bound mean | Baseline median | Queue-bound median |
|---|---:|---:|---:|---:|
| M1 | 25.22062% | 22.66425% | 24.14642% | 23.78560% |
| M2 | 16.56748% | 12.47878% | 12.75354% | 12.29689% |

## Limits

- The queue counts are effective parameters inferred from probe timing.
  The probe does not establish a physical queue size. The window settings
  of 370, 512 and 630 bracket a sensitivity, and the larger M1 window and
  FP register counts in section 5 came from probe lower bounds; what
  waits in M2's chains and the reference clock are hypotheses. A transition
  of a probe is not taken for a structure.
- Both larger M1 windows at
  which they were run take time off fdtd2d and ludcmp (416: the first
  inputs; 512: the sixth, where it also corrects the over-predicted FP
  kernels and brings fdtd2d-6 to 0.65 points from the 50% limit); 630
  breaks the limit on M2 doitgen. The fetch rule was not run.
- The counted captures were made once, on one core of one machine of each
  kind. Twelve remote runs tested the combined candidate, followed by four M1
  queue-only runs. Section 6 reports the subsequent full run with the target
  builders.
- The capacity probe still reveals a gap. The model charges every filler from the first,
  where both cores take about 160 at no cost, and counts a NOP as a window
  entry; the rows from 256 fillers to the rise stay serial at a window of
  256, 20 to 43% slow, with or without the bound.
- Not solved: what slows fdtd2d, vips and ludcmp on the M1 (so that a window
  at the counted lower bound is too fast for them); what bounds the overlap
  of M2's doitgen
  chains; the clock of the M2 references; M2's fetch rule against its
  already-fast kernels; Grace.
- 40 local runs, one each; fourteen are measured inputs with a window other
  than 256, all on first inputs; the reduced inputs have no reference.
- Eight remote runs, one each, at one bracket size, on inputs chosen for
  their residuals. They are not a comparison: no statement about the
  accuracy of M1's 240 inputs, or of any device, follows from them.
- The M1-cycles counterfactual changes four rules and the ELF layout at
  once; it is not a simulation of any M2 configuration and isolates none of
  the four.
- M2 probes are evidence about M2. Where they are cited for an M1 kernel
  they are an indication only.
- Grace has no accessible machine and no native diagnostic references.
- References, workload binaries, input lists, quality classes and the
  certified comparison are not changed by this work.
