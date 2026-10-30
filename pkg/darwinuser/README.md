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
The shared cache needs a separate sparse mapping implementation; increasing
these bounds to eagerly allocate an entire runtime cache is not the design.

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

The current local M2 runtime executes its own chained self-fixup loop and stops
at the first unimplemented Mach trap, `task_self_trap` (-28), after 87,073 user
instructions. It has not yet reached benchmark main or completed a program.
The checked-in native observations are separate from this incomplete execution.

References: [XNU executable activation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_exec.c),
[XNU commpage definitions](https://github.com/apple-oss-distributions/xnu/blob/main/osfmk/arm/cpu_capabilities.h).
