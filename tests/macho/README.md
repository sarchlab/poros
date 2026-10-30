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

The loader maps the executable and explicitly
supplied dyld, creates the kernel-to-dyld argument stack and starts dyld.
The retained 24-case checkpoint from `7556bc3`, using captured native runtime
and per-executable startup profiles and an explicit warm shared cache, reaches BSD trap 381 (`__mac_syscall`,
Sandbox query) after 1,471,645 user instructions for integer/1000. It has executed
the real cached dyld, retired the original dyld mapping, managed reply rights,
and reserved its released address range through a real `mach_msg2`/`mach_vm_map`
request. The 24 retained cases reach 1,471,269–1,556,144 instructions at that same
read-only syscall-permission query. They have not reached benchmark main.
That snapshot predates the selected sandbox-query handler. Current single-case
validation completes the selected NAME2OID and captured integer-value queries,
then consumes the explicitly captured AMFI policy and executes its SIMD flag
packing, reads captured kernel settings and uses the selected runtime directory
namespace. Integer/1000 now consumes the captured v4 roots bitmaps and cache file identity,
queries its captured code-signing status, and allocates anonymous memory. It reaches
BSD339 (`fstat64`) on stderr in dyld logging setup after 1,513,063 instructions.
This remains incomplete startup.
The 24-case samples have not been relabeled as a run of this newer collector.

`scripts/macho-startup.c` captures documented kernel apple metadata from each
explicit executable as a newly spawned, suspended child, then kills/reaps that
child without running its code. Fixed safe argv and an empty environment prevent
arguments from spoofing metadata keys. Only nonsecret selected fields are
retained; actual host stack guards, malloc entropy and pointer-munging values
are omitted. The guest builds these from the recorded deterministic entropy seed,
uses virtual stack addresses/port names, and retains native stack reservation
geometry, file identities, CDHash, bootHash, PFZ pointer and security configuration.
Generic arm64 does not synthesize `ptrauth_disabled`; that kernel key describes
a different downgraded-arm64e launch. An observed `arm64e_abi` library hint is
preserved independently of main subtype.

`--startup-profile` binds this metadata to exact executable/dyld SHA identities.
Capture verifies hashes before/after suspension and canonical identity resolution,
and rejects changed files throughout measurement/simulation. Four profiles are
hoisted once in the retained dataset with per-case hash references. BSD `fsgetpath`
resolves only these explicit guest identities, never arbitrary host filesystem
paths. Its 16 native cases cover copyin/error precedence, low-word FSID behavior,
NUL-inclusive length and unchanged output on failure. Native submap evidence and
XNU's arm64 contract reserve the entire [63GiB,64GiB) commpage region; unobserved
PFZ instructions remain unavailable rather than invented. Successful bring-up
alone will not establish equivalent native startup timing.
This is **not successful benchmark execution or a whole-program accuracy
measurement**. Missing dependencies remain failures with an exact guest PC.

```
go build -o /tmp/poros ./cmd/poros
/tmp/poros macho --dyld /usr/lib/dyld tests/macho/integer.macho 500000
# Omit --runtime-profile to diagnose the first required commpage read.
# The collection command below captures and supplies a matching host profile.
python3 scripts/macho-native.py --poros /tmp/poros --output /tmp/macho-native.json
```

`native-m2.json` preserves native correctness and three raw `wait4` user-time
observations per case on the recorded Apple M2/macOS runtime. No sample-quality
acceptance or accuracy claim follows from these observations. This bring-up
collector uses Python `Popen`; on the recorded host it takes the fork/exec path,
so child pre-exec overhead has not been excluded from its diagnostic samples.
The final calibration protocol must use explicit spawn and verify its accounting
boundary. Incomplete
simulator startup is recorded separately and must not be compared to native
whole-program user time. Source, binary and dyld hashes identify the inputs.

Apple system binaries are not redistributed. Portable tests use independently
constructed fixtures from `internal/machotest`; these are not substitute native
runtimes. Native checks require ARM64 macOS and inspect that machine's real dyld.

`isa-native-m2.json` records owned native bitfield and pointer-authentication
probes. The signed/unsigned extraction cases independently confirm the UBFX/
SBFX alias-width semantics used by dyld's fixup walker. PAC negative cases are
run both as arm64 and arm64e: the former has authentication disabled; the latter
traps on incorrect authentication. These are functional ISA/ABI observations,
not timing measurements. Probe sources are included for inspection/rebuilding.

