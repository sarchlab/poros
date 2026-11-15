# Grace native user time (GH200)

Native user-time measurements of all 264 whole-program points on an NVIDIA
GH200 (Grace, Arm Neoverse V2), measured on 2026-11-14 at source commit
`3f02e1ede8522c6f2b552397aec87863bf67ae07`. `summary.json` holds one row per
point: median, P10/P90, min/max, the three batch medians, median system time,
quality and the expected output identity. This is measurement data only; it is
not attached to the `grace` device in `evaluation/devices.json`.

| | |
|---|---|
| Points | 264 (255 usable, 9 noisy) |
| Noisy | trisolv-1 … trisolv-6, fdtd2d-6, vips-1, vips-5 |
| Outputs | all 5,808 runs match the committed output SHA-256 and length |
| Typical GH200 / M2 user time | 1.04× (P10 0.71×, P90 1.49×; 255 points usable on both) |

## Why the programs were relinked

The host kernel (`6.8.0-1064-nvidia-64k`) uses 64 KiB pages. The committed ELFs
are linked with 4 KiB alignment (`tests/usertime/layout.ld`), so they crash
(SIGSEGV) when run directly on it. They were rebuilt with 64 KiB alignment; the
committed ELFs themselves are unchanged.

## How the programs were built

1. Toolchain: `aarch64-linux-musl-gcc` 14.2.0, GNU ld 2.44, musl 1.2.5, built
   with musl-cross-make 0.9.11 using the settings of the Homebrew `musl-cross`
   formula (FiloSottile/homebrew-musl-cross), without isl and kernel headers.
   Every source download matched the formula's SHA-256.
2. Check: with this toolchain, the unchanged `tests/benchmarks/build.sh` and
   `tests/usertime/build.sh` rebuild all 46 committed ELFs byte for byte, and
   `provenance.json` (compiler, linker, `libc.a`, `libgcc.a`) is identical.
3. Relink: in a separate copy, the only changes were
   `ALIGN(4096)` → `ALIGN(65536)` (twice) in `tests/usertime/layout.ld` and
   `-Wl,-z,max-page-size=65536` added to both `build.sh` scripts.
4. Comparison of committed and relinked ELFs: 46 programs, 236 functions,
   11,578 instructions. 11,211 instructions are byte-identical; the other 367
   differ only in the offset of PC-relative instructions (`adrp`, branches),
   whose targets moved with the layout. All `.rodata` and `.data` sections are
   identical.

## How the programs were measured

Following the Mach-O protocol in [the user-time README](../README.md): one
warmup per point, then 21 retained processes per point in three batches of
seven full-suite passes, each pass in a random order (seed 20261114). The
metric is direct-child `wait4.ru_utime` from `scripts/native_usertime.py`
(`child_usage`), including startup and output. Every run had to exit 0, write
nothing to stderr and match the committed output identity; nothing was retried
or dropped. The quality rule is the existing one (`native_usertime.quality`).

| | |
|---|---|
| Host | gh02, NVIDIA GH200, 72 Neoverse-V2 cores, Ubuntu 22.04 |
| Kernel | 6.8.0-1064-nvidia-64k, 64 KiB pages, HZ=1000, no `nohz_full`, no isolated CPUs |
| CPU | process pinned to CPU 36, governor `performance`, scaling max 3.429 GHz |
| Time | 2026-11-14 16:04–16:08 UTC; no other users or heavy processes at the start, load average 1.1 before and 1.2 after |

Core identity during each run and instantaneous frequency were not observed.
The 9 noisy points are short runs (6–45 ms); vips and trisolv still spend
10–30% of their time in the kernel.

The raw capture (every run), the relinked ELFs, the capture and comparison
scripts and the toolchain configuration are kept outside the repository and
are available on request. In the raw capture, `environment.linker` reads
`collect2 version 14.2.0`; the linker is GNU ld 2.44.
