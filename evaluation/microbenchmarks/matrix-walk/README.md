# Matrix traversal

[Poros #89](https://github.com/sarchlab/poros/issues/89) tracks the timing
difference between row-first and column-first traversal of row-major arrays.
The same scalar loop computes `B[i] = A[i] + 1` in both orders. It changes
which stride goes in the inner loop, and verifies every result. The inputs
cover a small control, an unpadded 256 × 256 matrix, the same matrix with 16
padding elements per row, and an unpadded 512 × 512 matrix.

This diagnostic has a separate reference because it measures a warmed
kernel, while the application dataset measures whole-program user time.
It is excluded from application accuracy aggregates and the standard
campaign. `reference.json` records two native sessions of 41 alternating
row/column pairs per case, timing 32 traversals after one warmup. It includes
fixed-core M1 and M2 Linux captures, an M5 Max macOS capture with
user-initiated QoS, and a macOS M2 cross-check. macOS samples lack core
attribution. The three-device report uses the fixed-core M2 data.

The [expanded sweep](sweep/README.md) records 60 cases on each of three Apple devices.

## Run and report

From the repository root with GCC 14.2.0 targeting AArch64 Linux:

```sh
suite=evaluation/microbenchmarks/matrix-walk
tmp=$(mktemp -d)
go build -trimpath -o "$tmp/poros" ./cmd/poros
aarch64-linux-musl-gcc -O1 -fno-tree-vectorize -fno-loop-interchange \
    -fno-unroll-loops -fno-stack-protector -fno-pie -nostdlib -static -no-pie \
    -Wl,--build-id=none -Wl,-T,tests/usertime/layout.ld \
    -I tests/benchmarks tests/usertime/start.S "$suite/matrix_walk.c" \
    -o "$tmp/matrix.elf"
mkdir "$tmp/results"
for device in m1 m2 m5-max; do
    for shape in 32:32 256:256 256:272 512:512; do
        n=${shape%:*}
        width=${shape#*:}
        for order in r c; do
            for repeats in 1 33; do
                "$tmp/poros" elf --mode ooo --target "$device" --timeout 8m \
                    --max-instructions 100000000 "$tmp/matrix.elf" "$order" "$n" "$width" "$repeats" \
                    > "$tmp/results/$device-$n-$width-$order-$repeats.json"
            done
        done
    done
done
python3 "$suite/report.py" "$tmp/results" "$tmp/report"
```

Build the simulator in a clean committed checkout. The report verifies the
guest outputs and execution identity. It estimates the cost of one warmed
traversal by subtracting the one-traversal guest's cycles from the
33-traversal guest's cycles and dividing by 32. This amortizes initialization
and result checking. Differences in final cache state can still affect the
subtraction. The ratio is column-first divided by row-first, and the signed
ratio error is relative to the native median paired ratio.

For a native capture, build `native_bench.c` with `-O1` and vectorization
and loop unrolling disabled. On Apple Clang pass
`-fno-vectorize -fno-slp-vectorize -fno-unroll-loops`. With GCC use
`-fno-tree-vectorize -fno-loop-interchange -fno-unroll-loops`.
Include `tests/benchmarks` and run `matrix-native 256 272 32`.
On Linux pin it to a known performance core with `taskset`. Run each case in
two independent processes. Each process emits
`size,row_width,trial,order,repeats,elapsed_ns`, with two rows per trial.
