#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
# Override CROSS_COMPILE to use the recorded toolchain from another location.
compiler=${CROSS_COMPILE:-aarch64-linux-musl-}gcc
output=${1:-startup.elf}
export LC_ALL=C SOURCE_DATE_EPOCH=0
"$compiler" -nostdlib -static -no-pie -march=armv8-a \
  -Wl,--build-id=none,-T,layout.ld -o "$output" start.S
