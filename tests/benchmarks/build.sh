#!/bin/sh
set -eu
cd "$(dirname "$0")"
cc=${CC_AARCH64:-aarch64-linux-musl-gcc}
for source in ./*.c ./support/*.c; do
  "$cc" -O1 -ffp-contract=off -fno-tree-vectorize -fno-if-conversion \
    -fno-if-conversion2 -fno-unroll-loops -fno-stack-protector -fno-pie \
    -nostdlib -static -no-pie -Wl,--build-id=none -Wl,-T,../usertime/layout.ld \
    ../usertime/start.S "$source" -lc -lgcc -o "${source%.c}.elf"
done
python3 provenance.py "$cc"
