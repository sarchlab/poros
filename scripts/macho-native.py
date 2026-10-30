#!/usr/bin/env python3
"""Native correctness and raw user-time observations for Mach-O runtime bring-up.

This is not an accuracy report. Poros startup failures remain explicit; native
observations must not be compared to partial simulator instruction counts.
"""
import argparse
import base64
import datetime
import ctypes
import hashlib
import json
import mmap
import os
from pathlib import Path
import platform
import signal
import struct
import subprocess
import tempfile
import time

from workload_contract import check_output, digest, load_manifest
from macho_symbols import MAX_DYLD_BYTES, stack_probe_offset

ROOT = Path(__file__).resolve().parents[1]


def observe(binary, args):
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        p = subprocess.Popen(args, executable=str(binary), env={}, stdout=out, stderr=err, stdin=subprocess.DEVNULL)
        deadline = time.monotonic() + 30
        while True:
            pid, status, usage = os.wait4(p.pid, os.WNOHANG)
            if pid:
                break
            if time.monotonic() >= deadline:
                p.kill()
                _, status, _ = os.wait4(p.pid, 0)
                p.returncode = os.waitstatus_to_exitcode(status)
                raise TimeoutError("native workload exceeded 30 seconds")
            time.sleep(0.01)
        p.returncode = os.waitstatus_to_exitcode(status)
        out.seek(0)
        err.seek(0)
        stdout, stderr = out.read(), err.read()
        if p.returncode != 0 or stderr:
            raise ValueError(f"native failed: exit={p.returncode}, stderr={stderr!r}")
        return stdout, dict(user_seconds=usage.ru_utime, system_seconds=usage.ru_stime,
                            minor_faults=usage.ru_minflt, major_faults=usage.ru_majflt)


def native_access_errno(name):
    library=ctypes.CDLL(None,use_errno=True)
    query=library.access
    query.argtypes=[ctypes.c_char_p,ctypes.c_int]
    query.restype=ctypes.c_int
    ctypes.set_errno(0)
    return 0 if query(os.fsencode(name),0)==0 else ctypes.get_errno()


def verify_captured_access_absences(profile):
    for name in profile.get("access_enoent_paths",[]):
        if native_access_errno(name)!=2:
            raise ValueError("captured access F_OK absence changed: "+name)


def verify_captured_stat_absences(profile):
    """Recheck exact stat observations without inventing an open/lstat namespace."""
    for name in profile.get("stat_enoent_paths", []):
        try:
            os.stat(name)
        except OSError as error:
            if error.errno == 2:
                continue
            raise ValueError("captured stat absence changed: " + name) from error
        raise ValueError("captured stat absence became present: " + name)


def native_stat64(descriptor):
    """Decode the same defined 144-byte ARM64 stat64 ABI as macho-stat.h."""
    library = ctypes.CDLL(None, use_errno=True)
    query = library.fstat64
    query.argtypes = [ctypes.c_int, ctypes.c_void_p]
    query.restype = ctypes.c_int
    output = ctypes.create_string_buffer(b"\xa5" * 160, 160)
    if query(descriptor, output) != 0:
        raise OSError(ctypes.get_errno(), "shared-memory stat64 failed")
    raw = output.raw
    if raw[144:] != b"\xa5" * 16 or any(raw[28:32]) or any(raw[124:144]):
        raise ValueError("unsupported native stat64 layout or reserved fields")
    value = lambda kind, offset: struct.unpack_from("<" + kind, raw, offset)[0]
    metadata = dict(device=value("I",0), mode=value("H",4), links=value("H",6),
                    inode=str(value("Q",8)), uid=value("I",16), gid=value("I",20),
                    rdev=value("I",24), size=str(value("q",96)), blocks=str(value("q",104)),
                    block_size=value("i",112), flags=value("I",116), generation=value("I",120))
    for name, offset in (("atime",32),("mtime",48),("ctime",64),("birthtime",80)):
        metadata[name] = dict(seconds=str(value("q",offset)), nanoseconds=str(value("q",offset+8)))
    return metadata


