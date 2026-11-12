#!/usr/bin/env python3
"""Shared assembly throughput probes: functional CI, native reference, simulation."""
from report_records import covered_cases

import argparse
import base64
from datetime import datetime, timezone
from dataclasses import dataclass
from typing import Callable
import hashlib
import json
import math
from pathlib import Path
import platform
import re
import statistics
import shutil
import struct
import subprocess
import tempfile

import devices

ROOT = Path(__file__).resolve().parents[1]
DIRECTORY = "evaluation/microbenchmarks/throughput/"
ELF = DIRECTORY + "throughput.elf"
REFERENCE = DIRECTORY + "m2.json"
MEASUREMENT_INPUTS = [DIRECTORY + name for name in ("cases.def", "loops.S", "driver.h", "native.c")]
# Recorded collection provenance, not prerequisites for reusing native measurements.
COLLECTOR_SOURCES = [DIRECTORY + name for name in ("linux.c", "entry.S", "build.sh", "throughput.elf")]
COLLECTOR_SOURCES += ["scripts/throughput_microbenchmark.py", "tests/usertime/layout.ld"]
INPUTS = MEASUREMENT_INPUTS + COLLECTOR_SOURCES
PROTOCOL = "assembly-throughput-v1"
ITERATIONS = (1024, 2048)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def expected(case, iterations):
    return {"nop": 0, "add": 256 * iterations, "fp": 128 * iterations,
            "load": 36, "store": 164, "mixed": 96 * iterations + 10}[case["kind"]]


@dataclass(frozen=True)
class Suite:
    directory: str
    protocol: str
    kinds: tuple[str, ...]
    checksum: Callable
    collector_scripts: tuple[str, ...] = ("scripts/throughput_microbenchmark.py",)

    @property
    def elf(self):
        return self.directory + self.directory.rstrip("/").split("/")[-1] + ".elf"

    @property
    def reference(self):
        return self.directory + "m2.json"

    @property
    def measurement_inputs(self):
        return [self.directory + name for name in ("cases.def", "loops.S", "driver.h", "native.c")]

    @property
    def collector_sources(self):
        return [self.directory + name for name in ("linux.c", "entry.S", "build.sh")] + [self.elf, *self.collector_scripts, "tests/usertime/layout.ld"]

    @property
    def inputs(self):
        return self.measurement_inputs + self.collector_sources


THROUGHPUT = Suite(DIRECTORY, PROTOCOL, ("nop", "add", "fp", "load", "store", "mixed"), expected)


def cases(root=ROOT, suite=THROUGHPUT):
    return parse_cases((Path(root) / suite.directory / "cases.def").read_text(), suite)


def parse_cases(definition, suite=THROUGHPUT):
    result = []
    for line in definition.splitlines():
        match = re.fullmatch(r"PROBE\(([a-z0-9_]+), ([0-9]+), ([a-z0-9_]+)\)", line)
        if not match or match.group(3) not in suite.kinds:
            raise ValueError("invalid assembly probe case definition")
        name, count, kind = match.groups()
        result.append(dict(id=name, instructions_per_iteration=int(count), kind=kind))
    if not result or len({c["id"] for c in result}) != len(result):
        raise ValueError("empty/duplicate assembly probe cases")
    return result


def execute(binary, root, case, index, iterations, mode, revision=None, profile=None, suite=THROUGHPUT, device=None):
    args = [str(binary), "elf", "--mode", mode, *devices.arguments(device), "--max-instructions", "2000000", "--timeout", "90s",
            str(Path(root) / suite.elf), str(index), str(iterations)]
    process = subprocess.run(args, capture_output=True, check=True, timeout=100)
    row = json.loads(process.stdout)
    if process.stderr:
        raise ValueError("probe process wrote stderr")
    return validate_execution(row, root, case, index, iterations, mode, revision, profile, suite, device=device)


def validate_execution(row, root, case, index, iterations, mode, revision=None, profile=None, suite=THROUGHPUT, device=None):
    if (row["termination"] != "exit" or row["exit_status"] != 0 or row["stderr_base64"]
            or row["binary_sha256"] != digest(Path(root) / suite.elf)
            or row["args"] != ["program", str(index), str(iterations)] or row["env"]
            or base64.b64decode(row["stdout_base64"], validate=True) != struct.pack("<Q", suite.checksum(case, iterations))
            or type(row["instructions"]) is not int or not 0 < row["instructions"] <= 2000000):
        raise ValueError("invalid assembly probe execution: " + case["id"])
    if revision is not None and (row["build"].get("vcs.revision") != revision or row["build"].get("vcs.modified") != "false"):
        raise ValueError("assembly probe simulator revision mismatch")
    if mode == "ooo":
        timing = row["timing"]
        if (timing["retired_instructions"] != row["instructions"] or type(timing["user_cycles"]) is not int
                or timing["user_cycles"] <= 0 or (profile is not None and any(timing[k] != profile[k] for k in ("configuration", "cache_config")))):
            raise ValueError("invalid assembly probe timing/profile")
        devices.check_timing(timing, device)
    elif "timing" in row:
        raise ValueError("functional probe unexpectedly timed")
    return row


