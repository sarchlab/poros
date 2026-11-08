# Single-core memory diagnostics

This suite measures random dependent accesses, available memory parallelism,
and scalar streaming throughput. It changes no simulator timing parameter or
memory mechanism. See [the umbrella plan](plan.md) for the later model work.

The refreshed matrix has 140 points. Streaming Read/Write/Copy retain ten total
working sets: 8, 32, 64, 128, and 256 KiB; 1, 8, 16, 32, and 64 MiB.
Each of the five chase patterns also includes 96, 160, and 192 KiB;
12, 14, 18, 20, 24, 48, 96, 128, and 256 MiB. These are diagnostic coordinates, not claims
about the capacities of physical M2 cache levels.

| Pattern | Work per complete pass | Primary plotted metric |
| --- | --- | --- |
| Random chase, 1 chain | One pointer load per 128-byte node | Dependent-load loop ns/load |
| Random chase, 2/4/8/16 chains | Same total node count, divided among independent cycles | Aggregate ns/load |
| Read | Eight scalar 64-bit loads and independent sums per block | Effective GB/s |
| Write | Eight scalar 64-bit stores per block | Effective GB/s |
| Copy | Eight scalar loads and eight scalar stores per block | Effective GB/s |

## Inputs and computation identity

A fixed-seed xorshift64 generator drives Fisher–Yates permutation. Each chase
node occupies 128 bytes, with one pointer in its first eight bytes. Split the
permutation into equal disjoint cycles for the selected chain count. Every
complete pass visits every node exactly once across the chains. Total residency
stays constant across chain counts. An extra 17 steps per chain makes the final
output depend on the permutation rather than always returning the cycle's start.
The independent Python oracle checks normalized final offsets and the expected
instruction difference. Chase outputs do not depend on the number of complete
passes; the exact simulated instruction difference verifies that count, and the
ELF/Mach-O function-byte audit carries the same loop-count behavior to native.
The seed and relative layout match between platforms;
actual pointer values and executable addresses need not match.

Read and Write use the whole buffer. Copy uses its first half as source and
second half as destination, so both buffers are included in the stated working
set. Read accumulates all values modulo 2^64. Write and Copy verify every output
word outside the native timer. Initialization and verification cannot be removed
by the compiler: the loops are external assembly, Read returns its sums, and
written data is checked afterward. No Darwin-reserved x18 register is used and
all callee-saved registers touched by a probe are preserved.

`loops.S` supplies both executable formats. Retained start/end symbols permit
an exact byte comparison of all eight function bodies in the final Linux ELF
and native Mach-O binaries. Wrappers, startup, virtual addresses and input
construction are not claimed to be instruction-identical. `driver.h` uses an
explicit chain-count table shared by the case definitions, without relying on
variable shift instructions in guest setup. The checked-in ELF remains small;
its buffer and permutation workspace are BSS. `memory.elf` reserves 128 MiB
plus a 4 MiB permutation; only the five 256 MiB cases use `memory-large.elf`,
which reserves 256 MiB plus an 8 MiB permutation. Both share exact probe bytes. One native binary reserves 256 MiB and measures
only each case's declared footprint.
`memory-large.elf` explicitly requests a 512 MiB image budget; the default loader budget
remains 256 MiB. Older base revisions without this option are marked unsupported
for those five points, with no substituted or patched base binary.

## Measurement contract

Each short and long run independently initializes data and completes two warmup
passes. Physical first touch happens once per process; later native trials reuse
the mapped pages. Neither initialization nor first touch is inside the timer. The timed function then executes N or 2N
complete passes. Chase's extra 17 steps and the fixed setup/teardown cancel in
the difference. Native timing uses `CLOCK_THREAD_CPUTIME_ID` around only the
probe call, on one thread requesting user-interactive QoS. The entire ELF runs
in the simulator; subtracting identical initialization, warmup and verification
removes their fixed work. Equal-width pass arguments keep parsing instruction
counts constant. No fast-forward or synthetic cache-state injection is used.

Native collection starts at a bounded minimum amount of work and doubles the
pass count until a pilot differential reaches 100 ms, then records nine alternating
short/long pairs. The reference stores the median and interpolated P10/P90 of
these differences. A usable point requires a median of at least 20 ms and
`(P90-P10)/median <= 0.10`; otherwise it stays visible as noisy. P10/P90 are
variability summaries, not confidence intervals. All pilot and paired records,
collection timestamps, binaries, source snapshots, and the byte audit remain in
the external evidence directory. Git retains compact plotting data and hashes.
All 140 active memory references are freshly captured with this protocol. Prior
raw captures remain external historical evidence; application references are unchanged.

Simulation uses enough complete passes to cover at least 65,536 pointer loads
or 1 MiB of streaming work in the differential; larger footprints require at
least one full pass. Every execution checks the expected checksum, exit status,
input hash, source revision, and model configuration. The long-minus-short
instruction count must exactly match the hand-written loop. Per-execution host
time and instruction budgets bound failures rather than treating timeouts as
valid observations.

The single-chain metric includes loop overhead and translation effects; it is
not an isolated physical cache-hit latency. Multiple chains measure aggregate
throughput, not latency per simultaneously outstanding request. Streaming byte
counts include bytes explicitly read/written by the loop: Copy counts source
reads plus destination writes. They exclude cache-line allocation, writeback,
prefetch and coherence traffic. Stores need not be globally drained at the
return boundary; the large differential measures steady-state scalar store
throughput, not write visibility latency. Neither scalar bandwidth nor a
working-set transition establishes peak vector bandwidth or a specific cache
capacity. Native core identity and instantaneous frequency are unobserved;
native time is never presented as measured cycles.

## Running and reporting

Build with `sh evaluation/microbenchmarks/memory/build.sh` and the AArch64 musl
GCC toolchain. Ordinary PR CI runs:

```
python3 scripts/memory_microbenchmark.py smoke --poros /tmp/poros --qemu /usr/bin/qemu-aarch64
```

This checks all eight patterns at 8 and 32 KiB and the five 96 KiB chase cases in functional and timing modes,
with the same ELF independently executed by QEMU. Native collection on a quiet
Apple M2 host is explicit:

```
python3 scripts/memory_microbenchmark.py collect --evidence-dir /tmp/memory-capture
```

Post-merge accuracy preparation binds the memory matrix and reference alongside
the existing application plan. Existing worker shards execute the memory points
with the exact prepared base/head binaries. Each point is assigned once;
aggregation validates the raw pairs and consumes their compact results without
running the large memory probes again. The workflow's trigger policy is unchanged.
For a diagnostic-only local campaign, the same prepared plan supports
`accuracy_parallel.py memory-shard` and `memory-aggregate`; these commands never
claim that application accuracy was rerun.

The shared report renderer shows logarithmically spaced working-set curves,
point tooltips, native P10–P90 vertical variability ranges, quality, base/head values, and collapsed tables after the
plots. These diagnostics remain outside application S-curves, correlations,
ranking and error summaries. Archived reports without this suite remain readable;
reports declaring it must contain complete matching evidence. The
`accuracy_site.py memory-preview` command renders a complete memory-only artifact
through the same plotting implementation, with an explicit diagnostic-only label.
