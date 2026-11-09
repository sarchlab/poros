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
For the current declaration it covers 511 points: 264 whole-program points
(including the 18 legacy microbenchmark points once), 17 head-only throughput
points, and 13 forwarding + 4 scheduling + 73 capacity + 140 memory paired points.
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

For a live campaign, prepare the clean base/head binaries, profiles and committed
references first. The first snapshot may have no results; it still carries all
mandatory references, including the capacity boundary block. Run the ordinary
source-bound shards, then aggregate their already completed raw records. No
prelaunch simulation barrier is necessary. The exporter reads `shard.json` or
`shard.json.gz`; compressed deliveries need a matching `receipt.json` with the
uncompressed SHA-256, head commit, both binary digests and returncode0. It never
expands the entire campaign locally. Its process cache retains only validated
canonical rows, keyed by immutable file metadata and receipt digest; a changed
file is revalidated. New incomplete downloads without receipts remain pending.

```sh
python3 NEW/scripts/accuracy_progress.py --prepared RUN/prepared --source FROZEN_SOURCE \
  --results RUN/shards --run-id RUN_ID --attempt 1 --status RUN/status.json \
  --probe-progress RUN/probe-progress.json --output RUN/snapshot.json --interval 15
python3 NEW/scripts/accuracy_parallel.py aggregate --prepared RUN/prepared \
  --source FROZEN_SOURCE --results RAW_SHARDS --run-id RUN_ID --attempt 1 \
  --output RUN/final/report.json --checkpoints RUN/probe-checkpoints \
  --progress RUN/probe-progress.json
```

The optional campaign status file binds `head`, `base`, `run_id`, completed shard
indices and active shard indices (`active` object). It supplies activity only;
completed results still require raw validation. `phase: complete` is accepted only
when strict validation confirms every application and diagnostic result. A full
264/264 whole-program subtotal alone never means the diagnostic campaign is finished. The exporter
retries transient failures without replacing its last good atomic snapshot.

The aggregate's optional checkpoint directory records each short/long probe
execution atomically. Keys bind the measured source manifest, binary digest and
revision, suite, ELF digest, role/profile, case/index and iteration count. Reuse
checks the key/hash plus the same full raw output, revision, profile and counter
validator used after execution. A changed or corrupt checkpoint fails closed;
it does not silently replay completed simulations. Each completed pair updates
the canonical progress report, including unequal head/base prefixes; active and
failed states preserve the last validated completed rows. Application and memory
records are replayed from validated shard evidence, never resimulated.

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
application points. Render that artifact with:

```sh
python3 scripts/accuracy_site.py memory-preview --report /path/memory-report.json --source . --output /path/memory-preview
```

This explicitly labeled diagnostic preview uses `accuracy_site.memory_panels`,
the same graphs and collapsed table used by the complete and live reports. It
contains no reused or relabeled application results and makes no new application
accuracy claim. See [the measurement contract](../evaluation/microbenchmarks/memory/README.md).
