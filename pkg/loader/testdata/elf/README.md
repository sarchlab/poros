# Reproducible ELF startup fixture

`startup.elf` is a real GNU-linked AArch64 Linux executable. `start.S` and
`layout.ld` produce three distinct RX, R and RW load mappings, initialized data
and a 64-byte BSS. `_start` checks stack alignment and initial BSS before issuing
Linux exit (syscall 93). This fixture is the loader/startup-layout witness.
End-to-end process execution is covered separately by [tests/elf](../../../../tests/elf/README.md),
including its own argv/env/auxv-focused startup executable.

Recorded build toolchain on macOS arm64:

- `aarch64-linux-musl-gcc (GCC) 14.2.0`
- `GNU ld (GNU Binutils) 2.44`
- Command/flags and deterministic environment are in `build.sh`.
- `provenance.json` hashes the source, linker script, build script and full ELF.
- `.file "start.S"` prevents the compiler's random temporary object filename
  from entering the ELF symbol table. Build IDs are disabled; no timestamps or
  host paths are embedded.

Rebuild with the recorded toolchain, for example:

```sh
CROSS_COMPILE=/opt/homebrew/opt/musl-cross/bin/aarch64-linux-musl- ./build.sh /tmp/startup.elf
cmp startup.elf /tmp/startup.elf
```

Two independent builds were compared byte-for-byte. A different compiler/linker
version must not silently replace the recorded fixture. Normal Linux and macOS
Go tests read this checked-in ELF and validate all hashes; no cross-compiler is
needed and a missing artifact fails the test. Header mutations exercise negative
cases and section-header-free loading without replacing this real build witness.
