# Poros roadmap

Updated: 2026-10-29. This is a development and acceptance plan, not a claim
that the capabilities below are already implemented. Milestone IDs are local
planning IDs; corresponding GitHub milestones have not yet been created.

## Target and boundaries

Poros aims to execute general-purpose ARM programs and predict their CPU
performance accurately. The initial software target is AArch64 Linux user-mode
execution of ELF binaries. Recompilation is allowed. Threads and files are in
scope; network services may initially use deterministic responses and modeled
latencies. GPU execution and full-system OS boot are outside this phase.

The hardware target includes Apple M1–M6 CPU families, their chip variants and
core types, and NVIDIA ARM CPUs. Grace is the first NVIDIA implementation track;
other NVIDIA CPUs need separate profiles and validation. Hardware differences
require both configurable parameters and replaceable model mechanisms, not just
frequency scaling. Separate instruction semantics, Linux runtime, core timing,
and platform topology.

Each workload/profile pair records five distinct acceptance levels: loaded,
executed, terminated, functionally correct, and performance validated. Hardware
profiles separately record functional coverage, modeled structure, and actual
hardware-validation status. A profile without matching hardware evidence remains
unvalidated even when its configuration is complete.

Static versus dynamic linking remains an implementation decision. M1 starts with
a minimal static ELF to bound the first loader milestone; this does not restrict
the long-term target. M2 selects and records a concrete libc/linking configuration.

## Existing issues

