# Live and CI accuracy reports

`accuracy_site.detail` renders both modes, including the same application charts,
base/head statistics, timing curves/tables and separate microbenchmark section.
The comparison table uses the intersection of qualified application cases with
both predictions. The head overview explicitly reports its own qualified count.

To display a campaign's compact observations:

```sh
python3 scripts/accuracy_site.py serve --snapshot /path/live-snapshot.json --bundle /path/final-bundle --port 18763
```

The server binds only to `127.0.0.1`. A producer writes the snapshot JSON atomically
(rename a temporary file), with `kind: poros-accuracy-snapshot-v2`, a schema-4
`report` whose artifact status is **partial**, and `progress` containing state,
completed/required case and shard counts, active case IDs, errors and an ISO
8601 timestamp with timezone. The report carries the declared inventory,
completed validated rows and revision-specific profiles. `reference_text` carries
exact UTF-8 application reference bytes, including whitespace and the final
newline, bound to the measured head's file manifest. Every declared suite also
requires exact `<suite>_reference_text` and `<suite>_provenance`; the latter has
exactly `head_commit`, `base_commit`, `head_binary_sha256`,
`base_binary_sha256`, and `model_profiles`, all matching the report.
Throughput uses `report.throughput_simulation` (17 head-only cases); the other
suites use objects with exactly `base` and `head` lists. Empty lists are pending,
never zero-cycle predictions. Each list may be an **ordered subset** of its
reference cases, and base/head subsets may differ. Unknown, duplicate, reordered,
invalid-count and nonpositive-cycle rows are rejected. All native references and
profiles are checked even when no simulations have finished.

The schema-4 report and `accuracy_site.validate_compact_evidence` are shared by
live and CI. Only the snapshot wrapper is version2; existing complete schema-4
bundles retain their strict coverage and archive validation. Snapshot-v1 producers
must migrate to exact reference text and the five-key throughput provenance.
`accuracy_live.validate_snapshot` specifies the wrapper contract. The exporter
validates raw execution identities, full outputs, counts and profiles before
constructing the same application rows as the serial comparator. The renderer
never runs simulations or collects hardware measurements.

The top three-segment bar counts **measurement points**, not simulator executions.
For the current declaration it covers 552 points: 264 whole-program points
(including the 18 legacy microbenchmark points once), 17 head-only throughput
points, and 13 forwarding + 4 scheduling + 73 capacity + 41 recurrence + 140
memory paired points.
A whole-program point includes its required functional and model-role evidence;
a paired diagnostic point completes only when both role rows are present. An
explicitly validated unsupported-base row resolves that role. A partial pair or
a point assigned to an active worker is in progress; neither can inflate the
completed segment. IDs are namespaced by suite, so the legacy memory group and
memory diagnostics are distinct. Complete CI pages use the same calculation.

The exporter derives memory activity from the prepared plan's memory assignments
for active shards and emits it through the existing `diagnostic_progress` role/ID
field. These are active-worker assignments, not a claim that every assigned point
is simultaneously executing. Role activity and partial pairs collapse to one
point. The existing 264-case and shard transport counters remain supporting
subtotals; the global denominator is derived from the declared reference cases.

All declared suites and subgroups remain visible while pending, running, partial,
failed or complete. Available native points remain visible with missing predicted
points labeled pending. Optional `report.diagnostic_progress` maps suite names to
`{state, active, errors}`; `active` contains `{role, id}` objects, and errors are
strings. Active IDs must be declared, distinct and absent from completed rows.
Complete means every required role/case is present. The three legacy whole-program
microbenchmark groups retain their own panels and 18 points within the existing
264-point inventory. None of the diagnostic suites enter application statistics.

For a head that declares the forwarding suite (any of its measurement inputs,
ELF fixture or reference in `report.head.files`), the snapshot must also contain
all of the following. Historical heads without that declaration omit these
fields; stray forwarding evidence on an undeclaring head is rejected.

- `forwarding_reference_text`: the exact UTF-8 contents of the committed
  `evaluation/microbenchmarks/forwarding/m2.json` file as a JSON string. The
  SHA-256 of these bytes must equal
  `report.head.files["evaluation/microbenchmarks/forwarding/m2.json"]` before
  parsing. Preserve the file's whitespace and final newline; do not parse and
  reserialize it. A separate parsed `forwarding_reference` payload is rejected.
