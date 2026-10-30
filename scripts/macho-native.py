#!/usr/bin/env python3
"""Native correctness and raw user-time observations for Mach-O runtime bring-up.

This is not an accuracy report. Poros startup failures remain explicit; native
observations must not be compared to partial simulator instruction counts.
"""
import argparse
import base64
import datetime
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import tempfile
import time

from workload_contract import check_output, digest, load_manifest

ROOT = Path(__file__).resolve().parents[1]


def observe(binary, args):
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        p = subprocess.Popen(args, executable=str(binary), env={}, stdout=out, stderr=err)
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


def runtime_profile():
    with tempfile.TemporaryDirectory(prefix="poros-runtime-profile-") as directory:
        binary = Path(directory) / "profile"
        compiler = subprocess.check_output(["xcrun", "--find", "clang"], text=True).strip()
        sdk = subprocess.check_output(["xcrun", "--sdk", "macosx", "--show-sdk-path"], text=True).strip()
        source = ROOT / "scripts/macho-runtime.c"
        subprocess.run([compiler, "-isysroot", sdk, "-arch", "arm64", str(source), "-o", str(binary)], check=True, timeout=30)
        profile = json.loads(subprocess.check_output([str(binary)], env={}, timeout=5))
        return profile, dict(source_sha256=digest(source), binary_sha256=digest(binary),
                             compiler=subprocess.check_output([compiler, "--version"], text=True).strip())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--poros", required=True)
    parser.add_argument("--dyld", default="/usr/lib/dyld")
    parser.add_argument("--output", required=True)
    parser.add_argument("--shared-cache", help="explicit matching native warm cache file")
    parser.add_argument("--samples", type=int, default=3)
    options = parser.parse_args()
    if platform.system() != "Darwin" or platform.machine() != "arm64":
        parser.error("native observations require ARM64 macOS")
    if not 1 <= options.samples <= 100:
        parser.error("samples must be in [1,100]")
    manifest = load_manifest(ROOT)  # also verifies frozen Linux sources/binaries
    folder = ROOT / "tests/macho"
    build = json.loads((folder / "build.json").read_text())
    for name, expected in build["sources"].items():
        if digest(ROOT / name) != expected:
            raise ValueError("Mach-O source hash mismatch: " + name)
    for name, expected in build["binaries"].items():
        if digest(folder / name) != expected:
            raise ValueError("Mach-O binary hash mismatch: " + name)
    profile, probe = runtime_profile()
    result = dict(runtime_profile=profile, runtime_probe=probe, schema=1, purpose="Mach-O native correctness and incomplete runtime bring-up",
                  accuracy_claim=False, collector_sha256=digest(__file__), collected_utc=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                  os=subprocess.check_output(["sw_vers"], text=True).strip(),
                  cpu=subprocess.check_output(["sysctl", "-n", "machdep.cpu.brand_string"], text=True).strip(),
                  architecture=platform.machine(), dyld_sha256=digest(options.dyld),
                  poros_sha256=digest(options.poros), manifest_sha256=digest(ROOT / "tests/usertime/manifest.json"),
                  protocol=dict(samples=options.samples, native_timeout_seconds=30, clock="wait4 child ru_utime", env={},
                                quality="raw observations only; no acceptance threshold or accuracy comparison"),
                  build=build, cases=[])
    shared_files = None
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json") as profile_file:
        json.dump(profile, profile_file)
        profile_file.flush()
        for case in manifest["cases"]:
            binary = folder / (case["workload"] + ".macho")
            samples = []
            for _ in range(options.samples):
                stdout, usage = observe(binary, case["args"])
                check_output(case, stdout)
                samples.append(usage)
            sim = subprocess.run([str(Path(options.poros).resolve()), "macho", "--dyld", options.dyld,
                                  *( ["--shared-cache", options.shared_cache] if options.shared_cache else [] ),
                                  "--runtime-profile", profile_file.name, "--argv0", case["args"][0], str(binary), *case["args"][1:]],
                                 capture_output=True, timeout=120)
            if sim.returncode != 1 or sim.stderr:
                raise ValueError(f"unexpected bring-up command status: {sim.returncode}, {sim.stderr!r}")
            witness = json.loads(sim.stdout)
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
            if witness["binary"]["sha256"] != digest(binary) or witness["dyld"]["sha256"] != result["dyld_sha256"]:
                raise ValueError("simulator/native input identity mismatch")
            if witness["termination"] not in ("unsupported_instruction", "unsupported_syscall") or witness["instructions"] < 1:
                raise ValueError("unexpected runtime bring-up outcome: " + str(witness))
            result["cases"].append(dict(id=case["id"], args=case["args"], binary_sha256=digest(binary),
                                         native_correct=True, stdout_base64=base64.b64encode(stdout).decode(),
                                         samples=samples, simulator=witness))
    destination = Path(options.output)
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(json.dumps(result, indent=2) + "\n")
    print(f"{len(result['cases'])} native oracle passes; simulator startup incomplete for all cases; {destination}")


if __name__ == "__main__":
    main()
