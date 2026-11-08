# Execution-unit scheduling probes

These four scalar assembly probes compare a joined dependency graph with and
without independent work. Each outer iteration repeats eight blocks:

1. Reset the independent accumulator to zero.
2. Run eight dependent MADD updates (multiplier/increment one), or eight FMADD
   updates (multiplier one, increment 0.5).
3. Fan out the main result to six integer or four FP destination registers.
4. In interleaved cases only, run eight ADD or FADD updates on the independent
   accumulator.
5. Add that accumulator into the main result, joining the paths before the next
   block can proceed.

This order presents independent work after instructions whose operands become
ready in the future. The final join prevents the independent chain simply
lagging behind and overlapping a later block, which would hide a reservation
error in steady-state throughput. Controls execute the same reset/fan-out/join
without the independent updates. Each iteration has 64 long-chain updates.
Instruction counts including loop control are 130/194 for integer and 114/178
for FP control/interleaved cases.

The primary comparison is time per outer iteration, retaining the same long
chain while adding independent work. Time per instruction is also shown but
cannot by itself isolate overlap because instruction counts differ. Checksums
consume all final fan-out registers and the independent accumulator, and the
joined main chain retains earlier independent updates. The analytic outputs are
384*N+12 / 768*N-28 (integer) and 128*N+6 / 256*N-6 (FP). Values remain exactly
representable over the declared native and simulated iteration ranges. Assembly
prevents compiler dead-code elimination. These chosen operand patterns are not
measurements of arbitrary operand latency or proof of a physical M2 scheduler
structure.

`loops.S` supplies identical function bodies to the native Mach-O and Linux ELF
builds. The entry, argument parsing and output wrappers differ. The native driver
uses thread CPU time and nine alternating long/short pairs; each recorded timing
subtracts an N-iteration execution from a 2N-iteration execution. Warmup occurs
before sampling. Core identity and instantaneous frequency are unobserved. Model
cycles are converted with each revision's declared frequency, not a measured
native clock rate. Native P10/P90 describe variability, not confidence bounds.

The native reference binds these measurement sources separately from historical
collector provenance. Its compact plotting statistics are committed; raw paired
measurements and binary/disassembly audits are retained with the collection
experiment. Existing throughput and forwarding references are unchanged.

Build the ELF with `sh evaluation/microbenchmarks/scheduling/build.sh` using the
same AArch64 musl toolchain as the existing probes. Run functional/timing smoke
checks with `python3 scripts/scheduling_microbenchmark.py smoke --poros PATH`.
Native collection on Apple M2 uses `collect --evidence-dir PATH`; collect only
when local simulation/build work is quiescent. The normal ELF CI job runs smoke
checks. Accuracy aggregation runs base/head probes, validates both results and
renders a separate microbenchmark section through the common report renderer.
These probes never enter application S-curves, correlations or error summaries.
