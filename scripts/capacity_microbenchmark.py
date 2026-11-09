#!/usr/bin/env python3
"""FP capacity and divider probes using the shared assembly evidence runner."""
import argparse
import struct
import json
import math
import re
import statistics
import subprocess
from functools import partial
from pathlib import Path

import throughput_microbenchmark as shared

ROOT = shared.ROOT
DIRECTORY = "evaluation/microbenchmarks/capacity/"
PROTOCOL = "assembly-capacity-v1"


def expected(case, iterations):
    kind = case["kind"]
    if kind not in ("fdiv", "fsqrt"):
        return {"window": 5, "rob": 2, "ports": 3}[kind]
    lanes = int(case["id"].split("lanes")[1])
    if "normal" not in case["id"]:
        return lanes
    # Normal FDIV alternates 1.5 and 2/1.5 (an even number per lane).
    # FSQRT(x+1) converges to the golden ratio, never a power-of-two input.
    return int(lanes * (1.5 if kind == "fdiv" else (1 + 5 ** .5) / 2))



SUITE = shared.Suite(DIRECTORY, PROTOCOL,
                     ("window", "rob", "ports", "fdiv", "fsqrt"), expected,
                     ("scripts/throughput_microbenchmark.py", "scripts/capacity_microbenchmark.py"))
ELF, REFERENCE = SUITE.elf, SUITE.reference
MEASUREMENT_INPUTS, COLLECTOR_SOURCES, INPUTS = SUITE.measurement_inputs, SUITE.collector_sources, SUITE.inputs
ITERATIONS = shared.ITERATIONS
digest = shared.digest
cases = partial(shared.cases, suite=SUITE)
parse_cases = partial(shared.parse_cases, suite=SUITE)
execute = partial(shared.execute, suite=SUITE)
smoke = partial(shared.smoke, suite=SUITE)
measure = partial(shared.measure, suite=SUITE)

validate_simulation = shared.validate_simulation


def validate_boundary(value, sources):
    if (value.get("protocol") != "capacity-boundary-abba-v1" or value.get("iterations") != 200000
            or not re.fullmatch(r"[0-9a-f]{64}", value.get("native_binary_sha256", ""))
            or value.get("sources") != {name: sources[name] for name in MEASUREMENT_INPUTS}):
        raise ValueError("capacity boundary provenance mismatch")
    expected_ids = [f"fp_g{gate}_n{n}" for gate in (64, 96)
                    for n in (320, 336, 344, 348, 350, 352, 354, 356, 358, 360, 362, 364, 366, 368, 370, 372, 384)]
    if [r.get("id") for r in value.get("cases", [])] != expected_ids:
        raise ValueError("capacity boundary coverage mismatch")
    for row in value["cases"]:
        samples = row.get("samples", [])
        if [sample.get("trial") for sample in samples] != list(range(9)):
            raise ValueError("capacity boundary trials missing")
        for sample in samples:
            if any(type(sample.get(k)) not in (int, float) or not math.isfinite(sample[k]) or sample[k] <= 0
                   for k in ("ratio", "target_ns", "control_ns")):
                raise ValueError("invalid capacity boundary sample")
            if not math.isclose(sample["ratio"], sample["target_ns"] / sample["control_ns"], rel_tol=1e-12):
                raise ValueError("capacity boundary ratio mismatch")
        ratios = sorted(sample["ratio"] for sample in samples)
        if (row.get("p10_ratio") != ratios[0] or row.get("median_ratio") != ratios[4] or row.get("p90_ratio") != ratios[7]
                or row.get("qualified") is not ((ratios[7] - ratios[0]) / ratios[4] <= .20)):
            raise ValueError("capacity boundary quality/summary mismatch")
    return value


def validate_reference(reference, files=None):
    shared.validate_reference(reference, files, SUITE)
    validate_boundary(reference.get("boundary", {}), reference["sources"])
    return reference


def load_reference(root=ROOT):
    reference = shared.load_reference(root, SUITE)
    return validate_reference(reference)


def collect(output, evidence_dir=None):
    if evidence_dir is None:
        raise ValueError("capacity collection requires retained boundary and raw evidence directory")
    boundary_data = json.loads((Path(evidence_dir) / "boundary.json").read_text())
    sources = {name: digest(ROOT / name) for name in MEASUREMENT_INPUTS}
    validate_boundary(boundary_data, sources)
    shared.collect(output, SUITE, evidence_dir)
    reference = json.loads(Path(output).read_text())
    reference["boundary"] = boundary_data
    validate_reference(reference)
    Path(output).write_text(json.dumps(reference, indent=2) + "\n")


def declared(files):
    return any(name in files for name in [*MEASUREMENT_INPUTS, ELF, REFERENCE])


def validate_comparison(simulations, declared_cases):
    if not isinstance(simulations, dict) or set(simulations) != {"base", "head"}:
        raise ValueError("capacity requires both base and head simulation")
    for rows in simulations.values():
        validate_simulation(rows, declared_cases)


