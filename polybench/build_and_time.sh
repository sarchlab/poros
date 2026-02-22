#!/bin/bash
# build_and_time.sh - Compile and time all 30 PolyBench/C benchmarks
# for MINI and SMALL datasets, reporting median of 7 runs.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
POLYBENCH_DIR="$SCRIPT_DIR/PolyBenchC-4.2.1"
UTILITIES="$POLYBENCH_DIR/utilities"
BUILD_DIR="$SCRIPT_DIR/build"
CSV_FILE="$SCRIPT_DIR/timing.csv"
LOG_FILE="$SCRIPT_DIR/build.log"

CC=clang
CFLAGS="-O3 -fno-vectorize -fno-slp-vectorize -DPOLYBENCH_TIME"
LDFLAGS="-lm"
RUNS=7
TIMEOUT_SEC=30

# All 30 benchmarks: relative path from PolyBenchC-4.2.1 root
BENCHMARKS=(
  "datamining/correlation/correlation"
  "datamining/covariance/covariance"
  "linear-algebra/blas/gemm/gemm"
  "linear-algebra/blas/gemver/gemver"
  "linear-algebra/blas/gesummv/gesummv"
  "linear-algebra/blas/symm/symm"
  "linear-algebra/blas/syr2k/syr2k"
  "linear-algebra/blas/syrk/syrk"
  "linear-algebra/blas/trmm/trmm"
  "linear-algebra/kernels/2mm/2mm"
  "linear-algebra/kernels/3mm/3mm"
  "linear-algebra/kernels/atax/atax"
  "linear-algebra/kernels/bicg/bicg"
  "linear-algebra/kernels/doitgen/doitgen"
  "linear-algebra/kernels/mvt/mvt"
  "linear-algebra/solvers/cholesky/cholesky"
  "linear-algebra/solvers/durbin/durbin"
  "linear-algebra/solvers/gramschmidt/gramschmidt"
  "linear-algebra/solvers/lu/lu"
  "linear-algebra/solvers/ludcmp/ludcmp"
  "linear-algebra/solvers/trisolv/trisolv"
  "stencils/adi/adi"
  "stencils/fdtd-2d/fdtd-2d"
  "stencils/heat-3d/heat-3d"
  "stencils/jacobi-1d/jacobi-1d"
  "stencils/jacobi-2d/jacobi-2d"
  "stencils/seidel-2d/seidel-2d"
  "medley/deriche/deriche"
  "medley/floyd-warshall/floyd-warshall"
  "medley/nussinov/nussinov"
)

DATASETS=("MINI_DATASET" "SMALL_DATASET")

# Compute median of N values (pass all values as arguments)
medianN() {
  printf '%s\n' "$@" | sort -g | sed -n "$(( ($# + 1) / 2 ))p"
}

mkdir -p "$BUILD_DIR"
echo "benchmark,dataset,time_seconds" > "$CSV_FILE"
: > "$LOG_FILE"

total=${#BENCHMARKS[@]}
datasets_count=${#DATASETS[@]}
echo "Building and timing $total benchmarks x $datasets_count datasets..."

for bench in "${BENCHMARKS[@]}"; do
  name="$(basename "$bench")"
  src="$POLYBENCH_DIR/$bench.c"
  include_dir="$POLYBENCH_DIR/$(dirname "$bench")"

  for ds in "${DATASETS[@]}"; do
    ds_lower="$(echo "$ds" | sed 's/_DATASET//' | tr '[:upper:]' '[:lower:]')"
    outbin="$BUILD_DIR/${name}_${ds_lower}"

    # Compile
    compile_cmd="$CC $CFLAGS -D$ds -I$UTILITIES -I$include_dir $UTILITIES/polybench.c $src -o $outbin $LDFLAGS"
    echo "Compiling $name ($ds_lower)..." | tee -a "$LOG_FILE"
    if ! eval "$compile_cmd" >> "$LOG_FILE" 2>&1; then
      echo "  FAILED to compile $name ($ds_lower). See build.log." | tee -a "$LOG_FILE"
      echo "$name,$ds_lower,COMPILE_ERROR" >> "$CSV_FILE"
      continue
    fi

    # Run 7 times and collect timing
    times=()
    run_ok=true
    for run in $(seq 1 $RUNS); do
      # PolyBench prints timing to stdout
      t=$(perl -e 'alarm shift; exec @ARGV' "$TIMEOUT_SEC" "$outbin" 2>/dev/null || true)
      if [ -z "$t" ]; then
        echo "  Run $run of $name ($ds_lower): no output or timeout" | tee -a "$LOG_FILE"
        run_ok=false
        break
      fi
      times+=("$t")
    done

    if [ "$run_ok" = true ] && [ ${#times[@]} -eq $RUNS ]; then
      med=$(medianN "${times[@]}")
      echo "  $name ($ds_lower): ${times[*]} -> median $med" | tee -a "$LOG_FILE"
      echo "$name,$ds_lower,$med" >> "$CSV_FILE"
    else
      echo "  $name ($ds_lower): timing failed" | tee -a "$LOG_FILE"
      echo "$name,$ds_lower,RUN_ERROR" >> "$CSV_FILE"
    fi
  done
done

echo ""
echo "Done. Results in $CSV_FILE"
echo "Build log in $LOG_FILE"
