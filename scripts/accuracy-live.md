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

Snapshots remain unpublishable even when all cases have finished. Only an
existing bundle that passes the normal archive validator and matches the
campaign's source, binaries, profiles, reference and inventory can transition
the page to a complete report. No archive/publisher validation is relaxed.
Snapshot-only pages link only to available JSON; completed pages expose only
validated bundle assets. Incomplete, failed or waiting pages refresh every
15 seconds. Malformed/intermediate input preserves the last validated snapshot
with a warning; a producer silent for 45 seconds is visibly stale.
