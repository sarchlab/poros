# Static AArch64 Linux process execution

`Load(path, Options)` and `Parse(bytes, Options)` create a fresh `Process` with
initial architectural registers and a checked address space. `loader.LoadELF` /
`ParseELF` are the lower-level image parsers. They inspect program headers only;
stripped or absent section headers do not affect loading. `Process.Run` executes
from the ELF entry with the existing interpreter semantics and checked memory.
The existing Mach-O extraction and function-level harness remain available.
ELF execution does not yet drive the timing model (#47).

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

Resource bounds are explicit: 16 MiB file, 128 program headers, 256 MiB total
page-rounded ELF image, guest addresses from 4 KiB through `2^48-1`. The stack
ends at `0x0000fffffffff000`, defaults to 1 MiB and permits page-aligned sizes
from 4 KiB through 8 MiB. ELF/stack overlap fails. Unmapped accesses are errors; there is no implicit stack growth. These are initial simulator limits, not
claims about every Linux executable. The process is returned only after the
entire image and startup state have passed validation.

## ABI state

PC is the ELF entry, SP is 16-byte aligned, and all initial GPRs are zero,
including x0 (no dynamic-linker cleanup function). FP/SIMD and NZCV state also
start at zero. Arguments are on the stack, not in
x0/x1. Memory holds argc, argv pointers + NULL, envp pointers + NULL, auxv +
AT_NULL, and NUL-terminated strings. Empty Args becomes a single empty argv[0],
as modern Linux execve does. Environment is exactly the supplied list. `Load` defaults EXECFN to the supplied
file path; set `Options.ExecName` to the same explicit guest name when identical
startup bytes across host paths are required. `Parse` defaults it to "program". Each
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

## Execution and captured syscalls

`Process.Registers` is the authoritative full `emu.Registers`; successful steps
update it and faulting instructions leave it unchanged. `emu.StepChecked` shares
ISA handlers with the legacy interpreter, returns the decoded instruction and
branch/SVC event (`Branched` means committed PC differs from prior PC+4), and requires atomic checked memory operations. Paired stores
validate/transfer their complete span; paired loads and address writeback commit
only after success. Legacy timing access widths/order remain unchanged for previously working forms.
Single-register Q loads/stores now transfer both halves as two 8-byte accesses;
literal loads now use PC-relative addresses. These correct previously wrong
legacy behavior and have explicit functional/access-stream tests.
Buffers supplied to `CheckedMemory` are ephemeral and must not be retained.

`Run(context, RunOptions)` is one-shot. A repeated call returns an error; final
registers/memory remain inspectable. There are no application hooks or synthetic
LR halt: RET to an invalid address faults. Scalar integer/FP instructions supported
by the interpreter are available, plus Q transfers. SIMD arithmetic, unknown
instructions, nonzero SVC immediates, non-NOP HINT instructions and unsupported
syscalls terminate explicitly. Full FPCR/FPSR emulation is not provided; the
validated FP profile is the interpreter's default rounding behavior. Unscaled
GPR-to-S/D and S/D-to-GPR conversions are supported; AdvSIMD scalar-register and
fixed-point conversion forms are explicitly rejected, as are scalar SIMD integer
forms such as ADD Dd,Dn,Dm. Finite, in-range conversion inputs are the validated
profile: NaN/out-of-range float-to-integer conversions currently inherit Go's
host-dependent results rather than ARM saturation/NaN rules. Single-precision
FMADD/FMSUB/FNMADD may also vary across hosts because Go may fuse their expressions.
Do not use those inputs/operations for cross-host FP correctness claims. Binary64
FMA uses explicit single rounding and is covered by the independent ELF edge
fixture. Full conversion/FP-state fidelity remains separate work; this PR does
not substitute a double-rounded binary32 workaround.

Linux AArch64 SVC #0 uses x8 for the syscall number, x0–x2 for arguments:

- `write` (64) captures stdout (fd 1) or stderr (fd 2). The fd is truncated to
  32 bits like Linux's integer argument. Other descriptors return `-EBADF` (-9).
  Invalid readable ranges return `-EFAULT` (-14). Zero count with a valid fd
  succeeds even with a null pointer. Count is capped at Linux's `0x7ffff000`.
- Captured writes are atomic whole-buffer operations: a fault anywhere returns
  EFAULT with no output. This matches the tested QEMU user behavior but does
  **not** model Linux pipe partial writes when a large buffer crosses a faulting
  page. Host pipe blocking, signals, partial writes and scheduling are outside
  this captured-stream profile.
- `exit` (93) and `exit_group` (94) finish the single guest thread with `x0 & 255`.
- Other syscall numbers produce `unsupported_syscall`, not a fabricated return
  value. This intentionally differs from Linux/QEMU for existing syscalls.

Defaults are 1,000,000 completed instructions, a 2-second host deadline and
1 MiB combined captured output (maximum configurable output 16 MiB). Context
cancellation/deadline, instruction exhaustion and output exhaustion have distinct
termination reasons. The deadline is a host safety bound, never simulated time.
Output-limit failure emits no partial bytes and does not retire that SVC.
Successful or errno-returning syscalls count as completed instructions; faults
and unsupported operations do not. Guest failure is a structured `Result`;
invalid host configuration is a Go error. No guest output is written to host
stdout/stderr by `Run`.

```sh
go run ./cmd/poros elf --argv0 fixture --env MODE=test \
  tests/elf/startup.elf alpha beta
```

The CLI emits a JSON object with ELF SHA-256, exact argv/env, termination,
optional guest exit status, completed instructions, PC, diagnostic and base64
stdout/stderr bytes. Host environment is not inherited. CLI status 0 means guest
exit 0, status 1 means guest nonzero exit/fault/limit, and status 2 means a host
configuration/load error. Flags precede the ELF path; subsequent arguments belong
to the guest. The API can represent an empty argv[0]; CLI defaults it to `program`.

## Validation and references

Tests verify the real compiled ELF, page bytes/BSS/permissions, stack pointers,
strings, auxv, registers, empty/bounded startup inputs and malformed files. Fuzz
seeds exercise the same parser. [The functional fixture suite](../../tests/elf/README.md)
executes real ELF entry points and compares results with Linux ARM64/QEMU. It
includes atomic fault/writeback tests and independent numerical checks. Existing
90 kernel cases remain unchanged.

- [ELF program header specification](https://gabi.xinuos.com/elf/07-pheader.html)
- [Linux create_elf_tables](https://github.com/torvalds/linux/blob/master/fs/binfmt_elf.c)
- [Linux execve argument normalization](https://github.com/torvalds/linux/blob/master/fs/exec.c)
- [Linux AArch64 ELF definitions](https://github.com/torvalds/linux/blob/master/arch/arm64/include/asm/elf.h)

The maximum requested output budget is 64 MiB; the default remains 1 MiB.
Performance cases request only their complete declared output size (at least the
default), and reports explicitly identify older bases whose image or output
limits cannot accommodate that exact ELF/input.
