#!/usr/bin/env python3
"""Required static-ELF functional/reference gate. No optional or skipped cases."""
import argparse
import base64
import hashlib
import json
import math
import os
from pathlib import Path
import platform
import signal
import struct
import subprocess
import sys
import time

REQUIRED = ("hello", "startup", "memory", "compute", "syscall-errors", "bad-load",
            "bad-store", "nx", "unsupported-syscall", "unsupported-instruction",
            "loop", "output-limit", "exit-status", "arithmetic-loop")
ROOT = Path(__file__).resolve().parents[1]


def provenance(poros):
    revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    dirty = bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT))
    paths = {ROOT / "tests/elf/manifest.json"}
    for pattern in ("pkg/emu/*.go", "pkg/linuxuser/*.go", "pkg/loader/*.go", "cmd/poros/*.go", "scripts/elf-*.py", "tests/elf/*.c", "tests/elf/*.S", "tests/elf/*.ld", "tests/elf/*.sh"):
        paths.update(ROOT.glob(pattern))
    return {"git_sha": revision, "git_dirty": dirty,
            "poros_binary_sha256": hashlib.sha256(Path(poros).read_bytes()).hexdigest(),
            "sources": {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(paths)}}


def inventory(manifest, root):
    cases = manifest["cases"]
    if tuple(c["name"] for c in cases) != REQUIRED:
        raise ValueError("required ELF inventory is missing, duplicated or reordered")
    for source in ("start.S", "guest.c", "layout.ld", "build.sh"):
        if hashlib.sha256((root / source).read_bytes()).hexdigest() != manifest["sources"][source]:
            raise ValueError("source hash mismatch: " + source)
    for case in cases:
        if case["file"] != case["name"] + ".elf":
            raise ValueError("unexpected fixture path")
        if hashlib.sha256((root / case["file"]).read_bytes()).hexdigest() != case["sha256"]:
            raise ValueError("ELF hash mismatch: " + case["name"])
    return cases


def capture(command, env=None, timeout=5, executable=None):
    start = time.perf_counter_ns()
    try:
        p = subprocess.run(command, executable=executable, env=env,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout)
        status, reason, stdout, stderr = p.returncode, "exit" if p.returncode >= 0 else "signal", p.stdout, p.stderr
    except subprocess.TimeoutExpired as exc:
        status, reason, stdout, stderr = None, "time_limit", exc.stdout or b"", exc.stderr or b""
    return {"exit_status": status, "termination": reason, "stdout_base64": base64.b64encode(stdout).decode(),
            "stderr_base64": base64.b64encode(stderr).decode(), "elapsed_ns": time.perf_counter_ns() - start,
            "instructions": None}


def raw(record, field):
    return base64.b64decode(record[field + "_base64"], validate=True)


def validate(case, poros, reference):
    if poros["termination"] != case["termination"] or poros["exit_status"] != case["exit_status"]:
        raise ValueError("Poros termination/status mismatch")
    if type(poros["instructions"]) is not int or not 0 < poros["instructions"] <= case["max_instructions"]:
        raise ValueError("missing or out-of-bounds guest instruction count")
    if case["name"] == "loop" and poros["instructions"] != case["max_instructions"]:
        raise ValueError("instruction budget did not stop at the exact bound")
    for channel in ("stdout", "stderr"):
        if not (case["name"] == "compute" and channel == "stdout") and raw(poros, channel).hex() != case[channel + "_hex"]:
            raise ValueError("Poros " + channel + " mismatch")
        # QEMU prints its own diagnostic for guest signals. This remains captured
        # as reference stderr and is not mistaken for guest output.
        if channel == "stderr" and case["reference_exit"] in (-signal.SIGSEGV, -signal.SIGILL):
            continue
        if not (case["name"] == "compute" and channel == "stdout") and raw(reference, channel).hex() != case["reference_" + channel + "_hex"]:
            raise ValueError("reference " + channel + " mismatch")
    expected_reason = "time_limit" if case["reference_exit"] is None else "signal" if case["reference_exit"] < 0 else "exit"
    if reference["termination"] != expected_reason or reference["exit_status"] != case["reference_exit"]:
        raise ValueError("reference termination/status mismatch")
    if case["name"] == "compute":
        # This oracle is independent of both machines and of manifest bytes.
        for result in (poros, reference):
            integer, fused, ratio, edge, inexact = struct.unpack("<Qdddd", raw(result, "stdout"))
            if integer != 96 or not all(math.isfinite(x) for x in (fused, ratio, edge, inexact)):
                raise ValueError("invalid integer/FP result")
            if not math.isclose(fused, 3.25, rel_tol=1e-14, abs_tol=1e-15) or not math.isclose(ratio, .875, rel_tol=1e-14, abs_tol=1e-15):
                raise ValueError("FP tolerance failed")
            if struct.unpack("<Q", struct.pack("<d", edge))[0] != 0xbc90000000000000:
                raise ValueError("single-rounding FMA bit pattern failed")
            if not math.isclose(inexact, 1 / 10, rel_tol=1e-14, abs_tol=1e-15):
                raise ValueError("inexact division tolerance failed")


