#!/usr/bin/env python3
"""Value-varying forwarding probes: functional CI, native reference, simulation."""
from report_records import covered_cases

import argparse
import base64
from datetime import datetime, timezone
import hashlib
import json
import math
from pathlib import Path
import platform
import re
import statistics
import struct
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
DIRECTORY = "evaluation/microbenchmarks/forwarding/"
ELF = DIRECTORY + "forwarding.elf"
REFERENCE = DIRECTORY + "m2.json"
MEASUREMENT_INPUTS = [DIRECTORY + name for name in ("cases.def", "loops.S", "driver.h", "native.c")]
# Recorded collection provenance, not prerequisites for reusing native measurements.
COLLECTOR_SOURCES = [DIRECTORY + name for name in ("linux.c", "entry.S", "build.sh", "forwarding.elf")]
COLLECTOR_SOURCES += ["scripts/forwarding_microbenchmark.py", "tests/usertime/layout.ld"]
INPUTS = MEASUREMENT_INPUTS + COLLECTOR_SOURCES
PROTOCOL = "assembly-forwarding-v1"
UPDATES_PER_ITERATION = 64
ITERATIONS = (1024, 2048)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def cases(root=ROOT):
    return parse_cases((Path(root) / DIRECTORY / "cases.def").read_text())


def parse_cases(definition):
    result = []
    for line in definition.splitlines():
        match = re.fullmatch(r"PROBE\(([a-z0-9_]+), ([0-9]+), (gpr|gpr32|select|fp)\)", line)
        if not match:
            raise ValueError("invalid forwarding case definition")
        name, count, kind = match.groups()
        result.append(dict(id=name, instructions_per_iteration=int(count), kind=kind))
    if not result or len({c["id"] for c in result}) != len(result):
        raise ValueError("empty/duplicate forwarding cases")
    return result


