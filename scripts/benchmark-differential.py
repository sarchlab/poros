#!/usr/bin/env python3
"""Compare complete benchmark ELF outputs against QEMU; never time QEMU as hardware."""
import argparse
import base64
import hashlib
import json
import math
import pathlib
import platform
import struct
import shutil
import subprocess
import sys

EXPECTED = set("atax gemm 2mm 3mm bicg gesummv gemver doitgen trisolv durbin jacobi1d syrk syr2k trmm symm cholesky lu floydwarshall covariance correlation gramschmidt ludcmp nussinov adi fdtd2d heat3d jacobi2d seidel2d deriche swaptions blackscholes streamcluster canneal fluidanimate raytrace vips bodytrack facesim ferret x264".split())


def digest(data):
    return hashlib.sha256(data).hexdigest()


def check_output(case, data):
    offset = 0
    for output in case["outputs"]:
        width = {"double": 8, "int": 4}[output["type"]]
        end = offset + width * output["count"]
        if end > len(data):
            raise ValueError("truncated output")
        if output["type"] == "double":
            if not all(math.isfinite(x[0]) for x in struct.iter_unpack("<d", data[offset:end])):
                raise ValueError("nonfinite output")
        offset = end
    if offset != len(data):
        raise ValueError("unexpected trailing output")


def check_result(case, completed, expected):
    if completed.returncode or completed.stderr:
        raise ValueError(f"Poros failed: {completed.stderr.decode(errors='replace')}")
    result = json.loads(completed.stdout)
    if result["termination"] != "exit" or result["exit_status"] != 0 or base64.b64decode(result["stderr_base64"], validate=True):
        raise ValueError("guest did not exit normally with empty stderr")
    actual = base64.b64decode(result["stdout_base64"], validate=True)
    check_output(case, actual)
    check_output(case, expected)
    if actual != expected:
        raise ValueError("complete output differs from identical ELF in QEMU")
    return result, actual


def validate_inventory(root, manifest, provenance):
    workloads = manifest["workloads"]
    if manifest["schema_version"] != 1 or len(workloads) != 40 or {w["name"] for w in workloads} != EXPECTED:
        raise ValueError("missing/duplicate/unexpected benchmark inventory")
    required = {"tests/benchmarks/manifest.json", "tests/benchmarks/runtime.h", "tests/benchmarks/build.sh", "tests/benchmarks/provenance.py", "tests/usertime/start.S", "tests/usertime/layout.ld", "tests/benchmarks/support/math.c", "tests/benchmarks/support/math.elf"}
    for w in workloads:
        required.update(w[key] for key in ("source", "kernel", "binary"))
        dimensions = w["dimensions"]
        if not dimensions or len({d["name"] for d in dimensions}) != len(dimensions):
            raise ValueError("missing/duplicate dimension names")
        for d in dimensions:
            if not d["name"] or type(d["min"]) is not int or type(d["max"]) is not int or not 1 <= d["min"] <= d["max"]:
                raise ValueError("invalid dimension bounds")
        cases = w["cases"]
        if len(cases) != 6 or [c["id"] for c in cases] != list("123456") or len({tuple(c["args"]) for c in cases}) != 6:
            raise ValueError("incomplete or duplicate six-point inventory")
        for c in cases:
            if not c["outputs"] or any(o["type"] not in ("double", "int") or type(o["count"]) is not int or not 1 <= o["count"] <= 1048576 for o in c["outputs"]):
                raise ValueError("invalid complete output shape")
            if len(c["args"]) != len(w["dimensions"]):
                raise ValueError("dimension shape mismatch")
            for arg, dimension in zip(c["args"], w["dimensions"]):
                if not arg.isascii() or not arg.isdecimal() or not dimension["min"] <= int(arg) <= dimension["max"]:
                    raise ValueError("out-of-bounds dimension")
    if provenance["schema"] != 1 or not provenance.get("compiler") or not provenance.get("linker") or not required <= provenance["files"].keys():
        raise ValueError("incomplete ELF build provenance")
    if set(provenance.get("libraries", {})) != {"libc.a", "libgcc.a"}:
        raise ValueError("missing linked archive provenance")
    for name, expected in provenance["files"].items():
        if pathlib.PurePosixPath(name).is_absolute() or ".." in pathlib.PurePosixPath(name).parts or digest((root / name).read_bytes()) != expected:
            raise ValueError(f"stale ELF provenance: {name}")
    return workloads


