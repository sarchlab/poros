# Whole-program user-time accuracy

Poros compares predicted whole-program user time with direct-child
`wait4.ru_utime` from the **identical AArch64 ELF and arguments** running natively
in an ARM64 Linux VM on an Apple M2. QEMU is a correctness oracle only. Guest
initialization, computation, and output all belong to the measured program.
Simulator-host elapsed time, system CPU time, and native macOS timing are not
substituted for Linux child user time. VM core identity and instantaneous
frequency remain unknown; hardware timing accuracy is **unverified**.

The error is `100 * abs(simulated - reference) / min(simulated, reference)`.
No offset fitting, batching, or anchor removal is used. The CPU timing profile assumes 3.49 GHz.

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

Both inventories execute the same rebuilt ELF files. There are no alternate
algorithm implementations or outer repetition loops. PARSEC names still mean
simplified compute kernels, not full applications. Tiled Facesim extends its
valid mesh, x264 extends the padded search frame, and bodytrack bounds particle
states to preserve meaningful likelihoods. All old small-case native outputs
were checked byte-for-byte against the previous ELFs.

The application summary covers 246 points across 41 programs. Each qualified
point contributes once to the mean. Integer, memory and branch supply 18 points
for the separate microbenchmark section. Retained manifests record historical
split labels for reproducibility. All workloads may guide model tuning.

## Native evidence and quality

Each formal case executes one warmup and 21 measured processes. A single complete
264-point final campaign is used regardless of its qualification count. A case
is excluded from error statistics, but remains visible, when any measured user
time is nonpositive, the median is below 100 microseconds, or
`(P90-P10)/median > 0.20` (the unchanged order-statistic definition). No sample is
selected, retried, replaced by total CPU time, or selected by simulator error.

The recorded final campaign has **247 qualified points and 17 noisy points**
across all 44 workloads, with no nonpositive references. These are measurement
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
contributes to the native/QEMU SHA256 comparison, with exact length, normal exit,
shape/finiteness checks and the existing microprogram analytic checks. A hash
comparison replaces storage of the native byte sequence, not execution or output
coverage. The report does not claim raw observations are publicly downloadable.

The performance oracle runs identical ELFs natively and in QEMU, comparing every
output byte. A clean, separately built Go checker supplies independent equations
for QR, Black-Scholes, bodytrack, canneal, raytrace, Facesim, and x264. Other
programs receive complete shape/finiteness and native/QEMU checks; they are not
claimed to have a separate mathematical oracle. Every Facesim force and positive
deformation determinant is checked; energy gradients cover the original fixture,
all seven deformation classes, and the partial final block. QEMU and semantic
checks run outside the native timing phase.

Capture requires clean committed support code, exact source/ELF identities, and
an exact ARM64 Docker image digest. Heavy simulation, compilation, and tests are
stopped during native acquisition. Capture output is written outside the clean
checkout and validated there. Only the derived compact reference is committed.
The single recorded collection commit is the actual clean commit measured;
publication verifies collector and input hashes against that exact Git object.

## Parallel CI and base compatibility

Evaluation builds head/base once and freezes the full 264-case plan. It then runs
132 deterministic shards, two cases per shard, with at most 16 concurrent jobs.
Each case requires head functional and head OoO execution. Base OoO is required
whenever the exact base can load that ELF and retain its complete output.

Per-process bounds are 2 billion instructions and 45 minutes. Output is bounded by
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
collection commit in the target branch's history. **Merge PR #57 with a merge
commit (`gh pr merge 57 --merge`), not squash or rebase.** Deleting its feature
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
