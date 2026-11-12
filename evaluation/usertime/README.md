# Whole-program user-time accuracy

Poros compares simulated AArch64 ELF whole-program user time with native macOS
**Mach-O** measurements on Apple M2. Both use the same GCC computation assembly
and linked musl math, checked by the original 175-function audit plus the
five-function Streamcluster repetition audit and exact complete-output
comparisons for all 264 input points. macOS reserves x18, so
14 workloads have documented register renaming; startup, argument/output
adapters, addresses and page placement differ. The entire executable instruction
stream is therefore not claimed to be identical.

The reference measures direct-child `wait4.ru_utime`, including native startup,
initialization, computation and output. Simulator-host elapsed time and system
CPU time are not substituted. Core identity and instantaneous frequency remain
unknown; hardware timing accuracy is **unverified**. Linux VM measurements are
historical evidence only and cannot feed new active accuracy comparisons.

The error is `100 * abs(simulated - reference) / min(simulated, reference)`.
No offset fitting or anchor removal is used. The CPU timing profile assumes 3.49 GHz.

## Independent correctness and performance inventories

- `tests/usertime/manifest.json` and `tests/benchmarks/manifest.json` retain every
  existing small correctness case and its complete output shape. Bounds and
  source/ELF hashes reflect the enlarged wrappers.
- `tests/usertime/performance.json` declares six larger timing points for integer,
  memory, branch, and MVT. Memory uses 32 million accesses at six working-set sizes.
- `tests/benchmarks/performance.json` declares six actual dimension tuples and
  complete output shapes for each of the other 40 programs.

Manifest `reference_file` fields name local raw-capture or historical protocol files;
the active committed plotting reference is `evaluation/usertime/performance-v1.json`.

Both inventories execute the same rebuilt ELF files except Streamcluster's
performance-only `streamcluster-repeat.elf`. Its six points use n = 160000,
200000, 250000, 315000, 397000, 500000, dim = 32, and 16 whole-program
passes per process. Every pass initializes, computes, and writes the complete
output. The original computation assembly and the small correctness ELF are
unchanged; an audited wrapper repeats the original guest entry. The two
executables therefore represent different workload sizes, not a timing offset. PARSEC names still mean
simplified compute kernels, not full applications. Tiled Facesim extends its
valid mesh, x264 extends the padded search frame, and bodytrack bounds particle
states to preserve meaningful likelihoods. All old small-case native outputs
were checked byte-for-byte against the previous ELFs.

The application summary covers 246 points across 41 programs. Each qualified
point contributes once to the mean. Integer, memory and branch supply 18 points
for the separate microbenchmark section. Retained manifests record historical
split labels for reproducibility. All workloads may guide model tuning.

## Native evidence and quality

Each formal case executes one warmup and 21 measured processes. The original 264-point campaign is retained for 258 cases. Six Streamcluster
points were replaced by one separately captured, predefined six-size campaign;
all 21 trials of each selected case are retained. A case
is excluded from error statistics, but remains visible, when any measured user
time is nonpositive, the median is below 100 microseconds, or
`(P90-P10)/median > 0.20` (the unchanged order-statistic definition). No sample is
selected, retried, replaced by total CPU time, or selected by simulator error.

The active reference has **264 qualified points**, across all 44 workloads,
with no nonpositive references. Application coverage is 246/246. The six
Streamcluster replacement points all pass the unchanged quality rule. These are measurement
quality results, not a claim of improved simulator accuracy.

Only compact plotting data is committed: `performance-v1.json` contains each
case's median, P10/P90, extrema, sample/nonpositive counts, quality, and complete
native output SHA256/length, with exact collection/source/ELF identities. The
serializer first validates every local raw sample and full output, then derives
these statistics. Full raw campaigns, diagnostic pilots, and output witnesses
stay local; they are not Git or Pages assets. CI retains execution records as
transient workflow artifacts. Superseded active references are removed, while
previously published historical bundles remain readable.

