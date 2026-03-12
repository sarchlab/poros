#!/bin/bash
# Unified build script for PolyBench and PARSEC benchmarks
# Usage: ./scripts/compile_all.sh

set -e

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "========================================"
echo "  Compile All Benchmarks"
echo "========================================"
echo ""

# Step 1: Build PolyBench
echo "=== Step 1: PolyBench ==="
if [ -f "$REPO_ROOT/polybench/build_and_time.sh" ]; then
    echo "Running polybench/build_and_time.sh..."
    cd "$REPO_ROOT/polybench"
    bash build_and_time.sh
else
    echo "WARNING: polybench/build_and_time.sh not found, skipping PolyBench"
fi
echo ""

# Step 2: Build PARSEC
echo "=== Step 2: PARSEC ==="
if [ -f "$REPO_ROOT/parsec/build.sh" ]; then
    echo "Running parsec/build.sh..."
    cd "$REPO_ROOT"
    bash parsec/build.sh
else
    echo "WARNING: parsec/build.sh not found, skipping PARSEC"
fi
echo ""

# Step 3: Build simulator
echo "=== Step 3: Simulator ==="
cd "$REPO_ROOT"
echo "Building m2sim..."
go build -o m2sim ./cmd/m2sim
echo "OK"
echo ""

echo "========================================"
echo "  All builds complete"
echo "========================================"
