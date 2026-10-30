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
tested, including no execution for a pre-cancelled context. The 80 remaining legacy baseline cases and eight pipeline variants stay
unchanged after removing MVT’s two superseded rows. Existing function-level hooks remain a legacy adapter only.

## Workload suite and builds

`tests/usertime/manifest.json` schema 2 defines four ordered six-point sweeps:

| Workload | Sweep coordinates | Split |
| --- | --- | --- |
| integer | 500000, 1000000, 2000000, 4000000, 6000000, 8000000 iterations | calibration |
| memory | 8192, 65536, 262144, 1048576, 8388608, 33554432 working-set bytes | held_out |
| branch | 500000, 1000000, 2000000, 4000000, 6000000, 8000000 iterations | held_out |
| MVT | n=8,63,120,129,400,1536 | calibration |

Memory performs 4,194,304 read/modify/write accesses per run, plus initialization
of every element in the declared working set. Larger working sets therefore also
have larger initialization costs. Its curve is not a pure cache-latency experiment
or constant-total-work comparison. The largest set gets one full traversal;
smaller sets get an integer number of traversals. Cache-size boundaries refer to
the configured model (128 KiB L1, 16 MiB L2), not independently measured capacities.

MVT initializes the matrix/vectors inside the guest, computes both matrix-vector
products and writes all 2*n binary64 output elements. Its active data bytes are
`8*(n*n+4*n)`. Fixed maximum BSS buffers keep all six dimensions within the
existing checked address-space limit; indexing uses the runtime dimension.
Invalid/empty/overflowing arguments, oversized matrices and non-power-of-two
memory sets are rejected before computation. No simulator address/name hook,
manual argument register setup or function-byte extraction is used.

Builds use GCC 14.2.0 / binutils 2.44 with the recorded build script, including scalar
`-O1`, disabled vectorization/if-conversion/unrolling and `-ffp-contract=off`.
The latter is a reproducibility choice, not a hardware FMA claim. Rebuild with:

```sh
CC_AARCH64=/opt/homebrew/opt/musl-cross/bin/aarch64-linux-musl-gcc tests/usertime/build.sh
```

Sources/build script and each ELF have SHA-256 identities in the manifest.
`workload_contract.py` validates complete four-group/six-point inventories,
coordinates, role, arguments and binary/source identities. Removing, reordering
or duplicating cases fails. Schema 1 is supported only to read previously
published six-case bundles; active measurements use schema 2.

## Native records and correctness

`scripts/usertime.py capture` directly executes these same Linux ARM64 ELFs,
with one warm-up process followed by 21 retained child processes per case.
`wait4.ru_utime` is direct-child user CPU time; wall time, system CPU time and
simulator host cost are not substituted. Each output and exit status is checked.
Scalar outputs use independent arithmetic oracles. The MVT oracle forms exact
integer dot-product numerators and converts once, independently of the guest’s
sequential floating-point accumulation. Every finite element must satisfy
`abs(got-want) <= max(1e-11, 1e-12*max(abs(got), abs(want)))`; output truncation and NaN/Inf fail.
Functional and timing executions must have identical output bytes and instruction
counts in addition to each passing the independent oracle.

`m2-linuxvm-user-v2.json` contains all 24 cases and all 504 samples from the first
successful capture, plus system time, source/build/binary/input identities,
measurement harness and imported contract hashes, container image identity and
hardware description. Active comparisons intentionally bind the exact native collector and
`workload_contract.py` bytes: any edit to either file, including a validator or
comment change, requires a fresh capture. This conservative provenance policy
has no hash bypass; retained historical bundles remain self-contained.

It was collected on an Apple M2 Mac14,3 with 24 GiB RAM using
native ARM64 execution in a Docker Linux VM. No QEMU timing is used. Heavy local
simulation/testing was paused, but exclusive host isolation, aggregate host load,
core type and instantaneous frequency were not measured or controlled. VM user
accounting and unknown scheduling remain limitations, not bare-metal ground truth.
The old six-case reference, separate Mach-O witness and MVT kernel-ROI data are
superseded and removed from the active tree; published historical bundles remain
self-contained and readable. Git preserves rollback history.

Quality rules are unchanged: any nonpositive/nonfinite raw user-time sample
marks the case invalid_nonpositive; a median below 100 microseconds is below the
conservative policy floor; P10-P90 spread above 20% of median is noisy. This floor
is not a measured clock resolution. All 24 cases remain visible. The retained
capture qualifies only branch 6M/8M: calibration 0/12, held_out 2/12. MVT 400 and 1536
medians are1.265ms and8.058ms respectively, but are unqualified (zero sample and
noise respectively). No sample, point or workload was rerolled or removed to
improve accuracy. Unqualified points have no reported error; empty means display
an em dash, never0%.

Example acquisition (existing native Linux ARM64 image):

```sh
docker run --rm --network none --read-only --tmpfs /tmp:rw,nosuid,size=64m \
  -e PYTHONDONTWRITEBYTECODE=1 -e POROS_REFERENCE_IMAGE=IMAGE_DIGEST \
  -v "$PWD:/src:ro" -v /private/tmp:/out -w /src poros-elf-reference:local \
  python3 scripts/usertime.py capture --samples 21 --measurement-id NEW_ID \
  --environment virtualized --hardware 'exact machine/environment' --output /out/reference.json
```

