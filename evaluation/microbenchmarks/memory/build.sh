#!/bin/sh
set -eu
cd "$(dirname "$0")"
for capacity in 128 256; do
    output=memory.elf
    if [ "$capacity" = 256 ]; then output=memory-large.elf; fi
    aarch64-linux-musl-gcc -O2 -fno-tree-vectorize -mgeneral-regs-only -ffreestanding -fno-builtin \
        -DMAX_BYTES=\(${capacity}UL\*1024\*1024\) \
        -nostdlib -static -no-pie -Wl,--build-id=none,-z,max-page-size=4096 \
        -Wl,-T,../../../tests/usertime/layout.ld entry.S linux.c loops.S -o "$output"
done