def read_shared_memory_snapshot(name):
    """Read only an explicitly selected POSIX object; never create or modify it."""
    library = ctypes.CDLL(None, use_errno=True)
    query = library.shm_open
    query.argtypes = [ctypes.c_char_p, ctypes.c_int, ctypes.c_uint]
    query.restype = ctypes.c_int
    descriptor = query(os.fsencode(name), os.O_RDONLY, 0)
    if descriptor < 0:
        raise OSError(ctypes.get_errno(), "shared-memory snapshot open failed", name)
    try:
        before = native_stat64(descriptor)
        size = int(before["size"])
        if not 0 < size <= 65536:
            raise ValueError("shared-memory snapshot exceeds capture bound")
        with mmap.mmap(descriptor, size, access=mmap.ACCESS_READ) as view:
            contents = bytes(view)
        with mmap.mmap(descriptor, size, access=mmap.ACCESS_READ) as view:
            repeated = bytes(view)
        after = native_stat64(descriptor)
        if contents != repeated or before != after:
            raise ValueError("shared-memory object changed during capture")
        return contents, before
    finally:
        os.close(descriptor)


def verify_captured_shared_memory(profile):
    for entry in profile.get("read_only_shared_memory", []):
        expected = base64.b64decode(entry["contents_base64"], validate=True)
        if not 0 < len(expected) <= 65536 or hashlib.sha256(expected).hexdigest() != entry["sha256"]:
            raise ValueError("invalid shared-memory snapshot identity")
        contents, stat = read_shared_memory_snapshot(entry["name"])
        if contents != expected or stat != entry["stat"]:
            raise ValueError("captured shared-memory snapshot changed: " + entry["name"])



def preserve_failure(result, destination, diagnostic):
    """A failed gate still retains its native observations and bounded diagnosis."""
    result["bringup_failure_diagnostic"] = diagnostic
    destination = Path(destination)
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(json.dumps(result, indent=2) + "\n")


def diagnose_startup(binary, options, case, profile, startup, clock=None):
    source = ROOT / "scripts/macho-diagnostic.go"
    expected_source = digest(source)
    with tempfile.TemporaryDirectory(prefix="poros-macho-diagnostic-") as directory:
        folder = Path(directory)
        configuration = dict(Clock=clock,Quiescence="single-thread-no-scheduling",Binary=str(binary.resolve()), Dyld=options.dyld,
                             Cache=options.shared_cache or "", ExecName=str(binary.resolve()),
                             Args=case["args"], Runtime=profile, Startup=startup,
                             MaxInstructions=options.max_instructions,
                             TimeoutSeconds=options.execution_timeout_seconds)
        config = folder / "configuration.json"
        config.write_text(json.dumps(configuration))
        with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
            status = run_owned_process(["go", "run", str(source), str(config)], 120,
                                       cwd=ROOT, stdout=out, stderr=err)
            out.seek(0); err.seek(0)
            raw, errors = out.read(1 << 20), err.read(32768)
        if status != 0:
            raise ValueError(f"diagnostic failed: status={status}, stderr={errors.decode(errors='replace')}")
        if digest(source) != expected_source:
            raise ValueError("diagnostic source changed during execution")
        return dict(source_sha256=expected_source, observation=json.loads(raw))