- `report.forwarding_simulation`: an object with exactly `base` and `head`, each
  an ordered subset of the 13 declared probes (all 13 for a complete report). Every row contains `id`,
  `instructions`, `cycles` and `updates`. Counts are differences between 1,024
  and 2,048 iterations: `updates` is 65,536, and `instructions` must equal 1,024
  times the probe's declared instructions per iteration. Cycles must be a
  positive integer. Both binaries execute the head-declared ELF fixture and
  must pass analytic output, instruction-count, exact revision and role-specific
  model-profile validation, including when the base source predates the suite.
- `forwarding_provenance`: an object with exactly `head_commit`, `base_commit`,
  `head_binary_sha256`, `base_binary_sha256` and `model_profiles`.
  `head_commit` equals `report.head.commit`; the other values equal the
  correspondingly named fields in `report`, with both complete model-profile
  declarations retained.

A head declaring the scheduling suite (any measurement input, ELF fixture or
reference under `evaluation/microbenchmarks/scheduling/` in `report.head.files`)
also requires the analogous scheduling fields. Historical heads without the
suite omit them, and stray scheduling evidence is rejected.

- `scheduling_reference_text`: exact UTF-8 bytes of the committed
  `evaluation/microbenchmarks/scheduling/m2.json`, transported as a JSON string.
  Its SHA-256 must match the corresponding `report.head.files` entry, including
  whitespace and the final newline. Parsed `scheduling_reference` is rejected.
- `report.scheduling_simulation`: exactly `base` and `head`, each an ordered
  subset of the four reference cases (all four for a complete report). Each row contains `id`, `instructions`
  and positive integer `cycles`. Counts subtract 1,024 from 2,048 iterations;
  instructions must equal 1,024 times the case's declared per-iteration count.
  Both roles execute the head-declared scheduling ELF and pass analytic output,
  instruction-count, exact binary revision and role-specific profile checks.
  Unlike forwarding rows, scheduling rows have no `updates` field.
- `scheduling_provenance`: exactly `head_commit`, `base_commit`,
  `head_binary_sha256`, `base_binary_sha256` and `model_profiles`, matching
  `report` in the same way as forwarding provenance above.

The final bundle retains exact scheduling reference bytes as `scheduling.json`
and both simulated series. The common renderer shows the scheduling graphs
and default-collapsed table only in the separate microbenchmark section; these
points do not enter application error or correlation summaries.

A head declaring the capacity suite (any measurement input, ELF fixture or
reference under `evaluation/microbenchmarks/capacity/` in `report.head.files`)
requires all three capacity fields below. Historical heads without the suite
omit them; stray capacity evidence on an undeclaring head is rejected.

- `capacity_reference_text`: the exact UTF-8 contents of committed
  `evaluation/microbenchmarks/capacity/m2.json`, including its embedded `boundary`
  block, transported as one JSON string. Its SHA-256 must match the corresponding
  `report.head.files` entry. Preserve whitespace and the final newline; do not
  parse and reserialize the reference. A parsed `capacity_reference` is rejected.
- `report.capacity_simulation`: exactly `base` and `head`, each an ordered list
  of the **73** declared cases in reference order (all 73 for a complete report). Each row contains `id`,
  `instructions`, and positive integer `cycles`, with no `updates` field.
  Counts subtract 1,024 from 2,048 iterations (`capacity.ITERATIONS`): instructions
  equal 1,024 times that case's declared per-iteration count. Both binaries run
  the head-declared capacity ELF and must pass analytic checksum, instruction
  count, exact binary revision, and role-specific model-profile validation,
  including when the base revision predates the suite.
- `capacity_provenance`: exactly `head_commit`, `base_commit`,
  `head_binary_sha256`, `base_binary_sha256`, and `model_profiles`.
  `head_commit` equals `report.head.commit`; all other values equal their named
  fields in `report`, retaining both complete profile declarations.

The final bundle retains the exact reference bytes as `capacity.json` and both
simulated series. The shared renderer shows native capacity/divider controls,
paired boundary ratios with P10/P90 dispersion, and collapsed tables only in
the microbenchmark section. These points never enter application accuracy,
S-curves, CDFs, or correlations.

A head declaring the recurrence suite (any measurement input, ELF fixture or
reference under `evaluation/microbenchmarks/recurrence/` in `report.head.files`)
requires the three recurrence fields below. Historical heads without the suite
omit them; stray recurrence evidence on an undeclaring head is rejected.

