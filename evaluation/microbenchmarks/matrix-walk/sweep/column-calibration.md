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
