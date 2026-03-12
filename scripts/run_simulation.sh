#!/bin/bash
# Run m2sim simulator on all benchmarks and collect predicted times
# Usage: ./scripts/run_simulation.sh [output_file]
#
# Requires: go build ./cmd/m2sim has been run first

set -e

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT="${1:-$REPO_ROOT/simulation_results.txt}"

echo "=== M2Sim Simulation Run ==="
echo "Output: $OUTPUT"
echo ""

cd "$REPO_ROOT"

# Build simulator if needed
if [ ! -f ./m2sim ]; then
    echo "Building m2sim..."
    go build -o m2sim ./cmd/m2sim
fi

echo "Running all benchmarks..."
echo ""

# Run all benchmarks with comparison table
./m2sim run --all 2>&1 | tee "$OUTPUT"

echo ""
echo "=== Results saved to $OUTPUT ==="
