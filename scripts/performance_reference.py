#!/usr/bin/env python3
"""Compact plotting references derived from complete, locally validated captures."""
from datetime import datetime
import argparse
import base64
import hashlib
import json
import math
from pathlib import Path
import re
import statistics

import compute_usertime as compute
import native_usertime as native
import workload_contract as contract

PATH = "evaluation/usertime/performance-v1.json"
PROTOCOL = "performance-summary-v1"
NUMERICAL = {"gramschmidt", "blackscholes", "bodytrack", "canneal", "raytrace", "facesim", "x264"}


def inventory(root):
    return compute.expanded_manifest(contract.load_manifest(root, performance=True), compute.contract(root))


def output_size(case):
    if "outputs" in case:
        return sum(o["count"] * (8 if o["type"] == "double" else 4) for o in case["outputs"])
    return 16 * int(case["args"][1]) if case["workload"] == "mvt" else 8


def quality(row):
    if row["nonpositive_samples"]:
        return "invalid_nonpositive"
    median = row["median_user_seconds"]
    if median < .0001:
        return "below_precision_floor"
    if (row["p90_user_seconds"] - row["p10_user_seconds"]) / median > .2:
        return "noisy"
    return "usable"


def summarize(case, samples, sha, length, numerical):
    values = sorted(s["user_seconds"] for s in samples)
    row = dict(id=case["id"], input_sha256=compute.object_digest(case), sample_count=len(values),
               nonpositive_samples=sum(v <= 0 for v in values), min_user_seconds=values[0],
               p10_user_seconds=values[int(.1*(len(values)-1))], median_user_seconds=statistics.median(values),
               p90_user_seconds=values[int(.9*(len(values)-1))], max_user_seconds=values[-1],
               output_sha256=sha, output_bytes=length, numerical_check=numerical)
    row["quality"] = quality(row)
    return row


def validate(reference, declared):
    compute.validate_expanded(declared)
    if reference.get("schema") != 1 or reference.get("protocol") != PROTOCOL or reference.get("inventory_sha256") != compute.object_digest(declared):
        raise ValueError("compact reference protocol/input identity mismatch")
    if reference.get("quality_policy") != compute.QUALITY_POLICY or reference.get("metric") != "child_user_cpu_seconds" or reference.get("timer") != "wait4.ru_utime":
        raise ValueError("compact reference accounting/quality mismatch")
    if reference.get("collection_dirty") is not False or reference.get("oracle_commit") != reference.get("collection_commit") or not re.fullmatch(r"[0-9a-f]{40}", reference.get("collection_commit", "")):
        raise ValueError("compact reference requires one exact clean collection/oracle commit")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", reference.get("container_image", "")) or reference.get("environment") != "virtualized" or reference.get("machine") not in ("aarch64", "arm64"):
        raise ValueError("invalid compact measurement environment")
    try:
        start = datetime.strptime(reference["started_at_utc"], "%Y-%m-%dT%H:%M:%SZ")
        end = datetime.strptime(reference["completed_at_utc"], "%Y-%m-%dT%H:%M:%SZ")
    except (KeyError, ValueError):
        raise ValueError("invalid compact measurement timestamps") from None
    if end < start or reference.get("core_identity") != "unknown" or reference.get("instantaneous_frequency") != "unknown":
        raise ValueError("invalid measurement scope/time order")
    for key in ("hardware", "platform", "measurement_id"):
        if not isinstance(reference.get(key), str) or not reference[key]:
            raise ValueError("missing measurement environment identity")
    for key in ("collector_sources", "input_sources"):
        if not isinstance(reference.get(key), dict) or not reference[key]:
            raise ValueError("missing compact source identities")
        for name, sha in reference[key].items():
            compute.relative(name)
            if not re.fullmatch(r"[0-9a-f]{64}", sha):
                raise ValueError("invalid compact source digest")
    if set(reference["collector_sources"]) != set(compute.support(compute.PROTOCOL)):
        raise ValueError("incomplete collection protocol sources")
    if not re.fullmatch(r"[0-9a-f]{64}", reference.get("numerical_oracle_sha256", "")):
        raise ValueError("missing numerical oracle executable identity")
    for key in ("micro_manifest_sha256", "build_manifest_sha256"):
        if not re.fullmatch(r"[0-9a-f]{64}", reference.get(key, "")):
            raise ValueError("missing collection manifest identity")
    expected_inputs = dict(declared["compute"]["build"]["files"])
    expected_inputs[contract.PERFORMANCE_MANIFEST] = reference["micro_manifest_sha256"]
    expected_inputs["tests/benchmarks/provenance.json"] = reference["build_manifest_sha256"]
    expected_inputs.update({"tests/usertime/"+name: sha for name, sha in declared["original"]["sources"].items()})
    expected_inputs.update({"tests/usertime/"+c["file"]: c["sha256"] for c in declared["original"]["cases"]})
    if reference["input_sources"] != expected_inputs:
        raise ValueError("compact input sources differ from exact ELF/build inventory")
    if [r["id"] for r in reference["cases"]] != [c["id"] for c in declared["cases"]]:
        raise ValueError("missing/duplicate/reordered compact measurements")
    for case, row in zip(declared["cases"], reference["cases"]):
        if row.get("input_sha256") != compute.object_digest(case) or type(row.get("sample_count")) is not int or row["sample_count"] != 21:
            raise ValueError("compact case identity/sample count mismatch")
        count = row.get("nonpositive_samples")
        if type(count) is not int or not 0 <= count <= 21:
            raise ValueError("invalid nonpositive sample count")
        values = [row.get(k + "_user_seconds") for k in ("min", "p10", "median", "p90", "max")]
        if any(type(v) not in (int, float) or not math.isfinite(v) or v < 0 for v in values) or values != sorted(values) or (values[0] == 0) != bool(count):
            raise ValueError("invalid compact timing distribution")
        if row.get("quality") != quality(row):
            raise ValueError("compact quality differs from unchanged policy")
        numerical = "equations" if case["workload"] in NUMERICAL else "shape_finite" if "outputs" in case else "analytic"
        if row.get("numerical_check") != numerical or type(row.get("output_bytes")) is not int or row["output_bytes"] != output_size(case) or not re.fullmatch(r"[0-9a-f]{64}", row.get("output_sha256", "")):
            raise ValueError("invalid complete-output oracle identity")
    return reference


