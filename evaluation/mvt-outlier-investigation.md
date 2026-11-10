# MVT 3072 outlier investigation

Status: **investigation open; no model change and no fix claimed.** This file
records the verified starting data, the hypotheses and the tests planned to
separate them. Results are added only after they are measured.

## Starting data

Source: the completed canonical report for head `63abde6` (merged as `a27923d`),
246 qualified application points, mean error 16.86193%, median 14.57811%
(error = |predicted − native| / min). One application point exceeds 50%:

## Workload

`tests/usertime/mvt.c` has three phases over a flat `n × n` array of doubles
with row pitch `8n` bytes: row-major initialisation (one integer remainder per
element), a row-major matrix–vector product, and a transposed product that
walks each column with a stride of `8n` bytes (24,576 bytes at n = 3072, which
is 1.5 pages of 16 KiB, 384 lines of 64 bytes and 192 lines of 128 bytes).

## Hypotheses

- **H1 – L2 set conflict.** The model indexes its 16 MiB, 16-way, 128-byte-line
  L2 by address modulo 8,192 sets. A 192-line stride reaches 128 sets, which
  hold 2,048 lines; a column needs 3,072. The other five strides reach 256 to
  1,024 sets, enough for a whole column (3584 only just: 4,096 lines for
  3,584). Whether the M2's L2 has this geometry, and what a real physically
  indexed L2 does with this stride, is not established.
- **H2 – L1 set conflict.** By the same arithmetic the six strides reach only
  2 to 16 of the 256 L1 sets, so a column cannot stay in L1 at any size. This
  may shape the common cost but should not single out 3072.
- **H3 – prefetch.** The stride prefetcher may fail to cover, or may evict, the
  column walk at this stride.
- **H4 – miss handling.** Under conflict the model may be bound by its 12 miss
  entries, 335-cycle memory latency or 16 bytes/cycle memory transport more
  tightly than the hardware is.
- **H5 – instruction side.** Flags, floating-point class or stack-pointer
  dependency gaps noted by the audit. Examined only if memory does not account
  for the difference.
- **H6 – missing native cost at other sizes.** The native per-element cost
  rises with n from 3200 to 3584; the model's does not. Address translation
  (each column access lands on a new page) is not modelled and is a candidate.

## Planned tests

1. Reproduce 315,508,551 cycles and 217,196,663 instructions on the unmodified
   tree.
2. Per-phase counters (hits, misses, evictions, prefetches, merged requests,
   transport waits per level) that leave cycles and output unchanged, checked
   by a parity run.
3. Set-occupancy arithmetic from the ELF's real array addresses for the six
   formal sizes and for nearby sizes, compared with the counters.
4. Controlled synthetic strided walks over power-of-two-rich and competing
   non-power-of-two strides, to test that any mechanism behaves as a general
   rule and not as a fit to one size.
5. One-factor model variants to measure which single factor moves 3072 and
   what the same factor does to 3200–3712 and to the control workloads.
6. Published sources on the M2 cache organisation and, if needed, a bounded
   native diagnostic probe on M2 Linux requested through the coordinator.

## Scope and unchanged references

- Benchmark sources, ELF files, native references and published report data
  are unchanged.
- No behaviour keyed on benchmark name, path, hash or size; no reference
  lookup; no threshold tuning; no arbitrary delay.
- Finite resources, real arrival times, issue reordering and eviction and
  writeback dependencies stay in the model.
- Microbenchmarks are evaluated separately and are outside the single-outlier
  statement above.
- A negative finding is a valid result of this work.
