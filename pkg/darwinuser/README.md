# Darwin user-process bring-up

`Load` requires a normal Mach-O executable and an explicitly supplied dyld file.
It snapshots and validates both files, maps segments/BSS with Mach permissions,
reserves inaccessible segments, and creates a bounded stack containing the
main Mach header, argc, argv, environment and Apple executable-path vector.
Only explicitly supplied guest strings are used. Guest load-command paths are
metadata; they do not authorize arbitrary host filesystem access.

The executable retains its link address. Dyld receives the documented fixed
`DyldBase` slide; the stack uses `StackTop`. These deterministic choices are
reported simulator policy, not measured macOS ASLR. Dyld's LC_UNIXTHREAD entry
is the initial PC; LC_MAIN is metadata for dyld, never a shortcut to main.
The runtime's existing relocation instructions must execute normally.

`Execution` implements the same `emu.Execution` interface used by both CPU
models. Current CLI exposure is functional bring-up only. Unsupported ISA and
system interfaces fail explicitly; no imported function is replaced by a host
implementation. Successful full-program execution, Darwin syscall/cache
support, and timing integration are subsequent work within this draft change.

The loader does structural validation, not native code-signature enforcement.
It bounds individual files to 64 MiB and mapped bytes to 128 MiB per image.
Reader mappings in `usermem` now provide bounded lazy private pages for the
shared-cache loader. The backing files must remain open and unchanged; every
store validates permissions and loads all affected pages before modifying any
bytes. Protection transitions respect each image segment maximum. The explicit cache handle parses bounded split-cache tables and slide-v5 chains,
keeps pages private per process, and models the warm shared-region query. The
caller owns its lifetime and closes it after all borrowing processes finish.