`--shared-cache /absolute/path/to/dyld_shared_cache_arm64e` supplies a matching
warm cache to the CLI or native collection harness. Cache files are read-only,
SHA-bound inputs; no Apple runtime files are committed. The retained data hoists
common file identities and runtime state once, with per-case SHA references.
The 120-second harness deadline includes host hashing/setup; guest instruction
and runtime limits remain independent. Hosted checks without this option retain
the explicit unsupported shared-region query witness.

Owned XPAC probes replay both instruction/data strip results on tagged and
noncanonical synthetic values in arm64 and arm64e. LDAR/STLR probes replay 192
native width, alignment, base-register and value cases. These extend functional
ISA evidence; release/acquire and pointer-strip latency are not calibrated.

`kernel-native-m2.json` retains the owned raw Mach mapping and SVC/exclusive
contract probes. `isa-native-m2.json` also contains exclusive-monitor granule,
alignment and permissions cases, MOVI/XTN vectors, and 288 independently checked
non-temporal load/store pairs. Full 128-bit vector results and raw probe outcomes
are retained with source/binary hashes. These are functional semantic evidence.
The bring-up collector records explicit instruction, execution-time and outer
setup budgets; reaching a budget rejects the collection rather than counting as
a discovered runtime dependency. Defaults are 2M instructions, 10s simulator execution
host time and 120s including runtime hashing.

Runtime ISA coverage includes independently replayed BFXIL (640), RBIT/REV (544),
CSINV (1536), LDADD values (160), LDADD monitor interactions (136), and vector
lane inserts (1360). Element insertion preserves all other destination lanes;
timing tracks that old-destination dependency. Fourteen native deallocation
cases verify both readability and mapping removal. Eight post-startup dyld-info
registration cases verify finalized metadata rejects changes; first-startup
registration is checked with the documented kernel state machine and owned
Mach-O section fixtures, not claimed as directly native-observed.

The kernel namespace now counts self-port send references and retains ordinary
and typed reply receive rights, including their context and queue limit. Owned
native probes cover self-reference acquisition/release, 24 null/dead-name cases,
12 construction cases, and 14 BSD `munmap` cases. Construction keeps an allocated
right when its final name copyout fails; unmapping updates both actual mappings
and allocator reservations. Destruction/guard exceptions remain unsupported.

The synchronous task-kobject message path decodes real `mach_msg2` packed
arguments and the `mach_vm_map` MIG payload. The same VM allocator serves both
the fast trap and full RPC, including current/maximum protection clamping.
Thirty-one native exchanges verify success/error framing, the default trailer,
partial trailers for small receive buffers, header precedence and map effects
that survive receive failure. These are kernel ABI operations, not skipped user
library calls. Other services/descriptors/options and mixed-permission partial
copyout faults remain explicit unsupported dependencies. All user runtime
instructions before each trap still execute normally.

Native SIMD coverage also includes 300 unsigned lane-to-GPR moves, 84 vector
zero comparisons, and 200 ORN vector cases, including aliases and preserved
register/flag state. Timing dependency tests link the vector load to the GPR
consumer. The accompanying ordinary-RegSP classifier fix and its complete
88-case cycle-impact audit are documented in `pkg/cpu/testdata/README.md`.

Selected Sandbox syscall decisions are now captured for each newly spawned,
suspended benchmark's own PID before dyld runs; the native PID is not retained.
The guest implements the observed read-only `syscall-unix` query from that
explicit per-image profile. Twenty-one native raw-call cases verify full
160-byte request copyin and an eight-byte result without overwriting its tail.
Only independently observed zero decisions are supported; other decisions,
unlisted syscalls, other policies/operations, and partial copyout faults remain
explicitly unsupported. The collector launches its metadata helper in an owned
process group, terminates that group and waits for the helper on every completion path, including
timeout, error and malformed output, so suspended children cannot escape cleanup.

Selected sysctl name/OID associations are captured with an optional read-only
integer value. The selected `security.mac.lockdown_mode_state` is a global XNU
`SYSCTL_INT` initialized from NVRAM, as defined in
[Apple lockdown_mode.c](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/lockdown_mode.c).
Its value is captured in the common runtime profile; it is never hardcoded or
queried from the host by guest code. The captured environment must remain stable
through native measurements; this differs from per-process Sandbox decisions.
Thirty-four native NAME2OID cases verify input termination, size-only queries,
whole-OID copyout, short-buffer errors, output protection, and error-side length
preservation. Unknown names do not imply observed absence; unsupported namespace
queries fail explicitly. Invalid non-null/read-only length pointers and mixed
copyout permissions remain unsupported because their native contract is not
established. Fourteen additional raw native integer-query cases verify exact
four-byte values, size queries, short buffers, null-length behavior and output
faults. Uncaptured values and write requests remain unsupported. Native tests
never change the host setting. The bounded string-value path is checked against
14 empty-bootargs and 14 public nonempty `kern.ostype` observations using the same
XNU string handler. Production capture verifies bootargs is empty before emitting
it, avoiding accidental retention of custom boot arguments. Unlisted settings
still fail explicitly.

