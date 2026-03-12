#!/bin/bash
# Run each PARSEC kernel natively and collect wall-clock timing
# Usage: ./parsec/time_native.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
KERNEL_DIR="$SCRIPT_DIR/kernels"

echo "=== PARSEC Native Timing ==="
echo ""

# Collect results
results=""

for exe in "$KERNEL_DIR"/*_native; do
    if [ ! -x "$exe" ]; then
        continue
    fi
    name=$(basename "$exe" _native)
    echo "--- Running $name ---"
    output=$(timeout 60 "$exe" 2>&1) || {
        echo "  FAILED or TIMEOUT"
        continue
    }
    echo "$output"
    echo ""

    # Extract per-iteration time
    per_iter=$(echo "$output" | grep "Per iteration" | awk '{print $3}')
    if [ -n "$per_iter" ]; then
        results="$results$name,$per_iter\n"
    fi
done

echo ""
echo "=== Summary (per iteration, ns) ==="
echo "benchmark,time_ns"
echo -e "$results"
