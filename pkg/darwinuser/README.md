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