Raw references
are frozen by identity; changes to workload bytes, inputs, protocol or environment
require a new capture. Generic `measure -- PROGRAM ARGS` remains available for
native executables, but Mach-O measurements do not become same-ELF references.

## Required PR report

`scripts/usertime-ci.sh BASE` builds clean exact head/base binaries and runs all 24
required cases: head functional mode and head/base OoO timing mode. Every case
uses the same ELF/arguments and frozen CPU/cache profile across both revisions.
Missing cases, wrong output, base failure, changed identities or mismatched
profiles fail artifact acceptance. The comparison has a10minute per-execution
budget and150million-instruction limit. It runs on every PR/main push; the full
inventory is never silently reduced to smoke cases. A pre-timing predecessor is
explicitly unavailable, not assigned a fabricated base score.

Schema2 report JSON embeds the validated workload manifest. The archived bundle
retains the reference, manifest, source/ELF files, native collector and imported
contract module. The publisher verifies their hashes against the exact source
commit and can still render schema1 bundles without requiring new files.
History trends separate different references/profiles, so the new suite is not
spliced onto a six-case trend.

Formula `absolute-difference-over-min-v2` is unchanged:
`100*abs(predicted-reference)/min(predicted,reference)`. Only finite positive,
qualified times contribute. Base/head errors, direction and percentage-point
changes use the same reference. Artifact completeness, functional correctness
and hardware accuracy remain separate; hardware accuracy stays unverified and
there is no numeric accuracy cutoff or tuning in this migration.

## Published accuracy dashboard

The dashboard is published at <https://sarchlab.github.io/poros/calibration/>.
GitHub Pages must be configured as **Deploy from a branch → gh-pages → / (root)**.
No personal access token or extra service is required. The publishing job uses
`contents: write` to update that branch and `pages: write` to explicitly request
a Pages build, then waits for a successful build that contains its publication.
A default `GITHUB_TOKEN` push alone does not trigger a branch-based Pages build.
Because the build API initially may return the previous `latest` build, polling
ignores unrelated older successes/errors until the requested commit or a descendant
is built, errors, or the bounded timeout expires.

Every PR update produces an HTML preview plus JSON/Markdown and a complete
evidence bundle in the `user-time-accuracy-ATTEMPT` artifact, retained for 30 days. PR
reports never enter the public archive. Each main push evaluates its latest
commit; intermediate commits in one push are not individually measured.
`Publish accuracy reports` consumes only completed main-push Evaluation runs,
verifying their workflow, repository, commit and attempt. Artifact names include
the attempt number. A rerun of failed jobs may reuse the newest earlier artifact
from the same run and commit, preserving its original evidence-attempt link. Failed/cancelled runs
are visible, including an explicit missing-report record when no artifact exists.
Early validation failures retain the original diagnostic JSON unchanged inside a
bundle bound to the requested target; the page explicitly states that no executed
source/binary identity was recorded.
A release/manual accuracy build is not a full Evaluation run; its status is
reported separately, and manual recovery queries full Evaluation proof for the
exact commit before qualifying it. Only a complete report with a successful
Evaluation run can become the latest successful main report. The homepage separately queries current main's public
GitHub status; if that query is unavailable it labels the displayed snapshot.

The archive retains the **20 newest ordinary main commits**, ordered by main's
first-parent ancestry rather than completion time. Published releases permanently
pin the exact tag commit and do not occupy those 20 slots. Releasing a commit
already in the archive reuses its original successful report bytes, base revision
and measurements. Only validated passing evidence is reused; missing, failed or
invalid evidence triggers a read-only build of the exact release commit.
Automatic release events run the workflow stored in the tagged revision, so they
apply only to tags that contain this workflow. For older/missed tags or recovery,
run `Publish accuracy reports` manually **from main** with `release_tag` set to an
existing published release. The resolver rejects drafts/nonexistent releases,
records the stable release ID and exact tag SHA (including annotated tags), and
rechecks both before publication. Automatic release events also bind that SHA to
the original event's `GITHUB_SHA`. An old target predating this timing protocol
receives an explicit unsupported/failure report through this manual path. No
release is created by the workflow. Published prereleases follow the same permanent
retention policy. Deleting a GitHub release does not automatically delete its evidence.

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
Evaluation identity is ordered separately from publication-job identity, so a
release/manual run cannot hide a later Evaluation completion. The newest verified
Evaluation outcome controls latest-success eligibility: a later failed or pending
attempt makes that commit ineligible, while an older event or a weaker lookup for
the same attempt cannot erase completed proof. A failed report can be replaced by
a successful retry without overwriting earlier successful measurement evidence.

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
already run. `workflow_dispatch` on main with empty `release_tag` recovers the main
report;
with a tag it backfills an existing published release. Release/manual artifacts
also carry attempt-specific names, with validated earlier-attempt reuse when only
failed jobs are rerun. Publication failure is a failed Actions job, not a silent
green result.
