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
Current integer/1000 validation with the corrected ISA, captured runtime profile,
and the explicit warm shared cache retires 2,250,441 instructions and stops at
`MRS X8, S3_6_C15_C1_5` at PC `0x180466fbc`, with no guest
stdout/stderr. It has executed real dyld code, consumed captured kernel policies,
resolved selected runtime directories, queried main-file metadata, opened/mapped the same immutable executable bytes,
and resolved their canonical path. It executes scalar BICS, captured non-enforced
BTI and vector REV64, observes the captured stat64 absence of the disk libSystem
path, and resolves the library in the real shared cache. The guest then constructs
its version-7 kernel page-in linking request; the modeled kernel installs a private
format-6 fixup pager over the signed main data page and applies BSD mprotect.
It subsequently executes native-validated UXTL/UXTL2, BSL, SMAXV and LDAPR,
and reads the captured DTrace DOF capability. It opens a private helper descriptor
and registers DOF metadata using the captured lazy mode, without host device
forwarding. Correct Mach VM_PROT_COPY handling preserves private dirty data and
allows validated non-executable shared-cache data to become writable. The captured
access(F_OK) absence and exact TPRO capability byte lead to an unimplemented
Apple thread-protection register read. Earlier
bring-up counts followed incorrect COPY error returns and are not comparable
progress measures. This remains incomplete startup, not benchmark
main or successful exit. The old superseded bring-up dataset has been removed. Current whole-program
Mach-O calibration data will be retained after successful execution is validated.

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

The bring-up collector emits native correctness and three raw `wait4` user-time
observations per case as a CI artifact. These are diagnostic samples, with no
sample-quality acceptance or accuracy claim. Its Python `Popen` launch takes the
fork/exec path on the recorded host, so child pre-exec overhead is not excluded.
The final calibration protocol must use explicit spawn and verify its accounting
boundary. Incomplete simulator startup must not be compared to native whole-program
user time. Source, binary and dyld hashes identify the inputs; there is currently
no checked-in whole-program Mach-O calibration dataset.

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

### Scalar correctness and launch descriptors

Native scalar oracles cover 7,168 CNEG conditions/flags/width/alias combinations,
3,584 register-count LSL/LSR/ASR/ROR cases, and 48 REV16/REV32 cases. CNEG now
negates when its displayed condition holds. The six historical CNEG opcode
compensations in Canneal, VIPS and x264 were restored to their original compiler
conditions together with this fix. No benchmark algorithms, input sizes, ELF
binaries or CPU parameters changed. All 88 strict historical baseline cases pass
with their existing counts, digests and cycles; no golden values were changed
for this correction. Register-count preferred aliases now read
and mask the count register; treating those operands as immediate zero had made
dyld compare `__PAGEZERO` and `__LINKEDIT` equal. The actual comparison sequence
and a delayed-load shift-count dependency are regression tests.

The launch profile now records explicit `/dev/null` stdin and two distinct empty
regular output files. Native child `dup2` actions establish those descriptors;
Poros owns private buffers and shared open-file descriptions for dup aliases.
BSD339 fstat64 and BSD4 write have native 10/18-case oracles. Combined output is
bounded (default 1 MiB), retained in reports as base64, and never written through
host guest descriptors. Post-write stat timestamps/blocks and partly accessible
copy buffers remain explicit unsupported cases. Native temporary-file metadata
is per launch, not a promise of identical inode/timestamps across samples.

BSD338 stat64 resolves only captured main-executable aliases. Its optional
metadata is captured from the same open file descriptor whose bytes are hashed;
before/after identity, size, timestamps and hashes reject changed/replaced input.
The profile must match the loaded image's SHA, size and inode. Eleven native
stat64 cases verify the full 144-byte payload, errors and output boundaries;
partly writable outputs remain unsupported before mutation. Unknown paths never
become fabricated ENOENT results or implicit access to the host filesystem.


### Immutable main-file access

The Mach-O loader owns one complete file snapshot. Parse copies caller input;
Load takes ownership of its bounded read. Segment copies initialize private guest
memory, while `ReadAt` reads the unchanged original file including universal
container bytes. Neither caller input changes, writable guest segments nor
private mmap writes can alter subsequent file reads. Each open owns an offset;
dup aliases share it. Closing a descriptor leaves existing mappings alive.

BSD3 read has 24 native cases, including original-count validation before FD
lookup, short reads, EOF with invalid output pointers, shared/independent offsets,
and output bounds. BSD197 supports the observed private file mappings, including
aligned file offsets, native max protection, lazy backing, final partial-page zero
tail, and independent private writes. Fifteen native mapping cases and five native
counterexamples establish the supported boundary. Whole pages beyond EOF,
MAP_FIXED replacement, legacy unaligned/zero-length/no-sharing forms, nonzero
hints and WRITE-only protection remain explicitly unsupported before mutation.
The latter cases have native behavior different from the supported UNIX03 path;
they are not converted to fabricated EINVAL or zero-filled mappings. Execute
transitions on owned memory require a future explicit signing/JIT policy.

Scalar BICS has 4,608 native width/shift/alias/NZCV cases. Its NZCV destination and
CSET/CINC flag sources participate in timing dependencies. A delayed-load chain
checks the flag consumers against an independent instruction. The separate
80-kernel-case plus 8-pipeline-case audit passes with unchanged strict baseline
counts, digests and cycles; no golden values changed for this correction; this remains
functional bring-up with no whole-program accuracy claim.


