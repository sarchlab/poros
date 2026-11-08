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