def smoke(binary, root=ROOT, suite=THROUGHPUT):
    for index, case in enumerate(cases(root, suite), 1):
        samples = []
        for iterations in (16, 32):
            functional = execute(binary, root, case, index, iterations, "functional", suite=suite)
            timed = execute(binary, root, case, index, iterations, "ooo", suite=suite)
            if functional["instructions"] != timed["instructions"]:
                raise ValueError("functional/timing mismatch")
            samples.append(timed["instructions"])
        if samples[1] - samples[0] != 16 * case["instructions_per_iteration"]:
            raise ValueError("incorrect smoke loop instruction count")
        print(case["id"], "functional + ooo checksum passed", flush=True)


def summarize_samples(case, samples):
    if len(samples) != 2:
        raise ValueError("probe short/long execution pair missing")
    row = dict(id=case["id"], instructions=samples[1]["instructions"] - samples[0]["instructions"],
               cycles=samples[1]["timing"]["user_cycles"] - samples[0]["timing"]["user_cycles"])
    validate_simulation([row], [case])
    return row


def measure_case(binary, root, case, index, revision, profile, suite=THROUGHPUT, *, checkpoint=None, device=None):
    samples = []
    # Passed only when a device is requested, like every later execution option.
    options = {} if device is None else {"device": device}
    for n in ITERATIONS:
        collect = lambda: execute(binary, root, case, index, n, "ooo", revision, profile, suite, **options)
        validate = lambda row: validate_execution(row, root, case, index, n, "ooo", revision, profile, suite, **options)
        samples.append(checkpoint(case, index, n, collect, validate) if checkpoint else collect())
    summarize_samples(case, samples)
    return samples


def measure(binary, root, revision, profile, suite=THROUGHPUT, *, checkpoint=None, progress=None, device=None):
    declared = cases(root, suite)
    rows = []
    for index, case in enumerate(declared, 1):
        if progress:
            progress(rows, case["id"])
        samples = measure_case(binary, root, case, index, revision, profile, suite, checkpoint=checkpoint,
                               **({} if device is None else {"device": device}))
        rows.append(summarize_samples(case, samples))
        validate_simulation(rows, declared, partial=True)
        if progress:
            progress(rows, None)
    validate_simulation(rows, declared)
    return rows


def validate_simulation(rows, declared, *, partial=False):
    declared = covered_cases(rows, declared, partial=partial)
    for row, case in zip(rows, declared):
        if (type(row.get("instructions")) is not int or row["instructions"] != (ITERATIONS[1] - ITERATIONS[0]) * case["instructions_per_iteration"]
                or type(row.get("cycles")) is not int or row["cycles"] <= 0):
            raise ValueError("invalid assembly probe counters")