AMFI dyld policy is captured separately for each exact executable. An owned
constructor runs in a diagnostic launch of the unchanged main; the collector
requires no entitlements, equal code-signing status with its suspended env{}
launch, and the original output oracle. Its modified environment and timings
are excluded from ground truth. The resulting policy and source/library hashes
are explicit provenance, not a universal allow rule or a claim to model
arbitrary hardened/sandboxed apps. Fourteen raw native cases cover the 16-byte
request and eight-byte copyout; unknown flags/inputs without a captured rule and
partial copyout remain unsupported. Both diagnostic paths share bounded process
group cleanup, tested with errors, malformed output and stopped descendants.

Additional native SIMD replay covers 252 GPR DUP cases, 240 immediate BIC cases,
616 USHL cases, 112 UZP1 cases, all 256 MOVI V.2D byte masks and 512 MOVI V.8B/16B
byte-immediate cases, plus 200 register BIC cases. The vector
operations preserve flags, handle aliases and clear the unused upper half where
specified. Timing tests verify cross-bank and old-destination dependencies;
these tests do not calibrate instruction latency.

The development-firmware flag (`_COMM_PAGE_DEV_FIRM`, offset 0x84 in both
commpage views) is captured as an optional uint32. An absent capture stays
unmapped; zero is represented only when observed. The mappings are read-only.

Directory descriptors now represent the structural guest root and explicitly
captured OS/Rosetta Cryptex and shared-cache directories. `open`, `openat`, `dup`,
`close`, `fstatat64` and `fcntl(F_GETPATH)` operate on this private namespace;
there is no host filesystem forwarding. Aliases are verified against native
inode/device identity, and `O_NOFOLLOW_ANY` on the captured symlink paths returns
the observed Darwin ELOOP (62). Unknown paths and traversal remain unsupported,
not invented ENOENT results. Relative AT_FDCWD remains unsupported until a guest
working-directory contract exists. Native captures bind the shared-cache
parent directory to its runtime path. Defined stat64 fields are serialized into
144 bytes only after native padding/reserved bytes were verified zero. Directory
alias lifetime, lowest-free-FD allocation, metadata copyout and exact path/NUL
copyout have native replay tests; unmodeled partial copyout remains explicit.
The actual collector-to-CLI check loaded a 3,194-byte runtime profile and reached
the same dynamic-region fault, with a nonzero exit status and no timing claim.

The native collector omits the optional Rosetta directory when the host reports
ENOENT. It does not install Rosetta or fabricate a guest ENOENT result: querying
an uncaptured directory remains explicitly unsupported. Other capture errors fail.

The selected v4 roots descriptors and bitmap layout are derived from the matching
installed dyld methods, not asserted as a public cross-version ABI. Native data
records 4,115 images and two 515-byte bitmaps; even all-zero values are captured,
not synthesized. Read-only mapping validates ranges, disjoint bitmaps and exact
agreement where the first bitmap overlaps the 16-byte cache-path read envelope.
Bytes 85–87 remain unknown and unmapped. The runtime JSON is bounded at 64 KiB,
with smaller field/count bounds; the captured local profile is 5,200 bytes.

Cache `fsgetpath` reuses the native dynamic-region FileIdTuple and canonical path.
Unknown identities, including unknown inodes on a captured mount, stop explicitly;
a known filesystem does not imply a complete negative file namespace. CS_OPS_STATUS
returns the per-image status already verified during startup/AMFI capture. Its
null-output and size-ignored behavior follows [XNU csops_internal](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_proc.c).
The Mach allocate and map traps share anonymous VM allocation with their distinct
argument framing and zero-size behavior. Both preserve native map-before-copyout
side effects for a read-only address slot; mixed-permission copyout stays unsupported.
Owned native probes cover these contracts, including exact output extents and
preserved Mach flags/registers. No CPU timing parameters change in this slice.
