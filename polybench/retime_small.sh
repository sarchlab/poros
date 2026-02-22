#!/bin/bash
# Re-time benchmarks with SMALL dataset times < 100µs.
# Uses 21-run median for extra precision.
set -euo pipefail

BUILD_DIR="$(cd "$(dirname "$0")" && pwd)/build"
RUNS=21

medianN() {
  printf '%s\n' "$@" | sort -g | sed -n "$(( ($# + 1) / 2 ))p"
}

# Small benchmarks (SMALL < 100µs in timing.csv)
SMALL_BENCHMARKS=(trisolv jacobi-1d durbin gesummv atax mvt bicg gemver syrk)

echo "=== SMALL dataset (21-run median) ==="
for name in "${SMALL_BENCHMARKS[@]}"; do
  bin="$BUILD_DIR/${name}_small"
  if [ ! -x "$bin" ]; then echo "$name: binary not found"; continue; fi
  times=()
  for run in $(seq 1 $RUNS); do
    t=$("$bin" 2>/dev/null || true)
    [ -z "$t" ] && continue
    times+=("$t")
  done
  if [ ${#times[@]} -eq $RUNS ]; then
    med=$(medianN "${times[@]}")
    sorted=$(printf '%s\n' "${times[@]}" | sort -g | tr '\n' ' ')
    echo "  $name: median=$med  all=[$sorted]"
  else
    echo "  $name: got only ${#times[@]}/$RUNS runs"
  fi
done

echo ""
echo "=== MINI dataset (21-run median) ==="
for name in "${SMALL_BENCHMARKS[@]}"; do
  bin="$BUILD_DIR/${name}_mini"
  if [ ! -x "$bin" ]; then echo "$name: binary not found"; continue; fi
  times=()
  for run in $(seq 1 $RUNS); do
    t=$("$bin" 2>/dev/null || true)
    [ -z "$t" ] && continue
    times+=("$t")
  done
  if [ ${#times[@]} -eq $RUNS ]; then
    med=$(medianN "${times[@]}")
    sorted=$(printf '%s\n' "${times[@]}" | sort -g | tr '\n' ' ')
    echo "  $name: median=$med  all=[$sorted]"
  else
    echo "  $name: got only ${#times[@]}/$RUNS runs"
  fi
done