def validate_reference(reference, files=None, suite=THROUGHPUT):
    if (reference.get("protocol") != suite.protocol or reference.get("timer") != "CLOCK_THREAD_CPUTIME_ID"
            or reference.get("core_identity") != "unknown" or reference.get("instantaneous_frequency") != "unknown"
            or reference.get("hardware") != "Apple M2" or reference.get("trials") != 9
            or reference.get("assumed_hz") != 3490000000):
        raise ValueError("invalid assembly probe reference protocol/environment")
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
    if set(sources) != set(suite.inputs) or any(not re.fullmatch(r"[0-9a-f]{64}", sha) for sha in sources.values()):
        raise ValueError("incomplete assembly probe provenance")
    if files is not None and any(files.get(name) != sources[name] for name in suite.measurement_inputs):
        raise ValueError("assembly probe reference differs from source")
    if files is not None and not re.fullmatch(r"[0-9a-f]{64}", files.get(suite.elf, "")):
        raise ValueError("assembly probe ELF absent from tested source")
    definition = reference.get("definition", "")
    if hashlib.sha256(definition.encode()).hexdigest() != sources[suite.directory + "cases.def"]:
        raise ValueError("assembly probe case definition digest mismatch")
    declared = reference.get("cases", [])
    if [{k: row[k] for k in ("id", "instructions_per_iteration", "kind")} for row in declared] != parse_cases(definition, suite):
        raise ValueError("assembly probe native case coverage mismatch")
    if not declared or len({r["id"] for r in declared}) != len(declared):
        raise ValueError("missing/duplicate assembly probe native cases")
    for row in declared:
        if (not re.fullmatch(r"[a-z0-9_]+", row["id"]) or row["kind"] not in suite.kinds
                or type(row["instructions_per_iteration"]) is not int or row["instructions_per_iteration"] < 3
                or type(row["iterations"]) is not int or row["iterations"] != 500000000 // row["instructions_per_iteration"]):
            raise ValueError("invalid assembly probe native case")
        values = [row[k + "_seconds"] for k in ("p10", "median", "p90")]
        if any(type(v) not in (int, float) or not math.isfinite(v) or v <= 0 for v in values) or values != sorted(values):
            raise ValueError("invalid assembly probe native distribution")
    return reference


def load_reference(root, suite=THROUGHPUT):
    root = Path(root)
    reference = json.loads((root / suite.reference).read_text())
    validate_reference(reference, {name: digest(root / name) for name in [*suite.measurement_inputs, suite.elf]}, suite)
    if [{k: row[k] for k in ("id", "instructions_per_iteration", "kind")} for row in reference["cases"]] != cases(root, suite):
        raise ValueError("assembly probe manifest differs from reference")
    return reference


def collect(output, suite=THROUGHPUT, evidence_dir=None):
    hardware = subprocess.check_output(["sysctl", "-n", "machdep.cpu.brand_string"], text=True).strip()
    if hardware != "Apple M2":
        raise ValueError("M2 reference collection requires Apple M2")
    declared = cases(suite=suite)
    if evidence_dir is not None:
        evidence_dir = Path(evidence_dir)
        evidence_dir.mkdir(parents=True, exist_ok=True)
    reference = dict(protocol=suite.protocol, hardware=hardware, platform=platform.platform(),
                     collected_at_utc=datetime.now(timezone.utc).isoformat(), timer="CLOCK_THREAD_CPUTIME_ID",
                     core_identity="unknown", instantaneous_frequency="unknown", assumed_hz=3490000000, trials=9,
                     compiler=subprocess.check_output(["clang", "--version"], text=True).splitlines()[0],
                     sources={name: digest(ROOT / name) for name in suite.inputs},
                     definition=(ROOT / suite.directory / "cases.def").read_text(), cases=[])
    with tempfile.TemporaryDirectory() as folder:
        binary = Path(folder) / "native"
        subprocess.run(["clang", "-O2", "-arch", "arm64", str(ROOT / suite.directory / "native.c"),
                        str(ROOT / suite.directory / "loops.S"), "-o", str(binary)], check=True)
        reference["native_binary_sha256"] = digest(binary)
        if evidence_dir is not None:
            shutil.copyfile(binary, evidence_dir / "native")
        for index, case in enumerate(declared, 1):
            iterations = 500000000 // case["instructions_per_iteration"]
            lines = subprocess.check_output([str(binary), str(index), str(iterations)], text=True).splitlines()
            if evidence_dir is not None:
                (evidence_dir / (case["id"] + ".json")).write_text(json.dumps(dict(case=case, iterations=iterations, raw_lines=lines), indent=2) + "\n")
            differences = []
            for trial, line in enumerate(lines):
                number, seconds, short_checksum, long_checksum = line.split()
                value = float(seconds)
                if (int(number) != trial or int(short_checksum) != suite.checksum(case, iterations)
                        or int(long_checksum) != suite.checksum(case, 2 * iterations) or not math.isfinite(value) or value <= 0):
                    raise ValueError("native probe checksum/timer failed")
                differences.append(value)
            if len(differences) != 9:
                raise ValueError("incomplete native samples")
            differences.sort()
            row = dict(case, iterations=iterations, p10_seconds=differences[0], median_seconds=statistics.median(differences), p90_seconds=differences[7])
            reference["cases"].append(row)
            print(case["id"], row["median_seconds"], flush=True)
    validate_reference(reference, suite=suite)
    if reference["sources"] != {name: digest(ROOT / name) for name in suite.inputs}:
        raise ValueError("assembly probe collection source changed during measurement")
    Path(output).write_text(json.dumps(reference, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    smoke_parser = commands.add_parser("smoke")
    smoke_parser.add_argument("--poros", required=True, type=Path)
    collect_parser = commands.add_parser("collect")
    collect_parser.add_argument("--output", type=Path, default=ROOT / REFERENCE)
    args = parser.parse_args()
    if args.command == "smoke":
        smoke(args.poros)
    else:
        collect(args.output)


if __name__ == "__main__":
    main()
