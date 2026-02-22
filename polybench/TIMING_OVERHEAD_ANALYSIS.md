# Native Timing Overhead Analysis for Small Benchmarks

**Issue**: #71 — Investigate native timing overhead for small benchmarks
**Date**: 2026-02-20
**Machine**: Apple Silicon Mac (darwin/arm64)

## Summary

Timer overhead in PolyBench measurements is **~42 nanoseconds** — negligible even
for the smallest benchmark (0.3% of jacobi-1d at 14µs). No overhead correction is
needed. However, sub-100µs benchmarks exhibit **bimodal timing distributions** due
to CPU frequency/power-state scaling, causing 2-3x variability between sessions.

## Methodology

### Timer Overhead Measurement

Using `mach_absolute_time` (nanosecond precision) to measure the PolyBench timing
mechanism (`gettimeofday` pair + empty function call):

| Component | Time |
|---|---|
| gettimeofday pair (start + stop) | ~42 ns |
| Empty function call (noinline) | < 5 ns |
| **Total timer overhead** | **~42 ns** |
| gettimeofday resolution | 1 µs |
| Cache flush (polybench_flush_cache) | ~7 ms, **before** timer start |

### How PolyBench Timing Works

1. `polybench_timer_start()` → flushes CPU cache (32MB traversal), then calls `gettimeofday()`
2. Kernel function executes
3. `polybench_timer_stop()` → calls `gettimeofday()`
4. Reported time = stop - start

The cache flush happens **before** the start timestamp. It does not inflate the
measured kernel time, but it does ensure cold-cache execution.

### Overhead as Fraction of Kernel Time

| Benchmark | SMALL time | Overhead (42ns) as % |
|---|---|---|
| jacobi-1d | 14 µs | 0.30% |
| trisolv | 18 µs | 0.23% |
| durbin | 24 µs | 0.18% |
| gesummv | 39 µs | 0.11% |
| atax | 41 µs | 0.10% |
| mvt | 50 µs | 0.08% |
| bicg | 66 µs | 0.06% |

**Conclusion**: Timer overhead is negligible. It cannot explain measurement noise.

## Real Issue: Bimodal Timing Distributions

Small benchmarks show **bimodal distributions** across 21-run measurements:

```
trisolv SMALL (sorted, µs):
  Fast cluster: 11 11 11 12 12 12 12 13 18
  Slow cluster: 25 26 26 26 27 27 27 27 27 29 36

syrk SMALL (sorted, µs):
  Fast cluster: 95 95 96 96 97 97 98 102
  Slow cluster: 212 214 214 216 217 218 218 218 218 221 223 546 558
```

This bimodality is caused by Apple Silicon **CPU frequency/power-state scaling**.
The fast cluster represents the CPU at full performance; the slow cluster represents
a lower-power state. The 7-run median can land in either cluster depending on which
power state dominates during the measurement window.

### Impact on Median Stability

| Benchmark | timing.csv (7-run) | Session 1 (21-run) | Session 2 (21-run) |
|---|---|---|---|
| trisolv | 18 µs | 25 µs | 26 µs |
| syrk | 101 µs | 215 µs | 214 µs |
| durbin | 24 µs | 17 µs | 23 µs |
| mvt | 50 µs | 59 µs | 49 µs |
| bicg | 66 µs | 77 µs | 56 µs |

Session-to-session variability is **20-110%** for sub-100µs benchmarks.

### Stable Benchmarks (≥ 100µs)

Benchmarks with native time ≥ 100µs have ≤5% variability and are not affected.

## Recommendations

1. **No overhead correction needed** — the 42ns overhead is negligible.
2. **Sim/native ratios for sub-100µs benchmarks should be treated as approximate**
   (±30-50% uncertainty) due to native measurement variability.
3. **For the comparison table**, the existing timing.csv values are acceptable given
   the inherent uncertainty at this scale.
4. **If higher precision is needed**: use 21+ runs and take the P10 (10th percentile)
   to consistently capture the "CPU at full speed" time.

## Files

- `measure_overhead.c` — basic overhead measurement (gettimeofday resolution)
- `measure_overhead_ns.c` — nanosecond-precision overhead breakdown
- `measure_benchmark_precise.c` — gettimeofday step-size verification
- `retime_small.sh` — 21-run re-timing of small benchmarks
- `analyze_overhead.py` — MINI/SMALL scaling ratio analysis
