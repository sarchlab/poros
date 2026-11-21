#!/usr/bin/env python3
"""Derive Apple performance-core references from retained Linux trials and exact ELFs."""
import argparse
from collections import Counter, defaultdict
from datetime import datetime, timezone
import gzip
import hashlib
import io
import json
import math
from pathlib import Path
import subprocess
import tarfile
import tempfile

import compute_usertime as compute
import linux_reference
import workload_contract as contract

DATA = "evaluation/usertime/apple-pcore"
SCRIPT = "scripts/pcore_reference.py"
FIXTURES = "tests/apple-pcore"
COLLECTOR = f"{DATA}/collect.py"

INPUTS = f"{DATA}/inputs.json"
LAYOUT = f"{DATA}/layout.ld"
CHIPS = {"m1": ("Apple M1", "Apple Mac mini (M1, 2020)"),
         "m2": ("Apple M2", "Apple Mac mini (M2, 2023)")}
SCOPE = ("Whole-program Linux user time, including startup and output. The ARM64 ELF is identical in the simulator and on the native host. "
         "The collector ran with CPU affinity restricted to a CPU in the highest-performance cluster.")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def reference_path(chip):
    return f"{DATA}/{chip}-reference.json"


def completion_path(chip):
    return f"{DATA}/{chip}-completion.json"


def evidence_paths(chip):
    return (f"{DATA}/{chip}-metadata.json", f"{DATA}/{chip}-trials.jsonl.gz", completion_path(chip), COLLECTOR, INPUTS, LAYOUT, SCRIPT)


def chip_for(hardware):
    return next((chip for chip, (name, _) in CHIPS.items() if name == hardware), None)


def validate_affinity(meta, chip):
    require(meta.get("hardware") == CHIPS[chip][1] and meta.get("target") == chip
            and meta.get("platform", "").startswith("Linux-") and meta.get("page_size") == 16384
            and meta.get("timer") == "wait4.ru_utime" and meta.get("warmup_blocks") == 1 and meta.get("measured_blocks") == 21,
            "invalid Apple native collection environment")
    cpu = meta.get("core")
    require(type(cpu) is int and cpu >= 0 and meta.get("affinity") == [cpu] and meta.get("governor") == "performance",
            "Apple native capture requires a single pinned CPU and performance governor")
    policies = meta.get("cpufreq_policies", [])
    require(isinstance(policies, list) and policies and all(isinstance(p, dict) and type(p.get("maximum_khz")) is int
            and p["maximum_khz"] > 0 and isinstance(p.get("cpus"), list) for p in policies), "missing CPU topology")
    pinned = [p for p in policies if cpu in p["cpus"]]
    require(len(pinned) == 1 and pinned[0]["maximum_khz"] == max(p["maximum_khz"] for p in policies)
            and pinned[0].get("governor") == "performance", "Apple native capture did not select the highest-performance cluster")
    capacities = meta.get("cpu_capacity", {})
    if capacities:
        require(capacities.get(str(cpu)) == max(capacities.values()), "pinned CPU has lower capacity")
    return cpu


def core_identity(cpu):
    return f"pinned CPU {cpu} (highest-performance cluster)"


