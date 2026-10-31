# Complete ELF compute benchmarks

This suite replaces the remaining 40 embedded instruction-array kernels with
complete static AArch64 Linux ELF programs. Together with MVT and the integer,
memory and branch programs in `tests/usertime`, Poros exposes 44 named workloads.
The eleven PARSEC names denote the existing simplified compute kernels, not the
full PARSEC applications.

Each program parses bounded decimal dimensions, initializes all inputs, runs its
original C kernel, writes complete binary outputs, and exits through Linux SVC.
`runtime.h` supplies only argument and output transport. The Linux runtime has no
application names, addresses, Go data initialization, or host math callbacks.
Musl `log`/`exp` and associated library code run as ordinary guest instructions.
The build links only the required archive objects; no libc startup, dynamic
loader, TLS, or new syscall emulation is needed.

## Build and run

```
CC_AARCH64=/opt/homebrew/opt/musl-cross/bin/aarch64-linux-musl-gcc tests/benchmarks/build.sh
go run ./cmd/poros list
go run ./cmd/poros run --case 3 gemm
go run ./cmd/poros run --mode functional gemm 7 9 11
go run ./cmd/poros run --cores 2 --case 3 canneal
go run ./cmd/poros elf --max-instructions 200000000 --timeout 5m tests/benchmarks/facesim.elf 32
```

`--root` selects the repository assets when running outside the checkout.
`--cores` runs independent process copies with private L1 and modeled shared L2;
identical virtual addresses are shared cache tags. It does not implement Linux
threads, shared guest memory, or calibrated address-space isolation. The runner
owns process lifetimes, closes all executions, and terminates its simulation.

The committed binaries use GCC 14.2.0 and GNU ld 2.44. `provenance.json` records
compiler/linker identities, linked libc/libgcc archive hashes, and exact hashes
of the build script, manifest, wrappers, original included kernels, runtime,
startup, linker layout and ELF bytes. Source changes require rebuilding before
validation; the catalog rejects stale bindings. Rebuilding with the recorded
toolchain should leave the binaries unchanged. No generated oracle output is
committed. Native references compile the same sources with the host C compiler
and math library using `NATIVE_REFERENCE`; only argv/stdout transport differs.

## Correctness contract

`manifest.json` declares six points, dimension bounds, output element types and
complete output lengths for every migrated name. Points cover minima, small
nontrivial/tail shapes and original default sizes. The default is point 6.
All arrays have fixed bounded static capacity; that capacity is not claimed as
a measured active working set. Integer outputs must match exactly. Floating
outputs must be finite and agree with independently compiled native execution
within 1e-10 absolute + 1e-9 relative tolerance. Bodytrack additionally checks
relative error down to subnormal weights, so zero cannot replace tiny nonzero
likelihoods. QR checks orthogonality and reconstruction; Black-Scholes uses an
erfc reference with the source CDF approximation's error bound; Facesim checks
Neo-Hookean equations, positive determinants, force balance and energy gradients.
Ray hits/misses and Canneal Manhattan costs have independent exact references.

The identical ELF is also executed in QEMU, requiring **byte-for-byte complete
output equality** with Poros, including every tail. On Linux ARM64, `--native`
adds a third execution of those same bytes. Missing cases, abnormal exits,
stderr, truncated output, nonfinite values, duplicate inventory and stale build
provenance fail validation. QEMU/native elapsed times here are not calibration
measurements. The original four-workload raw references remain unchanged; a
separate 240-point direct-child Linux ARM64 user-time cohort is included in the
[264-case accuracy report](../../evaluation/usertime/README.md).

```
go test -short ./...
go test -timeout 45m ./...
python3 scripts/benchmark-differential.py --poros /path/to/poros --output /tmp/benchmark-report.json
```

Short Go CI runs point 3 for all 40 names in functional/inorder/OoO modes, plus
invalid-input cases and existing MVT/microbenchmark checks. Full Go CI runs all
240 new points in all three modes (720 complete executions). Each execution has
a 200M instruction, five-minute host time, and 16MiB output limit. Both tiers
retain loader/ABI/ISA/cache/pipeline tests, guest libm edge cases, branch-prediction
variants and multicore lifecycle/error checks. The full QEMU differential always
covers all 240 points. Reports retain inputs, build provenance, instruction counts
and hashes of the compared complete outputs, without huge raw float JSON.

## Explicit replacement map

Each name below replaces its former same-named `pkg/benchmark` setup closure,
embedded instruction arrays, application-specific register/data setup and legacy
golden rows. Isolated CPU/ISA fixtures remain. The obsolete registry and golden
files are removed rather than retained as a second execution route.