Complete output validation remains byte-exact: every decoded simulator byte
contributes to the native-output SHA256 comparison, with exact length, normal exit,
shape/finiteness checks and the existing microprogram analytic checks. A hash
comparison replaces storage of the native byte sequence, not execution or output
coverage. The report does not claim raw observations are publicly downloadable.

The original performance oracle ran identical ELFs natively and in QEMU,
comparing every output byte. The six repeated Streamcluster replacements use
verified native output blocks and full functional/timing simulator checks, plus
the five-function ELF/Mach-O computation/wrapper audit; they were not rerun in
QEMU. A clean, separately built Go checker supplies independent equations
for QR, Black-Scholes, bodytrack, canneal, raytrace, Facesim, and x264. Other
programs receive complete shape/finiteness and native/QEMU checks; they are not
claimed to have a separate mathematical oracle. Every Facesim force and positive
deformation determinant is checked; energy gradients cover the original fixture,
all seven deformation classes, and the partial final block. QEMU and semantic
checks run outside the native timing phase.

The original frozen ELF/input source is commit `65a8fb82832720fc5f9f72303828da82c515a126`.
Native capture used local experiment scripts, not a committed collector at that
revision. The compact `performance-macho-summary-v1` protocol records this
explicitly: collector, capture, build, smoke and instruction-audit hashes identify
retained local evidence; publication verifies the ELF/input hashes against the
real historical collection commit and the current execution inventory. Native binary and assembly identities
and register adaptations are recorded per workload. No container identity is
invented for this native macOS campaign.

The completed capture has 21 retained processes per point in three chronological
batches of seven randomized full-suite passes. Batch medians and spread remain
visible in the downloadable reference. The existing noise rule is unchanged.
Hardware captures are reused; CI runs the simulator on Linux and does not need a
Mac or recollect hardware measurements.

To import the original locally retained capture against its original frozen
input checkout, without measuring again:

```sh
python3 scripts/macho_reference.py --root /path/to/original-frozen-input-checkout \
  --evidence /path/to/capture-directory \
  --output /path/to/new-compact-reference.json
```

The evidence directory contains `capture.json`, `evidence/{build,instruction-audit,smoke}.json`,
the original collector scripts, `assembly/`, and measured `bin/` files. Import
checks 5,544 trial records, 264 warmups, output identities, trial/batch membership,
timestamps, frozen input hashes, measured executable/assembly hashes and 175
unique successful audit records with matching instruction counts. It validates
stored disassembly-audit evidence; it does not rerun the external disassembler.
Original audit scripts and their external tools remain part of the retained
local experiment. Import never overwrites an existing output. Commit only the
compact plotting reference, not captures, native Mach-O executables or witnesses.

The additive `streamcluster_replacement` record identifies the six newer rows,
their raw capture and five-function instruction audit, the measured ELF/Mach-O
hashes, and frozen assembly/wrapper sources. Its base commit is `d527552`, but
the wrapper experiment was uncommitted at acquisition; new files are not claimed
to exist at that historical commit. `historical_input_sources` preserves the
original capture bindings while `input_sources` binds the current execution
inventory. All other 258 compact rows are unchanged. No superseded timing rows
or raw samples are added to Git.

To import that retained six-point capture without executing a program:

```sh
python3 scripts/streamcluster_reference.py --evidence /path/to/scaling-experiment \
  --output-blocks /path/to/verified-single-pass-blocks \
  --output /path/to/new-compact-reference.json
```

The importer checks the full focused capture's trial/warmup census, timestamps,
outputs, executable identities, frozen sources and stored five-function audit.
Verified output blocks reconstruct the complete 16-pass output hashes. It does
not use the experiment's stale expanded-C build inventory. The explicit frozen
assembly recipe in `tests/benchmarks/build.sh` reproduces the measured ELF.

## Parallel CI and base compatibility

Evaluation builds head/base once and freezes the full 264-case plan. It then runs
132 deterministic shards, two cases per shard, with at most 16 concurrent jobs.
Each case runs head OoO and checks its complete output against the native reference.
Base OoO also runs when the exact base supports the image and output size.

