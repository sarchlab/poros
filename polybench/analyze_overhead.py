#!/usr/bin/env python3
"""
Analyze timing overhead for small PolyBench benchmarks.

Reads timing.csv and computes MINI/SMALL ratios to detect constant overhead.
If computation scales as O(N^k), then SMALL/MINI ratio = (N_small/N_mini)^k.
A ratio much lower than expected suggests constant overhead dominates MINI measurements.
"""
import csv, sys

# Known dataset sizes (N_mini, N_small) for each benchmark
# From the PolyBench header files
SIZES = {
    "trisolv":       (40, 120),
    "atax":          ((38, 42), (116, 124)),
    "bicg":          ((38, 42), (116, 124)),
    "gesummv":       (30, 90),
    "mvt":           (40, 120),
    "durbin":        (40, 120),
    "jacobi-1d":     (20, 40),
    "gemver":        (40, 120),
    "syrk":          (20, 60),
    "trmm":          (20, 60),
    "gemm":          (20, 60),
    "symm":          (20, 60),
}

data = {}
with open("timing.csv") as f:
    reader = csv.DictReader(f)
    for row in reader:
        key = (row["benchmark"], row["dataset"])
        data[key] = float(row["time_seconds"])

print(f"{'Benchmark':<16} {'MINI (µs)':>10} {'SMALL (µs)':>11} {'Ratio S/M':>10} {'Notes'}")
print("-" * 65)

benchmarks_sorted = sorted(set(k[0] for k in data.keys()))
for bench in benchmarks_sorted:
    mini = data.get((bench, "mini"))
    small = data.get((bench, "small"))
    if mini and small and mini > 0:
        ratio = small / mini
        mini_us = mini * 1e6
        small_us = small * 1e6
        note = ""
        if small_us < 100:
            note = "<100µs"
        if mini_us < 20:
            note += " MINI<20µs"
        print(f"{bench:<16} {mini_us:>10.1f} {small_us:>11.1f} {ratio:>10.1f}   {note}")

print("\n--- Small benchmarks (SMALL < 100µs) ---")
print(f"{'Benchmark':<16} {'SMALL (µs)':>11} {'MINI (µs)':>10} {'overhead_est':>13}")
print("-" * 55)
for bench in benchmarks_sorted:
    mini = data.get((bench, "mini"))
    small = data.get((bench, "small"))
    if small and small * 1e6 < 100:
        mini_us = mini * 1e6 if mini else 0
        small_us = small * 1e6
        # If we assume the overhead is ~constant C, and SMALL ≈ k * MINI_real,
        # with k = (N_small/N_mini)^2, then:
        # MINI_measured = MINI_real + C
        # SMALL_measured = k * MINI_real + C
        # From these: MINI_real = (SMALL_measured - C) / k
        # And: C = MINI_measured - MINI_real = MINI_measured - (SMALL_measured - C) / k
        # C * (1 - 1/k) = MINI_measured - SMALL_measured/k
        # C = (MINI_measured - SMALL_measured/k) / (1 - 1/k)
        # We need to know k. Without knowing exact complexity, we can report that
        # for very small benchmarks, the measurement is at the gettimeofday resolution limit.
        print(f"{bench:<16} {small_us:>11.1f} {mini_us:>10.1f}   resolution-limited")
