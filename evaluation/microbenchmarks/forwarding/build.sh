#!/bin/sh
set -eu
cd "$(dirname "$0")"
aarch64-linux-musl-gcc -O2 -fno-tree-vectorize -mgeneral-regs-only -ffreestanding -fno-builtin \
    -nostdlib -static -no-pie -s -Wl,--build-id=none,-z,max-page-size=4096 \
    -Wl,-T,../../../tests/usertime/layout.ld entry.S linux.c loops.S -o forwarding.elf
