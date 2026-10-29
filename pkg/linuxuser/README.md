# Static AArch64 Linux startup

`Load(path, Options)` and `Parse(bytes, Options)` create a fresh `Process` with
initial general registers and a checked address space. `loader.LoadELF` /
`ParseELF` are the lower-level image parsers. They inspect program headers only;
stripped or absent section headers do not affect loading. No emulator, syscall
handler or timing model is attached in this stage (#45); #46 supplies execution.
The existing Mach-O extraction and function-level emulator are unchanged.

## Supported profile

- ELF64, little-endian, AArch64, version 1, `ET_EXEC`, System V or Linux OS ABI.
- Static non-PIE images with nonoverlapping, ascending, page-rounded `PT_LOAD`
  mappings. The file offset and virtual address must agree modulo 4 KiB and any
  declared alignment. Invalid bounds, permissions and arithmetic overflow fail.
- The program header table must be file-backed in a readable load segment so
  `AT_PHDR` is a real guest pointer. Explicit `PT_PHDR`, when present, is checked
  against that mapping and must precede load segments.
- Entry is 4-byte aligned and covers a complete instruction inside an executable
  segment. This is not an instruction-set coverage check.
- File-backed segments include the first mapped page's prefix. Pure BSS
  (`p_filesz=0`) maps anonymous zero pages, including the prefix. BSS and the remainder of
  its last page are zero. Without BSS, the mapped file page tail is retained;
  bytes beyond EOF are zero. Access permissions apply to complete 4 KiB pages.
- `PT_NULL` and bounded `PT_NOTE` metadata are ignored. Interpreter, dynamic,
  TLS, RELRO, GNU properties and unknown program header types are explicitly
  unsupported. This stage does not silently promise those runtime semantics.
- Missing `PT_GNU_STACK` means a non-executable RW stack. An explicit stack
  header must request RW only. No automatic heap, guard-page mapping or vDSO.

Resource bounds are explicit: 16 MiB file, 128 program headers, 64 MiB total
page-rounded ELF image, guest addresses from 4 KiB through `2^48-1`. The stack
ends at `0x0000fffffffff000`, defaults to 1 MiB and permits page-aligned sizes
from 4 KiB through 8 MiB. ELF/stack overlap fails. Unmapped accesses are errors; there is no implicit stack growth. These are initial simulator limits, not
claims about every Linux executable. The process is returned only after the
entire image and startup state have passed validation.

## ABI state

PC is the ELF entry, SP is 16-byte aligned, and all initial GPRs are zero,
including x0 (no dynamic-linker cleanup function). An eventual execution backend
must also initialize FP/SIMD state to zero. Arguments are on the stack, not in
x0/x1. Memory holds argc, argv pointers + NULL, envp pointers + NULL, auxv +
AT_NULL, and NUL-terminated strings. Empty Args becomes a single empty argv[0],
as modern Linux execve does. Environment is exactly the supplied list. Each
list is bounded to 4096 entries and the combined NUL-terminated string payload
(including executable name) to 128 KiB. Embedded NUL and stack overflow fail.

Auxv supplies PHDR/PHENT/PHNUM, PAGESZ, BASE=0, FLAGS=0, ENTRY, UID/EUID/GID/EGID
(fixed simulated unprivileged identity 1000), PLATFORM="aarch64", HWCAP/HWCAP2=0,
CLKTCK=100, SECURE=0, RANDOM and EXECFN. No host capability or identity leaks in.
`Options.Random` explicitly provides 16 deterministic AT_RANDOM bytes; its zero
default is reproducible simulation data, **not cryptographic entropy**. ASLR,
real credentials, hardware capability discovery and dynamic runtime support
are outside this profile.

## Address space and errors

`Memory.Read`, `Write` and `Fetch` take guest addresses and caller-owned buffers.
They enforce R, W and X independently and validate the entire request before
copying: a fault never leaves a partial write or partial output buffer. Reads
never allocate memory. Zero-length accesses are no-ops. `Mappings` returns only
metadata copies, so callers cannot bypass permissions through backing slices.
`loader.ELFError`, `StartupError` and `MemoryError` support `errors.As` alongside
human-readable diagnostics. Lower-level parsed image fields are caller-owned;
Process constructors always parse fresh bytes and never trust a caller-mutated
image. Each process owns independent backing storage.

## Validation and references

Tests verify the real compiled ELF, page bytes/BSS/permissions, stack pointers,
strings, auxv, registers, empty/bounded startup inputs and malformed files. Fuzz
seeds exercise the same parser. Existing 90 kernel cases remain unchanged.

- [ELF program header specification](https://gabi.xinuos.com/elf/07-pheader.html)
- [Linux create_elf_tables](https://github.com/torvalds/linux/blob/master/fs/binfmt_elf.c)
- [Linux execve argument normalization](https://github.com/torvalds/linux/blob/master/fs/exec.c)
- [Linux AArch64 ELF definitions](https://github.com/torvalds/linux/blob/master/arch/arm64/include/asm/elf.h)