def require_complete(results):
    if tuple(r["name"] for r in results) != REQUIRED or any(r.get("status") != "pass" for r in results):
        raise ValueError("missing, skipped or failed required ELF cases")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--poros", required=True)
    parser.add_argument("--reference", choices=("native", "qemu"), default="qemu")
    parser.add_argument("--qemu", default="qemu-aarch64")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    root = ROOT / "tests/elf"
    report = {"schema": 1, "status": "fail", "reference": args.reference,
              "platform": platform.platform(), "cases": []}
    try:
        report["provenance"] = provenance(args.poros)
        if args.reference == "native" and (platform.system() != "Linux" or platform.machine() not in ("aarch64", "arm64")):
            raise ValueError("native reference requires Linux ARM64")
        if args.reference == "qemu":
            report["reference_version"] = subprocess.check_output([args.qemu, "--version"], text=True).splitlines()[0]
        else:
            report["reference_version"] = platform.release()
        cases = inventory(json.loads((root / "manifest.json").read_text()), root)
        for case in cases:
            item = {"name": case["name"], "binary_sha256": case["sha256"], "args": case["args"], "env": case["env"], "status": "fail"}
            report["cases"].append(item)
            cmd = [args.poros, "elf", "--argv0", case["args"][0], "--max-instructions", str(case["max_instructions"]), "--max-output", str(case["max_output"])]
            for env in case["env"]:
                cmd += ["--env", env]
            cmd += [str(root / case["file"])] + case["args"][1:]
            host = capture(cmd)
            if host["termination"] != "exit" or host["exit_status"] not in (0, 1) or raw(host, "stderr"):
                raise ValueError("Poros CLI failed: " + repr(host))
            poros = json.loads(raw(host, "stdout"))
            if poros["binary_sha256"] != case["sha256"] or poros["args"] != case["args"] or poros["env"] != case["env"]:
                raise ValueError("Poros input provenance mismatch")
            expected_cli = 0 if case["termination"] == "exit" and case["exit_status"] == 0 else 1
            if host["exit_status"] != expected_cli:
                raise ValueError("CLI exit status mismatch")
            if not report["provenance"]["git_dirty"]:
                if poros.get("build", {}).get("vcs.revision") != report["provenance"]["git_sha"] or poros.get("build", {}).get("vcs.modified") != "false":
                    raise ValueError("Poros binary is not built from this clean checkout")
            item["poros"] = poros
            item["poros_wall_ns"] = host["elapsed_ns"]
            env = dict(entry.split("=", 1) for entry in case["env"])
            path = str(root / case["file"])
            if args.reference == "qemu":
                ref_cmd = [args.qemu, "-0", case["args"][0], path] + case["args"][1:]
                reference = capture(ref_cmd, env=env, timeout=.25 if case["name"] == "loop" else 5)
            else:
                reference = capture(case["args"], executable=path, env=env, timeout=.25 if case["name"] == "loop" else 5)
            item["reference"] = reference
            validate(case, poros, reference)
            item["status"] = "pass"
        require_complete(report["cases"])
        report["status"] = "pass"
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as exc:
        report["error"] = str(exc)
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n")
    print(f'ELF differential: {report["status"]}, {sum(c["status"] == "pass" for c in report["cases"])}/{len(REQUIRED)} passed; {args.output}')
    return 0 if report["status"] == "pass" else 1


if __name__ == "__main__":
    sys.exit(main())
