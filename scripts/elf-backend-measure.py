#!/usr/bin/env python3
"""Measure ELF startup/loop overhead in one Linux environment; not CPU calibration."""
import argparse
import hashlib
import importlib.util
import json
import platform
import statistics
import struct
import subprocess
from pathlib import Path

spec = importlib.util.spec_from_file_location("gate", Path(__file__).with_name("elf-differential.py"))
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--poros", required=True)
    parser.add_argument("--qemu", default="qemu-aarch64")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    report = {"schema": 1, "platform": platform.platform(), "samples_per_case": 5,
              "scope": "host wall time includes process startup; not guest CPU calibration",
              "qemu_version": subprocess.check_output([args.qemu, "--version"], text=True).splitlines()[0],
              "poros_binary_sha256": hashlib.sha256(Path(args.poros).read_bytes()).hexdigest(),
              "provenance": gate.provenance(args.poros), "measurements": []}
    cases = [("hello", [], b"hello, ELF\n", b"stderr\n")]
    cases += [("arithmetic-loop", [str(n)], struct.pack("<Q", n*(n-1)//2), b"") for n in (100, 10000, 100000)]
    for name, guest_args, expected_out, expected_err in cases:
        path = str(gate.ROOT / "tests/elf" / (name + ".elf"))
        for backend in ("poros", "qemu", "qemu-tb-trace"):
            command = [args.poros, "elf", "--argv0", "fixture", "--timeout", "10s", path] if backend == "poros" else [args.qemu, "-0", "fixture"]
            if backend == "qemu-tb-trace":
                command += ["-d", "exec,nochain", "-D", "/dev/null"]
            if backend != "poros":
                command += [path]
            command += guest_args
            samples, instructions = [], None
            for _ in range(5):
                result = gate.capture(command, env={}, timeout=15)
                if result["termination"] != "exit" or result["exit_status"] != 0:
                    raise ValueError("backend run failed: " + repr(result))
                if backend == "poros":
                    result_guest = json.loads(gate.raw(result, "stdout"))
                    if result_guest["termination"] != "exit" or result_guest["exit_status"] != 0:
                        raise ValueError("guest failed")
                    instructions = result_guest["instructions"]
                    report["poros_build"] = result_guest["build"]
                    if not report["provenance"]["git_dirty"] and (result_guest["build"].get("vcs.revision") != report["provenance"]["git_sha"] or result_guest["build"].get("vcs.modified") != "false"):
                        raise ValueError("Poros binary does not match clean source checkout")
                else:
                    result_guest = result
                if gate.raw(result_guest, "stdout") != expected_out or gate.raw(result_guest, "stderr") != expected_err:
                    raise ValueError("incorrect backend result")
                samples.append(result["elapsed_ns"])
            report["measurements"].append({"case": name, "args": ["fixture"] + guest_args, "env": [],
                "fixture_sha256": hashlib.sha256(Path(path).read_bytes()).hexdigest(),
                "backend": backend, "command": command, "instructions": instructions,
                "wall_ns": samples, "median_wall_ns": statistics.median(samples)})
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n")


if __name__ == "__main__":
    main()
