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
CPU Spec and cache configuration are reported. Cache state starts cold; user
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