| Name | Dimensions | Six points | Complete output |
| --- | --- | --- | --- |
| atax | m, n | 1×1; 7×8; 19×20; 39×41; 97×102; 390×410 | y, tmp |
| gemm | ni, nj, nk | 1×1×1; 2×2×2; 3×3×4; 6×7×8; 15×17×20; 60×70×80 | C |
| 2mm | ni, nj, nk, nl | 1×1×1×1; 2×2×2×2; 2×2×3×4; 4×5×7×8; 10×12×17×20; 40×50×70×80 | D, tmp |
| 3mm | ni, nj, nk, nl, nm | 1×1×1×1×1; 2×2×2×2×2; 2×2×3×3×4; 4×5×6×7×8; 10×12×15×17×20; 40×50×60×70×80 | G, E, F |
| bicg | m, n | 1×1; 2×2; 5×6; 11×12; 29×31; 116×124 | s, q |
| gesummv | n | 1; 5; 12; 25; 62; 250 | y, tmp |
| gemver | n | 1; 2; 6; 12; 30; 120 | w, x, A |
| doitgen | nr, nq, np | 1×1×1; 2×2×2; 3×3×3; 2×2×3; 6×5×7; 25×20×30 | A, sum |
| trisolv | n | 1; 8; 20; 40; 100; 400 | x |
| durbin | n | 1; 8; 20; 40; 100; 400 | y |
| jacobi1d | tsteps, n | 1×3; 2×8; 5×20; 10×40; 25×100; 100×400 | A, B |
| syrk | n, m | 1×1; 2×2; 4×3; 8×6; 20×15; 80×60 | C |
| syr2k | n, m | 1×1; 2×2; 4×3; 8×6; 20×15; 80×60 | C |
| trmm | m, n | 1×1; 2×2; 3×4; 6×8; 15×20; 60×80 | B |
| symm | m, n | 1×1; 2×2; 3×4; 6×8; 15×20; 60×80 | C |
| cholesky | n | 1; 2; 6; 12; 30; 120 | A |
| lu | n | 1; 2; 6; 12; 30; 120 | A |
| ludcmp | n | 1; 2; 6; 12; 30; 120 | A, x, y |
| floydwarshall | n | 1; 3; 9; 18; 45; 180 | path |
| covariance | m, n | 1×2; 2×3; 4×5; 8×10; 20×25; 80×100 | result, mean |
| correlation | m, n | 1×2; 2×3; 4×5; 8×10; 20×25; 80×100 | result, mean, stddev |
| gramschmidt | m, n | 2×1; 3×2; 7×5; 15×8; 31×17; 60×60 | R, Q, A |
| nussinov | n | 1; 3; 9; 18; 45; 180 | table |
| adi | tsteps, n | 1×3; 2×4; 2×3; 4×6; 10×15; 40×60 | u, v |
| fdtd2d | tmax, nx, ny | 1×1×1; 2×2×2; 2×3×4; 4×6×8; 10×15×20; 40×60×80 | ex, ey, hz |
| heat3d | tsteps, n | 1×3; 2×4; 2×3; 4×3; 10×5; 40×20 | A, B |
| jacobi2d | tsteps, n | 1×3; 2×4; 4×6; 4×9; 10×22; 40×90 | A, B |
| seidel2d | tsteps, n | 1×3; 2×4; 2×6; 4×12; 10×30; 40×120 | A |
| deriche | w, h | 1×1; 3×2; 9×6; 19×12; 48×32; 192×128 | result |
| swaptions | n | 1; 2; 4; 5; 12; 50 | dF, A |
| blackscholes | n | 1; 7; 16; 63; 256; 1024 | prices |
| streamcluster | n, dim | 1×1; 10×1; 25×1; 51×3; 128×8; 512×32 | result |
| canneal | n, m | 1×2; 163×10; 409×25; 819×51; 2048×128; 8192×512 | cost |
| fluidanimate | n | 1; 2; 3; 7; 13; 16 | density, pressure, fx, fy, fz |
| vips | w, h | 3×3; 4×5; 7×9; 15×16; 31×33; 64×64 | result |
| bodytrack | n, state_dim, obs_dim | 1×1×1; 2×3×2; 7×6×8; 15×4×5; 31×6×8; 64×6×8 | result |
| ferret | nq, nd, dim, k | 1×1×1×1; 2×5×3×2; 3×7×5×4; 7×31×8×4; 11×63×13×4; 16×128×16×4 | result |
| x264 | range | 1; 2; 3; 4; 6; 8 | result |
| raytrace | nrays | 1; 2; 3; 7; 15; 32 | result |
| facesim | ntets | 1; 2; 7; 15; 23; 32 | forces |

Gram-Schmidt deliberately replaces the rank-deficient old 60×80 initialization
with a full-rank, well-conditioned 60×60 default and requires m≥n. A 60-row matrix
cannot have 80 orthonormal columns; the old finite golden did not establish QR
correctness. The algorithm is unchanged. Facesim retains all 32 corrected M0
tetrahedra in order and the original 20-vertex rest/deformed geometry; smaller
points use prefixes. Raytrace retains the original seven-node/four-triangle BVH
and varies ray count (including hits, misses and oblique rays). X264 retains its
128×128 frame/16×16 block and varies search radius. All other defaults retain the
original dimensions and input formulas; explicitly fused input expressions use
C fma to preserve intended rounding.
