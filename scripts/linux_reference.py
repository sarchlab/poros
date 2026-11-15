#!/usr/bin/env python3
"""Bind an external native Linux summary to reproducible, exact ELF inputs.

The contributor's summary is retained unchanged. This adapter records its
identity and the separate relinked fixtures; it does not certify absent raw
captures or infer a hardware model from native timings.
"""
import argparse
from collections import Counter
from datetime import datetime
import hashlib
import io
import json
import math
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import tempfile

import compute_usertime as compute
import workload_contract as contract

PROTOCOL = "performance-linux-summary-v1"
SUMMARY_PROTOCOL = "linux-native-64k-relink-user-v1"
SUMMARY = "evaluation/usertime/grace-native/summary.json"
REFERENCE = "evaluation/usertime/grace-native/performance-v1.json"
SCRIPT = "scripts/linux_reference.py"
HARDWARE = "NVIDIA Grace (Arm Neoverse V2)"
SCOPE = ("Exact SHA-256-matched 64 KiB-aligned native Linux ELF; whole-child wait4 user time, "
         "including startup and output. Contributor summary retained; raw captures and the "
         "original collection tools are external and have not been independently audited here.")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def verify_original_inputs(root, commit, inputs):
    """Case labels alone cannot bind arguments: check the measured Git inputs."""
    archive = subprocess.check_output(["git", "-C", str(root), "archive", commit, *sorted(inputs)])
    with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
        actual = {item.name: hashlib.sha256(tar.extractfile(item).read()).hexdigest()
                  for item in tar if item.isfile()}
    require(actual == inputs, "current workload arguments/sources differ from exact native collection commit")


def validate_environment(environment):
    require(isinstance(environment, dict) and environment.get("machine") == "aarch64"
            and environment.get("page_size") == 65536 and environment.get("cpu_model") == "Neoverse-V2"
            and environment.get("platform", "").startswith("Linux-")
            and type(environment.get("pinned_cpu")) is int and environment["pinned_cpu"] >= 0
            and environment.get("affinity") == [environment["pinned_cpu"]],
            "invalid Grace native Linux environment")
    require(all(isinstance(environment.get(k), str) and environment[k]
                for k in ("hostname", "kernel", "governor", "toolchain", "linker")),
            "incomplete Grace native Linux environment")


def validate_summary(summary, canonical, outputs):
    require(summary.get("schema") == 1 and summary.get("protocol") == SUMMARY_PROTOCOL,
            "unsupported external native Linux summary")
    require(re.fullmatch(r"[0-9a-f]{40}", summary.get("source_commit", "")) is not None
            and contract.SHA256.fullmatch(summary.get("capture_sha256", "")) is not None,
            "missing external collection identity")
    environment = summary.get("environment", {})
    validate_environment(environment)
    try:
        times = [datetime.strptime(summary[k], "%Y-%m-%dT%H:%M:%SZ") for k in ("started_at_utc", "completed_at_utc")]
    except (KeyError, ValueError):
        raise ValueError("invalid external collection timestamps") from None
    require(times[0] <= times[1], "invalid external collection time order")
    expected = {c["file"] if "outputs" in c else "tests/usertime/" + c["file"] for c in canonical["cases"]}
    require(set(summary.get("binaries", {})) == expected
            and all(contract.SHA256.fullmatch(s) for s in summary["binaries"].values()),
            "external native ELF coverage mismatch")
    rows = summary.get("cases", [])
    require([r.get("id") for r in rows] == [c["id"] for c in canonical["cases"]],
            "external native input coverage/order mismatch")
    import performance_reference as performance
    for case, row in zip(canonical["cases"], rows):
        require(row.get("workload") == case["workload"] and type(row.get("point")) is int and row["point"] == case["point"]
                and type(row.get("sample_count")) is int and row["sample_count"] == 21,
                "external native case/sample identity mismatch")
        values = [row.get(k + "_user_seconds") for k in ("min", "p10", "median", "p90", "max")]
        require(all(type(v) in (int, float) and math.isfinite(v) and v > 0 for v in values)
                and values == sorted(values), "invalid external native timing distribution")
        require(row.get("quality") == performance.quality(dict(row, nonpositive_samples=0)),
                "external quality differs from unchanged policy")
        batches = row.get("batch_median_user_seconds")
        require(isinstance(batches, list) and len(batches) == 3
                and all(type(v) in (int, float) and math.isfinite(v) and v > 0 for v in batches),
                "invalid external chronological batch statistics")
        output = outputs[case["id"]]
        require(type(row.get("output_bytes")) is int and row["output_bytes"] == output["output_bytes"]
                and row.get("output_sha256") == output["output_sha256"],
                "external complete-output identity mismatch")
    return summary