- `recurrence_reference_text`: the exact UTF-8 contents of committed
  `evaluation/microbenchmarks/recurrence/m2-linux.json`, transported as one JSON
  string whose SHA-256 matches the corresponding `report.head.files` entry. This
  reference is a native Linux capture on a pinned M2 performance core and names
  its host, core, cycle scale and counter checks; it is not a macOS reference.
  A parsed `recurrence_reference` is rejected.
- `report.recurrence_simulation`: exactly `base` and `head`, each an ordered
  subset of the **41** declared cases (all 41 for a complete report). Each row
  contains `id`, `instructions`, positive integer `cycles` and `updates`. Counts
  subtract 128 from 256 outer trips (`recurrence.ITERATIONS`): instructions and
  updates equal 128 times the case's declared per-trip counts. Both binaries
  run the head-declared recurrence ELF and must pass analytic output,
  instruction-count, exact binary revision and role-specific profile checks,
  including when the base revision predates the suite.
- `recurrence_provenance`: exactly `head_commit`, `base_commit`,
  `head_binary_sha256`, `base_binary_sha256` and `model_profiles`, matching
  `report` in the same way as forwarding provenance above.

The final bundle retains the exact reference bytes as `recurrence.json` and
both simulated series. The shared renderer plots native and simulated **cycles
per update** in one unit, so no frequency converts either side, with a
collapsed table, only in the microbenchmark section. These points never enter
application accuracy, S-curves, CDFs, or correlations.

For a live campaign, prepare the clean base/head binaries, profiles and committed
references first. The first snapshot may have no results; it still carries all
mandatory references, including the capacity boundary block. Run the ordinary
source-bound application/memory shards and independent diagnostic workers in
parallel, then aggregate their already completed raw records. No
prelaunch simulation barrier is necessary. The exporter reads `shard.json` or
`shard.json.gz`; compressed deliveries need a matching `receipt.json` with the
uncompressed SHA-256, head commit, both binary digests and returncode0. It never
expands the entire campaign locally. Its process cache retains only validated
canonical rows, keyed by immutable file metadata and receipt digest; a changed
file is revalidated. New incomplete downloads without receipts remain pending.

```sh
python3 NEW/scripts/accuracy_parallel.py diagnostic-tasks --prepared RUN/prepared \
  --source FROZEN_SOURCE --run-id RUN_ID --attempt 1 --output RUN/diagnostics.json
# A scheduler assigns each listed diagnostic_index once to its own directory.
python3 NEW/scripts/accuracy_parallel.py diagnostic --prepared RUN/prepared \
  --source FROZEN_SOURCE --run-id RUN_ID --attempt 1 --diagnostic-index 0 \
  --checkpoints RUN/diagnostics/0 --output RUN/diagnostics/0/diagnostic.json
python3 NEW/scripts/accuracy_progress.py --prepared RUN/prepared --source FROZEN_SOURCE \
  --results RUN/shards --run-id RUN_ID --attempt 1 --status RUN/status.json \
  --checkpoints RUN/diagnostics --diagnostic-status RUN/diagnostic-status.json \
  --output RUN/snapshot.json --interval 15
python3 NEW/scripts/accuracy_parallel.py aggregate --prepared RUN/prepared \
  --source FROZEN_SOURCE --results RAW_SHARDS --run-id RUN_ID --attempt 1 \
  --output RUN/final/report.json --checkpoints RUN/diagnostics
```

The task inventory is derived from the frozen source and binds the original
`plan_id`; creating it does not change `plan.json` or existing shard identities.
The current 148 diagnostic points comprise 17 head-only throughput points and
13 forwarding, 4 scheduling, 73 capacity and 41 recurrence points with both
roles: 558 raw short/long executions. Each worker owns one directory and its `diagnostic.json`
status; a directory lock rejects concurrent writers.
Conflicting point or campaign ownership is rejected without overwriting the
existing status receipt. Checkpoints are named
`<suite>-<role>-<suite_index>-<iterations>.json`, with one-based suite indices.
The normal CI workflow schedules this matrix alongside application/memory
shards. Aggregation waits for both matrices, validates all raw evidence and
builds the report without executable binary permissions or simulator calls.
Only successful diagnostic jobs upload aggregatable raw artifacts. Each job
first uploads its status separately, then uploads raw evidence as its final
step on success. Failed or interrupted points therefore leave no partial raw
artifact in the aggregate's download pattern. Re-running failed jobs reuses
successful points and shards from earlier attempts; their simulations need not
run again. Status artifacts remain available for diagnosing failed attempts.

