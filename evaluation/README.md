# Evaluation contract

Poros separates complete-program functional correctness, simulator regression,
and hardware accuracy. Passing CI does not certify a hardware model. Use Go 1.27.0.

## Workloads and required coverage

All 41 existing compute kernels execute as static AArch64 Linux ELF programs.
Together with integer, memory and branch, the named CLI exposes 44 workloads.
`evaluation/workloads.json` records the 41 compute workloads plus 6 planned
applications. Planned applications never count as passed. The detailed
[complete ELF contract](../tests/benchmarks/README.md) and its manifest define
240 inputs for 40 newly migrated kernels. MVT retains its existing six-point
contract in `tests/usertime/manifest.json`; its source and data are unchanged.

Full Go regression requires 720 new complete executions (40 names × 6 points × 3
modes), 40 invalid-input groups, 3 MVT mode cases, 3 existing microbenchmark cases,
3 guest-libm mode cases and 1 multicore lifecycle case: **770 required checks**.
Short regression uses point 3 for every new name in all 3 modes and the same
remaining checks: **170 required checks**. All test-bearing packages must pass.
Loader/ABI faults, isolated ISA/cache/pipeline fixtures, prediction-disabled
execution and multicore accounting/lifecycle remain covered. The obsolete
embedded registry, initial-state fingerprints and 88 whole-kernel golden rows
are removed; they are not maintained as a second execution route.

Every PR also requires all 240 new inputs to run against QEMU using the identical
ELF bytes with normal exit 0, empty stderr and byte-identical complete output.
Native separately compiled C provides an additional compiler/platform oracle in
Go tests. Independent QR, Black-Scholes, Bodytrack, Facesim, Canneal and Raytrace
checks cover meaningful numerical/integer properties. Guest `log`/`exp`/`sqrt`
edge checks include subnormals, signed zero, overflow and domain errors. This is
bounded instruction/workload support, not full ARM FPCR/FPSR conformance.

Missing, duplicate, skipped or failed required cases, incomplete packages,
nonfinite outputs, malformed logs and stale ELF source bindings reject acceptance.
The required names and case counts are independent of observed test output.

## Running and provenance

```sh
scripts/evaluation-ci.sh static
scripts/evaluation-ci.sh short
scripts/evaluation-ci.sh full
python3 scripts/benchmark-differential.py --poros /path/to/poros --output /tmp/benchmark-differential.json
scripts/usertime-ci.sh origin/main
```

The capture sidecar binds the tested commit, source hashes, Go/platform, log hash
and process exit code. Imported acceptance requires the same clean committed
source tree. Without a matching sidecar, provenance is unknown and cannot pass.
Keep generated evidence outside tracked files or under ignored
`evaluation-results/`. The ELF build manifest separately hashes original C kernels,
wrappers, startup/runtime/linker scripts, ELF binaries and linked archive identities.
Compiler/linker versions are recorded. Do not refresh provenance without rebuilding.

## Accuracy remains a separate contract

Separate performance manifests define six larger inputs for each of all 44
workloads, while correctness manifests keep the original small cases. Native
Linux ARM64 captures use one warmup and 21 user/system CPU samples per point.
Only derived plotting statistics and exact input/output identities are committed;
raw observations and full witnesses remain local. Each PR merge into `main`
reports all 264 points, including invalid/noisy references. Head functional and
OoO executions must validate every output; base OoO is required wherever the
exact base image and output limits support the input. Unsupported base cases
remain explicit and cannot contribute a paired error change. The application summary pools all qualified cases across the 41
compute workloads. Any workload may guide model tuning. Integer, memory and
branch appear in the separate microbenchmark section.

[Whole-program user-time evaluation](usertime/README.md) defines direct-child
`wait4.ru_utime` measurements on Linux ARM64 in an Apple M2 VM, output validation,
quality policy and publication. Simulator elapsed time and QEMU elapsed time are
never hardware references. Unknown core identity/frequency and VM effects keep
hardware accuracy unverified. Unusable points stay visible but are excluded from
error summaries. Raw measurements are never rescaled or offset.

## Modeled devices

`evaluation/devices.json` declares every device the simulator models: `m2`
(default), `m1`, `grace` and `m5-max`. The simulator calls a device a target
(`poros elf --target m1`); the evaluation scripts and workflows say **device**,
because their `target` is a Git revision. A Go test keeps the declaration equal
to the targets the simulator builds. Each entry states three things, and every
report repeats them:

- **hardware**: what the device stands for, for example `Apple M1`.
- **parameter_source**: the device whose model profile is simulated. Only `m2`
  has parameters of its own. The other three name `m2`: they run the M2 core,
  memory and frequency until their own are established, and are labeled as
  placeholders wherever they appear.
- **references**: the native reference file of each kind (whole-program user
  time and each diagnostic suite). Only `m2` has any.

Every device runs the complete campaign: all 264 programs functionally and
timed, and every diagnostic point, with every output checked. What differs is
what the result is compared with:

- A native reference is used only for the device whose `hardware` it records.
  An M2 file cannot be attached to, shown with or compared against another
  device, in a report, a live snapshot or a bundle.
- A device without a reference of some kind gets predictions for that kind:
  time, cycles, instructions and cycles per instruction. Its report has no
  reference time, error, direction, correlation or mean, and none is displayed
  as zero or as a placeholder. Program outputs are still checked against the
  committed complete-output identities, which are what each program must print
  on any device. The report names that file as its output oracle. The file is
  the M2 reference and is validated as a whole; none of its timings is shown
  or compared.
- The base revision is compared only if it models the device. A base that
  predates device declarations is `m2` and is run without a target, exactly as
  before. For any other device its predictions are absent and each case says
  `device_unavailable`.

Select a device with `--device` on `accuracy_parallel.py prepare`,
`usertime.py compare` or as the third argument of `scripts/usertime-ci.sh`;
later steps read it from the prepared plan. Omitting it means `m2`, and a source
revision without `devices.json` models only `m2`. An unknown name is an error.
Plans, shards, diagnostic checkpoints and reports all carry the device, so
evidence of one device is rejected by another's campaign even while two devices
share a binary and a profile.

```sh
python3 scripts/devices.py list
go build -o /tmp/poros ./cmd/poros
python3 scripts/devices.py smoke --poros /tmp/poros   # every device, real simulator
```

The smoke runs in pull-request CI. Full campaigns for a device other than `m2`
run on request: the `accuracy_device` input of the Evaluation workflow produces
the evidence bundle, and the `device` input of "Publish accuracy reports"
evaluates and publishes it. Merges to `main` continue to run and publish `m2`
only.

A commit has one published report, at `calibration/runs/<commit>/`, and the
report history has one row per commit. The report opens on a commit overview
and holds one view per device that commit declares, at
`calibration/runs/<commit>/#device-<id>`. The devices are the ones in that
commit's own `evaluation/devices.json`, a copy of which is retained beside the
report; a commit that predates the file is `m2` alone. A device that has not
published is shown as pending, so a commit that declares four devices and has
published `m2` is `partial`, and a later publication of another device joins
the same report. The `m2` bundle stays directly in `calibration/runs/<commit>/`
and each other device's is in `calibration/runs/<commit>/devices/<id>/`.

The commit overview pools observations instead of averaging devices: every
qualified application input of every device that has its own native reference
is one observation, and the error, correlation and ordering statistics and the
three overview charts are computed over all of them together. Devices without a
native application reference are listed with their predictions and add nothing
to those statistics. The exact layout and rules are in
[the report contract](../scripts/accuracy-live.md#one-report-per-commit).

Giving a device its own parameters needs a declared model profile for it, which
the scripts do not resolve yet: they refuse a device whose `parameter_source`
is not `m2` rather than validate it against the M2 profile. Giving a device a
measured whole-program reference needs a file at the path its entry names under
`references.application`, with that device's `hardware` and case identities
that match the committed inventories. The capture-provenance checks of that
file are still the ones written for the M2 Mach-O captures, and each diagnostic
suite still reads the single M2 reference file, so a second device's suite
references need those validators extended first.

## Memory diagnostics

The [single-core memory suite](microbenchmarks/memory/README.md) adds random
pointer chasing (1/2/4/8/16 independent chains) and scalar Read/Write/Copy
bandwidth over 140 points: 22 chase working sets × five chain counts, plus ten
streaming working sets × three patterns. Native Mach-O and simulated ELF
use byte-matched computation functions and a warmed long-minus-short protocol.
Small correctness/QEMU checks run in ordinary PR CI; the existing post-merge
accuracy workers execute the full suite, retaining exact base/head identities.
These results have their own plots and collapsed table and never change the
application error or correlation population. The initial suite changes no
memory-model mechanism or parameter.
