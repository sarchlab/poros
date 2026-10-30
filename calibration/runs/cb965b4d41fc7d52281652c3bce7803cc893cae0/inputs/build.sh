#!/bin/sh
set -eu
cd "$(dirname "$0")"
cc=${CC_AARCH64:-aarch64-linux-musl-gcc}
flags='-O1 -ffp-contract=off -fno-tree-vectorize -fno-if-conversion -fno-if-conversion2 -fno-unroll-loops -fno-stack-protector -fno-pie -nostdlib -static -no-pie -Wl,--build-id=none -Wl,-T,layout.ld'
for pair in '1 integer' '2 memory' '3 branch'; do
 set -- $pair
 "$cc" -DCASE="$1" $flags start.S program.c -o "$2.elf"
done
"$cc" $flags start.S mvt.c -o mvt.elf