Both optional scheduler status files below name the prepared plan they belong
to with `plan_id`, the value `prepare` prints and `diagnostic-tasks` records in
`diagnostics.json`. Campaigns of two devices can share `head`, `base` and
`run_id`, so those three do not identify a campaign. For a plan that names a
device the exporter refuses a status file without `plan_id`, or with another
plan's, before reading anything else from it. A plan of a source that predates
device selection names no device and is `m2` alone; its schedulers' status
files, which never carried `plan_id`, are still accepted on the three fields,
and a `plan_id` they do carry must match.

The optional campaign status file binds `head`, `base`, `run_id`, `plan_id`, completed shard
indices and active shard indices (`active` object). It supplies activity only;
completed results still require raw validation. `phase: complete` is accepted only
when strict validation confirms every application and diagnostic result. A full
264/264 whole-program subtotal alone never means the diagnostic campaign is finished. The exporter
retries transient failures without replacing its last good atomic snapshot.

The separate optional diagnostic status file binds `head`, `base`, `run_id`,
`plan_id`, `completed` task indices, `active` (index-to-worker object), and `failed`
entries with `index` and `error`. It supplies worker activity only; completed
rows always come from validated raw checkpoints. The exporter supports
out-of-order point completion and unequal base/head subsets in canonical
reference order. `--probe-progress` remains available for older serial
comparison producers, but cannot be combined with `--checkpoints`.

Each diagnostic worker records short/long executions atomically. Keys bind the measured source manifest, binary digest and
revision, suite, ELF digest, role/profile, case/index and iteration count. Reuse
checks the key/hash plus the same full raw output, revision, profile and counter
validator used after execution. A changed or corrupt checkpoint fails closed;
it never silently reruns completed simulations. Existing compatible raw
checkpoints retain their exact keys and can move into their owning task directory.
Aggregation recursively reads the checkpoint root and rejects missing,
unexpected, duplicate or conflicting evidence, with no execution fallback.
`aggregate` requires `--checkpoints` when the measured source declares diagnostic
points; historical sources without those suites do not require the argument.
Only `diagnostic.json`, `started.json` and `receipt.json` are transport metadata
and excluded from raw checkpoint indexing. Application and memory records are
replayed from validated shard evidence, never resimulated.

`--source` selects the frozen measured tree and its exact plan, references and
binaries. The implementation may come from a newer reporting-only checkout;
`report.head` remains the measured revision. `report.reporting` separately records
the reporting commit, dirty status and script hashes and appears in provenance.
This does not certify simulator equivalence: when reusing evidence after a
reporting-only change, separately verify that all model/Go/native/ELF/reference
files remain identical. No native references are recollected or relabeled.

Snapshots remain unpublishable even when all cases have finished. Only an
existing bundle that passes the normal archive validator and matches the
campaign's source, binaries, profiles, reference, inventory, completed rows and diagnostic results can transition
the page to a complete report. No archive/publisher validation is relaxed.
Snapshot-only pages link only to available JSON; completed pages expose only
validated bundle assets. Incomplete, failed or waiting pages refresh every
15 seconds. Malformed/intermediate input preserves the last validated snapshot
with a warning; a producer silent for 45 seconds is visibly stale.

## Devices

