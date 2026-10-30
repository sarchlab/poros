#!/bin/sh
set -eu
cd "$(dirname "$0")"
cc=${CC_AARCH64:-aarch64-linux-musl-gcc}
for pair in '1 integer' '2 memory' '3 branch'; do
 set -- $pair
 "$cc" -DCASE="$1" -O1 -fno-tree-vectorize -fno-if-conversion -fno-if-conversion2 -fno-unroll-loops -fno-stack-protector -fno-pie -nostdlib -static -no-pie -Wl,--build-id=none -Wl,-T,layout.ld start.S program.c -o "$2.elf"
done