def run_provenance(root, poros):
    git = ["git", "-c", "safe.directory=" + str(root), "-C", str(root)]
    revision = subprocess.check_output([*git, "rev-parse", "HEAD"], text=True).strip()
    dirty = bool(subprocess.check_output([*git, "status", "--porcelain"]))
    names = subprocess.check_output([*git, "ls-files", "--cached", "--others", "--exclude-standard", "-z"]).decode().split("\0")
    return {"git_sha": revision, "git_dirty": dirty, "poros_binary_sha256": digest(pathlib.Path(poros).read_bytes()),
            "sources": {name: digest((root / name).read_bytes()) for name in sorted(set(names)) if name and (root / name).is_file()}}


def check_build(result, state):
    build = result.get("build", {})
    if build.get("vcs.revision") != state["git_sha"] or build.get("vcs.modified") != str(state["git_dirty"]).lower():
        raise ValueError("Poros binary build does not match checkout revision/dirty state")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--poros", required=True)
    parser.add_argument("--require-clean", action="store_true")
    parser.add_argument("--qemu", default="qemu-aarch64")
    parser.add_argument("--native", action="store_true", help="also execute the identical ELF on this Linux ARM64 host")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    root = pathlib.Path(__file__).resolve().parents[1]
    manifest = json.loads((root / "tests/benchmarks/manifest.json").read_text())
    provenance = json.loads((root / "tests/benchmarks/provenance.json").read_text())
    workloads = validate_inventory(root, manifest, provenance)
    args.poros = str(pathlib.Path(shutil.which(args.poros) or args.poros).resolve())
    state = run_provenance(root, args.poros)
    if args.require_clean and state["git_dirty"]:
        raise ValueError("required clean checkout is dirty")
    if args.native and (platform.system() != "Linux" or platform.machine() not in ("aarch64", "arm64")):
        raise ValueError("--native requires Linux ARM64")
    qemu_version = subprocess.check_output([args.qemu, "--version"], text=True).splitlines()[0]
    report = {"schema": 1, "contract": "complete identical-ELF functional differential; not performance accuracy",
              "platform": platform.platform(), "qemu": qemu_version, "build": provenance,
              "manifest_sha256": digest((root / "tests/benchmarks/manifest.json").read_bytes()),
              "provenance": state, "acceptance_status": "unverified", "expected_cases": 240, "cases": [], "status": "fail"}
    failure = False
    for workload in workloads:
        cases = workload["cases"]
        if len(cases) != 6 or [c["id"] for c in cases] != list("123456"):
            raise ValueError("incomplete six-point inventory")
        elf = root / workload["binary"]
        for case in cases:
            record = {"id": workload["name"] + "/" + case["id"], "args": case["args"],
                      "binary_sha256": digest(elf.read_bytes()), "status": "fail"}
            try:
                oracle = subprocess.run([args.qemu, str(elf), *case["args"]], capture_output=True, timeout=300, check=True)
                if oracle.stderr:
                    raise ValueError("QEMU stderr is not empty")
                if args.native:
                    native = subprocess.run([str(elf), *case["args"]], capture_output=True, timeout=300, check=True)
                    if native.stderr or native.stdout != oracle.stdout:
                        raise ValueError("Linux ARM64 native/QEMU output mismatch")
                    record["native_sha256"] = digest(native.stdout)
                simulated = subprocess.run([args.poros, "elf", "--max-instructions", "200000000", "--timeout", "5m", "--max-output", "16777216", str(elf), *case["args"]], capture_output=True, timeout=360)
                result, actual = check_result(case, simulated, oracle.stdout)
                check_build(result, state)
                if result["args"] != ["program", *case["args"]] or result["env"] != []:
                    raise ValueError("Poros input provenance mismatch")
                if type(result["instructions"]) is not int or not 0 < result["instructions"] <= 200000000:
                    raise ValueError("invalid instruction count")
                if result["binary_sha256"] != record["binary_sha256"]:
                    raise ValueError("Poros ran a different binary")
                record.update(status="pass", instructions=result["instructions"], output_bytes=len(actual), output_sha256=digest(actual), qemu_sha256=digest(oracle.stdout))
            except (ValueError, KeyError, subprocess.SubprocessError) as error:
                failure = True
                record["error"] = str(error)
            report["cases"].append(record)
            print(record["id"], record["status"], record.get("error", ""), flush=True)
    if len(report["cases"]) != 240 or len({c["id"] for c in report["cases"]}) != 240:
        failure = True
    if run_provenance(root, args.poros) != state:
        failure = True
        report["error"] = "sources or executable changed during validation"
    if not failure:
        report["status"] = "pass"
        if not state["git_dirty"]:
            report["acceptance_status"] = "pass"
    destination = pathlib.Path(args.output)
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(json.dumps(report, indent=2) + "\n")
    return int(failure)


if __name__ == "__main__":
    sys.exit(main())
