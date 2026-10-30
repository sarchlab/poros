# Whole-program user CPU time

The default ELF timing scope is the complete single-threaded user program from
ELF entry through exit, including its own initialization and output preparation.
The loader, host syscall processing, copied output buffers, kernel execution,
page-fault service and off-CPU waiting are excluded. SVC is one serialized user
instruction with the pipeline's ordinary unit latency, not a guessed kernel
penalty. Older instructions retire before servicing it; younger instructions
wait for the boundary to retire. No ROI or extracted kernel is required.

`poros elf --mode ooo program.elf args` (or `--mode inorder`) uses the shared
checked runtime. Default frequency is an explicitly assumed 3.49 GHz; the full
CPU Spec and cache configuration are reported. Timing CLI frequency must be an
integer from 1 MHz through 1 THz; fractional or zero CLI values are rejected, so
reported Hz matches the simulation clock exactly. The Go Options zero value
selects the documented default. Cache state starts cold; user
initialization naturally warms it. There is no OS cache/TLB pollution model,
I-cache timing is not supplied by these existing pipelines, and this is not a
validated Apple hardware profile. Inorder branch metrics are explicitly unavailable. `--mode functional` remains the default.

The two timing models use the same process semantics and checked memory as the
functional path. All 14 required ELF fixtures compare final registers, readable
memory, outputs, exit/error state, and deterministic instruction/cycle counts.
Hand-derived tests check a minimal exit timeline. Deadline/cancellation is checked
at architectural steps; already-issued finite-latency work drains before return.
The CLI uses fixed bounded latencies (maximum DRAM latency 335 cycles); it does
not expose arbitrarily long pipeline stalls. Cancelled and timed-out loops are
tested, including no execution for a pre-cancelled context. Legacy 90-case baselines stay
unchanged. Existing function-level hooks remain a legacy adapter only.

## Native records

`scripts/usertime.py capture` runs the six required complete Linux ARM64 ELFs,
covering integer recurrence, memory read/modify/write, and data-dependent
branches at 2M and 8M iterations (compiler if-conversion is disabled and tested),
using one warm-up process and 21 measured child processes per case in the
retained record. `wait4.ru_utime` is per-child **user CPU time**, not wall time,
system CPU time or simulator host cost. Child stdout/stderr/status are checked
against an independent numerical oracle. Raw samples and system CPU time are
retained, together with ELF/source/build-manifest identity, input/environment,
measurement harness hash and environment description. The acquisition protocol
is in `scripts/native_usertime.py`, independently hashed from report rendering;
changing it requires a new capture. Measurements are serial;
large tests and independent reviews are stopped while sampling. The runner
measures direct children, not arbitrary subprocess trees.

The 100 microsecond quality floor is a conservative **policy**, not a measured
hardware clock resolution. Nonpositive/nonfinite samples are invalid. A P10-P90
spread greater than 20% of the median is marked noisy. These policies detect bad
samples; they do not prove an isolated CPU, stable frequency or bare-metal
validity. Rejected short initial experiments are not retained. The retained
larger cases avoid zero user-time samples; noisy observations remain diagnostic
and do not enter accuracy summaries. Do not subtract kernel time from wall time
to synthesize user time: use the OS user accounting field directly.

`m2-linuxvm-user-v1.json` is a Docker Linux ARM64 **VM** observation on an Apple
M2 host. Core identity and instantaneous frequency are unknown. It is useful for
exercising the same-ELF comparison and seeing errors, but does not validate a
bare-metal M2 core. macOS `measure -- PROGRAM ARGS` exercises the generic native
collector with a Mach-O counterpart; its separately retained witness is not an
ELF reference. macOS startup/runtime and code differ. No Darwin ABI simulation
or binary-equivalence claim is made.

Raw valid measurements are frozen by ID. Reuse them for code-only simulator PRs;
new workload bytes/inputs/protocol/environment require new measurements and
explicit provenance. Do not rewrite references to make a regression disappear.
Keep usable historical evidence; invalid superseded scratch experiments need
not be checked in.

## Required PR report

`scripts/usertime-ci.sh BASE` builds the exact clean head and base and runs the
same required six ELF inputs and hardware profile against one frozen reference.
The complete CPU/cache profile is frozen in the manifest; both revisions must
report an exact match, so changing defaults cannot silently change hardware.
It runs on every PR and main push, including documentation PRs (no automatic
reuse shortcut). Missing cases, wrong binary/source provenance, failed output,
base build/run errors and mismatched profiles fail artifact acceptance. The
single known predecessor before timing support is explicitly feature-unavailable;
this does not fabricate a base score. Later missing base support fails.

