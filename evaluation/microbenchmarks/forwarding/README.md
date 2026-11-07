# Store/load dependency probes

These 13 probes measure effective loop-carried dependency costs on the host M2.
Each outer iteration contains 64 updates. ELF and native macOS execute the same
assembly bodies; only their process startup and output harnesses differ.

| Probes | Update and control |
| --- | --- |
| `gpr64_register`, `gpr64_same` | Increment a 64-bit register; or load, increment, store the same address |
| `gpr64_disjoint` | Load a fixed source, add a changing independent counter, store elsewhere; throughput control |
| `gpr32_register`, `gpr32_same` | Corresponding 32-bit increment chains |
| `gpr64_indexed` | Same-address 64-bit chain using register-offset addressing with zero offset |
| `gpr64_indexed_load`, `gpr64_indexed_store` | Register-offset addressing on just the load or just the store |
| `select_register`, `select_same` | Increment then compare/select to wrap modulo 65,536; register or load/store chain |
| `fp_register`, `fp_same` | Add exactly 0.5 to a double; register or same-address load/store chain |
| `fp_disjoint` | Fixed double load plus changing independent double counter, stored elsewhere; throughput control |

All memory addresses fit within one aligned 64-byte line, warmed before timing.
The native dependency chains use zero displacement with a general-register base;
register-offset controls likewise use a zero-valued index. Nonzero scaled
immediate offsets and SP bases are allowed by the model as an unmeasured
extension of the same immediate-offset instruction class. These probes do not
establish those individual forms. The disjoint throughput controls use a separate
store address and are not dependency-latency evidence for nonzero displacement.
The same-address stores contain changing values. The select pair exercises both
select outcomes in long runs. It resembles the load/compare/select/store shape
in Nussinov but uses an explicit increment and wrap recurrence; it is not the
application's algorithm or a substitute for its accuracy test. Disjoint probes
have different dependency graphs and are not latency-subtraction baselines.

Native collection uses `CLOCK_THREAD_CPUTIME_ID`, user-interactive QoS and nine
pairs of n and 2n iterations, alternating their order. Each difference covers
approximately 500 million instructions. The report retains median, P10 and P90
of the nine positive differences, using nearest lower ranks (0 and 7). The
primary unit is ns/update, and spread is `(P90-P10)/median`. No core identity or
instantaneous frequency was observed. Any cycles inferred at 3.49 GHz are
estimates, not measured clock cycles.

Subtracting a class-matched register-chain median from its memory-chain median
estimates an effective incremental cost including instruction/address overhead.
It does not identify a physical forwarding pipeline, prove memory renaming, or
justify applying a timing parameter to other address/width/register classes.
The distribution endpoints are variability summaries, not confidence intervals.

`m2.json` binds the measurement sources, compiler, binary digest, timestamp and
protocol. Existing application references and the separate instruction
throughput reference are retained unchanged. Only compact plotting summaries
are committed; transient stdout and test logs remain outside the repository.

Build the ELF with `bash evaluation/microbenchmarks/forwarding/build.sh`.
Run functional/simulator smoke checks with
`python3 scripts/forwarding_microbenchmark.py smoke --poros /path/to/poros`.
On the intended quiet M2 host, explicitly collect a new reference with
`python3 scripts/forwarding_microbenchmark.py collect --audit-directory /tmp/new-probe-audit`.
The audit directory must not exist and retains all nine paired outputs locally.

Simulation differences use 1,024 and 2,048 iterations, check analytical output
and exact retired instruction counts, and require the declared source revision
and model profile. These probes belong only to the microbenchmark section;
they never enter application S-curves, correlations, or error aggregates.
