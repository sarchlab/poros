# Compare/select and memory-carried recurrence probes

These 36 probes separate three questions raised by the Nussinov inner loop,
whose only loop-carried dependency is `str` → same-address `ldr` → `cmp` →
`csel`: whether an adjacent compare/select pair costs one or two dependency
steps, what a same-address GPR store→load step costs in different program
shapes, and whether the stored value matters. The simulated ELF, the native
macOS executable and the native Linux executable assemble the same bodies; an
instruction audit requires every function to be word-identical in all three at
the same offset within a 64-byte line.

**Status: experimental diagnostics.** No native reference is committed yet and
the suite is not part of the accuracy report. The probes establish what the
model predicts and what must be measured; they do not calibrate anything.

Dense loops perform 64 updates per outer trip, tight loops run 64
single-update trips, and kernel replicas run 1,024 inner trips. `cases.def`
records instructions and reported updates per outer trip. `fwd_mid` and
`fwd_far` advance two and three chains in parallel and report the 32 updates
of one chain.

| Probes | Body and control |
| --- | --- |
| `clock_add`, `clock_fadd` | One-cycle integer and three-cycle FP register chains |
| `sel32_rr_adj`/`_sep`, `sel64_rr_adj`/`_sep` | Running maximum `cmp`/`csel` on W or X registers. Every update has the same four instructions; the separated form moves the independent accumulator between compare and select |
| `sel32_rr_adj_p8`/`_sep_p8`, `sel32_rr_adj_p4` | The W forms shifted 8 bytes (a second layout with no straddle) and the adjacent form shifted 4 bytes, where every fourth pair straddles a 64-byte line |
| `sel32_ri_adj`/`_sep`, `sel32_ri16_adj`, `sel32_shift_adj` | Compare with immediate 15, immediate 16, and `#1, lsl #12`; the selection alternates between both outcomes |
| `sel32_csinc_adj` | `cmp` followed by `csinc` instead of `csel` |
| `thr32_mix_adj`, `thr32_mix_sep` | Issue throughput, not latency: 64 independent units of one compare, one select and two adds. At eight instructions per cycle a unit needs four integer operations unless the adjacent pair issues as one; both orders have the same instructions, count and loop size |
| `mem32_max_const`, `mem32_max_chg`, `mem32_max_const_sep` | The maximum is loaded from and stored to one address. The stored value is constant, changes every update, or the pair is separated; one instruction separates each store from the next load |
| `mem64_add`, `mem64_add_alias`, `mem64_add_disp`, `mem64_partial` | Load, increment, store: same base register, a second base register holding the same address, displacement 8, and a 64-bit store read by a 32-bit load |
| `mem64_independent`, `mem_fadd` | The same instruction classes without a memory dependency; an FP load/`fadd`/store chain |
| `fwd_near`, `fwd_mid`, `fwd_far` | Load, two increments, store. Reload immediately after the store, or four and eight instructions later with two and three interleaved chains |
| `mem64_addr_far` | The recurrence runs through the load address while the stored data is ready early |
| `tight_add_mem`, `tight_max_mem`, `tight_max_reg` | One static store/load pair per taken branch, and its register-carried control |
| `nuss_const`, `nuss_chg`, `nuss_sep`, `nuss_reg` | The eleven-instruction Nussinov loop on L1-resident data at the application's line offset: constant maximum, changing maximum, index increment moved between `cmp` and `csel`, and register-carried maximum with an unrelated third load |

Compared forms keep the same instruction multiset, dynamic count, loop size and
branch position; only order, width, operand form, address form or data differ.
Results are consumed by the next update and by the returned value, so nothing
can be removed. The analytic outputs, with `T = 64 N` for `N` outer trips, are:

| Kind | Output |
| --- | --- |
| `count64`, `count128`, `sum64` | `T + 1`, `2T + 1`, `T` |
| `half` | `32N + 1` |
| `select_max` | `T(T+1)/2 + T + 1` |
| `select_low`, `select_shift` | `50T + T²/4`, `2500T + T²/4` |
| `select_inc` | `T(T+1)/2 + T/2 + T + 1` |
| `mix` | `6N − 3` |
| `max_const`, `max_chg` | `T + 5`, `5T + T(T+1)/2 + 5 + T` |
| `mid`, `far`, `trips` | `2(T + 1)`, `3(T + 1)`, `N` |
| `tight_max` | `T + 5` |
| `kernel_const`, `kernel_chg` | `4096N`, `1024N` |

The trip-count bound of 8,388,608 keeps every 32-bit value below 2^31.

## Native protocol

`native.c` runs the whole campaign in one process and prints only after the
last interval. It reads `CLOCK_THREAD_CPUTIME_ID` and a raw monotonic clock
around each assembly call; initialization, accounting and bookkeeping are
outside both. On Linux the thread is pinned to the CPU named on the command
line; macOS has no pinning and requests user-interactive QoS.

Before and after each interval the collector also reads hardware counters for
the measured thread when the system provides them: the kernel's per-process
accounting on macOS (`proc_pid_rusage`, one thread) or a user-mode `perf_event`
cycle/instruction group on Linux. If neither is available the capture says so.

1. One warmup call per probe (retained, never analysed), then a duration
   pilot at 1:2:4 trip counts of about 16 million instructions. The fastest
   observed thread-CPU rate selects `N` so that an `N` interval lasts about the
   target (24 ms by default) or longer. The choice depends on duration only.
2. Each block visits `clock_add` first and last and every other probe once
   between. A visit times `N, 2N, 2N, N` on even blocks and `2N, N, N, 2N` on
   odd blocks. Blocks `2j` and `2j+1` use mirrored orders; pair `j` starts five
   probes later and steps by the next stride coprime with the probe count.
3. The observation is `(mean c(2N) − mean c(N)) / (N · updates)`. With a
   hardware counter, `c` is counted cycles: cycles per update with no frequency
   assumption, and counted instructions must equal the declared loop plus a
   small harness constant. Without one, cycles per update is the ratio of the
   thread-CPU difference to the same block's `clock_add`: an inference from a
   one-cycle ADD chain, not a count. The reference names which scale it used;
   the time difference and the clock ratio are always kept beside it.
4. The reference describes the performance core. A visit that left it (any
   efficiency-core cycle on macOS, any departure from the pinned CPU on Linux)
   is tallied in `excluded_blocks` and left out of the percentiles; the
   all-visit median stays beside them. Nothing else is dropped or retried.
5. A block starts only if another fits before the deadline. An interval above
   0.5 s stops the campaign. Every completed mirrored block pair enters the
   summary and the raw capture is retained. Linux frequency reports at block
   boundaries are operating-system observations, not measurements.

`scripts/recurrence_microbenchmark.py` restates the schedule and trip-count
rule independently and rejects a capture that differs, checks every output
against the table above, and reports P10/median/P90 with the repository's
nearest-lower-rank convention. Relative spread is `(P90 − P10)/median`. A row
is `invalid` without a performance-core visit or with a nonpositive
difference, and `noisy` above 20% spread or with more than a quarter of its
visits excluded. A row below the threshold passes only that screen.

Build the simulated ELF with `sh evaluation/microbenchmarks/recurrence/build.sh`
and run functional and timing smoke checks with
`python3 scripts/recurrence_microbenchmark.py smoke --poros /path/to/poros`.
`native-check --directory DIR` builds the macOS collector and runs every probe
plus the complete schedule with a synthetic clock, so it measures nothing; the
same `dry` run prints identical bytes on every host. `build-linux --directory
DIR` cross-builds the static Linux collector with 16 KiB-compatible segments.
`collect` captures on a quiet local Apple M2, and `summarize` turns a capture
taken elsewhere into a reference. A native Linux capture is its own cohort in
`m2-linux.json`; it replaces no macOS reference.

Simulation differences use 128 and 256 outer trips, check analytic output and
exact retired instruction counts, and require the declared source revision and
model profile. These probes belong only to the microbenchmark section; they
never enter application S-curves, correlations, or error aggregates.