def runtime_profile():
    native_dyld=Path("/usr/lib/dyld")
    before_dyld=digest(native_dyld)
    with tempfile.TemporaryDirectory(prefix="poros-runtime-profile-") as directory:
        binary = Path(directory) / "profile"
        compiler = subprocess.check_output(["xcrun", "--find", "clang"], text=True).strip()
        sdk = subprocess.check_output(["xcrun", "--sdk", "macosx", "--show-sdk-path"], text=True).strip()
        source = ROOT / "scripts/macho-runtime.c"
        subprocess.run([compiler, "-isysroot", sdk, "-arch", "arm64", str(source), "-o", str(binary)], check=True, timeout=30)
        profile = json.loads(subprocess.check_output([str(binary)], env={}, timeout=5))
        for entry in profile.get("read_only_shared_memory", []):
            contents = bytes.fromhex(entry.pop("contents_hex"))
            if not 0 < len(contents) <= 65536 or len(contents) != int(entry["stat"]["size"]):
                raise ValueError("invalid captured shared-memory size")
            entry["contents_base64"] = base64.b64encode(contents).decode("ascii")
            entry["sha256"] = hashlib.sha256(contents).hexdigest()
        verify_captured_shared_memory(profile)
        cache_parent=str(Path(profile["cache_dynamic"]["cache_path"]).parent)
        if not any(d["canonical_path"]==cache_parent for d in profile["directories"]):
            raise ValueError("captured directory does not match native shared-cache location")
        with native_dyld.open("rb") as opened:
            dyld_bytes=opened.read(MAX_DYLD_BYTES+1)
        if hashlib.sha256(dyld_bytes).hexdigest()!=before_dyld:
            raise ValueError("native dyld changed during runtime capture")
        offset=stack_probe_offset(dyld_bytes,profile["native_dyld"]["uuid"])
        bti_source=ROOT/"scripts/macho-bti.c"
        bti_binary=Path(directory)/"bti"
        subprocess.run([compiler,"-isysroot",sdk,"-arch","arm64","-Wall","-Wextra","-Werror",str(bti_source),"-o",str(bti_binary)],check=True,timeout=30)
        observed=json.loads(run_startup_helper([str(bti_binary),hex(offset),profile["native_dyld"]["uuid"]]))
        if (observed["mode"]!="non-enforced" or observed["generic_arm64"] is not True or observed["checks_passed"]!=10
            or observed["feature"] not in (0,1) or observed["dyld_uuid"]!=profile["native_dyld"]["uuid"]
            or observed["cache_uuid"]!=profile["cache_dynamic"]["cache_uuid"] or observed["stack_probe_offset"]!=offset
            or digest(native_dyld)!=before_dyld):
            raise ValueError("branch target evidence differs from captured native runtime")
        profile["branch_target_mode"]=observed["mode"]
        verify_captured_stat_absences(profile)
        verify_captured_access_absences(profile)
        verify_captured_shared_memory(profile)
        return profile, dict(source_sha256=digest(source), source_dependencies_sha256={"macho-stat.h":digest(ROOT/"scripts/macho-stat.h"),"macho_symbols.py":digest(ROOT/"scripts/macho_symbols.py")}, binary_sha256=digest(binary),
                             branch_target_probe=dict(source_sha256=digest(bti_source),binary_sha256=digest(bti_binary),dyld_sha256=before_dyld,observation=observed,
                                protocol="generic-arm64 helper; UUID-matched dyld symbol; own child plain/mismatched landing and actual stack-probe entry/after-entry checks; no guarded-page enforcement claim"),
                             compiler=subprocess.check_output([compiler, "--version"], text=True).strip())


def validate_clock_anchor_capture(captured, profile):
    """Validate native clock origins and a fast-API bracket before replay."""
    if (captured["user_timebase"] != profile["user_timebase"] or captured["continuous_hwclock"] != profile["continuous_hwclock"]
        or captured["numer"] != profile["timebase_numer"] or captured["denom"] != profile["timebase_denom"]):
        raise ValueError("clock capture differs from runtime timebase")
    if (type(captured["numer"]) is not int or type(captured["denom"]) is not int
        or not 0<captured["numer"]<1<<32 or not 0<captured["denom"]<1<<32
        or (1_000_000_000*captured["denom"])%captured["numer"]
        or captured["user_timebase"] not in (1,2,3) or captured["continuous_hwclock"] not in (0,1)):
        raise ValueError("unsupported clock timebase")
    keys=("boot_time_microseconds","initial_counter","absolute_offset","continuous_offset","continuous_hardware_offset","time_of_day")
    anchor={key:captured[key] for key in keys}
    t=anchor["time_of_day"]; observation=captured["observation"]; mask=(1<<64)-1
    frequency=1_000_000_000*captured["denom"]//captured["numer"]
    numbers=[anchor[key] for key in keys[:-1]]+list(t.values())+list(observation.values())
    if any(type(value) is not int or not 0<=value<=mask for value in numbers):
        raise ValueError("clock capture integer outside ABI range")
    absolute=lambda raw:(raw+anchor["absolute_offset"])&mask
    before=observation["counter_before"];after=observation["counter_after"]
    if (not anchor["boot_time_microseconds"] or anchor["boot_time_microseconds"]!=observation["boot_api"]
        or anchor["boot_time_microseconds"]!=observation["boot_sysctl_usec"] or anchor["initial_counter"]!=after
        or after<before or not t["tick"] or t["seconds"]>=1<<63 or not t["scale"] or t["ticks_per_second"]!=frequency
        or absolute(before)<t["tick"] or absolute(after)-t["tick"]>=frequency):
        raise ValueError("invalid clock capture anchor/window")
    if not absolute(before)<=observation["absolute_before"]<=observation["absolute_after"]<=absolute(after):
        raise ValueError("absolute clock API outside raw-counter bracket")
    continuous_offset=(anchor["absolute_offset"]+anchor["continuous_offset"])&mask
    if captured["continuous_hwclock"] and continuous_offset!=anchor["continuous_hardware_offset"]:
        raise ValueError("inconsistent continuous clock origin")
    if not ((before+continuous_offset)&mask)<=observation["continuous_before"]<=observation["continuous_after"]<=((after+continuous_offset)&mask):
        raise ValueError("continuous clock API outside raw-counter bracket")
    def wall(raw):
        fraction=t["fraction"]+t["scale"]*(absolute(raw)-t["tick"])
        return (t["seconds"]+(fraction>>64))*1_000_000+(((fraction&mask)>>32)*1_000_000>>32)
    value=observation["timeofday_sec"]*1_000_000+observation["timeofday_usec"]
    if observation["timeofday_usec"]>=1_000_000 or not wall(before)<=value<=wall(after):
        raise ValueError("calendar API outside captured tuple bracket")
    return anchor


