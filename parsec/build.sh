#!/bin/bash
# Build all PARSEC kernel C files to ARM64 object files and native executables
# Usage: ./parsec/build.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
KERNEL_DIR="$SCRIPT_DIR/kernels"
CFLAGS="-target arm64-apple-macos -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops"

echo "=== PARSEC Build ==="
echo "Kernel directory: $KERNEL_DIR"
echo "CFLAGS: $CFLAGS"
echo ""

# Step 1: Compile all kernel .c files (excluding _main.c) to .o
echo "--- Compiling kernel .c to .o ---"
for src in "$KERNEL_DIR"/*.c; do
    base=$(basename "$src" .c)
    # Skip _main.c files
    if [[ "$base" == *_main ]]; then
        continue
    fi
    echo -n "  $base.c -> $base.o ... "
    clang $CFLAGS -c "$src" -o "$KERNEL_DIR/$base.o"
    echo "OK"
done
echo ""

# Step 2: Compile each _main.c + corresponding kernel .c to a native executable
echo "--- Building native timing executables ---"
for main_src in "$KERNEL_DIR"/*_main.c; do
    base=$(basename "$main_src" _main.c)
    kernel_src="$KERNEL_DIR/$base.c"
    exe="$KERNEL_DIR/${base}_native"

    if [ ! -f "$kernel_src" ]; then
        echo "  WARNING: $kernel_src not found, skipping $base"
        continue
    fi

    echo -n "  ${base}_main.c + $base.c -> ${base}_native ... "
    # Use native target for executables (not cross-compile)
    clang -O1 -fno-vectorize -fno-slp-vectorize -fno-unroll-loops \
        "$main_src" "$kernel_src" -lm -o "$exe"
    echo "OK"
done
echo ""

echo "=== Build complete ==="
ls -la "$KERNEL_DIR"/*.o "$KERNEL_DIR"/*_native 2>/dev/null || true
