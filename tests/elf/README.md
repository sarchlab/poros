# Required ELF execution fixtures

`manifest.json` records the source/build inputs, GCC 14.2.0 / GNU ld 2.44 toolchain,
ELF hashes, deterministic argv/env, expected bytes and termination policy. These
are real static non-PIE AArch64 GNU-produced executables with RX/R/RW load segments
and NX stack. Sources are freestanding C and assembly; no libc or program hooks.

```sh
CROSS_COMPILE=/opt/homebrew/opt/musl-cross/bin/aarch64-linux-musl- tests/elf/build.sh
# Alternatively set CROSS_COMPILE to the same toolchain's installation prefix.
go test ./pkg/loader ./pkg/linuxuser ./pkg/emu ./cmd/poros
python3 scripts/elf-differential_test.py
go build -o /tmp/poros ./cmd/poros
python3 scripts/elf-differential.py --poros /tmp/poros --output /tmp/elf-report.json
# On native Linux ARM64, also use --reference native.
```

Normal tests and CI consume checked-in ELFs, require all source/binary hashes and
never require a crosscompiler or skip missing fixtures. Rebuilding requires the
recorded toolchain; changing sources requires deliberately updating provenance
and independent expectations. The Go and Python required inventories are separate
from the manifest so deleting a manifest case cannot create a false green gate.
Harness self-tests reject missing, skipped, failed and mismatched references.

The 14 required cases cover hello/stdout/stderr; argc/argv/envp/auxv and stack
alignment; initialized data plus zeroed/writable BSS; integer and binary64 results;
zero-length/invalid-fd/invalid-pointer/fd-truncation syscall behavior; unmapped
load, RX store and NX stack fetch; unsupported syscall and instruction; bounded
infinite loop; output budget; exit_group's low-eight-bit status; and arithmetic
loop execution. Go tests additionally exercise partially mapped pair loads/stores,
pre/post-index writeback rollback, successful pair commit, real RET semantics,
context cancellation, exact instruction budgets and repeated-run rejection.

`compute.elf` returns integer 96, exact doubles 3.25 and 0.875, a genuine FMA
single-rounding edge `(1+2^-27)*(1-2^-27)-1` (bits `bc90000000000000`, -2^-54),
and inexact division 1/10. The Python oracle checks finite values, independent
integer/bit-pattern expectations and explicit `1e-15` absolute / `1e-14` relative
tolerance for the nonexact value; both machines' raw bytes are also recorded. Integer/FMA-edge bits must match
exactly; tolerance fields are not preempted by whole-buffer byte equality.
Negative oracle tests reject NaN/Inf/out-of-tolerance values and accept a one-ULP
perturbation within tolerance.

Differential reports record every case's ELF hash, exact inputs, both outputs as
base64, actual process status/termination, Poros completed instruction count and
reference count `null` (not collected). Success cases must match the independent
expected output and status on both backends. Fault cases expect SIGSEGV/SIGILL in
the reference. Unsupported-syscall, instruction-limit and output-limit cases have
explicit different simulator/reference policies; those are not falsely labeled
identical semantics. QEMU's own signal diagnostics are retained in reference
stderr but not compared as guest output. Both native Linux ARM64 and QEMU 8.2.2
pass the inventory locally; CI requires QEMU user and fails on missing tools.

See [the execution profile](../../pkg/linuxuser/README.md) for intentional syscall,
ISA and FP-state limits, and [the backend decision](backend-decision.md) for the
measured interpreter/QEMU comparison. These are functional tests, not timing
accuracy or broad compiler/ISA conformance claims.
