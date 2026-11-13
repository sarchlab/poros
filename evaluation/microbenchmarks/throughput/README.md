# Instruction throughput suite

These 17 probes are part of Poros's test suite. The same `loops.S` bodies run
natively on macOS/ARM64 and inside a static Linux ELF. `cases.def` is the shared
case inventory for both drivers and the measurement/report harness.

| Cases | Question |
|---|---|
| NOP, unroll 32/128/256/512 | Does throughput plateau as loop overhead falls? |
| ADD, 1/2/4/8/16 independent chains | Where do dependency and integer throughput limits separate? |
| FP ADD, 1/4/8/16 chains | How does scalar double-precision throughput scale with independent work? |
| Eight load or store destinations | What is throughput for a small repeatedly accessed working set? |
| Mixed integer, FP and loads, 4 or 16 FP chains | What changes when the loop uses different execution resources? |

All loop counts include `subs` and `b.ne`. The memory probes access 64 bytes;
initialization and warmup intend to keep this working set in L1, without claiming
that hardware cache residency was observed. Stores write 17..24 over initial
1..8, so the checksum catches a missing store. ADD/FP/mixed results are checked
against analytic sums. FP adds exactly representable 0.5 and remains far from
integer precision loss or overflow. Only caller-saved registers are used.

The NOP results constrain frontend throughput, not backend issue capacity.
Steady-state mixed throughput also cannot establish peak/burst issue width.
Actual core cycles, core identity and operating frequency are not observed;
3.49 GHz is only an explicit assumption for estimated native IPC. Poros eliminates
NOPs before execution while retaining frontend, ROB and retirement occupancy;
NOPs do not consume integer execution slots. The current default frontend admits
eight instructions per cycle; this is not a claim about peak backend issue width.

## Run and rebuild

```sh
sh evaluation/microbenchmarks/throughput/build.sh
go build -o /tmp/poros ./cmd/poros
python3 scripts/throughput_microbenchmark.py smoke --poros /tmp/poros
python3 scripts/throughput_microbenchmark.py collect
```

Rebuilding the committed ELF requires `aarch64-linux-musl-gcc` (GCC 14.2.0).
Native collection requires an Apple M2 and Apple clang. It uses thread CPU time
and user-interactive QoS, which does not guarantee a specific core. Every case
warms up first; each of nine trials executes adjacent short and long loops in
alternating order. Subtracting their times removes fixed harness overhead.
Each difference covers approximately 500 million instructions. The compact
reference retains source/executable identity, compiler/platform, iteration count,
and P10/median/P90 of the paired differences. P10/P90 select floor(p*(n-1)).
Negative or zero differences reject collection. Spread remains visible; there is
no assertion that a noisy result demonstrates a hardware width.

PR CI runs every case for 16 and 32 iterations, checking analytic checksums and
matching functional/OOO instruction counts. It does not collect native timing or
run the application accuracy campaign. A [local accuracy campaign](../../usertime/README.md#local-campaigns),
which no hosted job runs any more, additionally measures each probe at 1,024 and 2,048 iterations and
checks the expected instruction-count difference. Results appear only in the
microbenchmark section, excluded from the application S-curve/correlation.
Hardware collection is explicit and never silently replaces the reference in CI.
Native measurement inputs (`cases.def`, `loops.S`, `driver.h`, `native.c`) must
match the reference before reuse. Collector scripts and ELF-only wrappers/build
artifacts retain their historical hashes as provenance; changing those files
alone does not invalidate native timing. Every simulated execution separately
checks the current ELF digest, checksum, counts and simulator build identity.

The earlier two-loop frontend probe and its archived data remain readable for
historical reports; new reports measure and publish this suite in their place.
