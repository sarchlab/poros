# Ordinary Mach-O workload bring-up

The four checked-in files are ordinary ARM64 macOS executables linked with
libSystem and the system dyld. They use the same six-point coordinates and
independent result oracles as `tests/usertime/manifest.json`. Linux sources,
ELF binaries, reference measurements and accuracy reports remain unchanged.

`python3 tests/macho/build.py` uses Apple Clang and the installed macOS SDK.
`build.json` records the toolchain, options, source hashes and binary hashes.
Two consecutive builds on the recorded toolchain produced identical bytes.
The build stages unchanged workload sources and the shared runtime's numeric
parser; it substitutes only the output function with an ordinary `write` call.
For Branch, distinct empty assembly compiler barriers keep both branch arms
opaque to if-conversion. They emit no instructions; disassembly confirms an
actual `tbz` of bit 16. The ordinary C `main` adapts argc/argv to `guest_main`.
All initialization, arithmetic, output and exit remain native program work.

Current simulator status: the loader maps the executable and explicitly
supplied dyld, creates the kernel-to-dyld argument stack and starts dyld.
The installed runtime presently stops at its first unsupported instruction.
This is **not successful benchmark execution or a whole-program accuracy
measurement**. Missing dependencies remain failures with an exact guest PC.

```
go build -o /tmp/poros ./cmd/poros
/tmp/poros macho --dyld /usr/lib/dyld tests/macho/integer.macho 500000
python3 scripts/macho-native.py --poros /tmp/poros --output /tmp/macho-native.json
```

`native-m2.json` preserves native correctness and three raw `wait4` user-time
observations per case on the recorded Apple M2/macOS runtime. No sample-quality
acceptance or accuracy claim follows from these observations. Incomplete
simulator startup is recorded separately and must not be compared to native
whole-program user time. Source, binary and dyld hashes identify the inputs.

Apple system binaries are not redistributed. Portable tests use independently
constructed fixtures from `internal/machotest`; these are not substitute native
runtimes. Native checks require ARM64 macOS and inspect that machine's real dyld.
