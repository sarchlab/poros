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
bytes. Protection transitions respect each image segment maximum. The cache
parser and warm shared-region integration are still pending.

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
The next local stop is CASA in the real pthread runtime at `0x70007dd34`, after
88,856 user instructions for integer/1000. It has not reached benchmark main.
The checked-in native observations remain separate from incomplete execution.

The byte-vector ISA slice includes ORR, EOR, CMHI, UMINV, CNT, UADDLV and the
zero `MOVI Vd.2D` form. Owned native tests record 256 input/output vectors and
16,896 scalar comparisons; portable tests replay native outputs, overlap and
upper-register clearing cases. Other SIMD forms continue to fail explicitly.

References: [XNU executable activation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_exec.c),
[XNU commpage definitions](https://github.com/apple-oss-distributions/xnu/blob/main/osfmk/arm/cpu_capabilities.h).

Kernel references: [platform TLS](https://github.com/apple-oss-distributions/xnu/blob/main/osfmk/arm64/machine_routines.c),
[crossarch namespace](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_crossarch.c),
[CSR query contract](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_csr.c).