def bindings(root, summary):
    result = {}
    for source, expected in summary["binaries"].items():
        path = "tests/grace/" + source.removeprefix("tests/")
        require(contract.digest(Path(root) / path) == expected, "measured Grace ELF hash mismatch: " + source)
        result[source] = dict(file=path, sha256=expected,
                              mapped_image_bytes=contract.mapped_image_bytes(Path(root) / path))
    return result


def collection_outputs(root, summary):
    """Use only complete-output identities from the exact measured revision."""
    import performance_reference as performance
    raw = subprocess.check_output(["git", "-C", str(root), "show", summary["source_commit"] + ":" + performance.PATH])
    identity = dict(commit=summary["source_commit"], path=performance.PATH, sha256=hashlib.sha256(raw).hexdigest())
    return {r["id"]: r for r in json.loads(raw)["cases"]}, identity


def derive(root, *, git_root=None):
    """Deterministically derive the plotting view without changing native data."""
    import performance_reference as performance
    root = Path(root)
    git_root = root if git_root is None else Path(git_root)
    canonical = performance.inventory(root)
    summary = compute.read(root / SUMMARY)
    # Historical M2 timings are never read as Grace observations. Binding the
    # output oracle to collection_commit also removes any dependency on future
    # changes to the active M2 measurement file.
    outputs, oracle = collection_outputs(git_root, summary)
    validate_summary(summary, canonical, outputs)
    binary_bindings = bindings(root, summary)
    declared = compute.expanded_manifest(canonical["original"], canonical["compute"], binary_bindings)
    inputs = performance.input_sources(canonical, contract.digest(root / contract.PERFORMANCE_MANIFEST),
                                      contract.digest(root / "tests/benchmarks/provenance.json"))
    verify_original_inputs(git_root, summary["source_commit"], inputs)
    original = dict(inputs)
    inputs.update({b["file"]: b["sha256"] for b in binary_bindings.values()})
    inputs.update({SUMMARY: contract.digest(root / SUMMARY), SCRIPT: contract.digest(root / SCRIPT)})
    rows = []
    for case, row in zip(declared["cases"], summary["cases"]):
        numerical = "equations" if case["workload"] in performance.NUMERICAL else "shape_finite" if "outputs" in case else "analytic"
        rows.append(dict(row, input_sha256=compute.object_digest(case), nonpositive_samples=0, numerical_check=numerical))
    result = dict(schema=1, protocol=PROTOCOL, inventory_sha256=compute.object_digest(declared),
                  collection_commit=summary["source_commit"], source_inventory_sha256=compute.object_digest(canonical),
                  measurement_id="grace-native-" + summary["capture_sha256"],
                  metric="child_user_cpu_seconds", timer="wait4.ru_utime", quality_policy=compute.QUALITY_POLICY,
                  hardware=HARDWARE, environment="native", executable_format="ELF", machine="aarch64", page_size=65536,
                  platform=summary["environment"]["platform"], native_environment=summary["environment"],
                  core_identity="unknown", instantaneous_frequency="unknown", startup_included=True,
                  reference_scope=SCOPE, collector_origin="external-contributor-summary",
                  capture_evidence=dict(capture_sha256=summary["capture_sha256"], reported_samples=5544, reported_warmups=264),
                  source_summary=dict(path=SUMMARY, sha256=inputs[SUMMARY]), binary_bindings=binary_bindings,
                  output_oracle=oracle,
                  quality_counts=dict(Counter(row["quality"] for row in rows)),
                  original_input_sources=original, input_sources=inputs, collector_sources={},
                  micro_manifest_sha256=inputs[contract.PERFORMANCE_MANIFEST],
                  build_manifest_sha256=inputs["tests/benchmarks/provenance.json"],
                  started_at_utc=summary["started_at_utc"], completed_at_utc=summary["completed_at_utc"], cases=rows)
    performance.validate(result, declared, HARDWARE)
    return result


def derive_at(git_root, commit):
    """Publish a historical report from its files, independent of the checkout."""
    archive = subprocess.check_output(["git", "-C", str(git_root), "archive", commit])
    with tempfile.TemporaryDirectory(prefix="poros-linux-reference-") as folder:
        with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
            tar.extractall(folder, filter="data")
        return derive(folder, git_root=git_root)