BTI is explicit architectural effective state: unspecified or enforced landing
checks remain unsupported. The captured non-enforced mode applies only to a
generic ARM64 main and a matching native dyld identity. It does not claim the CPU
lacks FEAT_BTI (this M2 reports the feature present) or implement future guarded
mappings. Four BTI forms preserve all state in 64 native NZCV cases; ten native
branch checks include plain and mismatched landing targets plus the real dyld
stack-probe entry and the instruction after its BTI. Capture repeats those checks
in owned children, discovers the stack-probe symbol from bounded UUID-matched
Mach-O metadata, verifies its native opcode, and binds the result to runtime/cache
identity and source/binary hashes. Unknown HINT instructions still fail closed.

The page-in linking model verifies the original mapped file identity through
permission splits, complete-file offsets for universal binaries, and coverage
against the captured CDHash and SHA256 CodeDirectory page hashes. Private writes
are discarded on pager replacement and subsequent writes remain private; no host
file descriptors or mappings are used. Chain metadata and the original file
snapshot remain immutable. Missing segment coverage, out-of-page chains, invalid
bind ordinals and unmodeled formats fail explicitly. Seven native startup pointer
observations independently check two rebases and puts/printf/malloc binds,
including unsigned addends 7 and 255; the owned helper's dyld diagnostic confirms
kernel page-in linking. This is startup end-state evidence, not a raw syscall
return or performance observation. Post-startup calls are disabled by the real
shared-region sentinel. Twenty native BSD mprotect observations cover range,
alignment, maximum-protection and error behavior; extra protection flags and
execute transitions remain unsupported.

Stat ENOENT observations apply only to exact captured stat64 paths, never to open,
lstat or other unknown names. They are checked again before/after collection
operations, and conflicting known file/directory identities are rejected.

The collector uses an explicit 10-million-instruction/10-second execution budget
for this later startup checkpoint (host runtime hashing remains outside that
execution budget). An unexpected memory fault still fails CI. Before raising,
it saves the native samples and simulator witness, then invokes an owned
standalone diagnostic harness with the same inputs. That bounded diagnostic
records the last 16 instructions/syscalls, arguments/results, original/current
mapping protections, and whether main was reached; it uploads no Apple binaries.
The macOS 15 CI runtime progressed beyond its previously unsupported BSD mprotect
and exposed a later write-protection fault; this remains an unresolved failure,
not an accepted bring-up outcome or a reason to weaken memory protection.

LDAPR retains a distinct RCpc load opcode in functional and timing paths. Native
value/NZCV, alias/ZR, access-fault, within-16-byte alignment and 6,000 reservation
trials support its implemented behavior. The pipeline currently serializes
acquire operations conservatively; this is not a calibrated throughput claim.
The DTrace DOF byte is explicitly captured at commpage offset 0x92, including a
native value of one; absent profile fields remain unmapped rather than defaulting
to disabled. BSL reads its old destination mask in the timing dependency graph.


The per-image diagnostic constructor now validates both AMFI and lazy DTrace
helper behavior, with unchanged executable bytes and matching code-signing state.
An owned invalid DOF header must be accepted without parsing, its field must be
replaced by a generation, and removal of that exact owned generation must succeed.
Raw acceptance/cleanup/signing checks are retained in capture provenance. These
instrumented runs remain excluded from native timing; final measurements must
recheck applicability before and after collection. Uncaptured or eager helper
modes remain unsupported. The process owns registration metadata across descriptor
close/dup; it does not simulate a tracing consumer. Native open, eleven ioctl,
ten lifecycle and registry-limit observations cover generation reuse, input-size
bounds, low-word REMOVE arguments, full-width ioctl commands and registration
commit before failed copyout. Partial copyout remains explicitly unsupported.

Mach VM_PROT_COPY is a real private-copy request, not an invalid protection bit.
The bounded implementation reuses existing private AddressSpace pages when the
maximum already permits write. It preserves dirty bytes, backing-file isolation
and current/max protection semantics, and decodes scalar ABI arguments at their
native widths. Native sixteen-case protection, three maximum-protection cases and an independent private-file probe
cover this path. Maximum expansion, executable COPY and synthetic commpage/dynamic
fields remain unsupported. Shared-cache DATA is admitted only through its validated
CacheMapping backing, including protection splits; holes and replacement backings
cannot masquerade as that mapping. The hosted macOS15 failure diagnosed a legitimate
COPY request that previously returned a false KERN_INVALID_ARGUMENT; the memory-fault
acceptance gate remains strict.


The selected `/AppleInternal/XBS/.isChrooted` lookup is modeled only as its captured
`access(F_OK)` result. It has a separate namespace from stat64 observations,
requires native runtime identity, and is rechecked with native access before and
after collection. Other modes, uncaptured paths and host filesystem traversal
remain unsupported; no lookup is forwarded during simulated execution.


The exact byte at commpage `+0x10c`, consumed by the matching installed
`_dyld_call_with_writable_tpro_memory`, is captured read-only and bound to native
runtime identity. Its observed value is preserved, including zero when actually
captured; absence leaves the byte unmapped. This is installed-binary evidence,
not a claimed published XNU field definition. Adjacent unknown bytes remain
unmapped, and capturing the capability does not implement unobserved TPRO
register operations or authorize them as no-ops.