def expected(case, iterations):
    updates = 64 * iterations
    return {"gpr": updates + 1, "gpr32": (updates + 1) % (1 << 32),
            "select": updates % 65536, "fp": updates // 2 + 1}[case["kind"]]


def execute(binary, root, case, index, iterations, mode, revision=None, profile=None):
    args = [str(binary), "elf", "--mode", mode, "--max-instructions", "2000000", "--timeout", "90s",
            str(Path(root) / ELF), str(index), str(iterations)]
    process = subprocess.run(args, capture_output=True, check=True, timeout=100)
    row = json.loads(process.stdout)
    if process.stderr:
        raise ValueError("probe process wrote stderr")
    return validate_execution(row, root, case, index, iterations, mode, revision, profile)


def validate_execution(row, root, case, index, iterations, mode, revision=None, profile=None):
    if (row["termination"] != "exit" or row["exit_status"] != 0 or row["stderr_base64"]
            or row["binary_sha256"] != digest(Path(root) / ELF)
            or row["args"] != ["program", str(index), str(iterations)] or row["env"]
            or base64.b64decode(row["stdout_base64"], validate=True) != struct.pack("<Q", expected(case, iterations))
            or type(row["instructions"]) is not int or not 0 < row["instructions"] <= 2000000):
        raise ValueError("invalid forwarding execution: " + case["id"])
    if revision is not None and (row["build"].get("vcs.revision") != revision or row["build"].get("vcs.modified") != "false"):
        raise ValueError("forwarding simulator revision mismatch")
    if mode == "ooo":
        timing = row["timing"]
        if (timing["retired_instructions"] != row["instructions"] or type(timing["user_cycles"]) is not int
                or timing["user_cycles"] <= 0 or (profile is not None and any(timing[k] != profile[k] for k in ("configuration", "cache_config")))):
            raise ValueError("invalid forwarding timing/profile")
    elif "timing" in row:
        raise ValueError("functional probe unexpectedly timed")
    return row


def smoke(binary, root=ROOT):
    for index, case in enumerate(cases(root), 1):
        samples = []
        for iterations in (16, 32):
            functional = execute(binary, root, case, index, iterations, "functional")
            timed = execute(binary, root, case, index, iterations, "ooo")
            if functional["instructions"] != timed["instructions"]:
                raise ValueError("functional/timing mismatch")
            samples.append(timed["instructions"])
        if samples[1] - samples[0] != 16 * case["instructions_per_iteration"]:
            raise ValueError("incorrect smoke loop instruction count")
        if case["kind"] == "select":
            # Exercise both sides of the wrap/select boundary, not only the
            # initially increasing part covered by the short count check.
            for iterations in (1023, 1024, 1025):
                functional = execute(binary, root, case, index, iterations, "functional")
                timed = execute(binary, root, case, index, iterations, "ooo")
                if functional["instructions"] != timed["instructions"]:
                    raise ValueError("select wrap functional/timing mismatch")
        print(case["id"], "functional + ooo checksum passed", flush=True)


def measure(binary, root, revision, profile, *, checkpoint=None, progress=None):
    declared = cases(root)
    rows = []
    for index, case in enumerate(declared, 1):
        if progress:
            progress(rows, case["id"])
        samples = []
        for n in ITERATIONS:
            collect = lambda: execute(binary, root, case, index, n, "ooo", revision, profile)
            validate = lambda row: validate_execution(row, root, case, index, n, "ooo", revision, profile)
            samples.append(checkpoint(case, index, n, collect, validate) if checkpoint else collect())
        instructions = samples[1]["instructions"] - samples[0]["instructions"]
        cycles = samples[1]["timing"]["user_cycles"] - samples[0]["timing"]["user_cycles"]
        rows.append(dict(id=case["id"], instructions=instructions, cycles=cycles, updates=UPDATES_PER_ITERATION * (ITERATIONS[1] - ITERATIONS[0])))
        validate_simulation(rows, declared, partial=True)
        if progress:
            progress(rows, None)
    validate_simulation(rows, declared)
    return rows


def validate_simulation(rows, declared, *, partial=False):
    declared = covered_cases(rows, declared, partial=partial)
    for row, case in zip(rows, declared):
        if (type(row.get("instructions")) is not int or row["instructions"] != (ITERATIONS[1] - ITERATIONS[0]) * case["instructions_per_iteration"]
                or type(row.get("cycles")) is not int or row["cycles"] <= 0
                or type(row.get("updates")) is not int or row["updates"] != UPDATES_PER_ITERATION * (ITERATIONS[1] - ITERATIONS[0])):
            raise ValueError("invalid forwarding counters")


def declared(files):
    return any(name in files for name in [*MEASUREMENT_INPUTS, ELF, REFERENCE])


def validate_comparison(simulations, declared_cases, *, partial=False):
    if not isinstance(simulations, dict) or set(simulations) != {"base", "head"}:
        raise ValueError("forwarding requires both base and head simulation")
    for rows in simulations.values():
        validate_simulation(rows, declared_cases, partial=partial)


def validate_reference(reference, files=None):
    if (reference.get("protocol") != PROTOCOL or reference.get("timer") != "CLOCK_THREAD_CPUTIME_ID"
            or reference.get("core_identity") != "unknown" or reference.get("instantaneous_frequency") != "unknown"
            or reference.get("hardware") != "Apple M2" or reference.get("trials") != 9
            or reference.get("assumed_hz") != 3490000000):
        raise ValueError("invalid forwarding reference protocol/environment")
    if (not isinstance(reference.get("compiler"), str) or not reference["compiler"]
            or not isinstance(reference.get("platform"), str) or not reference["platform"]
            or not re.fullmatch(r"[0-9a-f]{64}", reference.get("native_binary_sha256", ""))):
        raise ValueError("missing native build identity")
    try:
        timestamp = datetime.fromisoformat(reference["collected_at_utc"])
        if timestamp.utcoffset() is None or timestamp.utcoffset().total_seconds() != 0:
            raise ValueError("non-UTC measurement time")
    except (KeyError, TypeError, ValueError):
        raise ValueError("invalid native collection timestamp") from None
    sources = reference.get("sources", {})
    if set(sources) != set(INPUTS) or any(not re.fullmatch(r"[0-9a-f]{64}", sha) for sha in sources.values()):
        raise ValueError("incomplete forwarding provenance")
    if files is not None and any(files.get(name) != sources[name] for name in MEASUREMENT_INPUTS):
        raise ValueError("forwarding reference differs from source")
    if files is not None and not re.fullmatch(r"[0-9a-f]{64}", files.get(ELF, "")):
        raise ValueError("forwarding ELF absent from tested source")
    definition = reference.get("definition", "")
    if hashlib.sha256(definition.encode()).hexdigest() != sources[DIRECTORY + "cases.def"]:
        raise ValueError("forwarding case definition digest mismatch")
    declared = reference.get("cases", [])
    if [{k: row[k] for k in ("id", "instructions_per_iteration", "kind")} for row in declared] != parse_cases(definition):
        raise ValueError("forwarding native case coverage mismatch")
    if not declared or len({r["id"] for r in declared}) != len(declared):
        raise ValueError("missing/duplicate forwarding native cases")
    for row in declared:
        if (not re.fullmatch(r"[a-z0-9_]+", row["id"]) or row["kind"] not in {"gpr", "gpr32", "select", "fp"}
                or type(row["instructions_per_iteration"]) is not int or row["instructions_per_iteration"] < 3
                or type(row["iterations"]) is not int or row["iterations"] != 500000000 // row["instructions_per_iteration"]):
            raise ValueError("invalid forwarding native case")
        values = [row[k + "_seconds"] for k in ("p10", "median", "p90")]
        if any(type(v) not in (int, float) or not math.isfinite(v) or v <= 0 for v in values) or values != sorted(values):
            raise ValueError("invalid forwarding native distribution")
    return reference


def load_reference(root):
    root = Path(root)
    reference = json.loads((root / REFERENCE).read_text())
    validate_reference(reference, {name: digest(root / name) for name in [*MEASUREMENT_INPUTS, ELF]})
    if [{k: row[k] for k in ("id", "instructions_per_iteration", "kind")} for row in reference["cases"]] != cases(root):
        raise ValueError("forwarding manifest differs from reference")
    return reference


def collect(output, audit_directory=None):
    audit = Path(audit_directory) if audit_directory else None
    if audit is not None:
        audit.mkdir(parents=True, exist_ok=False)
    hardware = subprocess.check_output(["sysctl", "-n", "machdep.cpu.brand_string"], text=True).strip()
    if hardware != "Apple M2":
        raise ValueError("M2 reference collection requires Apple M2")
    declared = cases()
    reference = dict(protocol=PROTOCOL, hardware=hardware, platform=platform.platform(),
                     collected_at_utc=datetime.now(timezone.utc).isoformat(), timer="CLOCK_THREAD_CPUTIME_ID",
                     core_identity="unknown", instantaneous_frequency="unknown", assumed_hz=3490000000, trials=9,
                     compiler=subprocess.check_output(["clang", "--version"], text=True).splitlines()[0],
                     sources={name: digest(ROOT / name) for name in INPUTS},
                     definition=(ROOT / DIRECTORY / "cases.def").read_text(), cases=[])
    with tempfile.TemporaryDirectory() as folder:
        binary = Path(folder) / "native"
        subprocess.run(["clang", "-O2", "-arch", "arm64", str(ROOT / DIRECTORY / "native.c"),
                        str(ROOT / DIRECTORY / "loops.S"), "-o", str(binary)], check=True)
        reference["native_binary_sha256"] = digest(binary)
        for index, case in enumerate(declared, 1):
            iterations = 500000000 // case["instructions_per_iteration"]
            raw = subprocess.check_output([str(binary), str(index), str(iterations)], text=True)
            if audit is not None:
                (audit / (case["id"] + ".txt")).write_text(raw)
            lines = raw.splitlines()
            differences = []
            for trial, line in enumerate(lines):
                number, seconds, short_checksum, long_checksum = line.split()
                value = float(seconds)
                if (int(number) != trial or int(short_checksum) != expected(case, iterations)
                        or int(long_checksum) != expected(case, 2 * iterations) or not math.isfinite(value) or value <= 0):
                    raise ValueError("native probe checksum/timer failed")
                differences.append(value)
            if len(differences) != 9:
                raise ValueError("incomplete native samples")
            differences.sort()
            row = dict(case, iterations=iterations, p10_seconds=differences[0], median_seconds=statistics.median(differences), p90_seconds=differences[7])
            reference["cases"].append(row)
            print(case["id"], row["median_seconds"], flush=True)
    validate_reference(reference)
    if reference["sources"] != {name: digest(ROOT / name) for name in INPUTS}:
        raise ValueError("forwarding collection source changed during measurement")
    serialized = json.dumps(reference, indent=2) + "\n"
    Path(output).write_text(serialized)
    if audit is not None:
        (audit / "reference.json").write_text(serialized)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    smoke_parser = commands.add_parser("smoke")
    smoke_parser.add_argument("--poros", required=True, type=Path)
    collect_parser = commands.add_parser("collect")
    collect_parser.add_argument("--output", type=Path, default=ROOT / REFERENCE)
    collect_parser.add_argument("--audit-directory", type=Path, help="Optional new local directory for raw trial stdout; do not commit")
    args = parser.parse_args()
    if args.command == "smoke":
        smoke(args.poros)
    else:
        collect(args.output, args.audit_directory)


if __name__ == "__main__":
    main()