| Issue | Status at planning | Roadmap role |
| --- | --- | --- |
| [#36: Akita v5 migration](https://github.com/sarchlab/poros/issues/36) | Completed | Existing foundation, merged in PR #43 |
| [#37: Configurable problem sizes](https://github.com/sarchlab/poros/issues/37) | Open | M0, size sweeps, and hardware validation |
| [#38: Apple CPU profiles](https://github.com/sarchlab/poros/issues/38) | Open | Hardware track; expand its scope from M1/M2/M5 to M1–M6 |
| [#39: Linux runtime and SPEC](https://github.com/sarchlab/poros/issues/39) | Open | M1, M2, and M5; split into bounded implementation tasks |
| [#41: Grace CPU](https://github.com/sarchlab/poros/issues/41) | Open | Grace profile, ISA coverage, topology, and validation |
| [#40: CPU–GPU integration](https://github.com/sarchlab/poros/issues/40) | Open | Future work; not a prerequisite for this roadmap |

Add dedicated tracking tasks for shared-memory threading, timed memory/coherence,
and AI/agent workloads when implementing these milestones. No issue is considered
complete merely because one related milestone is complete.

## M0 — Support matrix and regression infrastructure

**Deliverables**

- A versioned workload manifest: source/version, build recipe, compiler, ISA
  target, binary hash, inputs, expected results, and runtime requirements.
- Run metadata: CPU profile/version, thread count, seed, cache-start policy,
  measured region, simulation metrics, and reference-measurement provenance.
- Preserve existing kernel regressions and establish independent correctness
  references for new programs.
- Repository CI for build, vet, short tests, and complete regression runs.
  Code-analysis checks alone do not establish simulation correctness.
- Define accuracy formulas, calibration/held-out sets, reporting, and execution
  budgets before fitting new models. Record unresolved choices explicitly.

**Acceptance**

A clean environment can build, run, compare, and produce a machine-readable
report. Functional correctness, complete outputs, bound provenance and required
coverage are hard gates; missing assets and skipped required tests fail acceptance.
Whole-kernel cycle pinning has been retired. The separate 24-case accuracy report
shows base/head timing changes for review; it is not an automatic accuracy gate.

**Regression and rollback**

The legacy whole-kernel golden rows have been replaced by
[complete ELF coverage](tests/benchmarks/README.md): all 41 compute kernels run
through guest startup and normal process exit, with full-output references,
same-ELF differential checks and retained isolated pipeline/ISA tests. The frozen
M0 calibration split still applies; functional migration does not create an
independent historical holdout or establish hardware accuracy. Revert this ELF
migration as a coherent change if needed: restoring old patched instruction
arrays alone is incompatible with the corrected CNEG semantics.

## M1 — ELF entry to process exit

**Deliverables**

- Load AArch64 ELF segments, zero BSS, create guest address space and stack.
- Establish entry PC, arguments, environment, auxiliary vector, and ABI state.
- Implement the minimal syscall subset, initially including output and exit.
- Give actionable unsupported-instruction/syscall and invalid-memory diagnostics.
- Define the execution/timing interface. Evaluate the current interpreter against
  an existing execution backend using correctness, required timing information,
  and execution speed before committing to a replacement.

**Acceptance**

Minimal static ELF programs cover Hello World, arguments/environment, BSS,
stack alignment, integer and floating-point computation, and exit status. Run
from the real entry point and compare with the same ELF in a reference runtime.
Malformed binaries, invalid accesses, and infinite loops fail under defined
limits rather than hanging or silently succeeding.

**Regression and rollback**

No new test program requires embedded machine-code arrays or manual argument
register setup in Go. Preserve the function-level harness. New instruction forms
receive boundary-case and differential tests. Keep changes independently
revertible without removing the old harness.

## M2 — Complete single-threaded applications

**Deliverables**

- Select a reproducible libc/linking configuration and provide its required
  allocation, mapping, file, time, and startup semantics.
- Run a fixed BusyBox applet set: cat, wc, sort, grep, and sha256sum.
- Run zstd compression/decompression and SQLite database workloads.
- Provide reproducible guest filesystem inputs and explicit guest/host pointer
  and descriptor boundaries.

**Acceptance**

BusyBox output and exit status match references. zstd round trips reproduce
input bytes across multiple sizes. SQLite creation, insertion, queries, close,
and reopen preserve expected logical contents. Exercise missing files, EOF,
short I/O, allocation failure, and other relevant error paths.

**Regression and rollback**

Functional and timing modes produce the same architectural results. Do not add
application-specific address/name hooks or pretend an unsupported syscall
succeeded. Promote support only after the complete fixed test set passes.

## M3 — Real shared-memory threads

**Deliverables**

- Per-thread registers, stack, TLS, and shared process memory.
- Required thread creation, futex, synchronization, and termination semantics.
- Required ARM atomics, exclusive accesses, and acquire/release operations.
- Reproducible guest scheduling and explicit supported memory-order semantics.

**Acceptance**

Atomic counters, producer/consumer queues, message passing, barriers, and TLS
tests pass. Multithreaded zstd and a real pthread blackscholes program produce
correct results at 1/2/4 threads under multiple scheduling seeds. Check forbidden
memory-order outcomes, lost updates, deadlocks, and thread cleanup.

**Regression and rollback**

A fixed scheduling seed reproduces results. A conservative functional memory
model may be an intermediate step if labeled explicitly; it does not establish
multicore timing accuracy. Independent emulator replicas are not evidence of
shared-memory threading support.

## M4 — Multicore timing and memory hierarchy

**Deliverables**

- Asynchronous accesses and completion events with correct visibility ordering.
- Cache coherence, request queues, bandwidth, and interconnect contention.
- Profile-specific cache hierarchy and topology, plus required frontend,
  dependency, retirement, and memory-ordering mechanisms.
- Model translation/TLB effects needed by selected workloads, or explicitly
  bound claims where those effects remain absent.

**Acceptance**

Test line ping-pong, false sharing, pointer chasing, bandwidth saturation, and
working sets crossing cache boundaries. Verify visibility, completion, protocol
invariants, and progress. Validate scaling trends at 1/2/4 cores as well as
absolute timings; extend to larger configurations only with additional evidence.

**Regression and rollback**

Compare architectural results with the functional reference. Preserve ordering
and protocol tests permanently. Keep the previous model as an explicit legacy
profile and version new timing models independently; failed validation must not
silently change the default profile.

## M5 — Broader runtime and practical simulation speed

**Deliverables**

- CPython scripts for JSON, text, files, and SQLite.
- Compilation of a small C project, adding required subprocess/pipe/wait support.
- Dynamic linking, signals, and other runtime features as required by selected
  programs, with explicit support coverage.
- SPEC CPU2017 starting with 505.mcf_r test input and an external installation.
- Functional and detailed simulation speed reporting; ROI and fast-forward with
  defined timing-state warm-up. Full checkpointing is a separate bounded task if
  needed, not something provided by serializing pipeline State alone.

**Acceptance**

Python results match references. Compiler output executes and passes tests.
SPEC output passes its validation tools. On tractable cases, cross-check ROI
execution and warm-up against a fully detailed run. Report test-input,
reference-input, and timing coverage separately.

**Regression and rollback**

Fast-forward does not change functional results. Checkpoint/resume, if provided,
must preserve all necessary emulator, memory, thread, event, and cache state.
Missing SPEC assets mean not run, never passed. Track simulation throughput and
memory use on controlled hosts; set measured regression budgets rather than
flaky wall-time gates on arbitrary CI hosts.

## M6 — CPU-only AI and repeatable agent workflows

**Deliverables**

- llama.cpp with a small fixed model, measuring prefill and decode separately.
- A Python workflow that reads tasks, processes JSON, queries stored data,
  invokes tools, and writes results.
- Deterministic network responses and modeled latency for external services.
- Complete functional runs, with selected detailed-timing regions for long jobs.

**Acceptance**

Fix model, inputs, seeds, and thread counts. Use declared numerical tolerances.
Verify tool arguments/results, output files, and workflow completion. Network
waits advance simulated time. Report CPU execution, synchronization, file, and
network waits separately.

**Regression and rollback**

Freeze model/data/response identities. Host or real remote-service elapsed time
must not become guest CPU time. Keep a deterministic small workflow in routine
regression and larger cases in extended validation.

## Hardware track — Independent per-profile milestones

Run this track alongside software development, starting at M0.

| Stage | Deliverable |
| --- | --- |
| H0 | Profile schema: exact chip/core type, ISA features, topology, parameter provenance, validation state |
| H1 | First measured Apple single-core profile using available hardware |
| H2 | Grace single-core profile to exercise genuinely different model structures |
| H3 | Heterogeneous cores, cache hierarchy, and multicore/fabric validation, alongside M4 |
| H4 | Repeat the process per Apple M1–M6 variant and additional NVIDIA ARM CPU |

Do not gate an available profile on acquiring every target machine. Every profile
requires instruction latency/throughput, branch, cache/memory, multiple-size,
and held-out workload tests before being called performance validated. Parameter
sources are published, measured, fitted, or assumed; keep those categories visible.

On macOS, native microbenchmarks calibrate Apple hardware while ARM Linux
VM/container runs provide same-ELF comparisons. Record these as different
measurement environments. Control/report host load, frequency behavior, core
type, VM scheduling, compiler, binary, and cache state. Measurements without
confirmed core identity do not validate a specific core type. VM application
wall time is not automatically a bare-metal CPU-cycle reference. A Darwin ABI
or Mach-O application runtime is not required by the initial roadmap.

**Accuracy contract:** use complete-program user CPU time initially, with per-case
error `abs(simulated-reference)/min(simulated,reference)` for finite positive
qualified samples. Version the formula and publish direction, every case,
calibration/held-out coverage and aggregates, outliers and measurement uncertainty.
Previous reference-denominator percentage targets do not carry over. No numeric
accuracy threshold is enabled or claimed achieved; establish valid hardware
references and agree thresholds before calibration. The historical 15.8% figure
uses a different denominator and does not validate this contract.

## Common release gates and rollback policy

- Every PR: build, vet, short tests, representative available applications, and
  Akita schema/lifecycle checks.
- Before merge: relevant complete tests; full functional regression for changes
  to execution, scheduling, or memory semantics. Direct commits should satisfy
  equivalent checks appropriate to their contents.
- Periodic and release validation: all accepted workloads, size/thread sweeps,
  hardware accuracy, simulation speed, and memory consumption.
- Functional bugs block promotion. Unvalidated profiles remain experimental.
  Never silently fall back to another profile or fabricate a missing reference.
- Changes to retained component expectations require evidence: independent
  references for functional fixes, and old/new comparisons with rationale for
  intentional timing changes. Retain useful failing regression cases; obsolete
  whole-kernel snapshots need not remain in the active tree. Do not retain a bug
  solely to keep an expectation unchanged.
- Regressions after merge: prefer reverting the narrow responsible change and
  restore the last accepted default configuration. Retain the failing case as a
  permanent test. Avoid irreversible fixture/config migrations in feature PRs.
- A milestone closes only with a reproducible acceptance report identifying the
  commit, artifacts, commands, passed/failed/skipped cases, and remaining limits.

## Ordering and immediate focus

M0 -> M1 -> M2 -> M3, then M4 and M5 can proceed in parallel. M6 depends on M3
and M5; credible multicore performance additionally depends on M4. The hardware
track starts at M0 and validates profiles independently.

The first implementation tranche is M0–M2: reproducible regression infrastructure,
complete Linux ELF execution, and BusyBox/zstd/SQLite. M0 discussion should settle
manifest/report schemas, reference sources, initial workload versions/builds,
CI tiers and budgets, and accuracy methodology. This roadmap itself does not
implement those decisions.

## ELF timing implementation contract (#47)

The next stage measures complete-program user CPU time, not mandatory kernel ROIs.
Functional and timing modes share process/ISA semantics; native child-user-time
records are retained and every PR emits a source-bound base/head accuracy report.
The formula is versioned diff/min (absolute difference divided by the smaller
positive time). Earlier percentage targets require a fresh decision under this
formula and are not enabled gates. VM samples and unknown-core measurements stay
unverified for bare-metal hardware accuracy. See evaluation/usertime/README.md.

## MVT and six-point report suite (#48)

MVT uses whole-program user time, superseding the issue’s original mandatory ROI
wording. All six MVT dimensions and six points each for integer, memory and branch
are required in the unified accuracy report. MVT first left the embedded registry
in #48; all 40 remaining kernels have now migrated to complete ELF execution
under #49. Schema-1 archived reports remain readable while the
versioned manifest defines the new 24-case suite. No hardware-accuracy promotion
or parameter fitting is implied.
