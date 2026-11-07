# Simulation model declaration

`model-profile.json` declares the default OoO timing configuration used by the
accuracy harness. The Go test `TestDeclaredModelProfileMatchesRuntimeDefaults`
checks it against the actual CPU and cache defaults. Keep the declaration in
canonical JSON (sorted keys, two-space indentation, final newline).

This declaration is independent of frozen workload manifests and native reference
measurements. A calibration change updates the simulation declaration, without
rewriting the inputs or the hardware observations. Plans and reports bind head
and base profiles separately to their Git revisions; every execution must match
its role's profile. Before publication, both declarations are read from the exact
commits, so a self-consistent but fabricated base profile cannot pass. Archived
reports carry the profiles and source hashes and remain readable offline.

Revisions before this declaration use their recorded
`tests/usertime/performance.json` profile (or the earlier `manifest.json`). Historical reports keep their original
frozen-profile validation. The native input manifests and both native reference
files are unchanged by the frontend/dependency correction.

The default frontend admission limit is eight instructions per cycle. This is a
modeling choice supported by the native NOP and mixed-resource throughput probes;
it does not assert an eight-wide backend issue limit. NOP still uses frontend,
ROB and retirement capacity, but no integer execution slot. Ordinary registers
represented by `arm64asm.RegSP` participate in the scoreboard; actual SP and zero
register tracking retain their existing behavior. Load/store sharing and branch
redirect behavior are separate calibration work.

## Store-to-load timing hypothesis

The default OoO model has a one-cycle effective forwarding path for a GPR load
whose accesses each exactly match the youngest overlapping, still-in-flight
GPR store in address and width. The load result waits for both the load's own
address/unit readiness and the supplying store's recorded address/data issue
cycle. Store completion and buffer-capacity accounting remain unchanged. A
partial overlap, a width mismatch, an FP/SIMD access, a completed store, or an
ineligible member of a pair keeps the conventional path: wait for overlapping
stores to complete, then apply the existing five-cycle forwarding or normal
cache-access latency.

The fast path is a candidate model, not evidence of physical M2 memory renaming.
Earlier bounded experiments implicated the previous store-completion-plus-load
latency chain in Nussinov overprediction. They did not establish a universal
one-cycle latency or the GPR/FP distinction; apparent accuracy in FP controls
could include compensating errors. Matched native dependency probes and the
paired application campaign must assess these assumptions. No hardware
measurements, benchmark inputs, frequency, or cache defaults change here.

This change adds no configurable default, so `model-profile.json` is unchanged.
Hardcoded timing/dependency behavior is identified by the exact source revision
and binary hashes already bound into each plan and report; equal configuration
JSON does not imply equal simulator behavior. The source revision must therefore
be retained when comparing or reusing results. Old serialized store-buffer
entries without the new readiness/classification fields retain the conventional
path; newly produced states round-trip those fields.
