# FP destination capacity and divider controls

This suite adds native M2 evidence without changing any prior probe or application
reference. Its 73 cases are separate from application accuracy statistics.
The same `loops.S` bodies are assembled for native Mach-O and freestanding ELF.
All bodies use caller-saved registers, explicit instruction dependencies and
observable checksums; compiler optimization cannot delete the assembly fillers.

## Capacity discrimination

Each window loop contains two independent integer MUL-by-one chains (64 or 96
instructions each), separated by N independent `FADD d0,d30,d31` destinations.
Repeated writes to d0 allocate new speculative destinations without introducing
a dependency chain. An integer join feeds both gate seeds into the next iteration,
so later iterations cannot bypass the outstanding gates. FP results and gates
feed the checksum. NOP controls preserve the instruction/ROB footprint without
FP destinations; ungated FADD controls expose the port-limited slope. Varying
integer gate length changes dependency and ROB pressure without consuming FP
renames. These controls are necessary: a single timing kink does not uniquely
identify a physical register file.

The normal reference records nine alternating long-minus-short thread CPU-time
pairs for every case. The additional dense boundary experiment rotates/reverses
case order across nine trials and measures target/control in ABBA or BAAB order.
Each A/B measurement is itself an alternating short/long difference. Controls
use NOP 352 with the same gate length: they normalize scheduling/frequency drift
while keeping the long gate constant. The broader NOP sweep and ungated FP sweep
independently check instruction-footprint and throughput effects.

An initial exploratory acquisition informed the dense confirmation sweep. The
quality and rise criteria below were fixed before that second acquisition; the
whole sequential study was not preregistered. The boundary quality rule is `(p90-p10)/median <= 20%` for paired
ratios. The rise criterion is at least 15% above the low-count plateau with
nonoverlapping p10/p90 bands, applied independently at each gate length. Both
gates’ brackets are reported; in this acquisition both meet the criterion by 370.
Post-acquisition analysis in
`scripts/capacity_microbenchmark.py::infer_transition` specifies the plateau as
qualified counts ≤356 for each gate, summarized by the median of their medians
and maximum P90. A later qualified row must exceed 1.15 times that median and
have P10 above the plateau maximum P90. The bracket runs from the last qualified
non-rise to the first qualified rise: (366,370] for gate64 and (366,368] for gate96.
This deterministic interpretation is tested against the frozen data; it is not
represented as analysis code fixed before collection. Unqualified rows neither
establish nor narrow a bracket, and missing evidence yields no bracket.
Nine-sample bands use sorted
indices 0/4/7 for p10/median/p90; all samples are retained, including unqualified
rows. Bands describe dispersion, not confidence intervals. The local effective
transition is approximately 366–370 FP destinations. 360 is the rounded conservative
model lower bound, not an exact physical PRF count or an application-error fit.
The existing ROB 256 is unchanged and separately limits the default simulator.

The methodology is informed by [Jia's direct M2 experiments and source](https://jia.je/hardware/2026/07/06/apple-m2/)
and [Dougall Johnson's discussion of gate/queue confounds](https://dougallj.wordpress.com/2021/04/08/apple-m1-load-and-store-queue-measurements/).
The external FP32 result is not substituted for local FP64 measurement.

## Arithmetic discrimination

Each arithmetic loop executes 64 divide or square-root operations, arranged over
1, 4 or 16 active lanes. Trivial 1/1 and sqrt(1) cases are explicit operand controls.
Normal FDIV repeatedly computes 2/x starting at 1.5, with an even number of updates
per lane. Normal FSQRT computes sqrt(x+1), converging toward the golden ratio;
it includes 64 FADD as well as 64 FSQRT per iteration. Every active lane contributes
to the final checksum. The port and FADD costs must be accounted for when deriving
a square-root latency from the composite recurrence.

Native units are ns/iteration. QoS requests interactive scheduling, but does not
prove a pinned performance core. Core identity and instantaneous frequency are
unknown. The model assumes 3.49 GHz; conversions using it are not measured cycles.

## Reproduction and evidence

```sh
sh evaluation/microbenchmarks/capacity/build.sh
clang -O2 -arch arm64 evaluation/microbenchmarks/capacity/native.c \
  evaluation/microbenchmarks/capacity/loops.S -o /tmp/capacity-native
python3 scripts/capacity_microbenchmark.py boundary --binary /tmp/capacity-native \
  --output /tmp/capacity-evidence/boundary.json --evidence-dir /tmp/capacity-evidence/raw
python3 scripts/capacity_microbenchmark.py collect --evidence-dir /tmp/capacity-evidence
python3 scripts/capacity_microbenchmark.py smoke --poros /tmp/poros
```

Collection requires an Apple M2 and records source/compiler/native-binary hashes,
UTC timestamp, every case's instruction count and native timing bands. Boundary
sources/binary are hashed before and after sampling. All raw timings, native
binaries, and disassembly audits are retained externally; compact paired samples
and provenance are embedded in `m2.json`. Collector hashes identify the acquisition
version; later QEMU smoke support, post-acquisition `infer_transition`, and report
integration do not change the measured sources or samples. The committed ELF
still matches the acquisition SHA256 `38ac11ae36074d79de96283dd02e05db5bae2bb6b5930f1b7539ab9c38dfe44b`.
Native and ELF loop-object disassemblies
were compared word-for-word (34,035 identical instruction words). Object builds
can be reproduced with `clang -arch arm64 -c loops.S` and
`aarch64-linux-musl-gcc -c loops.S`, then disassembled with LLVM objdump.

CI checks analytic checksums and exact instruction-count differences in functional
and OoO ELF execution. Exact-head accuracy aggregation runs both base and head on
the same new ELF and includes a separate canonical report panel and collapsed
table. It does not add points to application S-curves, CDFs or correlations, and
it does not change the main-merge-only accuracy trigger policy.

## Interpretation of this acquisition

The normal FDIV one-lane case measures 3.2238 ns/operation versus 0.3249 ns/operation
with 16 lanes, a ratio of 9.92. Relative dependency/throughput evidence supports
an approximately 10-to-1 latency/issue interval without converting native time
into hardware cycles. The trivial FSQRT ratio, multiplied by its two-cycle issue
interval, is approximately 12.89; the normal sqrt(x+1) recurrence gives 16.22,
consistent with 13 plus 3 cycles for its additional FADD. These ratios assume
comparable frequency across the corresponding samples. Absolute native times
are about 11–12% above predictions at the model's assumed 3.49 GHz; that discrepancy
is disclosed rather than hidden by a frequency or arbitrary latency adjustment.
The normal FSQRT four-lane reference has 21.61% relative P10–P90 spread and is noisy;
it is retained, displayed and not used as a precise calibration constraint.