def clock_anchor(profile):
    source=ROOT/"scripts/macho-clock.c"
    source_hash=digest(source)
    with tempfile.TemporaryDirectory(prefix="poros-clock-anchor-") as directory:
        binary=Path(directory)/"clock"
        compiler=subprocess.check_output(["xcrun","--find","clang"],text=True).strip()
        sdk=subprocess.check_output(["xcrun","--sdk","macosx","--show-sdk-path"],text=True).strip()
        subprocess.run([compiler,"-isysroot",sdk,"-arch","arm64","-Wall","-Wextra","-Werror",str(source),"-o",str(binary)],check=True,timeout=30)
        captured=json.loads(subprocess.check_output([str(binary)],env={},timeout=5))
        if digest(source)!=source_hash:
            raise ValueError("clock collector source changed during capture")
        anchor=validate_clock_anchor_capture(captured,profile)
        return anchor,dict(source_sha256=source_hash,binary_sha256=digest(binary),observation=captured,
            protocol="selected native counter and coherent calendar tuple; no clock writes; frozen scale/offsets, virtual elapsed ticks only")


def run_owned_process(command, timeout, **kwargs):
    """Bound and reap a private process group, including stopped descendants."""
    process = subprocess.Popen(command, start_new_session=True, **kwargs)
    try:
        return process.wait(timeout=timeout)
    finally:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        finally:
            process.wait()


def run_startup_helper(command, timeout=10, pass_fds=()):
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        code = run_owned_process(command, timeout, env={}, stdout=out, stderr=err,pass_fds=pass_fds)
        if code:
            raise subprocess.CalledProcessError(code, command)
        out.seek(0)
        err.seek(0)
        data, diagnostic = out.read(65537), err.read(65537)
        if len(data) > 65536 or diagnostic:
            raise ValueError("invalid startup helper output")
        return data



def capture_startup_profile(helper, binary, dyld, identity):
    before_binary, before_dyld = digest(binary), digest(dyld)
    with open(binary,"rb") as executable, open(os.devnull,"rb") as stdin, tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        initial_stat=os.fstat(executable.fileno())
        if hashlib.file_digest(executable,"sha256").hexdigest()!=before_binary:
            raise ValueError("runtime input changed during startup metadata capture")
        descriptors=(stdin.fileno(),stdout.fileno(),stderr.fileno(),executable.fileno())
        captured = json.loads(run_startup_helper([str(helper),str(binary.resolve()),*(str(fd) for fd in descriptors)],pass_fds=descriptors))
        final_stat=os.fstat(executable.fileno())
        stable=lambda st:(st.st_dev,st.st_ino,st.st_size,st.st_mtime_ns,st.st_ctime_ns)
        executable.seek(0)
        if stable(initial_stat)!=stable(final_stat) or hashlib.file_digest(executable,"sha256").hexdigest()!=before_binary:
            raise ValueError("runtime input changed during startup metadata capture")
        metadata=captured["executable_stat"]
        if (int(metadata["device"])!=initial_stat.st_dev or int(metadata["inode"])!=initial_stat.st_ino
            or int(metadata["size"])!=initial_stat.st_size):
            raise ValueError("captured executable stat does not match opened input")
    fields = {item["key"]: item for item in captured["filtered_apple"]}
    if len(fields) != len(captured["filtered_apple"]) or "ptrauth_disabled" in fields:
        raise ValueError("unsupported duplicate or downgraded arm64e startup metadata")
    stack = fields["main_stack"]
    if stack["top_minus_allocation_base"] != stack["allocation_size"]:
        raise ValueError("unsupported native stack reservation geometry")
    profile = dict(executable_stat=captured["executable_stat"], binary_sha256=before_binary, dyld_sha256=before_dyld,standard_io=captured["standard_io"],
                   sandbox_syscall_permissions=captured["sandbox_syscall_permissions"],
                   executable_file=identity(fields["executable_file"]["value"]),
                   dyld_file=identity(fields["dyld_file"]["value"]),
                   cdhash=fields["executable_cdhash"]["value"].lower(),
                   boot_hash=fields["executable_boothash"]["value"].lower(),
                   pfz=int(fields["pfz"]["value"],16), stack_size=stack["stack_size"],
                   stack_allocation_size=stack["allocation_size"],
                   security_config=int(fields["security_config"]["value"],16))
    if int(profile["executable_file"]["object_id"])!=initial_stat.st_ino:
        raise ValueError("captured executable identity does not match opened input")
    if "arm64e_abi" in fields:
        profile["arm64e_abi"] = fields["arm64e_abi"]["value"]
    for path, expected in [(binary, before_binary), (dyld, before_dyld),
                           (profile["executable_file"]["path"], before_binary),
                           (profile["dyld_file"]["path"], before_dyld)]:
        if digest(path) != expected:
            raise ValueError("runtime input changed during startup metadata capture")
    for path in (binary, profile["executable_file"]["path"]):
        st=os.stat(path)
        if (st.st_dev,st.st_ino,st.st_size,st.st_mtime_ns,st.st_ctime_ns)!=stable(final_stat):
            raise ValueError("runtime input changed during startup metadata capture")
    return profile, captured["cs_status"]


