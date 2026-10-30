# Evaluation contract

Poros separates behavior regression, independent functional correctness and
hardware accuracy. Passing CI does not certify a hardware model. Use Go 1.27.0.

## Current workload interfaces

`evaluation/workloads.json` inventories 40 remaining embedded kernels, MVT as a
complete Linux ELF, and planned applications. The 40 legacy kernels retain 80
one/two-independent-emulator baseline cases plus eight pipeline variants. These
are not shared-memory multicore tests. MVT's two legacy rows and initial-state
fingerprint were removed only after replacement full-program coverage passed;
no other fixture row was regenerated. Useful ISA, memory and pipeline unit tests
remain. MVT's old function extractor, per-program Go initialization, ROI driver
and superseded measurement file have been removed.

The active ELF timing suite is `tests/usertime/manifest.json`: four six-point
sweeps, 24 cases. MVT dimensions are 8, 63, 120, 129, 400 and 1536. Integer and
branch vary iterations; memory varies working-set bytes at a fixed access count.
Guest initialization, computation and output are all executed from ELF entry.
The common loader/runtime supplies no workload-specific hooks or register setup.
See [the complete user-time contract](usertime/README.md) for raw measurements,
independent output validation, quality policy, report generation and publication.

The remaining embedded inventory keeps its declared calibration/validation roles.
Legacy workloads already informed model development, so they are not independent
historical holdouts. New ELF suite roles are frozen before measurement or tuning.
A planned or skipped workload never counts as passed.

## Running and provenance

```sh
scripts/evaluation-ci.sh static
scripts/evaluation-ci.sh short
scripts/evaluation-ci.sh full
scripts/usertime-ci.sh origin/main
# Re-evaluate a captured uncached regression run:
go run ./cmd/evaluate regression -capture evaluation-results/capture.json -tier full -log evaluation-results/go-test.jsonl -out evaluation-results/report.json
```

Full legacy regression requires all 88 named fixture cases and successful
completion of every test-bearing package. Short regression retains the eight
raytrace/blackscholes cases. Missing, skipped or failed required tests, missing
assets, malformed/truncated logs and invalid manifests fail acceptance. The
capture sidecar binds commit, tracked source hashes, Go/platform, test-log hash
and process exit code; imports must match the current clean committed tree.
Without a sidecar, imported results have unknown provenance and cannot pass
acceptance. Keep generated evidence outside tracked files or in ignored
`evaluation-results/`.

Every PR and main push also runs all 24 ELF cases in head functional mode and
base/head OoO timing modes, validates their complete outputs and produces a
source-bound user-time report. Missing cases cannot pass. Generic loader/ABI
negative tests remain, and short tests exercise MVT output and malformed inputs
in addition to the complete accuracy job. Reports distinguish functional success,
unchanged timing behavior and usable hardware comparisons.

No CPU parameter, accuracy threshold or timing formula is changed by adding
workloads. Native references use direct-child whole-program `wait4.ru_utime` on
Linux ARM64 in an Apple M2 VM; host simulator elapsed time and QEMU elapsed time
are never accuracy references. Unknown core identity, frequency and VM effects
keep hardware accuracy unverified. Unusable points remain visible but are
excluded from error summaries. Raw observations are never rescaled or offset.

## Portable hook evidence

The old digest was captured with Go1.27 on darwin/arm64. Go math.Exp and math.Log
use architecture-specific implementations, so byte hashes of host-hook results
are not portable. Linux AMD64 initially failed four Black-Scholes baseline cases
with identical instruction/cycle counts. A diagnostic translation of Go's AMD64
Exp algorithm changed65/1024 prices by at most1.25e-14 on the ARM host while
retaining106565 instructions; this isolates an actual rounding sensitivity, not
a guessed new Linux golden. Black-Scholes and Bodytrack are the explicit allowlist for the alternative
non-ARM64 check. Except for the separately corrected Facesim rows, fixtures retain their original
values. Independent numerical checks
cover all1024 option outputs; exact same-host whole-state digest equality links
those functional outputs to timing-mode executions. Other workloads keep frozen
digests unless separately audited. Each case report records which digest mode
was actually exercised, using the test's explicit evidence marker.

When a regression case uses a non-ARM64 digest substitute, the report requires
`TestRegistryBlackScholesNumericalReference` or
`TestRegistryBodytrackNumericalReference`, respectively, to pass and
records it in `independent_checks`, separately from the 88 legacy cases. Missing,
skipped, or failed numerical checks fail coverage. This reference is independent
of the simulated instruction stream, but shares the host math implementation.

The Linux AMD64 full run exposed host-dependent fusion in the legacy binary64
FMADD/FMSUB/FNMADD emulator implementation: Go contracted arithmetic on ARM64
but emitted separate multiply/add on AMD64. These three double-precision paths
now explicitly use single-rounded math.FMA with ARM operand signs. Exact tests
cover cancellation, intermediate overflow and signed zero; this arithmetic repair
did not change fixtures (the separate Facesim input correction did). This narrow semantic repair does not implement full FPCR/FPSR,
NaN payload handling or single-precision FMA portability. Those remain limitations,
so passing legacy kernels is not a claim of full ARM floating-point conformance.

Bodytrack was separately audited after canonicalizing its inputs: translating
Go's AMD64 Exp path changed 7 of 64 hook results and the final state digest on
identical inputs. Its companion checks all 64 likelihoods against the mathematical
projection/squared-error/Exp formula using actual registry inputs, with relative
error at most 1e-10 and finite checks. Swaptions retains its historical digest.
Facesim uses the corrected finite M0 digest and has no host-math exception. These are bounded workload
checks, not a claim that host math is correctly rounded or universally portable.

Registry initialization now specifies FMA grouping at 32 compiler-proven source
lines. Thirty-nine remaining kernels preserve their original ARM64 input bytes; Facesim uses
the corrected finite input. The fast
`TestInitialStateIdentity` fixture hashes initial register JSON followed by declared
data-region bytes in registry order, before any execution. It defines input
identity, not correct outputs; the original rows were captured on clean pre-change
commit 9269bca, then only the invalid Facesim input identity was replaced.
This prevents host compiler contraction from silently changing the workload itself.

Facesim's old fixture contained degenerate tetrahedra: 16/32 log arguments were
zero and 54/60 forces were NaN. Its replacement uses 32 fixed nondegenerate
four-vertex tuples covering all 20 vertices. Degenerate rest geometry now fails
setup. The independent registry test verifies rest inverses, positive finite
deformation determinants, all 60 finite forces, force balance and finite-difference
energy gradients. The separate emulator test also rejects nonfinite values.
Only Facesim's initial-state fingerprint and two output digests changed; its
8166 instructions and 5392 cycles per core remain unchanged. This corrects an
invalid workload, not NaN payload portability or general ARM floating-point rules.

Whole-program user CPU measurement and mandatory PR base/head reports are defined
in [usertime/README.md](usertime/README.md). Old accuracy thresholds are not enabled
for the new symmetric formula.
