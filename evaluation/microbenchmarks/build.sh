#!/bin/sh
set -eu
cd "$(dirname "$0")"
aarch64-linux-musl-gcc -nostdlib -static -no-pie -s -Wl,--build-id=none,-z,max-page-size=4096 \
    -Wl,-T,../../tests/usertime/layout.ld linux-entry.S frontend.S -o frontend.elf
