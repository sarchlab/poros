#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
compiler=${CROSS_COMPILE:-aarch64-linux-musl-}gcc
output=${1:-.}
mkdir -p "$output"
export LC_ALL=C SOURCE_DATE_EPOCH=0
names=(hello startup memory compute syscall-errors bad-load bad-store nx unsupported-syscall unsupported-instruction loop output-limit exit-status arithmetic-loop)
for index in "${!names[@]}"; do
  "$compiler" -O1 -ffreestanding -fno-builtin -fno-stack-protector \
    -fno-asynchronous-unwind-tables -fno-unwind-tables -fno-tree-vectorize \
    -nostdlib -static -no-pie -march=armv8-a -DCASE=$((index+1)) \
    -Wl,--build-id=none,-T,layout.ld -o "$output/${names[index]}.elf" start.S guest.c
done