def capture_main_policies(binary, library, case, expected_status, runtime):
    expected_capability=runtime["thread_protection_capability_byte"]
    """Diagnostic run only: same main bytes, explicit injected constructor.

    Validate native output and code-signing state against the suspended launch;
    never include this modified-environment observation in timing samples.
    """
    signing = subprocess.run(["codesign", "-d", "--entitlements", ":-", str(binary)],
                             capture_output=True, timeout=10, check=True)
    if signing.stdout.strip():
        raise ValueError("AMFI capture currently requires an executable without entitlements")
    with tempfile.TemporaryFile() as record, tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        fd=record.fileno()
        code=run_owned_process(case["args"],30,executable=str(binary),
            env={"DYLD_INSERT_LIBRARIES":str(library), "POROS_QUERY_FD":str(fd)},
            stdout=out,stderr=err,stdin=subprocess.DEVNULL,pass_fds=(fd,))
        out.seek(0);err.seek(0);record.seek(0)
        stdout,stderr,data=out.read(65537),err.read(65537),record.read(4097)
        if code or stderr or len(data)>4096:
            raise ValueError("instrumented AMFI capture failed")
        check_output(case,stdout)
        observed=json.loads(data)
        if (observed["cs_result"] or observed["cs_errno"] or observed["cs_status"]!=expected_status
            or observed["x0"] or observed["x1"] or observed["carry"] or observed["input_flags"]!=0):
            raise ValueError("AMFI diagnostic differs from suspended native process state")
        dtrace=observed["dtrace"]
        if (dtrace["opened"] is not True or dtrace["lazy_invalid_header_accepted"] is not True
            or dtrace["own_registration_removed"] is not True or dtrace["cs_result_after"]
            or dtrace["cs_status_after"]!=expected_status):
            raise ValueError("DTrace lazy policy or owned-registration cleanup was not verified")
        protection=observed["thread_protection_observation"]
        if (protection["stage"]!="native-tpro-call-entry-and-return" or protection["register"]!="S3_6_C15_C1_5"
            or protection["capability"]!=expected_capability):
            raise ValueError("unknown/mismatched native thread-protection observation")
        if expected_capability==3:
            values=protection["values"]
            if (protection["interposed"] is not True or protection["entry_capability"]!=3
                or not 1<=protection["calls"]<=16 or len(values)!=protection["calls"]):
                raise ValueError("native TPRO startup call was not observed")
            first=None
            for pair in values:
                if len(pair)!=2 or any(not isinstance(v,str) or not v.isdecimal() or not 0<=int(v)<2**64 for v in pair):
                    raise ValueError("invalid native thread-protection value")
                if first is None:first=pair[0]
                if pair!=[first,first]:
                    raise ValueError("native thread-protection state changed across startup calls")
        elif protection["interposed"] is not False or protection["calls"]!=0 or protection["values"]:
            raise ValueError("unexpected TPRO interception without captured capability3")
        registration=observed["pthread_registration"]
        size=registration["data_size"]
        before=bytes.fromhex(registration["input_hex"]);after=bytes.fromhex(registration["output_hex"])
        if (registration["stage"]!="native-first-registration" or registration["calls"]!=1
            or type(size) is not int or not 48<=size<=128 or len(before)!=size or len(after)!=size
            or int.from_bytes(before[:8],"little")!=size
            or not 0<registration["result"]<2**31):
            raise ValueError("native first pthread registration was not observed")
        # Identify changed output fields from the real first registration, not a
        # retry after libpthread has already initialized the process.
        known=set(range(0,8))|set(range(16,24))|set(range(36,48))
        if any(a!=b for i,(a,b) in enumerate(zip(before,after)) if i not in known):
            raise ValueError("unmodeled native pthread registration normalization")
        page_shift=runtime["kernel_page_shift"]
        if type(page_shift) is not int or not 12<=page_shift<=16:
            raise ValueError("invalid native kernel page shift")
        stack_top=int(registration["main_stack_top"])
        if int.from_bytes(after[36:44],"little")!=stack_top&~((1<<page_shift)-1):
            raise ValueError("native pthread stack hint differs from main stack top policy")
        if int.from_bytes(after[:8],"little") not in (48,52,56,60):
            raise ValueError("unsupported native pthread registration layout")
        pthread=dict(version=int.from_bytes(after[:8],"little"),features=registration["result"],
            main_qos=str(int.from_bytes(after[16:24],"little")),mutex_policy=int.from_bytes(after[44:48],"little"),
            code_signing_status=expected_status)
        return (dict(input_flags=str(observed["input_flags"]),output_flags=str(observed["output_flags"]),
                     code_signing_status=expected_status),
                dict(mode="lazy-untraced",code_signing_status=expected_status),pthread,observed)