def validate_metadata(reference, declared, hardware):
    """Offline validation retains the boundary between a summary and raw proof."""
    canonical = compute.expanded_manifest(declared["original"], declared["compute"])
    require(reference.get("hardware") == hardware == HARDWARE,
            "native Linux reference hardware mismatch")
    require(reference.get("binary_bindings") == declared.get("binary_bindings")
            and reference.get("source_inventory_sha256") == compute.object_digest(canonical),
            "native Linux reference binary/source inventory mismatch")
    require(reference.get("environment") == "native" and reference.get("executable_format") == "ELF"
            and reference.get("machine") == "aarch64" and reference.get("page_size") == 65536
            and reference.get("platform", "").startswith("Linux-")
            and reference.get("startup_included") is True and reference.get("reference_scope") == SCOPE
            and reference.get("collector_origin") == "external-contributor-summary"
            and reference.get("collector_sources") == {}, "invalid native Linux scope/protocol")
    native = reference.get("native_environment", {})
    validate_environment(native)
    require(native["platform"] == reference["platform"], "native Linux platform identity mismatch")
    proof = reference.get("capture_evidence", {})
    require(set(proof) == {"capture_sha256", "reported_samples", "reported_warmups"}
            and contract.SHA256.fullmatch(proof.get("capture_sha256", "")) is not None
            and proof["reported_samples"] == 5544 and proof["reported_warmups"] == 264,
            "invalid external native capture identity")
    require(reference.get("source_summary") == dict(path=SUMMARY, sha256=reference.get("input_sources", {}).get(SUMMARY))
            and contract.SHA256.fullmatch(reference["source_summary"].get("sha256", "")) is not None,
            "missing retained external summary identity")
    oracle = reference.get("output_oracle", {})
    require(set(oracle) == {"commit", "path", "sha256"} and oracle["commit"] == reference["collection_commit"]
            and oracle["path"] == "evaluation/usertime/performance-v1.json"
            and contract.SHA256.fullmatch(oracle["sha256"]) is not None,
            "invalid collection output-oracle identity")
    require(reference.get("original_input_sources") == {n: s for n, s in reference["input_sources"].items()
            if n not in {SUMMARY, SCRIPT} and n not in {b["file"] for b in reference["binary_bindings"].values()}},
            "native Linux original source binding mismatch")
    for row in reference["cases"]:
        batches = row.get("batch_median_user_seconds")
        require(isinstance(batches, list) and len(batches) == 3
                and all(type(v) in (int, float) and math.isfinite(v) and v > 0 for v in batches),
                "invalid native Linux chronological batches")
    require(reference.get("quality_counts") == dict(Counter(r["quality"] for r in reference["cases"])),
            "native Linux quality coverage mismatch")


def build(root):
    """Rebuild the documented layout from the exact external source revision."""
    root = Path(root).resolve()
    summary = compute.read(root / SUMMARY)
    import performance_reference as performance
    validate_summary(summary, performance.inventory(root), collection_outputs(root, summary)[0])
    with tempfile.TemporaryDirectory(prefix="poros-grace-elf-") as folder:
        tree = Path(folder)
        archive = subprocess.check_output(["git", "-C", str(root), "archive", summary["source_commit"]])
        with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
            tar.extractall(tree, filter="data")
        layout = tree / "tests/usertime/layout.ld"
        text = layout.read_text()
        require(text.count("ALIGN(4096)") == 2, "measured source linker layout changed")
        layout.write_text(text.replace("ALIGN(4096)", "ALIGN(65536)"))
        for script in ("tests/benchmarks/build.sh", "tests/usertime/build.sh"):
            path = tree / script
            path.write_text(path.read_text().replace("-Wl,--build-id=none", "-Wl,--build-id=none -Wl,-z,max-page-size=65536"))
            subprocess.run(["sh", script], cwd=tree, check=True)
        for source, expected in summary["binaries"].items():
            require(contract.digest(tree / source) == expected, "recompiled native ELF differs from measurement: " + source)
            destination = root / "tests/grace" / source.removeprefix("tests/")
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(tree / source, destination)
    result = derive(root)
    compute.write(root / REFERENCE, result)
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--build", action="store_true", help="recompile and verify all measured ELF hashes before importing")
    args = parser.parse_args()
    result = build(args.root) if args.build else derive(args.root)
    if not args.build:
        require(result == compute.read(args.root / REFERENCE), "derived native Linux reference changed")
    print(f"Grace native reference: {len(result['cases'])} points; {len(result['binary_bindings'])} exact ELF identities")