Every JSON/Markdown report distinguishes **artifact_status** (complete reproducible
report) from **accuracy_status** (currently unverified). Available usable VM
references can produce diagnostic numeric errors, never hardware-validation pass.
No accuracy cutoff has been approved, so there is no invented percentage gate.
Calibration (integer) and held-out (memory/branch) coverage and aggregates are
separate. Required inventory is independent of the manifest; cases cannot be
silently removed. Reports bind final source hashes and clean build revisions.

Formula `absolute-difference-over-min-v2`:

`100 * abs(predicted - reference) / min(predicted, reference)`

Only finite positive qualified times contribute. A 2x slowdown and 2x speedup
both have 100% error; direction is separately reported. Base/head errors and
percentage-point changes share the same reference, inputs and profile. Former
reference-denominator thresholds do not apply. New M0 pilot runs also label/use
v2; historical M0 raw timing records and old derived fields retain their original
meaning, and are not relabeled as new user-time measurements.

For the first timing PR, all rows remain hardware-unverified and base timing is
unavailable. That is an honest completed report, not a claim of calibrated
accuracy. Future calibration must first establish valid hardware references,
frequency/core policy and independently agreed thresholds.

## Published accuracy dashboard

The dashboard is published at <https://sarchlab.github.io/poros/calibration/>.
GitHub Pages must be configured as **Deploy from a branch → gh-pages → / (root)**.
No personal access token or extra service is required. The publishing job uses
`contents: write` to update that branch and `pages: write` to explicitly request
a Pages build, then waits for a successful build that contains its publication.
A default `GITHUB_TOKEN` push alone does not trigger a branch-based Pages build.

Every PR update produces an HTML preview plus JSON/Markdown and a complete
evidence bundle in the `user-time-accuracy` artifact, retained for 30 days. PR
reports never enter the public archive. Each main push evaluates its latest
commit; intermediate commits in one push are not individually measured.
`Publish accuracy reports` consumes only completed main-push Evaluation runs,
verifying their workflow, repository, commit and attempt. Failed/cancelled runs
are visible, including an explicit missing-report record when no artifact exists.
Only a complete report with a successful Evaluation run can become the latest
successful main report. The homepage separately queries current main's public
GitHub status; if that query is unavailable it labels the displayed snapshot.

The archive retains the **20 newest ordinary main commits**, ordered by main's
first-parent ancestry rather than completion time. Published releases permanently
pin the exact tag commit and do not occupy those 20 slots. Releasing a commit
already in the archive reuses its original successful report bytes, base revision
and measurements. If no report exists, a read-only job builds the exact release
commit; versions predating this protocol receive an explicit unsupported/failure
report. Published prereleases follow the same permanent retention policy. Deleting
a GitHub release does not automatically delete its evidence.

Each report is self-contained: raw native samples, manifest/hardware profile,
measurement harness, workload sources and ELF files accompany the original report
and checksums. This deliberately duplicates a small amount of evidence across
reports so cleanup has no shared-data dependency: removing an expired ordinary
report cannot remove data used by another report or release. All cleanup is
limited to owned `calibration/runs/` directories. Other files on `gh-pages` are
preserved; an absent root index links to the dashboard and `.nojekyll` makes the
site static. Normal Git history is retained; pruning the live site is not a
rewrite of the Pages branch's commit history.

Publishing uses `concurrency.queue: max` to preserve queued events, with bounded
optimistic Git push retries for concurrent manual changes. Non-PR evaluations
have unique concurrency groups; PR updates cancel superseded PR evaluations.
Late reports cannot move the latest pointer backward. A release and main report
for the same SHA share one directory. Successful report evidence is immutable;
later successful Evaluation attempts can establish CI success without replacing
that original evidence. A failed report can be replaced by a successful retry.

The site renders the report's numeric fields directly. It does not recalculate
errors, subtract fitted overheads, invent thresholds, or turn noisy observations
into qualified accuracy points. It includes per-case base/head times and errors,
coverage by calibration/held-out split, size curves, qualified reference/prediction
scatter, and history curves separated by reference, formula and full profile.
The current Linux VM limitations remain prominent on every report.

Validation entry points:

```sh
python3 scripts/accuracy_site_test.py
python3 scripts/accuracy_site.py bundle --report /path/report.json \
  --source . --output /tmp/accuracy-preview --commit "$(git rev-parse HEAD)"
python3 -m http.server 8000 --directory /tmp/accuracy-preview
```

The archive tests exercise retention, permanent pins, failed/successful retries,
out-of-order arrivals, malformed evidence, escaping, zero-error charts, provenance
boundaries and a real local bare-Git publish. A Pages workflow becomes active only
once this change and its whole-program timing dependency are merged into `main`.
A PR artifact/local preview is not proof that automatic production deployment has
already run. `workflow_dispatch` on main supports recovery without creating a
release. Publication failure is a failed Actions job, not a silent green result.