def boundary(binary, output, evidence_dir):
    """Dense, paired control sweep; raw ns remain primary, ratios control drift."""
    sources = {name: digest(ROOT / name) for name in MEASUREMENT_INPUTS}
    binary_sha = digest(binary)
    declared = cases()
    indexes = {c["id"]: i for i, c in enumerate(declared, 1)}
    evidence_dir = Path(evidence_dir)
    evidence_dir.mkdir(parents=True, exist_ok=True)
    counts = (320, 336, 344, 348, 350, 352, 354, 356, 358, 360, 362, 364, 366, 368, 370, 372, 384)
    rows = {f"fp_g{gate}_n{n}": [] for gate in (64, 96) for n in counts}
    iterations = 200000
    # Rotate and reverse case order per trial, rather than collecting all repeats
    # of one footprint together. The per-process ABBA control removes first-order
    # frequency drift; it cannot prove core identity or exact physical capacity.
    names = list(rows)
    for trial in range(9):
        order = names[trial:] + names[:trial]
        if trial % 2:
            order.reverse()
        for name in order:
            gate = int(name.split("_g")[1].split("_")[0])
            control = f"nop_g{gate}_n352"
            raw = subprocess.check_output([str(binary), str(indexes[name]), str(iterations), str(indexes[control]), str(trial)], text=True)
            (evidence_dir / f"{name}-trial{trial}.txt").write_text(raw)
            values = {indexes[name]: [], indexes[control]: []}
            for line in raw.splitlines():
                index, short, long, a, b = line.split()
                index = int(index)
                delta = float(long) - float(short)
                if (index not in values or not math.isfinite(delta) or delta <= 0 or not all(math.isfinite(float(v)) and float(v) > 0 for v in (short, long)) or int(a) != expected(declared[index - 1], iterations)
                        or int(b) != expected(declared[index - 1], 2 * iterations)):
                    raise ValueError("boundary timing/checksum failed")
                values[index].append(delta * 1e9 / iterations)
            if any(len(v) != 2 for v in values.values()):
                raise ValueError("incomplete boundary pair")
            target_ns = statistics.mean(values[indexes[name]])
            control_ns = statistics.mean(values[indexes[control]])
            rows[name].append(dict(trial=trial, target_ns=target_ns, control_ns=control_ns, ratio=target_ns / control_ns))
        print("boundary trial", trial, flush=True)
    result = dict(protocol="capacity-boundary-abba-v1", iterations=iterations,
                  native_binary_sha256=binary_sha, sources=sources,
                  quantiles="9 samples sorted: p10=min (index0), median=index4, p90=index7; all samples retained",
                  quality_rule="p90-p10 / median ratio <= 0.20; no sample dropping",
                  inference_rule="Compare both gate lengths; a capacity rise must exceed the low-count plateau by 15% and have nonoverlapping p10/p90 bands; report a bracket, not a physical PRF count.",
                  cases=[])
    for name, samples in rows.items():
        ratios = sorted(r["ratio"] for r in samples)
        result["cases"].append(dict(id=name, samples=samples, median_ratio=ratios[4],
                                    p10_ratio=ratios[0], p90_ratio=ratios[7],
                                    qualified=(ratios[7] - ratios[0]) / ratios[4] <= .20))
    if sources != {name: digest(ROOT / name) for name in MEASUREMENT_INPUTS} or binary_sha != digest(binary):
        raise ValueError("boundary inputs changed during collection")
    Path(output).write_text(json.dumps(result, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    smoke_parser = commands.add_parser("smoke")
    smoke_parser.add_argument("--poros", required=True, type=Path)
    smoke_parser.add_argument("--qemu", type=Path)
    collect_parser = commands.add_parser("collect")
    collect_parser.add_argument("--output", type=Path, default=ROOT / REFERENCE)
    collect_parser.add_argument("--evidence-dir", type=Path)
    boundary_parser = commands.add_parser("boundary")
    boundary_parser.add_argument("--binary", type=Path, required=True)
    boundary_parser.add_argument("--output", type=Path, required=True)
    boundary_parser.add_argument("--evidence-dir", type=Path, required=True)
    args = parser.parse_args()
    if args.command == "smoke":
        smoke(args.poros)
        if args.qemu:
            for index, case in enumerate(cases(), 1):
                for iterations in (16, 32):
                    process = subprocess.run([str(args.qemu), str(ROOT / ELF), str(index), str(iterations)], capture_output=True, check=True, timeout=30)
                    if process.stderr or process.stdout != struct.pack("<Q", expected(case, iterations)):
                        raise ValueError("QEMU capacity checksum failed: " + case["id"])
                print(case["id"], "QEMU checksum passed", flush=True)
    elif args.command == "boundary":
        boundary(args.binary, args.output, args.evidence_dir)
    else:
        collect(args.output, evidence_dir=args.evidence_dir)


if __name__ == "__main__":
    main()
