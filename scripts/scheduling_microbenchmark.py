#!/usr/bin/env python3
"""Execution-unit gap probes using the shared assembly evidence runner."""
import argparse
from functools import partial
from pathlib import Path

import throughput_microbenchmark as shared

ROOT = shared.ROOT
DIRECTORY = "evaluation/microbenchmarks/scheduling/"
PROTOCOL = "assembly-scheduling-v1"


def expected(case, iterations):
    scale, offset = {"int_chain": (384, 12), "int_interleaved": (768, -28),
                     "fp_chain": (128, 6), "fp_interleaved": (256, -6)}[case["kind"]]
    return scale * iterations + offset



SUITE = shared.Suite(DIRECTORY, PROTOCOL,
                     ("int_chain", "int_interleaved", "fp_chain", "fp_interleaved"), expected,
                     ("scripts/throughput_microbenchmark.py", "scripts/scheduling_microbenchmark.py"))
ELF, REFERENCE = SUITE.elf, SUITE.reference
MEASUREMENT_INPUTS, COLLECTOR_SOURCES, INPUTS = SUITE.measurement_inputs, SUITE.collector_sources, SUITE.inputs
ITERATIONS = shared.ITERATIONS
digest = shared.digest
cases = partial(shared.cases, suite=SUITE)
parse_cases = partial(shared.parse_cases, suite=SUITE)
execute = partial(shared.execute, suite=SUITE)
smoke = partial(shared.smoke, suite=SUITE)
measure = partial(shared.measure, suite=SUITE)
measure_case = partial(shared.measure_case, suite=SUITE)
validate_execution = partial(shared.validate_execution, suite=SUITE)
summarize_samples = shared.summarize_samples
validate_reference = partial(shared.validate_reference, suite=SUITE)
load_reference = partial(shared.load_reference, suite=SUITE)
collect = partial(shared.collect, suite=SUITE)
validate_simulation = shared.validate_simulation


def declared(files):
    return any(name in files for name in [*MEASUREMENT_INPUTS, ELF, REFERENCE])


def validate_comparison(simulations, declared_cases, *, partial=False):
    if not isinstance(simulations, dict) or set(simulations) != {"base", "head"}:
        raise ValueError("scheduling requires both base and head simulation")
    for rows in simulations.values():
        validate_simulation(rows, declared_cases, partial=partial)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    smoke_parser = commands.add_parser("smoke")
    smoke_parser.add_argument("--poros", required=True, type=Path)
    collect_parser = commands.add_parser("collect")
    collect_parser.add_argument("--output", type=Path, default=ROOT / REFERENCE)
    collect_parser.add_argument("--evidence-dir", type=Path)
    args = parser.parse_args()
    if args.command == "smoke":
        smoke(args.poros)
    else:
        collect(args.output, evidence_dir=args.evidence_dir)


if __name__ == "__main__":
    main()
