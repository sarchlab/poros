# M1 and M2 column-first timing investigation

This investigation starts at the row-calibrated revision in #90. It uses
the same ELF, two-count warmed timing, native references, and error
formula as the [sweep](README.md). The initial matched experiment holds
the matrix dimensions fixed and adds 16 elements to both row widths.
The number of elements traversed and retired instructions are identical
in each Poros pair. Timings below are microseconds per traversal.

| Device | Square | Hardware, unpadded → padded | Poros, unpadded → padded | Hardware speedup | Poros speedup |
| --- | ---: | ---: | ---: | ---: | ---: |
| M1 | 256 | 203.31 → 33.18 | 132.98 → 36.75 | 6.13× | 3.62× |
| M1 | 512 | 902.45 → 132.19 | 552.36 → 146.88 | 6.83× | 3.76× |
| M1 | 1024 | 5,082.46 → 575.98 | 2,196.00 → 590.80 | 8.82× | 3.72× |
| M2 | 256 | 187.87 → 32.73 | 134.06 → 34.58 | 5.74× | 3.88× |
| M2 | 512 | 860.43 → 129.05 | 525.21 → 137.78 | 6.67× | 3.81× |
| M2 | 1024 | 4,676.58 → 555.79 | 1,920.07 → 552.67 | 8.41× | 3.47× |

These twelve predictions were run at the clean #90 head `cfc19e5`.
The ELF SHA-256 is
`8b1a2c9081d9437d31f0c842544658df921dd1fa626955e6653072a1785578b3`.
Every guest returned its verification output and retired the expected
instruction count. The unpadded runs reproduce the prior refined
candidate at all six points.

## What the counters establish

At 256×256 and 512×512, padding reduces modeled L1 misses and write
allocations by about 16× on both targets. For example, each 256×256
traversal goes from about 131,073 to 8,193 L1 misses and from 65,536
to 4,096 write allocations. All demand fills in these warmed comparisons
hit modeled L2 and no page walks were counted. At 1024×1024, M2 still
counts no L2 demand misses or page walks in either configuration. M1
records substantially fewer L2 misses with padding on that shape.

The model already reproduces the direction of the padding effect. It
underprices the unpadded case while predicting the padded case more
closely. For example, M2's unpadded 1024×1024 is 2.44× faster in the
simulator than on hardware, whereas its padded timing differs by less
than 1%. A single frequency multiplier would degrade the padded
control. The zero simulated walks and L2 misses on the smaller inputs
also make those modeled paths poor explanations for the discrepancy
by themselves.

These are **simulated** cache events. They cannot establish native L1
miss counts or rule out native translation or other physical effects.
A prior CPU4 region-counter capture also showed a gap in cycles at
unpadded 256×256, about 9.5 cycles per element on both chips versus 6.50
on M1 and 7.14 on M2 in this model. That cycle-domain contrast comes from
a separate capture and does not convert the historical wall times.

## Next discriminating comparison

Use otherwise identical native load-only, store-only, and
load-dependent store loops at the same padded and unpadded shapes.
Compare counted cycles and, where validated on that host, L1 refill and
translation events. Simulation-side fill bandwidth, L2 hit latency,
write-allocation pressure, and issue overlap can then be ablated one
at a time. Preserve the padded controls and row-first cohort when
evaluating any candidate. No timing-model change is proposed in this
note.

## Native load/store isolation

The two original Linux performance cores (CPU 4 on each host) ran the
256² and 512² columns with both widths, in two passes with opposite
variant order. Each entry is the median across ten warmed regions.
All 320 regions passed the output check with equal PMU enabled and
running times. The two variants that remove a memory operation replace
the original instruction with a NOP (store removed) or a zero move
(load removed), preserving the trip count and instruction count.
The independent-store control keeps both addresses and memory operations
and replaces the loaded add with an immediate move. The native and guest
walk assembly match within each variant. The verified outputs account
for the deliberate changes to stored values.

Each cell is **hardware / Poros cycles per element**. The Poros runs
use the same clean simulator revision as above.

| Device | Square | Row width | Full loop | Load only | Store only | Independent store |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| M1 | 256 | 256 | 9.41 / 6.50 | 2.00 / 2.00 | 3.14 / 4.00 | 9.41 / 6.00 |
| M1 | 256 | 272 | 1.56 / 1.80 | 1.45 / 1.48 | 1.19 / 1.32 | 1.29 / 1.37 |
| M1 | 512 | 512 | 10.65 / 6.75 | 2.00 / 2.00 | 7.02 / 4.00 | 10.65 / 6.00 |
| M1 | 512 | 528 | 1.56 / 1.79 | 1.46 / 1.48 | 1.18 / 1.33 | 1.24 / 1.34 |
| M2 | 256 | 256 | 9.51 / 7.14 | 2.00 / 2.00 | 3.16 / 4.00 | 9.44 / 6.00 |
| M2 | 256 | 272 | 1.62 / 1.84 | 1.47 / 1.49 | 1.33 / 1.31 | 1.33 / 1.37 |
| M2 | 512 | 512 | 10.97 / 6.99 | 2.00 / 2.00 | 7.55 / 4.00 | 10.95 / 6.00 |
| M2 | 512 | 528 | 1.62 / 1.83 | 1.48 / 1.48 | 1.26 / 1.33 | 1.32 / 1.34 |

The load-only unpadded loops agree in cycles. Removing the dependency
from the load to the store barely changes the native unpadded cost.
At 256², the penalty appears primarily with both read and write traffic.
At 512², the native store-only path also slows substantially, while the
simulator stays at four cycles per element. This locates the discrepancy
in write-allocation or writeback pressure and its interaction with
demand refills. The experiment does not identify a physical queue,
bank, or address comparison rule.

In a simulator-only ablation at 256², halving the modeled L2-to-L1
fill bandwidth from 32 to 16 bytes/cycle moves unpadded time from
132.98 to 276.18 µs on M1 and 134.06 to 262.90 µs on M2. It also
moves the padded controls from 36.75 to 45.05 µs and from 34.58 to
42.43 µs, worsening those comparisons. Adding eight cycles to nominal
L2 hit latency instead moves the unpadded simulations to 140.43 µs
and 126.75 µs. Neither experiment models the conditional slowdown
satisfactorily. The next candidate should model contention or allocation
limits under simultaneous conflicting accesses and check both pure
stores and mixed traffic.