def utc(timestamp):
    return datetime.fromtimestamp(timestamp, timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def fixture(source):
    return f"{FIXTURES}/{source.removeprefix('tests/')}"


def validate_metadata(reference, declared, hardware):
    chip = chip_for(hardware)
    require(chip is not None and reference.get("hardware") == hardware, "Apple native reference hardware mismatch")
    meta = reference.get("native_environment", {})
    cpu = validate_affinity(meta, chip)
    require(reference.get("core_identity") == core_identity(cpu) and reference.get("platform") == meta["platform"]
            and reference.get("collection_commit") == meta.get("binary_origin_revision")
            and reference.get("started_at_utc") == utc(meta["started_unix"])
            and reference.get("environment") == "native" and reference.get("executable_format") == "ELF"
            and reference.get("machine") == "aarch64" and reference.get("page_size") == 16384
            and reference.get("startup_included") is True and reference.get("reference_scope") == SCOPE,
            "invalid Apple native reference scope")
    bindings = reference.get("binary_bindings", {})
    require(bindings == declared.get("binary_bindings") and all(b["file"] == fixture(source) for source, b in bindings.items()),
            "Apple native reference ELF binding mismatch")
    evidence = reference.get("capture_evidence", {})
    require(evidence.get("sample_count") == len(declared["cases"]) * 21 and evidence.get("warmup_count") == len(declared["cases"])
            and contract.SHA256.fullmatch(evidence.get("capture_sha256", "")) is not None,
            "invalid Apple raw capture evidence")
    require(reference.get("collector_sources") == {COLLECTOR: meta.get("collector_sha256")}
            and reference.get("input_sources", {}).get(INPUTS) == meta.get("inputs_sha256"),
            "Apple native collector or input digest mismatch")
    require(reference.get("quality_counts") == dict(Counter(r["quality"] for r in reference["cases"])), "Apple native coverage mismatch")


def derive(root, chip, *, git_root=None):
    import performance_reference as performance

    root = Path(root)
    git_root = root if git_root is None else git_root
    metadata_path, trials_path, *_ = evidence_paths(chip)
    meta = compute.read(root / metadata_path)
    cpu = validate_affinity(meta, chip)
    require(contract.digest(root / COLLECTOR) == meta["collector_sha256"] and contract.digest(root / INPUTS) == meta["inputs_sha256"],
            "Apple native collector or input files changed")
    canonical = performance.inventory(root)
    original = performance.input_sources(canonical, contract.digest(root / contract.PERFORMANCE_MANIFEST),
                                         contract.digest(root / "tests/benchmarks/provenance.json"))
    commit = meta["binary_origin_revision"]
    linux_reference.verify_original_inputs(git_root, commit, original)
    outputs, oracle = linux_reference.collection_outputs(git_root, {"source_commit": commit})
    captured = compute.read(root / INPUTS)
    capture_cases = {c["id"]: c for c in captured}
    require(len(captured) == len(capture_cases) == len(canonical["cases"])
            and set(capture_cases) == {c["id"] for c in canonical["cases"]}, "Apple native inventory coverage mismatch")
    bindings = {}
    for case in canonical["cases"]:
        source = case["file"] if "outputs" in case else "tests/usertime/" + case["file"]
        native = capture_cases[case["id"]]
        expected = outputs[case["id"]]
        require(native["path"] == source and all(native[k] == case[k] for k in ("args", "env", "workload", "point"))
                and all(native[k] == expected[k] for k in ("output_sha256", "output_bytes")), "Apple native input/output identity mismatch")
        path = fixture(source)
        bound = bindings.get(source)
        if bound is None:
            bound = dict(file=path, sha256=contract.digest(root / path), mapped_image_bytes=contract.mapped_image_bytes(root / path))
            bindings[source] = bound
        require(bound["sha256"] == native["sha256"], "Apple native ELF hash mismatch")
    declared = compute.expanded_manifest(canonical["original"], canonical["compute"], bindings)
    raw = gzip.decompress((root / trials_path).read_bytes())
    blocks, samples = defaultdict(set), defaultdict(list)
    for line in raw.splitlines():
        trial = json.loads(line)
        ident, block = trial["id"], trial["block"]
        case = capture_cases[ident]
        require(type(block) is int and 0 <= block <= 21 and ident not in blocks[block] and trial["exit_status"] == 0
                and trial["parent_cpu_at_reap"] == cpu and all(trial[k] == case[k] for k in ("output_bytes", "output_sha256")),
                "Apple native sample identity/correctness mismatch")
        seconds = trial["user_seconds"]
        require(type(seconds) in (int, float) and math.isfinite(seconds) and seconds >= 0, "invalid Apple native user time")
        blocks[block].add(ident)
        if block:
            samples[ident].append(trial)
    require(set(blocks) == set(range(22)) and all(ids == set(capture_cases) for ids in blocks.values()), "incomplete Apple native capture")
    rows = []
    for case in declared["cases"]:
        ident = case["id"]
        output = outputs[ident]
        numerical = "equations" if case["workload"] in performance.NUMERICAL else "shape_finite" if "outputs" in case else "analytic"
        rows.append(performance.summarize(case, samples[ident], output["output_sha256"], output["output_bytes"], numerical))
    inputs = dict(original)
    inputs.update({b["file"]: b["sha256"] for b in bindings.values()})
    inputs.update({p: contract.digest(root / p) for p in evidence_paths(chip)})
    capture_sha = hashlib.sha256(raw).hexdigest()
    capture_end = compute.read(root / completion_path(chip))
    # The compact completion record binds the measured end time to these raw bytes.
    require(capture_end.get("trials_sha256") == capture_sha, "Apple native trial digest differs from completion record")
    result = dict(schema=1, protocol=performance.PINNED_LINUX_PROTOCOL,
                  inventory_sha256=compute.object_digest(declared), collection_commit=commit,
                  measurement_id=f"{chip}-linux-pcore-{capture_sha}", metric="child_user_cpu_seconds", timer="wait4.ru_utime",
                  quality_policy=compute.QUALITY_POLICY, hardware=CHIPS[chip][0], environment="native", executable_format="ELF",
                  machine="aarch64", page_size=16384, platform=meta["platform"], native_environment=meta,
                  core_identity=core_identity(cpu), instantaneous_frequency="unknown", startup_included=True,
                  reference_scope=SCOPE, capture_evidence=dict(capture_sha256=capture_sha, sample_count=len(rows)*21, warmup_count=len(rows)),
                  binary_bindings=bindings, output_oracle=oracle, quality_counts=dict(Counter(r["quality"] for r in rows)),
                  input_sources=inputs, collector_sources={COLLECTOR: inputs[COLLECTOR]},
                  micro_manifest_sha256=inputs[contract.PERFORMANCE_MANIFEST], build_manifest_sha256=inputs["tests/benchmarks/provenance.json"],
                  started_at_utc=utc(meta["started_unix"]), completed_at_utc=utc(capture_end["completed_unix"]), cases=rows)
    return performance.validate(result, declared, CHIPS[chip][0])


def derive_at(git_root, commit, chip):
    data = subprocess.check_output(["git", "-C", str(git_root), "archive", commit])
    with tempfile.TemporaryDirectory(prefix="poros-pcore-reference-") as folder:
        with tarfile.open(fileobj=io.BytesIO(data)) as tar:
            tar.extractall(folder, filter="data")
        return derive(folder, chip, git_root=git_root)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("chip", choices=CHIPS)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--write", action="store_true")
    args = parser.parse_args()
    reference = derive(args.root, args.chip)
    path = args.root / reference_path(args.chip)
    if args.write:
        compute.write(path, reference)
    else:
        require(reference == compute.read(path), "committed Apple native reference differs from raw captures")
    print(f"{args.chip}: {len(reference['cases'])} points on {reference['core_identity']}")


if __name__ == "__main__":
    main()