def startup_profiles(binaries, dyld, cases, runtime):
    """Capture suspended kernel metadata, then separate diagnostic AMFI and lazy DTrace results.

    The helper uses fixed safe argv and env{}; workload argv cannot spoof keys.
    fsgetpath resolves only the two identities just captured from our own child.
    """
    thread_protection_capability=runtime["thread_protection_capability_byte"]
    library = ctypes.CDLL(None, use_errno=True)
    fsgetpath = library.fsgetpath
    fsgetpath.argtypes = [ctypes.c_void_p, ctypes.c_size_t, ctypes.c_void_p, ctypes.c_uint64]
    fsgetpath.restype = ctypes.c_ssize_t
    def identity(value):
        fsid, inode = (int(x, 16) for x in value.split(","))
        raw_fsid = ctypes.c_uint64(fsid)
        buffer = ctypes.create_string_buffer(8192)
        length = fsgetpath(buffer, len(buffer), ctypes.byref(raw_fsid), inode)
        if length < 1 or length > len(buffer) or buffer.raw[length-1] != 0:
            raise ValueError("native captured file identity cannot be resolved")
        path = buffer.raw[:length-1].decode("utf-8")
        if not path.startswith("/") or "\0" in path:
            raise ValueError("invalid native canonical identity path")
        return dict(fsid=str(fsid), object_id=str(inode), path=path)
    with tempfile.TemporaryDirectory(prefix="poros-startup-profile-") as directory:
        helper = Path(directory) / "profile"
        compiler = subprocess.check_output(["xcrun", "--find", "clang"], text=True).strip()
        source = ROOT / "scripts/macho-startup.c"
        sdk = subprocess.check_output(["xcrun", "--sdk", "macosx", "--show-sdk-path"], text=True).strip()
        command = [compiler, "-isysroot", sdk, "-arch", "arm64", "-Wall", "-Wextra", "-Werror", str(source), "-o", str(helper)]
        subprocess.run(command, check=True, timeout=30)
        probe = dict(source_sha256=digest(source), source_dependencies_sha256={"macho-stat.h":digest(ROOT/"scripts/macho-stat.h")}, binary_sha256=digest(helper),
                     compiler=subprocess.check_output([compiler, "--version"], text=True).strip(),
                     protocol="own suspended child; fixed argv program/1, env{}; stdin=/dev/null, stdout/stderr=distinct empty owned regular files; exact launch descriptor stat64 capture; filtered KERN_PROCARGS2; kill/reap; fsgetpath of captured identities")
        library=Path(directory)/"amfi.dylib"
        amfi_source=ROOT/"scripts/macho-amfi.c"
        if type(thread_protection_capability) is not int or not 0<=thread_protection_capability<=255:
            raise ValueError("invalid captured thread-protection capability")
        capture_tpro=thread_protection_capability==3
        subprocess.run([compiler,"-isysroot",sdk,"-arch","arm64","-Wall","-Wextra","-Werror",f"-DPOROS_CAPTURE_TPRO={int(capture_tpro)}","-dynamiclib",str(amfi_source),"-o",str(library)],check=True,timeout=30)
        probe["amfi_diagnostic"]=dict(source_sha256=digest(amfi_source),binary_sha256=digest(library),tpro_interpose_enabled=capture_tpro,
            protocol="owned injected constructor; same main SHA; no entitlements; CS status equals suspended env{} child; original output oracle; invalid owned DOF accepted lazily and own generation removed; CS status rechecked; native first bsdthread_register input/output and TPRO call entry/return interception; security_config0 required for initial state; excluded from timing")
        profiles = {}
        probe["main_policy_observations"]={}
        for binary in binaries:
            metadata,cs_status=capture_startup_profile(helper,binary,dyld,identity)
            case=next(c for c in cases if c["workload"]==binary.stem)
            metadata["amfi_dyld_policy"],metadata["dtrace_helper"],metadata["pthread_registration"],probe["main_policy_observations"][binary.stem]=capture_main_policies(binary,library,case,cs_status,runtime)
            protection=probe["main_policy_observations"][binary.stem]["thread_protection_observation"]
            if protection["capability"]==3:
                if metadata["security_config"]!=0:
                    raise ValueError("initial thread state requires the observed non-TPRO process policy")
                metadata["thread_protection_register"]=protection["values"][0][0]
            if digest(binary)!=metadata["binary_sha256"] or digest(dyld)!=metadata["dyld_sha256"]:
                raise ValueError("runtime input changed during AMFI capture")
            profiles[binary.stem]=metadata
        return profiles, probe


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--poros", required=True)
    parser.add_argument("--dyld", default="/usr/lib/dyld")
    parser.add_argument("--output", required=True)
    parser.add_argument("--shared-cache", help="explicit matching native warm cache file")
    parser.add_argument("--samples", type=int, default=3)
    parser.add_argument("--max-instructions", type=int, default=10_000_000)
    parser.add_argument("--execution-timeout-seconds", type=int, default=10)
    options = parser.parse_args()
    if platform.system() != "Darwin" or platform.machine() != "arm64":
        parser.error("native observations require ARM64 macOS")
    if not 1 <= options.samples <= 100:
        parser.error("samples must be in [1,100]")
    if not 1 <= options.max_instructions <= 100_000_000 or not 1 <= options.execution_timeout_seconds <= 60:
        parser.error("capture budgets require 1..100M instructions and 1..60 execution seconds")
    manifest = load_manifest(ROOT)  # also verifies frozen Linux sources/binaries
    folder = ROOT / "tests/macho"
    build = json.loads((folder / "build.json").read_text())
    for name, expected in build["sources"].items():
        if digest(ROOT / name) != expected:
            raise ValueError("Mach-O source hash mismatch: " + name)
    for name, expected in build["binaries"].items():
        if digest(folder / name) != expected:
            raise ValueError("Mach-O binary hash mismatch: " + name)
    initial_dyld = digest(options.dyld)
    profile, probe = runtime_profile()
    anchor, clock_probe = clock_anchor(profile)
    startup, startup_probe = startup_profiles([folder / name for name in sorted(build["binaries"])], options.dyld, manifest["cases"], profile)
    for name, metadata in startup.items():
        if metadata["binary_sha256"] != build["binaries"][name + ".macho"] or metadata["dyld_sha256"] != initial_dyld:
            raise ValueError("captured startup metadata differs from frozen build/runtime")
    def verify_inputs():
        verify_captured_shared_memory(profile)
        verify_captured_stat_absences(profile)
        verify_captured_access_absences(profile)
        if digest(options.dyld) != initial_dyld:
            raise ValueError("dyld changed during collection")
        for name, expected in build["binaries"].items():
            if digest(folder / name) != expected:
                raise ValueError("workload changed during collection: " + name)
    verify_inputs()
    result = dict(clock_anchor=anchor,clock_probe=clock_probe,startup_profiles=startup, startup_probe=startup_probe, runtime_profile=profile, runtime_probe=probe, schema=1, purpose="Mach-O native correctness and incomplete runtime bring-up",
                  accuracy_claim=False, collector_sha256=digest(__file__), collected_utc=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                  os=subprocess.check_output(["sw_vers"], text=True).strip(),
                  cpu=subprocess.check_output(["sysctl", "-n", "machdep.cpu.brand_string"], text=True).strip(),
                  architecture=platform.machine(), dyld_sha256=initial_dyld,
                  poros_sha256=digest(options.poros), manifest_sha256=digest(ROOT / "tests/usertime/manifest.json"),
                  protocol=dict(quiescence_policy="single-thread-no-scheduling",samples=options.samples, native_timeout_seconds=30,
                                simulator_max_instructions=options.max_instructions,
                                simulator_execution_timeout_seconds=options.execution_timeout_seconds,
                                simulator_outer_timeout_seconds=120, clock="wait4 child ru_utime", env={},
                                quality="raw observations only; no acceptance threshold or accuracy comparison"),
                  build=build, cases=[])
    shared_files = None
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json") as profile_file, tempfile.TemporaryDirectory(prefix="poros-startup-inputs-") as startup_directory:
        json.dump(profile, profile_file)
        profile_file.flush()
        clock_file=Path(startup_directory)/"clock.json"
        clock_file.write_text(json.dumps(anchor))
        startup_files = {}
        for name, metadata in startup.items():
            path = Path(startup_directory) / (name + ".json")
            path.write_text(json.dumps(metadata))
            startup_files[name] = path
        for case in manifest["cases"]:
            binary = folder / (case["workload"] + ".macho")
            samples = []
            for _ in range(options.samples):
                verify_inputs()
                stdout, usage = observe(binary, case["args"])
                verify_inputs()
                check_output(case, stdout)
                samples.append(usage)
            sim = subprocess.run([str(Path(options.poros).resolve()), "macho", "--dyld", options.dyld, "--quiescence", "single-thread-no-scheduling",
                                  *( ["--shared-cache", options.shared_cache] if options.shared_cache else [] ),
                                  "--max-instructions", str(options.max_instructions),
                                  "--timeout", str(options.execution_timeout_seconds)+"s",
                                  "--clock-anchor", str(clock_file),
                                  "--startup-profile", str(startup_files[case["workload"]]),
                                  "--runtime-profile", profile_file.name, "--argv0", case["args"][0], str(binary), *case["args"][1:]],
                                 capture_output=True, timeout=120)
            if sim.returncode != 1 or sim.stderr:
                raise ValueError(f"unexpected bring-up command status: {sim.returncode}, {sim.stderr!r}")
            verify_inputs()
            witness = json.loads(sim.stdout)
            if witness.get("quiescence_policy")!="single-thread-no-scheduling":
                raise ValueError("simulator virtual kernel policy mismatch")
            if witness.pop("clock_anchor",None)!=anchor:
                raise ValueError("simulator clock anchor differs from captured launch input")
            witness["clock_anchor_sha256"]=hashlib.sha256(json.dumps(anchor,sort_keys=True).encode()).hexdigest()
            metadata = startup[case["workload"]]
            if witness.pop("startup_profile", None) != metadata:
                raise ValueError("simulator startup profile differs from native capture")
            witness["startup_profile_sha256"] = hashlib.sha256(json.dumps(metadata, sort_keys=True).encode()).hexdigest()
            if witness.pop("runtime_profile", None) != profile:
                raise ValueError("simulator profile differs from captured native profile")
            witness["runtime_profile_sha256"] = hashlib.sha256(json.dumps(profile, sort_keys=True).encode()).hexdigest()
            files = witness.pop("shared_cache_files", None)
            if options.shared_cache:
                if not files:
                    raise ValueError("shared cache identities missing")
                if shared_files is None:
                    shared_files = files
                    result["shared_cache_files"] = files
                elif files != shared_files:
                    raise ValueError("runtime cache changed between cases")
                witness["shared_cache_manifest_sha256"] = hashlib.sha256(json.dumps(files, sort_keys=True).encode()).hexdigest()
            if witness["binary"]["sha256"] != build["binaries"][binary.name] or witness["dyld"]["sha256"] != result["dyld_sha256"]:
                raise ValueError("simulator/native input identity mismatch")
            result["cases"].append(dict(id=case["id"], args=case["args"], binary_sha256=digest(binary),
                                         native_correct=True, stdout_base64=base64.b64encode(stdout).decode(),
                                         samples=samples, simulator=witness))
            if witness["termination"] not in ("unsupported_instruction", "unsupported_syscall") or witness["instructions"] < 1:
                try:
                    diagnosis = diagnose_startup(binary, options, case, profile, metadata, anchor)
                    verify_inputs()
                except Exception as error:
                    diagnosis = dict(error=str(error)[:32768])
                preserve_failure(result, options.output, diagnosis)
                raise ValueError("unexpected runtime bring-up outcome: " + str(witness))
    verify_inputs()
    destination = Path(options.output)
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(json.dumps(result, indent=2) + "\n")
    print(f"{len(result['cases'])} native oracle passes; simulator startup incomplete for all cases; {destination}")


if __name__ == "__main__":
    main()