`functional-witness.json` stores the independent guest instruction counts from
264 clean functional executions in the v0.1.2 laboratory run. The collection
commit, simulator binary SHA-256 and run ID identify those executions. Each
entry binds the guest ELF digest, argv, environment and expected output length
and SHA-256. On a match, both timing runs must reproduce the count and the
report identifies the original functional run. A different functional engine
or unmatched input requires a fresh head functional execution. Updating timing
models alone permits reuse. To refresh the counts, use validated functional
records from a completed campaign with the corresponding build provenance.

To measure one input on the hosted Ubuntu runner, dispatch `Evaluation` with
`accuracy_case=streamcluster-6` (or another manifest ID). The job uses the
normal input, timing model and per-execution timeout and logs elapsed host time.

Per-process bounds are 2 billion instructions and 45 minutes, except the
16-pass Streamcluster ELF's bounded 6-billion-instruction and 90-minute
allowance. The subprocess guard adds 50 seconds to the same execution timeout. Output is bounded by
the case's declared complete output size, with a minimum 1 MiB default. Shard jobs
have a 300-minute ceiling covering two cases and their required executions. These
are finite safety budgets, not performance targets. Actual shard durations are
retained in `shard-summary.json`.

The loader image ceiling is 256 MiB, while the 16 MiB ELF-file and 128-program-header
bounds remain. The requested output ceiling is 64 MiB; its default remains 1 MiB.
The current performance protocol requires a compute-ELF-capable base; ancient
pre-compute comparisons are rejected. Historical archived reports remain valid.
Older capable bases have 64 MiB image/16MiB output limits. Capability is extracted only from
recognized declarations in the exact base Git objects, with full source/digest
proof retained. Unknown capabilities fail closed. Page-rounded `PT_LOAD` bytes
are recomputed from the exact ELF. Oversized cases record
`image_limit_unavailable` or `output_limit_unavailable`; build or execution errors
never turn into an exemption. The base executable is not patched.

Reports state separate head/base comparable counts. Error deltas exist only for
paired cases; means with different coverage are not presented as paired results.
Publication re-verifies the retained capability against the exact base commit.

Aggregation requires every expected case/role exactly once, successful upstream
jobs, complete outputs, positive counters, matching hardware profile, and exact
plan/source/base/reference/binary identities. Missing, duplicate, extra, stale,
malformed, or mixed records fail. It never falls back to serial execution or
fabricates success. Failed shard jobs retain partial records and diagnostics;
missing prepare/download artifacts produce an explicit failure report.

A failed-jobs/aggregate-only retry can reuse an earlier prepare/shard artifact
from the same run and identical plan. The newest attempt for each shard wins,
including failures; whole-workflow reruns create a new plan. The required final
leaf job is **User-time accuracy report**; GitHub displays it under the reusable
caller as **Accuracy / User-time accuracy report**. The publisher-facing artifact is
`user-time-accuracy-${run_attempt}`. Shards have read-only repository permissions.

## Publication and historical evidence

PRs introducing a new measurement campaign must preserve the actual clean
collection commit in the target branch's history. Use a history-preserving merge
commit, not squash or rebase. Deleting its feature
branch is then safe: full-history checkouts still contain the measured commit.
Prepare and publication reject a missing or non-ancestor collection commit with
an explicit diagnostic; a tag or separately retained branch is not required.

The existing branch-based Pages publisher remains. It archives recent 20 main
commits and permanently retains release reports. Publication verifies exact Git
source hashes, references, outputs, and base-capability proof. Historic report
schema 4 stores only report JSON/Markdown and compact plotting reference. Historic
report schemas 1/2/3 remain readable; explicit suite/protocol versions choose their exact
old inventories. A missing new performance manifest cannot silently fall back to
small cases. The serial comparison CLI remains for targeted reproduction. A shared reusable
workflow runs modern Evaluation and missing-evidence release/manual backfills
through the same parallel comparator. Releases reuse validated retained bundles
when available; historical small-cohort targets retain their historical serial
protocol. A failed parallel path never falls back to serial.