def load(root):
    root = Path(root)
    declared = inventory(root)
    reference = validate(compute.read(root / PATH), declared)
    for name, sha in reference["input_sources"].items():
        if contract.digest(root / name) != sha:
            raise ValueError("live source/ELF changed since measurement")
    return declared, reference


def export(root, micro_path, compute_folder, output):
    """Validate full local evidence before discarding it from the public format."""
    root, compute_folder = Path(root), Path(compute_folder)
    declared = inventory(root)
    micro = compute.read(micro_path)
    native.validate_capture_metadata(micro)
    full = compute.validate_reference(compute_folder, root)
    oracle = compute.read(compute_folder / "oracle.json")
    for key in ("container_image", "environment", "machine", "hardware", "platform", "core_identity", "instantaneous_frequency", "metric", "timer", "quality_policy"):
        if micro.get(key) != full.get(key):
            raise ValueError("mixed micro/compute measurement environment")
    micro_start = datetime.strptime(micro["started_at_utc"], "%Y-%m-%dT%H:%M:%SZ")
    micro_end = datetime.strptime(micro["completed_at_utc"], "%Y-%m-%dT%H:%M:%SZ")
    compute_start = datetime.strptime(full["started_at_utc"], "%Y-%m-%dT%H:%M:%SZ")
    if not micro_start <= micro_end <= compute_start:
        raise ValueError("mixed or overlapping native measurement campaigns")
    if micro["collection_commit"] != full["collection_commit"] or oracle["validation_commit"] != full["collection_commit"]:
        raise ValueError("mixed collection/oracle commits")
    if micro["manifest_sha256"] != contract.digest(root / contract.PERFORMANCE_MANIFEST) or micro["harness_sha256"] != contract.digest(root / "scripts/native_usertime.py") or micro["support_sha256"] != {"workload_contract.py": contract.digest(root / "scripts/workload_contract.py")}:
        raise ValueError("micro collection source changed")
    rows = []
    if len(micro["cases"]) != 24:
        raise ValueError("incomplete micro capture")
    for case, measured in zip(declared["original"]["cases"], micro["cases"]):
        if any(measured[k] != value for k, value in case.items()) or len(measured["samples"]) != 21:
            raise ValueError("micro capture identity/count mismatch")
        sha, length = None, None
        for sample in [measured["warmup_sample"], *measured["samples"]]:
            if sample["exit_status"] != 0 or sample["stderr_base64"]:
                raise ValueError("micro native execution failed")
            if any(type(sample[k]) not in (int, float) or not math.isfinite(sample[k]) or sample[k] < 0 for k in ("user_seconds", "system_seconds")):
                raise ValueError("invalid raw micro timing")
            data = base64.b64decode(sample["stdout_base64"], validate=True)
            contract.check_output(case, data)
            current = hashlib.sha256(data).hexdigest()
            if sha is not None and current != sha:
                raise ValueError("micro outputs differ between samples")
            sha, length = current, len(data)
        row = summarize(case, measured["samples"], sha, length, "analytic")
        if row["quality"] != measured["quality"] or row["median_user_seconds"] != measured["median_user_seconds"]:
            raise ValueError("micro statistics mismatch")
        rows.append(row)
    for case, measured, checked in zip(declared["compute"]["cases"], full["cases"], oracle["cases"]):
        rows.append(summarize(case, measured["samples"], measured["output_sha256"], measured["output_bytes"], checked["numerical_check"]))
    inputs = dict(declared["compute"]["build"]["files"])
    inputs[contract.PERFORMANCE_MANIFEST] = contract.digest(root / contract.PERFORMANCE_MANIFEST)
    inputs["tests/benchmarks/provenance.json"] = contract.digest(root / "tests/benchmarks/provenance.json")
    inputs.update({"tests/usertime/"+name: sha for name, sha in declared["original"]["sources"].items()})
    inputs.update({"tests/usertime/"+c["file"]: c["sha256"] for c in declared["original"]["cases"]})
    result = {key: full[key] for key in ("collection_commit", "collection_dirty", "container_image", "environment", "machine", "hardware", "platform", "core_identity", "instantaneous_frequency", "metric", "timer", "quality_policy", "completed_at_utc")}
    result.update(schema=1, protocol=PROTOCOL, measurement_id="m2-linuxvm-performance-v1", oracle_commit=oracle["validation_commit"], numerical_oracle_sha256=oracle["numerical_oracle_sha256"],
                  started_at_utc=micro["started_at_utc"], inventory_sha256=compute.object_digest(declared),
                  collector_sources=full["support_sha256"], input_sources=inputs, cases=rows,
                  micro_manifest_sha256=inputs[contract.PERFORMANCE_MANIFEST], build_manifest_sha256=inputs["tests/benchmarks/provenance.json"])
    validate(result, declared)
    if Path(output).exists():
        raise ValueError("never overwrite compact measurement data")
    compute.write(output, result)
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--micro", required=True)
    parser.add_argument("--compute", required=True)
    parser.add_argument("--output", required=True)
    a = parser.parse_args()
    export(a.root, a.micro, a.compute, a.output)
