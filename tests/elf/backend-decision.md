# Functional backend decision for #46

Keep the existing interpreter for Poros execution and use QEMU user as an
independent functional reference. QEMU is much faster on the measured loop, but
replacing the backend now would also require a new instruction/memory event
integration with the timing model. This PR does not perform that replacement or
claim timing accuracy for the new ELF path.

The [raw measurement](backend-measurement.json) captures five runs per row,
commands, tool version, executable/source/fixture hashes, argv/env and completed
Poros instructions. It was captured on the implementation tree before commit;
its recorded dirty state is intentional, not a claim of final-commit validation.
Final acceptance reports are generated separately from the clean reviewed head.
Poros was cross-compiled to Linux ARM64 and both executors ran in the **same**
Linux ARM64 Docker environment on the Apple M2 host. Wall time includes process
startup, JSON serialization for Poros, and host scheduling. These measurements
are simulator engineering costs, not native M2 performance calibration.

Median host wall time (milliseconds):

| Program / loop iterations | Poros | QEMU 8.2.2 | QEMU TB trace | Poros instructions |
|---|---:|---:|---:|---:|
| hello | 4.317 | 3.208 | 3.158 | 51 |
| arithmetic / 100 | 5.622 | 3.019 | 3.166 | 659 |
| arithmetic / 10,000 | 26.287 | 3.248 | 6.498 | 60,079 |
| arithmetic / 100,000 | 203.339 | 3.410 | 38.558 | 600,089 |

The short runs are dominated by startup/noise; a trace row slightly below an
untraced row is not evidence that tracing improves performance. The size sweep
exposes interpreter work separately from this startup floor. QEMU translation
amortizes loop execution, while the interpreter decodes and executes each guest
instruction. Transactional checked steps currently copy the architectural state;
that cost is included. Escape analysis also confirms that the fetch buffer,
transactional register copy and local checked executor escape to the host heap
per checked step. Reusing a stepper may reduce that overhead in later timing work;
this PR preserves the small explicit interface and records the cost rather than
rewriting execution architecture. This does not imply guest pages autoallocate,
and the legacy persistent executor does not incur those per-step allocations.
This is one small kernel, not a general performance claim.

The trace column uses `-d exec,nochain -D /dev/null`, so formatting and trace
hooks run without retaining a log. QEMU's `exec` output describes translation
blocks, not a ready-to-use per-instruction dependency/memory trace. It is useful
to illustrate instrumentation cost, not an estimate of a complete timing adapter.
Reference instruction counts are explicitly `null`; no count was collected.

| Need | Existing interpreter | QEMU user reference |
|---|---|---|
| Functional oracle | 14 real ELF cases; numerical checks independent of interpreter | Same ELF/input inventory on QEMU and native Linux ARM64 |
| ISA coverage | Limited implemented scalar instruction subset, explicit unsupported failure | Much broader AArch64/Linux implementation |
| Instruction/dependency visibility | Decoded instruction and branch/SVC event available; existing timing code already consumes decoded operands | Requires an integration via instrumentation or plugins; TB logging is insufficient |
| Memory visibility | Checked per-instruction memory boundary; legacy typed accesses preserved | Instruction/memory callbacks are possible, but no Poros timing adapter exists |
| Timing fidelity | Existing 90 kernel regressions retained; ELF timing integration is #47 | Fast functional execution is not a cycle model |

`emu.StepChecked` deliberately reuses ISA semantics and exposes an explicit event
boundary. `linuxuser` owns process lifetime, checked mappings, syscall policy and
budgets. The legacy timing harness still uses its original typed access grouping for
previously working forms. Single-Q and literal loads now have corrected results
and explicit access-stream regression tests.
An independent old/current MVT/GEMM probe found identical instruction counts and
no per-memory-instruction allocation increase after moving transfer scratch space
into the executor; noisy five-run timings showed no obvious slowdown. The final
90-case evaluation remains the regression gate.

The captured write profile validates the complete guest buffer before emitting
bytes. A coordinator probe found native Linux can partially write a 4096-byte
valid prefix of an 8192-byte fault-crossing buffer, while QEMU 8.2.2 returned
EFAULT with no bytes. Poros intentionally follows atomic captured-buffer behavior
in this phase; pipes, signal interruption and partial host writes are not modeled.
Unsupported syscalls fail explicitly in Poros even when the reference supports
them. These policy differences are named in the test inventory rather than
misrepresented as differential equality.

Reproduce in one Linux environment with Go-built Poros and QEMU user installed:

```sh
python3 scripts/elf-backend-measure.py --poros /path/to/poros \
  --output /tmp/elf-backends.json
```

References: [QEMU user-mode emulation](https://www.qemu.org/docs/master/user/main.html),
[QEMU TCG plugins](https://www.qemu.org/docs/master/devel/tcg-plugins.html).
Plugin APIs are version-dependent; these links describe the design space, not a
claim that every current API is available in the measured QEMU 8.2.2 build.
