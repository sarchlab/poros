# Memory accuracy umbrella

Establish evidence for single-core memory behavior before changing the model.
This plan and the first diagnostic suite do not change cache parameters, memory
latencies, or the application's hardware-reference data. Functional correctness
and complete measurements do not by themselves establish hardware accuracy.

## 1. Measure latency, parallelism, and effective bandwidth

Add deterministic random pointer chasing with 1, 2, 4, 8, and 16 independent
chains, plus sequential Read, Write, and Copy loops. Sweep total working-set
size from cache-resident buffers to buffers larger than the modeled last-level
cache, with extra points around observed transitions. Keep total resident bytes
constant when changing chain count; Copy's working set includes both buffers.
Use one thread initially. These probes complement the existing sequential
read/modify/write test rather than treating it as a latency measurement.

Use the same computation assembly for native Mach-O measurement and simulated
ELF execution. Initialize deterministic inputs, touch pages, and perform full
working-set warmup before the measured steady-state interval. Validate outputs,
loop instruction counts, and binary/source identities. Match the warmup and
long/short differential protocol on both paths; a short run that visits only a
small part of a declared working set is not a measurement of that working set.
Retain raw paired observations outside Git and commit only compact reference
statistics and the provenance needed to validate and plot them. Preserve all
existing reference measurements.

Report dependent-load latency, time per load for independent chains, and
application-visible bytes per second for streaming loops. The latter is
**effective bandwidth**, not measured cache-line or DRAM-bus traffic. Show
reference variability and qualification status. Do not convert native time to
measured cycles using an assumed CPU frequency, infer physical cache levels from
size alone, or describe scalar throughput as peak vector bandwidth.

Acceptance requires functional and timing outputs to agree with an independent
reference execution, reproducible native observations with declared quality
criteria, and source-bound base/head simulation results. Measure a large-case
runtime before fixing the full campaign schedule. Ordinary PR CI uses small
correctness cases; the existing post-merge-to-main accuracy workflow runs the
full diagnostic set without changing its trigger policy. Large diagnostic work
must be bounded and must not silently turn the report aggregation step into an
unbounded serial campaign.

Render the results through the existing report implementation in a separate
microbenchmark section, with plots before tables and tables collapsed by
default. Exclude these probes from all application S-curves, correlations,
ranking statistics, and application error summaries. Missing or mismatched
required evidence fails validation; it is not silently omitted.

## 2. Replace memory mechanisms with evidence

Use the diagnostics to distinguish dependent access latency, memory-level
parallelism, prefetch effects, and bandwidth limits. Review each proposed model
change separately with a causal hypothesis, mechanism-level regression tests,
and comparison against the relevant probes. A faster simulation or a better
application fit alone does not justify a parameter or mechanism.

Subsequent sub-PRs can integrate Akita cache and DRAM components with explicit
request/completion timing, queue and bandwidth limits, and correct data
availability. Follow Akita v5 configuration, lifecycle, and routing rules. Keep
functional results and completion ordering as hard gates. Translation, multicore
coherence, and other mechanisms require their own bounded design and evidence;
this first diagnostic PR does not implement them.

## 3. Evaluate and retain only supported improvements

For model changes, compare the same qualified application points and unchanged
hardware references, reporting absolute `diff/min` mean and median error,
Pearson r, Kendall's tau-b, correct-pair share, and individual improvements and
regressions. Keep microbenchmark evidence separate. Investigate large regressions
before accepting a mechanism rather than fitting compensating parameters.

Each sub-PR must be independently reviewable and revertible. Revert a model
change if it violates functional/completion invariants or its claimed effect
cannot be reproduced. Diagnostic probes and recorded measurements remain useful
even when a candidate model change is rejected. Hardware-validation status stays
unchanged until its separately defined evidence requirements are satisfied.