Everything above describes a report for a device that has native references,
which today is `m2`. A head that contains `evaluation/devices.json` also binds
every plan, shard, checkpoint, snapshot and report to one
[modeled device](../evaluation/README.md#modeled-devices). Heads without that
file, and all their retained evidence, are `m2` and are read exactly as before.

- `prepare --device NAME` chooses the device; the default is `m2`. The plan
  records `device` with exactly `id`, `head` and `base`. `head` and `base` are the
  declaration at each revision (`commit`, `path`, `sha256`, `declaration`), and
  `base` is null when the base revision has none. The device is part of the plan
  identity, so shard and diagnostic artifacts of two devices cannot be mixed.
- Each simulator that models the device is run with `--target NAME` and must
  report that target, its hardware and its parameter source in
  `timing.target`. A base revision that predates targets is `m2`: it is run
  without the argument and reports none. For any other device such a base is not
  run; tasks then have the single role `head`, paired suites keep an empty `base`
  list, and application rows have `base_status: device_unavailable`.
- Diagnostic checkpoint keys include the requested target identity. Keys of
  simulators that predate targets are unchanged.
- `report.device` repeats the plan's value. Offline validation checks the
  declaration hash against `report.head.files`; publication and the memory-only
  preview read both revisions from Git. A report that names no device and lists
  no declaration is accepted against Git only if its head revision has none.
- A failure keeps the device it was for. Before a plan or a comparison exists
  it records `device: {"id": NAME}`: `aggregate --device`, `accuracy_site.py
  failure --device` and a build or setup failure of `scripts/usertime-ci.sh BASE
  OUTPUT NAME` all do. A comparison that fails after resolving its device and
  before recording its source keeps the resolved `device` without `head`; such a
  report states the application reference availability its declaration gives
  and says the diagnostic suites are not established.
- The serial comparison takes the application reference its device declares.
  `usertime.py compare --reference` is optional and, when given, must name that
  file; `scripts/usertime-ci.sh` names none.

For a kind of reference the device does not have, the report and snapshot change
as follows; nothing native is substituted:

- No `reference_text`, `<suite>_reference_text` or `<suite>_provenance` is
  carried, and supplying one is rejected. The bundle holds only `report.json`
  and `report.md` when the device has no references at all.
- The report has no `reference` or `reference_sha256`. `output_oracle` names the
  committed complete-output identity file (`path`, `sha256`, `scope`). That file
  is the default device's reference. It is validated as a whole, timing fields
  included, and the device uses only its output identities: no timing from it
  is shown or compared.
- Application rows have `reference_quality: unmeasured` and no other
  `reference_*` field, no `error_percent`, `direction` or
  `error_change_percentage_points`. Summaries count zero comparable cases.
- `report.diagnostic_definitions` holds the exact case-definition text of each
  such suite (`cases.def`, or `cases.json` for memory), whose SHA-256 must equal
  the `report.head.files` entry. It supplies the case inventory that a native
  reference otherwise would.
- The page shows the device, its parameter source and its reference
  availability, replaces the accuracy charts with a notice, and lists
  predictions and simulated counters without reference or error columns.

A device's native application reference is the file its declaration names
under `references.application`, and it must record the declared `hardware`.
The plan binds that file's digest, a snapshot carries its bytes as
`reference_text` and a bundle as `reference.json`. The diagnostic suites still
have one committed reference file each, the M2 one; a declaration that names
another file for a suite, or gives the M2 file to other hardware, is refused
before a point of that suite runs.

Serve a device's snapshot the same way; the device is in the snapshot. Repeat
`--snapshot` with other devices of the same commit to serve that commit's
report while they run:

```sh
python3 NEW/scripts/accuracy_parallel.py prepare --base BASE --device m1 --shards 132 \
  --prepared RUN-M1/prepared --run-id RUN_ID --attempt 1
python3 scripts/accuracy_site.py serve --snapshot RUN-M1/snapshot.json --port 18763
python3 scripts/accuracy_site.py serve --snapshot RUN-M2/snapshot.json \
  --snapshot RUN-M1/snapshot.json --port 18763
```

### One report per commit

A commit has one report, whatever number of devices it declares. The Pages
archive lists commits, one row each, and nothing per device:

```text
calibration/index.json                       schema 2: one entry per commit
calibration/runs/<commit>/index.html         the commit's report
calibration/runs/<commit>/commit.json        declaration, device states, bundle digests, pooled summary
calibration/runs/<commit>/devices.json       the commit's own evaluation/devices.json, byte for byte
calibration/runs/<commit>/<bundle files>     the m2 bundle, where every published report already has it
calibration/runs/<commit>/devices/<id>/      each other device's bundle
```

The report is one document. Its views are sections reached by fragment links:
`#overview` for the commit and `#device-<id>` for each device, so a copied
address, back and forward all return to the same view. The `m2` view keeps the
element names of reports published before commits had devices, so old links
into a report still land. Elements of another device are named
`device-<id>_<name>`, and its files are linked under `devices/<id>/`. Where the
browser supports `:has()` only the addressed view is shown; otherwise the views
follow each other. No view depends on a script.

Which devices a commit has is that commit's own declaration, never the current
one:

- A report carries the declaration bound to its source inventory. `archive`
  retains it with its SHA-256 in `index.json`, `commit.json` and `devices.json`.
- A passing report without a declaration predates devices. Its commit is `m2`
  alone.
- A failure proves nothing about the declaration. The publisher gives `archive`
  its source repository, and `evaluation/devices.json` is then read from Git at
  exactly that commit. When that cannot be read either, the report says the
  supported devices are unknown and lists only the devices a run was attempted
  for. A failure filed in that state for a device the commit turns out not to
  declare is dropped once the declaration is known.
- Reports of one commit with different declarations, a device the commit does
  not declare, and publication metadata that names another device than the
  validated report are all refused before anything is stored.

A device arrives by publishing its bundle: it joins the commit's report without
touching the bundles of the others. Each declared device is shown as `report`,
`failed` or `pending`. The commit is `fail` if any device failed, `partial`
while a declared device has no report, and `pass` otherwise. Merges to `main`
publish `m2` only, so their commits are `partial` until other devices are
published. The latest successful main report is the newest main commit whose
evaluation succeeded and none of whose devices failed; pending devices do not
disqualify it. Each device keeps the Evaluation that produced its retained
evidence. A later publication that finds passing evidence already retained
reuses it: the device then records that publication's run and conclusion, the
history row shows the commit's newest Evaluation, and the evidence keeps the
Evaluation and evidence run it came from. Only evidence that is newly stored,
such as a report replacing a failure, brings its own Evaluation. A commit is
retained, evicted or pinned by a release as a whole, with every device's
bundle.

The commit overview pools raw observations. One observation is one device and
one application input with a usable native reference of that device, so the
same input on two measured devices is two observations, each labeled
`<device>/<case>`. The S-curve, the error distribution, the
reference-versus-prediction plot, mean and median error, Pearson r, Kendall
τ-b and the share of correctly ordered pairs are computed over the pooled
observations by the functions a single report uses. Nothing is an average of
per-device results. The base and head comparison uses the observations both
revisions predict. Microbenchmarks, devices without a native application
reference, failed devices and devices without a report contribute nothing; the
overview says which devices were pooled and which were not, and states that a
statistic is undefined rather than showing zero.

`index.json` entries of schema 1 are read as commits whose one report is the
`m2` device, and are rewritten as schema 2 on the next publication.

To look at the archive without publishing anything, keep the site one of the
tests builds with the real `archive` and renderer from synthetic transport
values, and serve it:

```sh
POROS_DEVICE_PAGES_PREVIEW=/tmp/poros-pages-preview python3 scripts/devices_test.py \
  CommitReportTests.test_staged_site_can_be_walked_from_history_to_commit_to_device
python3 -m http.server 18770 --bind 127.0.0.1 --directory /tmp/poros-pages-preview
# http://127.0.0.1:18770/calibration/
```

Its history has four commits. `aaaa…` declares four devices of its own: `m2`
and `syn-b` are measured, with different inputs and errors, `syn-c` has no
native reference and `syn-d` failed. `eeee…` has one unmeasured device reported
and three pending, `dddd…` has only a failure and an unknown declaration, and
`cccc…` predates devices. None of these values is a measurement.

## Memory diagnostics

A head declaring the memory suite additionally requires `memory_reference_text`,
`report.memory_simulation`, and `memory_provenance`. They use the same byte-bound
reference and exact base/head provenance contract described above. The final
bundle retains `memory.json`. Each simulated row contains `id`, `instructions`,
and positive integer `cycles`, in the reference's canonical case order. The
instruction difference equals `passes * instructions_per_pass` for that case;
pass counts vary by working-set size and must not be replaced by the older
suites' fixed 1,024-iteration difference.

The normal post-merge workflow executes memory points alongside application
points in the existing source-bound workers, then aggregates their raw pairs.
A local snapshot producer exports validated ordered memory subsets as shards
finish. Both roles remain present, with absent cases pending. Base image-capacity
unavailability stays explicit; it is not a missing head prediction.
For a memory-only development campaign, `accuracy_parallel.py memory-shard` and
`memory-aggregate` use the same prepared plan and exact binaries without running
application points. The artifact plots native against simulated values, so
these two commands refuse a device without a native memory reference; such a
device reports memory through its ordinary shards. When the source declares
devices the artifact records the plan's `device`, and the preview accepts it
only if that device, at both Git revisions, owns the memory reference. Render
that artifact with:

```sh
python3 scripts/accuracy_site.py memory-preview --report /path/memory-report.json --source . --output /path/memory-preview
```

This explicitly labeled diagnostic preview uses `accuracy_site.memory_panels`,
the same graphs and collapsed table used by the complete and live reports. It
contains no reused or relabeled application results and makes no new application
accuracy claim. See [the measurement contract](../evaluation/microbenchmarks/memory/README.md).
