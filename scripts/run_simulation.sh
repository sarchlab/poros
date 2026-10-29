#!/bin/bash
# Run poros simulator on all benchmarks and collect predicted times
# Usage: ./scripts/run_simulation.sh [output_file]
#
# Requires: go build ./cmd/poros has been run first

set -e

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT="${1:-$REPO_ROOT/simulation_results.txt}"

echo "=== Poros Simulation Run ==="
echo "Output: $OUTPUT"
echo ""

cd "$REPO_ROOT"

# Build simulator if needed
if [ ! -f ./poros ]; then
    echo "Building poros..."
    go build -o poros ./cmd/poros
fi

echo "Running all benchmarks..."
echo ""

# Run all benchmarks with comparison table
./poros run --all 2>&1 | tee "$OUTPUT"

echo ""
echo "=== Results saved to $OUTPUT ==="
