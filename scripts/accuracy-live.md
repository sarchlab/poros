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
(rename a temporary file), with `kind: poros-accuracy-snapshot-v1`, a schema-4
`report` whose artifact status is **partial**, and `progress` containing state,
completed/required case and shard counts, active case IDs, errors and an ISO
8601 timestamp with timezone. The report carries the declared inventory,
completed validated rows and revision-specific profiles. Optional throughput
reference/provenance data binds the 17 simulated probes to the same head binary
and profile. `accuracy_live.validate_snapshot` specifies the input contract;
the unit test includes a complete synthetic example. Producers must validate
shard execution identities and outputs before exporting compact rows; the
renderer never runs simulations or collects hardware measurements.

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
  an ordered list of the 13 declared probes. Every row contains `id`,
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
  list matching the four reference cases. Each row contains `id`, `instructions`
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

For a live campaign, prepare the clean base/head binaries and profiles, then
execute and validate **both roles of every declared forwarding and scheduling suite before starting
the 264-point campaign (246 application points and 18 legacy microbenchmark points)**. The first live snapshot can therefore carry the
mandatory microbenchmark evidence and exact reference bytes. If those checks
fail, resolve the failure before launching application shards; do not publish
placeholder measurements or relax the snapshot validator. This is producer
sequencing, not a second renderer or hardware collection step. The producer then
exports validated completed application rows atomically as the shards finish.
The final bundle carries the same reference bytes as `forwarding.json` and both
simulation series, and the existing renderer shows them only in the separate
microbenchmark section.

Snapshots remain unpublishable even when all cases have finished. Only an
existing bundle that passes the normal archive validator and matches the
campaign's source, binaries, profiles, reference and inventory can transition
the page to a complete report. No archive/publisher validation is relaxed.
Snapshot-only pages link only to available JSON; completed pages expose only
validated bundle assets. Incomplete, failed or waiting pages refresh every
15 seconds. Malformed/intermediate input preserves the last validated snapshot
with a warning; a producer silent for 45 seconds is visibly stale.
