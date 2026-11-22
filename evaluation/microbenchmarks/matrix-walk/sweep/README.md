# Matrix traversal sweep

This extends the [four-point diagnostic](../README.md) with 60 cases per
Apple target. Both arrays hold 32-bit integers. `points.json` stores logical
rows and columns, the two physical row widths in elements, and the
destination offset in bytes relative to its 16 KiB-aligned base.
`reference.json` stores paired native ratios, per-order medians and
hashes of the native captures. This is a warmed-kernel reference.

The inventory covers 28 unpadded squares, 18 common-padding points, six
rectangular stride points, four independent input/output-padding controls,
and four destination alignments. Alignment uses the 1024-square point,
which had the largest M5 Max column/row ratio among the unpadded 256, 512,
and 1024 squares in the common sweep.

Build `sweep.c` and `sweep_native.c` with the compiler options and startup,
runtime, and linker files specified in the [parent diagnostic](../README.md).
The native executable takes `rows cols a_width b_width b_offset_words pairs repeats`.
The ELF takes `order rows cols a_width b_width b_offset_words repeats`.
Convert the offset from bytes to words by dividing by four. Both executables
verify every output. Native experiments run two processes per case with 21
alternating pairs per process. Time each batch after one traversal of the
same order. Linux captures pin CPU 4. The macOS capture requests
user-initiated QoS and does not establish core placement.

The simulator runs each order with one traversal and with `1 + k`,
where `k = min(32, max(1, floor(524288 / (rows * columns))))`. Divide
the cycle difference by `k`. The 32, 256, 512, 1024 and 2048 unpadded
cases also run `1 + 2k` to check whether the warmed slope has settled.
The suite is separate from whole-program accuracy statistics.