Startup ABI: [Apple dyldStartup.s](https://github.com/apple-oss-distributions/dyld/blob/main/dyld/dyldStartup.s).

## Pointer authentication and virtual time

The main executable CPU subtype determines pointer-authentication mode. Generic
ARM64 uses disabled address authentication, as XNU does; arm64e main programs
retain enabled mode and fail explicitly until its semantics are implemented.
The disabled profile preserves PAC/AUT raw encodings and instruction retirement,
while their effective operation has no register effect. Authenticated branches
still perform their real branch, link and return operations. This is process
state, not a blanket instruction bypass. Native wrong-key/modifier tests differ
between arm64 and arm64e; owned probes and observations are in `tests/macho/`.

`--runtime-profile` supplies selected native commpage fields. The collection
helper reads documented nonsecret capabilities only. Unmodeled fields remain
unmapped; they are never silently populated with zero. The dynamic continuous
clock offsets are explicitly zero in the virtual epoch. Functional execution
advances the configured virtual counter one tick per retired instruction and
uses the native timebase ratio; this is deterministic functional time, not a
cycle-calibrated clock or a host wall-clock sample. Timing mode must supply its
own consistent counter progression before a timing accuracy claim is possible.

The current local M2 runtime executes its own chained self-fixups and the real
platform string routines. Modeled kernel calls provide task/thread/reply port
names, thread ID/TLS, page-rounded `mach_vm_protect`, native-architecture
crossarch errors, and CSR queries against an explicitly captured configuration.
SVC immediates are checked by the OS runtime; they are not architectural state.
With an explicit matching warm shared cache, the next local stop is unsupported Mach trap -19 (`mach_port_mod_refs`)
at `0x1804885e4`, after 1,183,915 user instructions
for integer/1000. It has not reached benchmark
main. CAS/SWP execute as real read-modify-write operations; DMB/DSB/ISB preserve
ordering. Both timing pipelines serialize atomic/barrier operations
conservatively. Their latency has not been calibrated against native hardware.
The checked-in native observations remain separate from incomplete execution.

The byte-vector ISA slice includes AND, ORR, EOR, CMHI, CMEQ, UMINV, CNT, UADDLV and the
zero `MOVI Vd.2D` form. Owned native tests record 256 input/output vectors and
25,088 scalar comparisons; portable tests replay native outputs, overlap and
upper-register clearing cases. Other SIMD forms continue to fail explicitly.

References: [XNU executable activation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_exec.c),
[XNU commpage definitions](https://github.com/apple-oss-distributions/xnu/blob/main/osfmk/arm/cpu_capabilities.h).

Kernel references: [platform TLS](https://github.com/apple-oss-distributions/xnu/blob/main/osfmk/arm64/machine_routines.c),
[crossarch namespace](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_crossarch.c),
[CSR query contract](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_csr.c).

The native runtime profile records measured CAS alignment behavior, independently
of CPU feature-bit interpretation: this M2 accepts a 32/64-bit data access within
one 16-byte granule, and faults on a crossing access. SP-based memory accesses
also require a 16-byte-aligned SP. Ordinary Linux/default emulator callers retain
their existing defaults. Native CAS/SWP results, alias cases and 64 offset cases
plus 16 SP-offset cases are replayed by portable tests.

`getentropy` produces bounded deterministic guest bytes from `--entropy-seed`
(default zero), with native Darwin size/pointer error semantics. This stream is
not cryptographic randomness. Together with the fixed virtual address layout,
it is a reproducibility policy, not a claim to reproduce the native process's
random allocator addresses or exact startup path lengths.

## Warm shared cache and pointer stripping

`--shared-cache` maps validated root/subcache files as kernel-provided initial
state, with zero slide and private lazy pages. The guest still executes dyld.
Root header placement, VM/file bounds, UUIDs, safe suffixes, maximum protections,
and slide chains are validated. Limits are 128 files, 256 mappings per file,
4096 mappings overall, 32 GiB input bytes, 64 GiB virtual extent and the existing
256 MiB resident lazy-page bound. All used file hashes are reported. Inputs must
remain unchanged; metadata checks detect ordinary replacement/truncation, not
adversarial same-size/mtime rewriting. This is not a whole-file snapshot.

The native runtime helper captures defined dynamic-region fields and binds them
to the cache UUID and extent. Magic/version are preserved; unknown v4 bytes are
unmapped. A mismatched profile or collision with a reserved image span fails.
Null shared-region detachment remains unsupported, rather than fake success.

XPACI/XPACD use an independently captured effective address-strip policy. On
the reference M2 it has 47 payload bits, bit 55 selecting the upper region and
lower-region data top-byte tags preserved. This applies independently of PAC
being enabled or disabled. Unspecified policies fail closed. It does not add
implicit tagged-memory translation. Both timing scoreboards track the unary
read/write dependency. LDAR/STLR use checked memory transactions and the
measured atomic-alignment policy, with conservative acquire/release ordering.

## Anonymous mappings and local exclusive accesses

The anonymous `mach_vm_map` trap supports fixed/anywhere reservations with
allocation tags, arbitrary alignment masks, page rounding, zero-filled private
lazy pages and maximum-protection checks. First-fit allocation is deterministic;
the complete shared region (including holes), commpage, inaccessible image spans
and stack stay reserved. Limits are 8 GiB anonymous virtual space and 4096
anonymous mappings, within the shared 256 MiB resident-page limit. Unsupported
flag families and executable anonymous allocation remain explicit errors.
Kernel copyin/map/copyout order is preserved: a failed result copyout leaves its
mapping installed. Eleven owned native trap cases check errors and side effects.
`getpid` returns 100 in the private single-process namespace. Current BSD calls
clear their second result register; Mach/platform conventions remain distinct.

The captured native runtime identity contains the actually running dyld UUID and
CPU subtype. A supplied file selecting another dyld slice is rejected.

LDXR/LDAXR/STXR/STLXR and CLREX use an explicit measured M2 local-monitor profile.
The reservation covers one 64-byte granule; same-granule second exclusive loads
can satisfy an original-address store, without an invented exact-width rule.
Access alignment follows the independently captured 16-byte policy, with SP 16
checks. Ordinary stores into the reserved granule clear it. Retired CAS/SWP/LDADD operations
clear it even on comparison mismatch or a different granule; handled SVC also
clears it. Failed local
reservations still validate write permission; faulting instructions do not
partially store or retire. This models the single owner of a private process,
not multicore global monitors or asynchronous invalidations. Native probes
validate reservation interactions, permissions, 192 alignment cases and raw SVC.

The required vector slice now also covers MOVI 2S/4S with LSL immediates, lower-half
XTN narrowing and checked LDNP/STNP pairs. Pair transfers retain all-or-nothing
fault behavior; both destinations/sources reach timing dependencies. Cache
allocation hints and ISA latencies remain uncalibrated; no parameter was fitted.

## Deallocation, dyld metadata and vector string reads

`mach_vm_deallocate` removes page-rounded mappings, preserves holes, splits
surviving backing ranges, and releases unused private lazy pages. Reallocating
the address gets fresh zero-filled storage. Image/stack reservations and anonymous
accounting shrink accordingly. Shared-cache and commpage submap removal remains
unsupported. Native tests include high unmapped ranges (successful no-op),
overflow, inaccessible mappings and ranges spanning holes.

The loader records the bounded `__all_image_info` section from dyld's `__DATA`
or `__DATA_DIRTY`. `proc_info` call 15 registers this task's new address/size,
without reading the buffer, then finalizes metadata according to XNU's
[task_set_dyld_info state machine](https://github.com/apple-oss-distributions/xnu/blob/main/osfmk/kern/task.c).
Foreign PID, null pointer, overflow and later updates return EINVAL. Other
proc_info calls remain unsupported.

Matching installed dyld code identifies v4 dynamic-region offsets 88/92 as
cryptex/cache path lengths including NUL. The native profile captures these
fields and each nonnull path's 16-byte-aligned read envelope, as used by its
vector strlen. Actual padding is preserved and checked against known fields;
unknown bytes 80–87 remain unmapped. This v4 evidence comes from the identified
native binary and read-only observations, not an assumed public v3 layout.

The executed ISA slice additionally includes scalar BFXIL, RBIT/REV and CSINV,
LDADD with real read/write memory events, and vector element insertions (MOV/INS)
with old-destination dependencies. Native vector outputs cover all B/H/S/D lane
indices and aliases. Conservative atomic serialization remains uncalibrated.
