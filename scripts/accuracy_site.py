#!/usr/bin/env python3
"""Build auditable, static accuracy reports and retain a bounded Pages archive.

No measurements or errors are recomputed here. The report JSON is authoritative.
Only calibration/ is owned by the archive; existing unrelated Pages files survive.
"""

import argparse
import hashlib
import html
import json
import math
import os
from pathlib import Path
import re
import shutil
import statistics
import subprocess
from workload_contract import LEGACY_IDS, case_ids, validate_manifest, manifest_path
import compute_usertime as compute
import performance_reference as performance
import frontend_microbenchmark as probe
import throughput_microbenchmark as throughput
import forwarding_microbenchmark as forwarding
import scheduling_microbenchmark as scheduling
import capacity_microbenchmark as capacity
import recurrence_microbenchmark as recurrence
from report_records import covered_cases
import memory_microbenchmark as memory
import model_profiles
import devices
from usertime import error_percent
from performance_compare import (MICRO_WORKLOADS, OUTPUT_ORACLE_SCOPE, application_reference, benchmark_summary,
                                 case_base_status, is_micro_case)

SCHEMA = 1
FORMULA = "absolute-difference-over-min-v2"
SHA = re.compile(r"[0-9a-f]{40}")
NAME = re.compile(r"[A-Za-z0-9_.-]+")
RUN_URL = re.compile(
    r"https://github.com/sarchlab/poros/actions/runs/[0-9]+(?:/attempts/[0-9]+)?"
)
EVALUATION_FIELDS = (
    "evaluation_run_id",
    "evaluation_run_attempt",
    "evaluation_conclusion",
    "evaluation_run_url",
)
TERMINAL = {
    "success",
    "failure",
    "cancelled",
    "timed_out",
    "action_required",
    "startup_failure",
    "skipped",
    "stale",
    "neutral",
}
REPO = "sarchlab/poros"
FRONTEND_REFERENCE = "evaluation/microbenchmarks/m2-frontend.json"
# Compatibility inventory is confined to schema-1 reports.
REQUIRED = list(LEGACY_IDS)


def report_manifest(report):
    if report.get("schema") in (3, 4):
        return compute.validate_expanded(report["workload_manifest"])
    if report.get("schema") == 2:
        if report["workload_manifest"].get("schema") != 2:
            raise ValueError("schema-2 report requires the 24-case schema-2 manifest")
        return validate_manifest(report["workload_manifest"])
    return {
        "schema": 1,
        "cases": [
            dict(id=i, split="calibration" if i.startswith("integer-") else "held_out")
            for i in LEGACY_IDS
        ],
    }


def read(path):
    def reject(value):
        raise ValueError("nonfinite JSON: " + value)

    return json.loads(Path(path).read_text(), parse_constant=reject)


def write(path, value):
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    Path(path).write_text(json.dumps(value, indent=2, allow_nan=False) + "\n")


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def git(source, *args):
    return subprocess.check_output(["git", "-C", str(source), *args], text=True).strip()


def sha(value):
    if not isinstance(value, str) or not SHA.fullmatch(value):
        raise ValueError("expected full commit SHA")
    return value


def regular(path):
    path = Path(path)
    if path.is_symlink() or not path.is_file():
        raise ValueError("expected regular file: " + str(path))
    return path


def safe_name(value):
    if not isinstance(value, str) or not NAME.fullmatch(value) or value in (".", ".."):
        raise ValueError("unsafe filename")
    return value


def check_report(report, commit, *, partial=False):
    if report.get("schema") not in (1, 2, 3, 4) or report.get("artifact_status") not in (("pass", "fail", "partial") if partial else ("pass", "fail")):
        raise ValueError("unsupported report")
    if partial and (report.get("schema") != 4 or report.get("artifact_status") != "partial"):
        raise ValueError("partial validation requires a schema-4 partial report")
    if report.get("accuracy_status") != "unverified":
        raise ValueError("unsupported accuracy verdict")
    if "reporting" in report:
        rendering = report["reporting"]
        sha(rendering["commit"])
        if type(rendering.get("dirty")) is not bool or not isinstance(rendering.get("files"), dict) or not rendering["files"]:
            raise ValueError("invalid reporting implementation identity")
        for name, value in rendering["files"].items():
            if not name.startswith("scripts/") or not re.fullmatch(r"[0-9a-f]{64}", value):
                raise ValueError("invalid reporting source digest")
    recorded_commit = report.get("head", {}).get("commit", report.get("head_commit"))
    expected_commit = sha(commit)
    if recorded_commit is None and report["artifact_status"] == "fail":
        # Preserve the producer's original early diagnostic. The bundle, not this
        # unmeasured failure, binds the requested target SHA.
        return
    if recorded_commit != expected_commit:
        raise ValueError("report commit mismatch")
    if report["artifact_status"] == "fail":
        return
    if report.get("formula") != FORMULA or report["head"].get("dirty") is not False:
        raise ValueError("invalid report formula/source state")
    if report["schema"] == 3:
        original = report["original_report"]
        check_report(original, commit)
        if original["artifact_status"] != "pass" or report["cases"][:24] != original["cases"] or report["head"] != original["head"]:
            raise ValueError("original24 evidence changed during composition")
        for key in ("base_commit", "base_binary_sha256", "head_binary_sha256", "base_status", "formula", "reference"):
            if report.get(key) != original.get(key):
                raise ValueError("composite metadata differs from original cohort: " + key)
        if not re.fullmatch(r"[0-9a-f]{64}", report.get("head_binary_sha256", "")):
            raise ValueError("missing simulator binary identity")
        refs = report["reference_files"]
        if set(refs) != {"original24", "compute240"} or refs["original24"] != original["reference_sha256"] or not all(re.fullmatch(r"[0-9a-f]{64}", value) for value in refs.values()):
            raise ValueError("invalid composite reference identity")
        if report["reference_sha256"] != compute.reference_identity(refs["original24"], refs["compute240"]):
            raise ValueError("composite reference mismatch")
        if report["summary"] != compute.summaries(report["cases"], report["workload_manifest"]["compute"]["suite"] == compute.PROTOCOL):
            raise ValueError("expanded summary mismatch")
        if report["compute_base_status"] not in ("available", "compute_elf_unavailable", "per_case_resource_limit"):
            raise ValueError("invalid compute baseline availability")
        if report["compute_base_status"] == "compute_elf_unavailable" and report["base_commit"] != "cb965b4d41fc7d52281652c3bce7803cc893cae0":
            raise ValueError("unrecognized unavailable compute base")
        for row in report["cases"][24:]:
            if report["compute_base_status"] == "per_case_resource_limit":
                continue
            if ("base" in row) != (report["compute_base_status"] == "available"):
                raise ValueError("missing required compute baseline")
    if report["schema"] == 4:
        for field in ("head_binary_sha256", "base_binary_sha256"):
            if not re.fullmatch(r"[0-9a-f]{64}", report.get(field, "")):
                raise ValueError("invalid simulator binary identity")
        devices.validate(report)
    elif "device" in report:
        raise ValueError("device selection requires a schema-4 report")
    referenced = devices.measured(report, "application")
    inventory = report_manifest(report)
    selected = covered_cases(report["cases"], inventory["cases"], partial=partial)
    performance = report.get("schema") == 4 or inventory.get("suite") == "whole-program-performance-six-point-v1" or (report.get("schema") == 3 and inventory["compute"]["suite"] == compute.PROTOCOL)
    if performance:
        capability = compute.validate_base_capability(report["base_capability"])
        if report.get("schema") == 3 and report["original_report"]["base_capability"] != capability:
            raise ValueError("base capability differs between cohorts")
        for row, case in zip(report["cases"], selected):
            expected_status = case_base_status(report, case)
            if row.get("mapped_image_bytes") != case["mapped_image_bytes"] or row.get("base_status") != expected_status or ("base" in row) != (expected_status == "available"):
                raise ValueError("missing or unjustified per-case base result")
            if "base" not in row and "error_change_percentage_points" in row:
                raise ValueError("unpaired error change")
    for row, declared in zip(report["cases"], selected):
        if not referenced:
            # No native reference for this device: a prediction only. Any
            # reference time, origin or error here would be another device's.
            if ("head" not in row or row.get("reference_quality") != devices.UNMEASURED
                    or any(key.startswith("reference_") and key != "reference_quality" for key in row)
                    or "error_change_percentage_points" in row
                    or any(key in row.get(label, {}) for label in ("head", "base") for key in ("error_percent", "direction"))):
                raise ValueError("unmeasured device row carries reference or accuracy values")
        elif (
            "head" not in row
            or not isinstance(row.get("reference_user_seconds"), (int, float))
            or not math.isfinite(row["reference_user_seconds"])
            or row["reference_user_seconds"] < 0
            or (
                row["reference_user_seconds"] == 0
                and row["reference_quality"] != "invalid_nonpositive"
            )
        ):
            raise ValueError("missing head or invalid reference time")
        if report["schema"] in (2, 3, 4) and row.get("functional_status") != "pass":
            raise ValueError("functional correctness evidence missing")
        expected_split = declared["split"]
        if row.get("split") != expected_split:
            raise ValueError("invalid case split")
        if referenced and row["reference_quality"] not in (
            "usable",
            "noisy",
            "below_precision_floor",
            "invalid_nonpositive",
        ):
            raise ValueError("unknown reference quality")
        for key in ("head", "base"):
            if key not in row:
                continue
            value = row[key]
            if not all(
                isinstance(value[k], (int, float))
                and math.isfinite(value[k])
                and value[k] > 0
                for k in ("predicted_user_seconds", "cycles", "instructions")
            ):
                raise ValueError("invalid timing values")
            if "error_percent" in value and (
                row["reference_quality"] != "usable"
                or not math.isfinite(value["error_percent"])
                or value["error_percent"] < 0
            ):
                raise ValueError("invalid error value")

    for split in (compute.SPLITS if report.get("schema") in (3, 4) else ("calibration", "held_out")):
        rows = [r for r in report["cases"] if r["split"] == split]
        summary = report.get("summary", {}).get(split, {})
        if summary.get("required") != len(rows) or summary.get("comparable") != sum(
            "error_percent" in r["head"] for r in rows
        ):
            raise ValueError("summary coverage mismatch")
        if performance and (summary.get("head_comparable") != sum("error_percent" in r["head"] for r in rows) or summary.get("base_comparable") != sum("error_percent" in r.get("base", {}) for r in rows)):
            raise ValueError("head/base summary coverage mismatch")
        for label in ("head", "base"):
            if label in summary:
                if not all(
                    isinstance(summary[label].get(k), (int, float))
                    and math.isfinite(summary[label][k])
                    and summary[label][k] >= 0
                    for k in ("mean_error_percent", "maximum_error_percent")
                ):
                    raise ValueError("invalid summary values")


def bundle(report_path, source, output, commit):
    """Snapshot exactly the evidence used by one report into an artifact directory."""
    commit = sha(commit)
    source, output = Path(source), Path(output)
    report = read(regular(report_path))
    check_report(report, commit)
    if report.get("schema") == 4:
        return bundle_compact(report_path, source, output, commit)
    if report.get("schema") == 3:
        return bundle_expanded(report_path, source, output, commit)
    output.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(report_path, output / "report.json")
    markdown = Path(report_path).with_suffix(".md")
    shutil.copyfile(regular(markdown), output / "report.md")
    files = ["report.json", "report.md"]
    if report["artifact_status"] == "pass":
        manifest = source / (manifest_path(report["workload_manifest"]) if report["schema"] == 2 else "tests/usertime/manifest.json")
        inputs = read(manifest)
        reference = (
            source
            / "evaluation/usertime"
            / safe_name(inputs.get("reference_file", "m2-linuxvm-user-v1.json"))
        )
        if digest(reference) != report["reference_sha256"]:
            raise ValueError("reference does not match report")
        if digest(manifest) != report["reference"]["manifest_sha256"]:
            raise ValueError("manifest does not match report")
        evidence = {
            "reference.json": reference,
            "manifest.json": manifest,
            "native_usertime.py": source / "scripts/native_usertime.py",
        }
        if inputs["schema"] == 2:
            evidence["workload_contract.py"] = source / "scripts/workload_contract.py"
        for name in set(inputs["sources"]) | {c["file"] for c in inputs["cases"]}:
            evidence["inputs/" + safe_name(name)] = source / "tests/usertime" / name
        for name, path in evidence.items():
            regular(path)
            relative = str(path.relative_to(source))
            if report["head"]["files"].get(relative) != digest(path):
                raise ValueError("source evidence mismatch: " + relative)
            target = output / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(path, target)
            files.append(name)
    write(
        output / "bundle.json",
        {
            "schema": SCHEMA,
            "commit": commit,
            "run_id": os.environ.get("GITHUB_RUN_ID"),
            "run_attempt": os.environ.get("GITHUB_RUN_ATTEMPT"),
            "files": {p: digest(output / p) for p in sorted(files)},
        },
    )
    validate_bundle(output, commit)
    (output / "index.html").write_text(
        detail(
            report,
            {
                "commit": commit,
                "conclusion": "preview",
                "run_url": "",
                "releases": [],
                "evidence_files": files,
                "measurement_completed_at": (
                    read(output / "reference.json").get("completed_at_utc")
                    if report["artifact_status"] == "pass"
                    else None
                ),
            },
            preview=True,
        )
    )


def validate_bundle(path, commit):
    path = Path(path)
    if path.is_symlink():
        raise ValueError("symlink bundle")
    manifest = read(regular(path / "bundle.json"))
    if manifest.get("schema") != SCHEMA or manifest.get("commit") != sha(commit):
        raise ValueError("bundle identity mismatch")
    if read(regular(path / "report.json")).get("schema") == 4:
        return validate_compact_bundle(path, commit, manifest)
    if read(regular(path / "report.json")).get("schema") == 3:
        return validate_expanded_bundle(path, commit, manifest)
    files = manifest["files"]
    allowed = {
        "report.json",
        "report.md",
        "reference.json",
        "manifest.json",
        "native_usertime.py",
        "workload_contract.py",
    }
    for name, expected in files.items():
        if name not in allowed and not (
            name.startswith("inputs/") and safe_name(name[7:])
        ):
            raise ValueError("unexpected bundle path")
        if (path / name).parent.is_symlink() or digest(
            regular(path / name)
        ) != expected:
            raise ValueError("bundle digest mismatch")
    report = read(regular(path / "report.json"))
    if not {"report.json", "report.md"}.issubset(files):
        raise ValueError("missing report digest")
    check_report(report, commit)
    if report["artifact_status"] == "pass":
        if (
            digest(path / "reference.json") != report["reference_sha256"]
            or digest(path / "manifest.json") != report["reference"]["manifest_sha256"]
        ):
            raise ValueError("report evidence mismatch")
        reference = read(path / "reference.json")
        if reference["harness_sha256"] != digest(path / "native_usertime.py"):
            raise ValueError("reference harness mismatch")
        inputs = read(path / "manifest.json")
        validate_manifest(inputs)
        if inputs.get("suite") == "whole-program-performance-six-point-v1":
            compute.native.validate_capture_metadata(reference)
        if case_ids(inputs) != case_ids(reference) or case_ids(inputs) != case_ids(
            report_manifest(report)
        ):
            raise ValueError("evidence inventory mismatch")
        if report["schema"] == 2:
            if inputs != report["workload_manifest"]:
                raise ValueError("report manifest differs from retained inventory")
            if reference.get("support_sha256") != {
                "workload_contract.py": digest(path / "workload_contract.py")
            }:
                raise ValueError("reference support module mismatch")
        for row, measured in zip(report["cases"], reference["cases"]):
            if (
                row["reference_quality"] != measured["quality"]
                or row["reference_user_seconds"] != measured["median_user_seconds"]
            ):
                raise ValueError(
                    "report reference values differ from retained measurements"
                )
            for label in ("head", "base"):
                if label in row and any(
                    row[label][k] != inputs["profile"][k]
                    for k in ("configuration", "cache_config")
                ):
                    raise ValueError("report hardware profile differs from manifest")
        required = allowed - {"report.md", "workload_contract.py"}
        if report["schema"] == 2:
            required.add("workload_contract.py")
        required |= {"inputs/" + safe_name(n) for n in inputs["sources"]}
        required |= {"inputs/" + safe_name(c["file"]) for c in inputs["cases"]}
        if not required.issubset(files):
            raise ValueError("incomplete evidence bundle")
        for name, value in inputs["sources"].items():
            if digest(path / "inputs" / safe_name(name)) != value:
                raise ValueError("input source mismatch")
        for case in inputs["cases"]:
            if digest(path / "inputs" / safe_name(case["file"])) != case["sha256"]:
                raise ValueError("input ELF mismatch")
    return report, manifest


def measurement_completed(report, path):
    if report.get("schema") == 3:
        return report["compute_reference"]["completed_at_utc"]
    if not devices.measured(report, "application"):
        return None  # Nothing was measured natively for this device.
    reference = read(path / "reference.json")
    return reference.get("streamcluster_replacement", {}).get("completed_at_utc", reference.get("completed_at_utc"))


def nested_regular(root, name):
    path = root / compute.relative(name)
    for parent in [path, *path.parents]:
        if parent == root.parent:
            break
        if parent.is_symlink():
            raise ValueError("symlink in evidence path")
    return regular(path)


def check_compute_source_identity(report, root, names):
    for name in names:
        if name.startswith("cohorts/compute/source/"):
            original = name[len("cohorts/compute/source/"):]
        elif name.startswith("cohorts/compute/reference/"):
            reference_dir = compute.REFERENCE if report["workload_manifest"]["compute"]["suite"] == compute.PROTOCOL else "evaluation/usertime/compute-linuxvm-v1"
            original = reference_dir + "/" + name[len("cohorts/compute/reference/"): ]
        else:
            continue
        if report["head"]["files"].get(original) != digest(nested_regular(root, name)):
            raise ValueError("compute retained file does not match tested commit: " + original)


def copy_compute_evidence(source, destination, protocol):
    """Choose historical evidence only from the report's explicit protocol."""
    roots = {compute.LEGACY_PROTOCOL: "evaluation/usertime/compute-linuxvm-v1", compute.PROTOCOL: compute.REFERENCE}
    if protocol not in roots:
        raise ValueError("unsupported compute evidence protocol")
    folder = source / roots[protocol]
    measured = compute.validate_reference(folder, source)
    if measured["protocol"] != protocol:
        raise ValueError("report/evidence compute protocol mismatch")
    paths = {"source/" + name: source / compute.relative(name) for name in compute.source_paths(source, protocol)}
    for name in {"reference.json", "oracle.json", "validation-report.json", "validation-harness.py"} | {compute.output_name(row, protocol) for row in measured["cases"]}:
        paths["reference/" + name] = folder / compute.relative(name)
    for name, path in paths.items():
        target = destination / compute.relative(name)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(regular(path), target)
    if set(paths) != compute.evidence_paths(destination):
        raise ValueError("incomplete explicitly selected compute evidence")
    return paths


def bundle_expanded(report_path, source, output, commit):
    report = read(report_path)
    output.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(report_path, output / "report.json")
    shutil.copyfile(Path(report_path).with_suffix(".md"), output / "report.md")
    if report["artifact_status"] != "pass":
        names = ["report.json", "report.md"]
        write(output / "bundle.json", dict(schema=SCHEMA, commit=commit, files={name: digest(output / name) for name in names}))
        validate_bundle(output, commit)
        (output / "index.html").write_text(detail(report, dict(commit=commit, conclusion="failure", run_url="", releases=[], evidence_files=names), preview=True))
        return
    # Reuse the exact schema2 evidence validator/packer for the immutable cohort.
    import tempfile
    with tempfile.TemporaryDirectory() as temporary:
        old = Path(temporary) / "report.json"
        write(old, report["original_report"])
        old.with_suffix(".md").write_text("# Original24 cohort\n\nSee report.json for the unchanged original comparison.\n")
        bundle(old, source, output / "cohorts/original", commit)
    old_manifest = read(output / "cohorts/original/bundle.json")
    names = {"report.json", "report.md", "cohorts/original/bundle.json"}
    names |= {"cohorts/original/" + name for name in old_manifest["files"]}
    names |= {"cohorts/compute/" + name for name in copy_compute_evidence(source, output / "cohorts/compute", report["workload_manifest"]["compute"]["suite"])}
    check_compute_source_identity(report, output, names)
    write(output / "bundle.json", dict(schema=SCHEMA, commit=commit, run_id=os.environ.get("GITHUB_RUN_ID"),
                                        run_attempt=os.environ.get("GITHUB_RUN_ATTEMPT"), files={name: digest(nested_regular(output, name)) for name in sorted(names)}))
    validate_bundle(output, commit)
    (output / "index.html").write_text(detail(report, dict(commit=commit, conclusion="preview", run_url="", releases=[], evidence_files=sorted(names), measurement_completed_at=measurement_completed(report, output)), preview=True))


def validate_expanded_bundle(path, commit, manifest):
    files = manifest["files"]
    for name, expected in files.items():
        if name not in ("report.json", "report.md") and not name.startswith(("cohorts/original/", "cohorts/compute/source/", "cohorts/compute/reference/")):
            raise ValueError("unexpected expanded bundle path")
        if digest(nested_regular(path, name)) != expected:
            raise ValueError("expanded bundle hash mismatch")
    report = read(nested_regular(path, "report.json"))
    check_report(report, commit)
    if report["artifact_status"] != "pass":
        if set(files) != {"report.json", "report.md"}:
            raise ValueError("failure bundle contains unvalidated evidence")
        return report, manifest
    old, old_manifest = validate_bundle(path / "cohorts/original", commit)
    if old != report["original_report"]:
        raise ValueError("retained original report mismatch")
    folder = path / "cohorts/compute"
    reference = compute.validate_reference(folder / "reference", folder / "source")
    if reference["contract"] != report["workload_manifest"]["compute"] or digest(folder / "reference/reference.json") != report["reference_files"]["compute240"]:
        raise ValueError("retained compute reference/contract mismatch")
    displayed = {key: reference[key] for key in ("measurement_id", "environment", "hardware", "platform", "core_identity", "instantaneous_frequency", "completed_at_utc")}
    if report["compute_reference"] != displayed:
        raise ValueError("displayed compute provenance differs from retained reference")
    expected = {"report.json", "report.md", "cohorts/original/bundle.json"} | {"cohorts/original/" + name for name in old_manifest["files"]} | {"cohorts/compute/" + name for name in compute.evidence_paths(folder)}
    if set(files) != expected:
        raise ValueError("incomplete expanded bundle evidence")
    check_compute_source_identity(report, path, files)
    for row, measured, declared in zip(report["cases"][24:], reference["cases"], reference["contract"]["cases"]):
        if row["reference_quality"] != measured["quality"] or row["reference_user_seconds"] != measured["median_user_seconds"]:
            raise ValueError("compute report/reference mismatch")
        for label in ("head", "base"):
            if label not in row:
                continue
            for key in ("configuration", "cache_config"):
                if row[label][key] != reference["contract"]["profile"][key]:
                    raise ValueError("compute report/profile mismatch")
            seconds = row[label]["predicted_user_seconds"]
            if type(row[label]["cycles"]) is not int or type(row[label]["instructions"]) is not int or not math.isclose(seconds, row[label]["cycles"] / declared["frequency_hz"], rel_tol=1e-12):
                raise ValueError("compute archived timing counts/frequency mismatch")
            if measured["quality"] == "usable":
                seconds, real = row[label]["predicted_user_seconds"], measured["median_user_seconds"]
                expected_error = 100 * abs(seconds-real) / min(seconds, real)
                if not math.isclose(row[label].get("error_percent", -1), expected_error, rel_tol=1e-12, abs_tol=1e-12):
                    raise ValueError("compute error formula mismatch")
                direction = "slower" if seconds > real else "faster" if seconds < real else "equal"
                if row[label].get("direction") != direction:
                    raise ValueError("compute error direction mismatch")
        if "base" in row and measured["quality"] == "usable":
            delta = row["head"]["error_percent"] - row["base"]["error_percent"]
            if not math.isclose(row.get("error_change_percentage_points", float("nan")), delta, rel_tol=1e-12, abs_tol=1e-12):
                raise ValueError("compute error change mismatch")
        elif "error_change_percentage_points" in row:
            raise ValueError("unqualified compute error change")
    return report, manifest


def bundle_compact(report_path, source, output, commit):
    report = read(report_path)
    output.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(report_path, output / "report.json")
    shutil.copyfile(regular(Path(report_path).with_suffix(".md")), output / "report.md")
    names = {"report.json", "report.md"}
    if report["artifact_status"] == "pass":
        for name, path in reference_files(report).items():
            shutil.copyfile(source / path, output / name)
            names.add(name)
    write(output / "bundle.json", dict(schema=SCHEMA, commit=commit,
          run_id=os.environ.get("GITHUB_RUN_ID"), run_attempt=os.environ.get("GITHUB_RUN_ATTEMPT"),
          files={name: digest(output / name) for name in names}))
    validate_bundle(output, commit)
    (output / "index.html").write_text(detail(report, dict(commit=commit, measurement_completed_at=measurement_completed(report, output) if report["artifact_status"] == "pass" else None), preview=True, frontend=read(output / "frontend.json") if "frontend.json" in names else None, throughput_reference=read(output / "throughput.json") if "throughput.json" in names else None, forwarding_reference=read(output / "forwarding.json") if "forwarding.json" in names else None, scheduling_reference=read(output / "scheduling.json") if "scheduling.json" in names else None, memory_reference=read(output / "memory.json") if "memory.json" in names else None, capacity_reference=read(output / "capacity.json") if "capacity.json" in names else None, recurrence_reference=read(output / "recurrence.json") if "recurrence.json" in names else None))


def validate_compact_bundle(path, commit, manifest):
    report = read(regular(path / "report.json"))
    names = {"report.json", "report.md"}
    if report["artifact_status"] == "pass":
        for name, source in reference_files(report).items():
            names.add(name)
            # The application reference is bound to the report's own digest below.
            if name != "reference.json" and digest(regular(path / name)) != report["head"]["files"].get(source):
                raise ValueError(name.removesuffix(".json") + (" diagnostic" if name == "frontend.json" else " reference")
                                 + " differs from tested source")
    if set(manifest["files"]) != names:
        raise ValueError("compact bundle membership mismatch")
    for name, expected in manifest["files"].items():
        if digest(regular(path / name)) != expected:
            raise ValueError("compact bundle hash mismatch")
    check_report(report, commit)
    if report["artifact_status"] != "pass":
        return report, manifest
    references = {name: regular(path / name).read_bytes() for name in names if name not in {"report.json", "report.md"}}
    validate_compact_evidence(report, references)
    return report, manifest



DIAGNOSTIC_SUITES = {"throughput": throughput, "forwarding": forwarding,
                     "scheduling": scheduling, "capacity": capacity, "recurrence": recurrence, "memory": memory}


def declared_diagnostics(report):
    files = report["head"]["files"]
    return {name: suite for name, suite in DIAGNOSTIC_SUITES.items()
            if (suite.ELF in files if name == "throughput" else suite.declared(files))}


def reference_files(report):
    """Bundle name -> source path of each native reference this report's device has.

    Every declared suite runs for every device. Only a device with a native
    reference of a kind retains, shows or compares that reference."""
    application = application_reference(report)
    files = {} if application is None else {"reference.json": application[0]}
    declared = declared_diagnostics(report)
    device = devices.entry(report)
    for name, suite in declared.items():
        if devices.measured(report, name):
            # A suite's reference is read from one committed file. A device
            # that declares another file for it is refused, not handed this one.
            if device is not None and device["references"][name] != suite.REFERENCE:
                raise ValueError(f"device {device['id']} declares its {name} reference at {device['references'][name]}, not {suite.REFERENCE}")
            files[name + ".json"] = suite.REFERENCE
    if "throughput" not in declared and FRONTEND_REFERENCE in report["head"]["files"]:
        files["frontend.json"] = FRONTEND_REFERENCE
    return files


def diagnostic_roles(value, name):
    """Simulators that predict a suite for a plan or report: throughput is
    head-only, and a base revision that cannot model the device predicts nothing."""
    return ("head",) if name == "throughput" or not devices.available(value, "base") else ("head", "base")


def validate_diagnostic_rows(value, name, simulations, cases, *, partial=False, base_large_image_support=None):
    """Coverage and counter checks of one suite's simulated rows, with or without a native reference."""
    suite = DIAGNOSTIC_SUITES[name]
    if name == "throughput":
        return suite.validate_simulation(simulations, cases, partial=partial)
    if devices.available(value, "base"):
        if name == "memory":
            return memory.validate_comparison(simulations, cases, base_large_image_support, partial=partial)
        return suite.validate_comparison(simulations, cases, partial=partial)
    if not isinstance(simulations, dict) or set(simulations) != {"base", "head"} or simulations["base"] != []:
        raise ValueError(name + " has base rows for a device the base revision does not model")
    (memory.validate_rows if name == "memory" else suite.validate_simulation)(simulations["head"], cases, partial=partial)


def definition_path(name):
    return memory.DEFINITION if name == "memory" else DIAGNOSTIC_SUITES[name].DIRECTORY + "cases.def"


def diagnostic_definitions(report, root):
    """Exact case definitions of the suites whose cases no native reference carries."""
    return {name: (Path(root) / definition_path(name)).read_bytes().decode("utf-8")
            for name in declared_diagnostics(report) if not devices.measured(report, name)}


def definition_cases(report, name):
    """A suite's declared cases, from the source-bound definition a report without its native reference retains."""
    text = report.get("diagnostic_definitions", {}).get(name)
    if not isinstance(text, str) or hashlib.sha256(text.encode()).hexdigest() != report["head"]["files"].get(definition_path(name)):
        raise ValueError(name + " case definition differs from tested source")
    return DIAGNOSTIC_SUITES[name].parse_cases(json.loads(text) if name == "memory" else text)


def diagnostic_inventory(report, name, native=None):
    """A suite's declared cases: its native reference's where the device has one, else the retained definition's."""
    return native["cases"] if native is not None else definition_cases(report, name)


def inventoried(report):
    """Whether a report records the source it tested. A failure can precede that."""
    return isinstance(report.get("head"), dict) and isinstance(report["head"].get("files"), dict)


def reference_availability(report, declared):
    """Native reference availability of a declared device, as far as a report establishes it.

    Which suites exist is read from the tested source. A failure that stopped
    before recording that source establishes the application reference alone."""
    if inventoried(report):
        return devices.reference_status(declared, ["application", *declared_diagnostics(report)])
    return ("application " + ("measured" if "application" in declared["references"] else devices.UNMEASURED)
            + "; diagnostic suites not established, the run failed before recording its source")


def device_summary(report):
    """How a report describes its device, or None for a report that predates device selection."""
    declared = devices.entry(report)
    if declared is None:
        return None
    return dict(id=declared["id"], hardware=declared["hardware"],
                parameters=devices.parameter_text(declared, report["device"]["head"]["declaration"]),
                references=reference_availability(report, declared))


def validate_compact_evidence(report, references, *, partial=False):
    """Same source, native, row and profile contract for live and CI evidence."""
    check_report(report, report["head"]["commit"], partial=partial)
    expected = reference_files(report)
    if set(references) != set(expected):
        raise ValueError("report reference membership mismatch")
    for name, source in expected.items():
        if not isinstance(references[name], bytes) or hashlib.sha256(references[name]).hexdigest() != report["head"]["files"].get(source):
            raise ValueError("report reference bytes differ from tested source: " + name)
    # A native reference describes one piece of hardware: the report's device.
    for name in expected:
        if name != "frontend.json":
            devices.require_reference(report, "application" if name == "reference.json" else name.removesuffix(".json"), json.loads(references[name]))
    declared_suites = declared_diagnostics(report)
    if set(report.get("diagnostic_definitions", {})) != {name for name in declared_suites if name + ".json" not in expected}:
        raise ValueError("retained case definitions differ from the suites without a native reference")
    for name, suite in DIAGNOSTIC_SUITES.items():
        simulations = report.get(name + "_simulation")
        capability = {"base_large_image_support": report.get("memory_base_large_image_support")} if name == "memory" else {}
        if name not in declared_suites:
            if simulations is not None or (name == "memory" and "memory_base_large_image_support" in report):
                raise ValueError(name + " suite absent from tested source")
        elif name + ".json" in expected:
            native = suite.validate_reference(json.loads(references[name + ".json"]), report["head"]["files"])
            validate_diagnostic_rows(report, name, simulations, native["cases"], partial=partial, **capability)
        else:
            validate_diagnostic_rows(report, name, simulations, definition_cases(report, name), partial=partial, **capability)
    if throughput.ELF in report["head"]["files"]:
        if "frontend_simulation" in report:
            raise ValueError("superseded frontend simulation in throughput report")
    elif probe.ELF in report["head"]["files"]:
        probe.validate(report.get("frontend_simulation"))
    elif "frontend_simulation" in report:
        raise ValueError("frontend probe is absent from the tested source")
    declared = report_manifest(report)
    if "reference.json" in expected:
        path, hardware = application_reference(report)
        measured = performance.validate(json.loads(references["reference.json"]), declared, hardware)
        if hashlib.sha256(references["reference.json"]).hexdigest() != report["reference_sha256"] or report["head"]["files"].get(path) != report["reference_sha256"]:
            raise ValueError("compact reference differs from tested source")
        metadata = {key: value for key, value in measured.items() if key not in ("cases", "input_sources", "collector_sources")}
        if report["reference"] != metadata:
            raise ValueError("displayed compact provenance mismatch")
        if any(report["head"]["files"].get(name) != sha for name, sha in measured["input_sources"].items()):
            raise ValueError("compact input differs from tested source/ELF")
        if "output_oracle" in report:
            raise ValueError("measured device report names an output oracle instead of its reference")
        observations = {row["id"]: row for row in measured["cases"]}
    else:
        # Outputs were checked against the committed program results; that file's
        # native timings belong to another device and are not part of this report.
        oracle = report.get("output_oracle")
        if (oracle != dict(path=performance.PATH, sha256=report["head"]["files"].get(performance.PATH), scope=OUTPUT_ORACLE_SCOPE)
                or "reference" in report or "reference_sha256" in report):
            raise ValueError("unmeasured device report must name only its output oracle")
        measured, observations = {}, {}
    profiles = model_profiles.validate(report)
    for row, case in zip(report["cases"], covered_cases(report["cases"], declared["cases"], partial=partial)):
        observed = observations.get(row["id"])
        if "streamcluster_replacement" in measured and row.get("reference_origin") != performance.reference_origin(measured, case):
            raise ValueError("plotted native acquisition identity mismatch")
        if observed is not None and (row["reference_quality"] != observed["quality"] or row["reference_user_seconds"] != observed["median_user_seconds"] or row.get("reference_p10_seconds") != observed["p10_user_seconds"] or row.get("reference_p90_seconds") != observed["p90_user_seconds"]):
            raise ValueError("compact plotted statistics mismatch")
        for label in ("head", "base"):
            if label not in row:
                continue
            value = row[label]
            if type(value["cycles"]) is not int or type(value["instructions"]) is not int or any(value[k] != profiles[label][k] for k in ("configuration", "cache_config")):
                raise ValueError("compact report timing profile/count mismatch")
            seconds = value["predicted_user_seconds"]
            if not math.isclose(seconds, value["cycles"] / case["frequency_hz"], rel_tol=1e-12):
                raise ValueError("compact timing frequency mismatch")
            if observed is not None and observed["quality"] == "usable":
                real = observed["median_user_seconds"]
                error = 100 * abs(seconds-real) / min(seconds, real)
                if not math.isclose(value.get("error_percent", -1), error, rel_tol=1e-12, abs_tol=1e-12) or value.get("direction") != ("slower" if seconds > real else "faster" if seconds < real else "equal"):
                    raise ValueError("compact error formula/direction mismatch")
            elif "error_percent" in value or "direction" in value:
                raise ValueError("unqualified compact reference has accuracy claim")
        if "base" in row and observed is not None and observed["quality"] == "usable":
            if not math.isclose(row.get("error_change_percentage_points", float("nan")), row["head"]["error_percent"]-row["base"]["error_percent"], rel_tol=1e-12, abs_tol=1e-12):
                raise ValueError("compact paired error change mismatch")
        elif "error_change_percentage_points" in row:
            raise ValueError("unpaired compact error change")
    if report["summary"] != compute.summaries(report["cases"], True):
        raise ValueError("compact summary mismatch")
    validate_diagnostic_progress(report, references, partial=partial)
    return report


def validate_diagnostic_progress(report, references, *, partial=False):
    declared = declared_diagnostics(report)
    progress = report.get("diagnostic_progress", {})
    if not isinstance(progress, dict) or set(progress) - set(declared):
        raise ValueError("unknown diagnostic progress section")
    for name, item in progress.items():
        roles = diagnostic_roles(report, name)
        native = json.loads(references[name + ".json"]) if name + ".json" in references else None
        ids = [case["id"] for case in diagnostic_inventory(report, name, native)]
        simulations = report[name + "_simulation"]
        rows = {"head": simulations} if name == "throughput" else simulations
        done = {role: {row["id"] for row in rows[role]} for role in roles}
        if (not isinstance(item, dict) or set(item) != {"state", "active", "errors"}
                or item["state"] not in ("pending", "running", "partial", "complete", "failed")
                or not isinstance(item["errors"], list) or any(not isinstance(error, str) for error in item["errors"])
                or not isinstance(item["active"], list)):
            raise ValueError("invalid diagnostic progress")
        active = []
        for row in item["active"]:
            if not isinstance(row, dict) or set(row) != {"role", "id"} or row["role"] not in roles or row["id"] not in ids or row["id"] in done[row["role"]]:
                raise ValueError("invalid active diagnostic case")
            active.append((row["role"], row["id"]))
        complete = all(len(done[role]) == len(ids) for role in roles)
        if len(active) != len(set(active)) or (item["state"] == "complete" and (not complete or active or item["errors"])):
            raise ValueError("invalid diagnostic completion")
        if item["state"] == "pending" and (any(done.values()) or active or item["errors"]):
            raise ValueError("pending diagnostic has results")
        if not partial and (not complete or item["state"] != "complete" or active or item["errors"]):
            raise ValueError("complete report has unfinished diagnostics")


def reusable_bundle(path, commit):
    try:
        report, _ = validate_bundle(path, commit)
        return report["artifact_status"] == "pass"
    except (OSError, ValueError, KeyError, TypeError):
        return False


def failure(output, commit, message, device=None, base_commit=None):
    """Record a run that produced no report. A named device keeps the failure
    with that device's reports; unnamed, it is the default device's as before.
    A producer that had already resolved its base revision records it."""
    output = Path(output)
    output.mkdir(parents=True, exist_ok=True)
    write(
        output / "report.json",
        {
            "schema": 1,
            "artifact_status": "fail",
            "accuracy_status": "unverified",
            **({} if base_commit is None else {"base_commit": sha(base_commit)}),
            "head_commit": sha(commit),
            "cases": [],
            "error": message,
            **({} if device is None else {"device": {"id": device_name(device)}}),
        },
    )
    (output / "report.md").write_text(
        "# User-time accuracy report\n\nArtifact: **fail**; accuracy: **unverified**\n\n"
        + message
        + "\n"
    )


def esc(value):
    return html.escape(str(value), quote=True)


def number(value, suffix=""):
    return "—" if value is None else f"{value:.4f}{suffix}"


CSS = """
:root{font-family:Inter,ui-sans-serif,system-ui,sans-serif;color:#172b35;background:#f5f7f7;font-size:15px}body{margin:0}main{max-width:1180px;margin:auto;padding:40px 24px 64px}a{color:#006e75;text-underline-offset:3px}header{border-bottom:1px solid #cedbde;padding-bottom:25px;margin-bottom:24px}.eyebrow{font-size:12px;letter-spacing:.15em;text-transform:uppercase;font-weight:700;color:#49717a}h1{font-size:clamp(28px,4vw,42px);letter-spacing:-.04em;margin:12px 0}h2{font-size:22px;margin:32px 0 14px}p{line-height:1.6}.muted{color:#58707a}.notice{border-left:4px solid #b77a13;background:#fff7df;padding:13px 18px;margin:18px 0}.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(190px,1fr));gap:14px}.card,figure{background:white;border:1px solid #d7e1e4;border-radius:9px;padding:18px;margin:0}.card strong{display:block;font-size:26px;margin-top:6px}.table{overflow:auto;border:1px solid #d7e1e4;border-radius:9px;background:white}table{border-collapse:collapse;width:100%;white-space:nowrap}td,th{padding:12px 14px;text-align:left;border-bottom:1px solid #e5ecee;font-variant-numeric:tabular-nums}th{background:#edf3f3;font-size:12px;text-transform:uppercase;color:#49616b}tr:last-child td{border-bottom:0}.badge{font-size:12px;padding:4px 8px;border-radius:20px;background:#edf1f2}.pass{background:#e1f3e9;color:#15523b}.fail{background:#fbe6df;color:#8f321b}.charts{display:grid;grid-template-columns:repeat(auto-fit,minmax(310px,1fr));gap:16px}svg{width:100%;height:auto}svg text{font:11px system-ui;fill:#526a75}figcaption{font-weight:650;margin-bottom:10px}.band-key{display:flex;flex-wrap:wrap;gap:8px 18px;list-style:none;padding:0;font-size:13px}.band-key li{white-space:nowrap}.band-key span{display:inline-block;width:12px;height:12px;margin-right:5px;border:1px solid #0002;vertical-align:-2px}#point-tip{position:fixed;z-index:10;pointer-events:none;background:#18343e;color:white;border-radius:5px;padding:8px 11px;max-width:min(380px,85vw);font-size:13px;line-height:1.4;white-space:pre-line}circle[data-tip]{cursor:crosshair}circle[data-tip]:focus{outline:none;stroke:#2c5059;stroke-width:2}details{margin:20px 0}pre{overflow:auto;background:#edf3f3;padding:16px;font-size:12px}.downloads{display:flex;flex-wrap:wrap;gap:16px}footer{margin-top:35px;color:#617780;font-size:12px}code{font-size:12px;overflow-wrap:anywhere}@media(max-width:500px){main{padding:24px 12px}.charts{grid-template-columns:1fr}td,th{padding:10px}.card{padding:13px}}
.cdf-percentiles{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px 12px;list-style:none;padding:0;font-size:13px}.cdf-percentiles li{border-left:3px solid;padding-left:8px}.cdf-percentiles strong{white-space:nowrap}
.views{position:sticky;top:0;z-index:5;display:flex;flex-wrap:wrap;gap:6px;background:#f5f7f7;padding:10px 0;margin-bottom:10px;border-bottom:1px solid #cedbde}.views a{padding:6px 12px;border:1px solid #cedbde;border-radius:6px;background:white;text-decoration:none}.views a:focus-visible{outline:2px solid #006e75;outline-offset:2px}.view,.view [id]{scroll-margin-top:72px}
"""


def page(title, content, script=""):
    return (
        '<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>'
        + esc(title)
        + "</title><style>"
        + CSS
        + "</style><main>"
        + content
        + "<footer>Poros · Whole-program user CPU time · Reproducible evidence, unadjusted predictions.</footer></main>"
        + script
        + "</html>\n"
    )


POINT_TIPS = """<div id="point-tip" role="tooltip" hidden></div><script>
(() => {
  const tip = document.getElementById('point-tip');
  function show(event) {
    let point = event.target.closest('[data-tip]');
    const svg = event.target.closest('svg');
    if (svg && event.type === 'pointermove') {
      // Dense plots can overlap. Choose the nearest center, not the last SVG element painted.
      const cursor = new DOMPoint(event.clientX, event.clientY).matrixTransform(svg.getScreenCTM().inverse());
      let best = 7 ** 2;
      point = null;
      for (const dot of svg.querySelectorAll('[data-tip]')) {
        const distance = (cursor.x - dot.cx.baseVal.value)**2 + (cursor.y - dot.cy.baseVal.value)**2;
        if (distance < best) { best = distance; point = dot; }
      }
      if (!point) tip.hidden = true;
    }
    if (!point) { tip.hidden = true; return; }
    tip.textContent = point.dataset.tip;
    tip.hidden = false;
    const box = point.getBoundingClientRect();
    const x = event.clientX ?? box.right, y = event.clientY ?? box.top;
    tip.style.left = Math.max(4, Math.min(x + 10, innerWidth - tip.offsetWidth - 4)) + 'px';
    tip.style.top = Math.max(4, Math.min(y + 12, innerHeight - tip.offsetHeight - 4)) + 'px';
  }
  for (const name of ['pointermove','focusin','click']) document.addEventListener(name, show);
  for (const name of ['pointerout','focusout']) document.addEventListener(name, e => {
    if (e.target.closest('[data-tip]')) tip.hidden = true;
  });
  document.addEventListener('keydown', e => { if (e.key === 'Escape') tip.hidden = true; });
  window.addEventListener('scroll', () => { tip.hidden = true; }, true);
})();
</script>"""


LIVE_REFRESH = """<script>
(() => {
  const root = document.getElementById('accuracy-report');
  const identity = root.dataset.reportIdentity;
  root.style.overflowAnchor = 'none'; // Scroll anchoring is restored explicitly below.
  // Semantic keys survive newly inserted charts and changing table row order.
  function key(node) {
    if (node === root) return 'report';
    if (node.id) return 'id:' + node.id;
    const tag = node.localName;
    if (tag === 'figure') return 'figure:' + node.querySelector('figcaption').textContent;
    if (tag === 'details') return 'details:' + node.querySelector('summary').textContent.split('·')[0].trim();
    if (/^h[123]$/.test(tag)) return tag + ':' + node.textContent;
    const parent = node.parentElement;
    if (!parent || !root.contains(parent)) return null;
    const prefix = key(parent);
    if (tag === 'tr' && node.querySelector('td')) return prefix + '/row:' +
      (node.dataset.rowKey || node.querySelector('td').textContent);
    if (node.dataset.pointKey) return prefix + '/point:' + node.dataset.pointKey;
    if (tag === 'a') return prefix + '/link:' + node.getAttribute('href');
    return prefix + '/' + tag + ':' + [...parent.children].filter(n => n.localName === tag).indexOf(node);
  }
  function apply(next) {
    const oldNodes = [...root.querySelectorAll('*')];
    const states = oldNodes.filter(n => n.localName === 'details' || n.scrollLeft || n.scrollTop)
      .map(n => ({key:key(n), open:n.localName === 'details' ? n.open : null, x:n.scrollLeft, y:n.scrollTop}));
    const focused = root.contains(document.activeElement) ? key(document.activeElement) : null;
    const anchors = [...root.querySelectorAll('figure,figcaption,details,summary,tr,h2,h3,pre')]
      .map(n => ({key:key(n), box:n.getBoundingClientRect()}))
      .filter(a => a.box.height && a.box.bottom > 0 && a.box.top < innerHeight)
      .sort((a,b) => Math.abs(a.box.top)-Math.abs(b.box.top));
    const x = scrollX, y = scrollY;
    document.getElementById('point-tip').hidden = true;
    root.replaceChildren(...next.childNodes);
    root.dataset.liveState = next.dataset.liveState;
    const nodes = new Map([...root.querySelectorAll('*')].map(n => [key(n), n]));
    // Reopen every disclosure before scroll writes force layout. Replacing
    // long open tables with collapsed ones can temporarily clamp window scroll.
    for (const saved of states) {
      const node = nodes.get(saved.key);
      if (node && saved.open !== null) node.open = saved.open;
    }
    for (const saved of states) {
      const node = nodes.get(saved.key);
      if (!node) continue;
      node.scrollLeft = saved.x; node.scrollTop = saved.y;
    }
    const focus = nodes.get(focused);
    if (focus) focus.focus({preventScroll:true});
    const anchor = anchors.find(a => nodes.has(a.key));
    const delta = anchor ? nodes.get(anchor.key).getBoundingClientRect().top - anchor.box.top : 0;
    window.scrollTo(x, anchor ? scrollY + delta : y);
  }
  async function refresh() {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10000);
    try {
      const response = await fetch(location.href, {cache:'no-store', signal:controller.signal});
      if (!response.ok) throw new Error('Report unavailable');
      const doc = new DOMParser().parseFromString(await response.text(), 'text/html');
      const next = doc.getElementById('accuracy-report');
      if (!next || next.dataset.reportIdentity !== identity ||
          !['polling','validated'].includes(next.dataset.liveState) ||
          !next.querySelector('#case-progress') || !next.querySelector('header') ||
          next.lastElementChild?.dataset.reportEnd !== identity) {
        throw new Error('Invalid or different report');
      }
      apply(next);
    } catch (_) {
      // Keep the last good view on transient HTTP, network or snapshot errors.
    } finally {
      clearTimeout(timeout);
      // Schedule only after this request finishes; never overlap requests.
      if (root.dataset.liveState !== 'validated') setTimeout(refresh, 15000);
    }
  }
  setTimeout(refresh, 15000);
})();
</script>"""


def chart(title, series, xlabel, ylabel, scatter=False, x_ticks=None, y_ticks=None,
          ideal=False, note="", y_bands=(), y_ranges=(), x_domain=None, step_start=None,
          percentile_guides=(), empty="No comparable points available."):
    """Plot the data domain; guide lines never contribute observations."""
    points = [(x, y) for _, values, _ in series for x, y, _ in values]
    if not points:
        return f"<figure><figcaption>{esc(title)}</figcaption><p>{esc(empty)}</p></figure>"
    xmin, xmax = x_domain if x_domain is not None else (min(x for x, _ in points), max(x for x, _ in points))
    ymin, ymax = (y_ticks[0][0], y_ticks[-1][0]) if y_ticks else (0, max([y for _, y in points] + [high for _, _, high, _ in y_ranges]) or 1)

    def px(x):
        return 72 + (x - xmin) / (xmax - xmin or 1) * 329

    def py(y):
        return 235 - (y - ymin) / (ymax - ymin or 1) * 215

    body = ""
    for lower, upper, color in y_bands:
        bottom, top = max(lower, ymin), min(upper, ymax)
        if bottom < top:
            body += f'<rect x="72" y="{py(top):.3f}" width="329" height="{py(bottom)-py(top):.3f}" fill="{color}"/>'
    body += '<path d="M72 20V235H401" fill="none" stroke="#bacbd0"/>'
    if y_ticks is None:
        y_ticks = [(ymax*i/4, f"{ymax*i/4:.3g}") for i in range(5)]
    for value, label in y_ticks:
        y = py(value)
        body += f'<path d="M72 {y}H401" stroke="#dae4e6"/><text x="66" y="{y+4}" text-anchor="end">{esc(label)}</text>'
    if x_ticks is None:
        values = [xmin + (xmax-xmin)*i/4 for i in range(5)] if xmin != xmax else [xmin]
        x_ticks = [(x, f"{x:.3g}") for x in values]
    for x, label in x_ticks:
        body += f'<text x="{px(x)}" y="253" text-anchor="middle">{esc(label)}</text>'
    if ideal and xmin <= min(xmax, ymax):
        stop = min(xmax, ymax)
        body += f'<path d="M{px(xmin)} {py(xmin)} L{px(stop)} {py(stop)}" stroke="#899ea7" stroke-dasharray="4 3"/>'
    for x, low, high, color in y_ranges:
        body += (f'<path class="variability-range" d="M{px(x):.3f} {py(low):.3f}V{py(high):.3f}'
                 f'M{px(x)-3:.3f} {py(low):.3f}h6M{px(x)-3:.3f} {py(high):.3f}h6" '
                 f'fill="none" stroke="{color}" opacity="0.65"/>')
    marks = {}
    for name, values, color in series:
        if not scatter:
            # Partial/live observations can arrive out of x order. Sort a copy
            # of complete observations so coordinates and labels stay paired.
            values = sorted(values, key=lambda point: point[0])
            line = [(x, y) for x, y, _ in values]
            if step_start is not None:
                line, previous = [], step_start
                for x, y, _ in values:
                    line.extend(((previous, y), (x, y)))
                    previous = x
            coords = " ".join(f"{px(x):.2f},{py(y):.2f}" for x, y in line)
            body += f'<polyline fill="none" stroke="{color}" points="{coords}"/>'
        for x, y, label in values:
            # Identical base/head observations must both be readable on hover.
            marks.setdefault((x, y), []).append((color, name + ": " + label))
    for (x, y), entries in marks.items():
        tip = esc(chr(10).join(label for _, label in entries))
        keys = []
        for _, label in entries:
            role, _, observation = label.partition(': ')
            observation = re.sub(r'^rank [0-9]+ · ', '', observation)
            case = re.split(r'[:\n]| · ', observation, maxsplit=1)[0]
            ranked = title.startswith('S-curve:') or title == 'Absolute error distribution (CDF)'
            keys.append([role, case] if ranked else [role, case, x])
        point_key = esc(json.dumps(keys))
        body += (f'<circle data-point-key="{point_key}" cx="{px(x):.3f}" cy="{py(y):.3f}" r="3.2" fill="{entries[-1][0]}" '
                 f'stroke="transparent" stroke-width="4" tabindex="0" aria-label="{tip}" data-tip="{tip}"/>')
    for fraction, value, label, color in percentile_guides:
        x = px(fraction)
        # Zero error has no finite log-axis ordinate. Its exact label remains
        # below the chart, but no positive marker is substituted for it.
        body += (f'<g class="percentile-guide" aria-label="{esc(label)}" pointer-events="none">'
                 f'<path d="M{x:.3f} 20V235" fill="none" stroke="{color}" stroke-dasharray="3 4" opacity="0.65"/>')
        if value is not None:
            y = py(value)
            body += (f'<path class="percentile-marker" d="M{x:.3f} {y-4:.3f}l4 4-4 4-4-4Z" '
                     f'fill="white" stroke="{color}" stroke-width="2"/>')
        body += '</g>'
    body += f'<text x="235" y="279" text-anchor="middle">{esc(xlabel)}</text><text x="6" y="12">{esc(ylabel)}</text>'
    legend = " · ".join(f'<span style="color:{color}">{esc(name)}</span>' for name, _, color in series)
    if ideal:
        legend += " · dashed: ideal y = x"
    return (f'<figure><figcaption>{esc(title)}</figcaption><svg viewBox="0 0 440 290" role="group" '
            f'aria-label="{esc(title)}">{body}</svg><p class="muted">{legend}</p>{note}</figure>')


# Upper ratio bound, legend label, and shared fill for both sides of 1×.
ERROR_BANDS = (
    (1.05, "≤5%", "#c4dfce"),
    (1.20, ">5–20%", "#d9e8ef"),
    (1.50, ">20–50%", "#f8e8bc"),
    (2.00, ">50–100%", "#f4cfad"),
    (math.inf, ">100%", "#efbdc5"),
)


def correlation_statistics(points):
    n = len(points)
    pearson = "—"
    concordant = discordant = measured_unequal = predicted_unequal = 0
    if n >= 2:
        mx, my = (sum(p[k] for p in points)/n for k in (0, 1))
        dx, dy = ([p[k]-mean for p in points] for k, mean in ((0,mx),(1,my)))
        denom = math.sqrt(sum(x*x for x in dx) * sum(y*y for y in dy))
        if denom:
            pearson = f"{sum(x*y for x,y in zip(dx,dy))/denom:.4f}"
        for i, (x, y, _) in enumerate(points):
            for xx, yy, _ in points[:i]:
                measured_unequal += x != xx
                predicted_unequal += y != yy
                if x != xx and y != yy:
                    if (x < xx) == (y < yy):
                        concordant += 1
                    else:
                        discordant += 1
    denominator = math.sqrt(measured_unequal * predicted_unequal)
    tau = f"{(concordant - discordant) / denominator:.4f}" if denominator else "—"
    pair = (f"{concordant / measured_unequal:.2%} ({concordant:,}/{measured_unequal:,})"
            if measured_unequal else "—")
    return dict(count=n, pearson=pearson, tau=tau, pair=pair)


def comparison_table(cases, unit="application cases"):
    shared = [c for c in cases if c.get("reference_quality") == "usable"
              and all("error_percent" in c.get(role, {}) for role in ("base", "head"))]
    body = (f'<h3>Base and head comparison</h3><p>Both rows use the same {len(shared)} qualified {unit} '
            f'with base and head predictions. The overview charts use all qualified head {unit.split()[-1]}.</p>'
            '<div class="table"><table><thead><tr><th>Model</th><th>Mean error</th><th>Median error</th>'
            '<th>Pearson r</th><th>Kendall τ-b</th><th>Pairs ordered correctly</th></tr></thead><tbody>')
    for role in ("base", "head"):
        metrics = correlation_statistics([(c["reference_user_seconds"] * 1000, c[role]["predicted_user_seconds"] * 1000, c["id"]) for c in shared])
        errors = [c[role]["error_percent"] for c in shared]
        values = [role, number(statistics.mean(errors), "%") if errors else "—",
                  number(statistics.median(errors), "%") if errors else "—", metrics["pearson"], metrics["tau"], metrics["pair"]]
        body += '<tr>' + ''.join('<td>' + esc(v) + '</td>' for v in values) + '</tr>'
    return body + '</tbody></table></div>'


def absolute_error_cdf(points):
    """Inverse empirical CDF: sorted error at rank i/n, without interpolation."""
    title = "Absolute error distribution (CDF)"
    ranked = sorted((error_percent(pred, ref), name, ref, pred) for ref, pred, name in points)
    n = len(ranked)
    if not n:
        return chart(title, [], "Cumulative fraction of cases", "Absolute error (%) · log scale")
    guides, labels = [], []
    for percentile, color in ((50, '#5a519e'), (75, '#2273a3'), (90, '#ad5700'), (95, '#a83163')):
        error = ranked[(percentile*n + 99)//100 - 1][0]
        value = '0' if error == 0 else f'{error:.2f}' if error >= .005 else f'{error:.3g}'
        label = f'P{percentile}: {value}%'
        labels.append(f'<li style="border-color:{color}">P{percentile}: <strong>{value}%</strong></li>')
        guides.append((percentile/100, math.log10(error) if error > 0 else None, label, color))
    percentiles = ('<ul class="cdf-percentiles" aria-label="Absolute error percentiles">' + ''.join(labels)
                   + '</ul><p class="muted">P90: at least 90% of cases have error at or below this value. '
                   'Nearest-rank percentiles; no interpolation.</p>')
    zeros = sum(error == 0 for error, _, _, _ in ranked)
    note = f'<p class="muted">Exact zero error: {zeros} / {n} ({zeros/n:.2%}).'
    if zeros == n:
        return f'<figure><figcaption>{title}</figcaption>{percentiles}{note} No positive errors to plot on a log axis.</p></figure>'
    note += ' Zero-error points are omitted from the log axis.</p>' if zeros else '</p>'
    note = percentiles + note
    values = []
    for i, (error, name, ref, pred) in enumerate(ranked, 1):
        if error == 0:
            continue
        label = (f'{name} · cumulative fraction {i}/{n} = {i/n:.6g} · absolute error {error:.17g}%'
                 f' · measured {ref:.17g} ms · predicted {pred:.17g} ms')
        values.append((i/n, math.log10(error), label))
    low, high = math.floor(min(y for _, y, _ in values)), math.ceil(max(y for _, y, _ in values))
    if low == high:
        low, high = low - 1, high + 1
    ticks = [(e, f'{10.0**e:g}') for e in range(low, high+1)]
    return chart(title, [("head", values, "#087e8b")], "Cumulative fraction of cases",
                 "Absolute error (%) · log scale", x_domain=(0, 1),
                 x_ticks=[(x/4, f'{x/4:g}') for x in range(5)], y_ticks=ticks,
                 step_start=zeros/n, note=note, percentile_guides=guides)


def qualified(case):
    """The inclusion rule of every accuracy view: a usable native reference and a positive head prediction."""
    return bool(case.get("reference_quality") == "usable" and case.get("head")
                and case["reference_user_seconds"] > 0 and case["head"]["predicted_user_seconds"] > 0)


def overview_charts(cases, heading='<h2 id="overall">Overall benchmark accuracy</h2>'):
    # All overview views and statistics use exactly the same qualified head cases.
    points = [(c["reference_user_seconds"] * 1000, c["head"]["predicted_user_seconds"] * 1000, c["id"])
              for c in cases if not is_micro_case(c) and qualified(c)]
    ranks, scatter = [], []
    band_counts = [0] * len(ERROR_BANDS)
    for i, (ref, pred, name) in enumerate(sorted(points, key=lambda p: (p[1]/p[0], p[2])), 1):
        factor = max(pred/ref, ref/pred)
        band = next(j for j, (upper, _, _) in enumerate(ERROR_BANDS) if factor <= upper)
        band_counts[band] += 1
        direction = "slower" if pred > ref else "faster" if pred < ref else "equal"
        label = f"{name} · measured {ref:.4f} ms · predicted {pred:.4f} ms · {factor:.3f}× {direction}"
        ranks.append((i, math.log2(pred/ref), f"rank {i} · {label}"))
        scatter.append((ref, pred, label))
    exponents = [y for _, y, _ in ranks] + [0]
    low, high = math.floor(min(exponents)), math.ceil(max(exponents))
    if low == high:  # Every prediction is exact.
        low, high = -1, 1
    ticks = [(e, f"{2**abs(e)}× " + ("faster" if e < 0 else "slower") if e else "1× (equal)")
             for e in range(low, high+1)]
    rank_ticks = [(r, str(r)) for r in sorted({1 + round(i*(len(ranks)-1)/4) for i in range(5)})]
    n = len(points)
    shading = []
    legend = []
    inner = 0
    for (limit, label, color), count in zip(ERROR_BANDS, band_counts):
        outer = math.log2(limit)
        shading.extend([(-outer, -inner, color), (inner, outer, color)])
        inner = outer
        share = f"{count/n:.2%}" if n else "—"
        legend.append(f'<li><span style="background:{color}"></span>{esc(label)}'
                      f' <strong>{count} ({share})</strong></li>')
    band_note = (f'<p>Absolute error across {n} qualified cases · |predicted − measured| / '
                 'min(predicted, measured)</p><ul class="band-key">' + "".join(legend) + "</ul>")
    s_curve = chart("S-curve: predicted time relative to measured time",
                    [("head", ranks, "#087e8b")], "Case rank (underestimate → overestimate)",
                    "Time ratio (log₂ scale)", x_ticks=rank_ticks, y_ticks=ticks,
                    note=band_note, y_bands=shading)
    metrics = correlation_statistics(points)
    pearson, tau, pair = (metrics[k] for k in ("pearson", "tau", "pair"))
    note = (f"<p><strong>Kendall’s τ-b = {tau}</strong><br>"
            f"<strong>Pairs ordered correctly: {pair}</strong><br>"
            f"Pearson correlation (r) = {pearson}</p>"
            f"<p class='muted'>{n} qualified cases. The percentage compares pairs with distinct measured times.</p>")
    comparison = chart("Qualified reference vs. prediction", [("head", scatter, "#087e8b")],
                       "Reference user time (ms)", "Predicted user time (ms)", scatter=True, ideal=True, note=note)
    return (heading + '<div class="charts">'
            + s_curve + absolute_error_cdf(points) + '</div><div class="charts" style="margin-top:16px">'
            + comparison + "</div>")


def frontend_table(reference, simulated=None):
    labels = {"nop": "NOP stream", "add": "Eight independent ADD chains"}
    rows = []
    for loop, label in labels.items():
        seconds = reference["loops"][loop]
        rate = reference["instructions_per_trial"] / seconds
        model = ""
        if simulated:
            counters = simulated[loop]
            model = f"<td>{counters['instructions']/counters['cycles']:.2f}</td>"
        rows.append(f"<tr><td>{label}</td><td>{seconds*1000:.3f}</td>"
                    f"<td>{rate/1e9:.2f}</td><td>{rate/reference['assumed_hz']:.2f}</td>{model}</tr>")
    return (f'<h3>Front-end throughput on {esc(reference["hardware"])}</h3>'
            f'<p>{esc(reference["os"])} · {esc(reference["date"])} · median of {reference["trials"]} trials '
            f'of {reference["instructions_per_trial"]:,} loop instructions. Estimated IPC uses '
            f'{reference["assumed_hz"]/1e9:g} GHz and thread CPU time. Core and operating frequency were unobserved.</p>'
            '<div class="table"><table><thead><tr><th>Native loop</th><th>Thread time (ms)</th>'
            '<th>Billion instructions/s</th><th>Estimated native IPC</th>'
            + ('<th>Simulated IPC</th>' if simulated else '') + '</tr></thead><tbody>'
            + "".join(rows) + '</tbody></table></div>'
            + ('<p>Simulated IPC uses the same assembly loops. Subtracting the counts for 4,096 iterations '
               'from those for 8,192 removes fixed process setup and exit costs.</p>' if simulated else '')
            + '<p><a href="frontend.json">Native reference data</a></p>')


def prediction_text(values, role, digits=4, missing="pending"):
    return format(values[role], f".{digits}f") if role in values else missing


def simulation_index(simulations):
    return {role: {row["id"]: row for row in rows} for role, rows in simulations.items()}


def diagnostic_progress(report, name, reference=None):
    """Execution progress of one suite. Without a native reference the cases
    come from the retained definition and the text says nothing native exists."""
    simulations = report.get(name + "_simulation", [] if name == "throughput" else {"base": [], "head": []})
    rows = {"head": simulations} if name == "throughput" else simulations
    predicted = diagnostic_roles(report, name)
    rows = {role: values for role, values in rows.items() if role in predicted}
    required = len(diagnostic_inventory(report, name, reference))
    completed = sum(len(values) for values in rows.values())
    total = required * len(rows)
    item = report.get("diagnostic_progress", {}).get(name, {})
    state = item.get("state", "complete" if completed == total else "partial" if completed else "pending")
    counts = "; ".join(f"{role} {len(values)}/{required}" for role, values in rows.items())
    errors = " ".join(item.get("errors", []))
    note = ('Native references are shown; missing predictions are pending.' if reference is not None
            else 'No native reference exists for this device: simulated counters only.')
    if name != "throughput" and "base" not in predicted:
        note += ' The base revision does not model this device.'
    return (f'<p id="{name}-progress" data-section-state="{esc(state)}"><strong>{esc(name.title())}: {esc(state)}</strong> · '
            f'{esc(counts)} executions complete. ' + note
            + (" " + esc(errors) if errors else "") + '</p>')


def unmeasured_diagnostic_table(report, name):
    """Simulated counters of a suite this device has no native reference for.

    Nothing native is plotted or compared, so no hardware's numbers can stand in."""
    cases = definition_cases(report, name)
    simulations = report.get(name + "_simulation", [] if name == "throughput" else {"base": [], "head": []})
    indexed = simulation_index({"head": simulations} if name == "throughput" else simulations)
    predicted = [role for role in ("base", "head") if role in diagnostic_roles(report, name)]
    table = (f'<details id="{name}-simulated-table"><summary>{esc(name.title())} simulated counters · {len(cases)} points</summary>'
             '<div class="table"><table><thead><tr><th>Point</th><th>Instructions</th>'
             + ''.join(f'<th>{role.title()} cycles</th><th>{role.title()} cycles/instruction</th>' for role in predicted)
             + '</tr></thead><tbody>')
    for case in cases:
        head = indexed["head"].get(case["id"], {})
        cells = [esc(case["id"]), f'{head["instructions"]:,}' if "instructions" in head else "pending"]
        for role in predicted:
            row = indexed[role].get(case["id"])
            if row is None:
                cells += ["pending", "—"]
            elif "cycles" not in row:
                cells += [esc(row.get("status", "unavailable")), "—"]
            else:
                cells += [f'{row["cycles"]:,}', f'{row["cycles"] / row["instructions"]:.4f}']
        table += f'<tr data-row-key="{esc(case["id"])}">' + ''.join('<td>' + cell + '</td>' for cell in cells) + '</tr>'
    return (table + '</tbody></table></div><p>Counters are differences between a short and a long run of the same assembly, '
            'with checked instruction counts and outputs. No native reference exists for this device, so nothing here is '
            'compared with hardware and nothing enters application statistics.</p></details>')


def throughput_table(reference, simulated=None, *, partial=False):
    throughput.validate_reference(reference)
    if simulated is not None:
        throughput.validate_simulation(simulated, reference["cases"], partial=partial)
    by_id = {row["id"]: row for row in simulated or []}
    rows = []
    for index, case in enumerate(reference["cases"]):
        instructions = case["iterations"] * case["instructions_per_iteration"]
        ns = case["median_seconds"] * 1e9 / instructions
        spread = (case["p90_seconds"] - case["p10_seconds"]) / case["median_seconds"]
        predicted = by_id.get(case["id"])
        model = f"<td>{predicted['instructions'] / predicted['cycles']:.2f}</td>" if predicted else "<td>pending</td>"
        rows.append(f"<tr><td>{esc(case['id'])}</td><td>{ns:.3f}</td>"
                    f"<td>{case['p10_seconds']*1e9/instructions:.3f}–{case['p90_seconds']*1e9/instructions:.3f}</td>"
                    f"<td>{100*spread:.1f}%</td><td>{1e9/ns/reference['assumed_hz']:.2f}</td>{model}</tr>")
    return ('<h3>Instruction throughput suite</h3>'
            f"<p>{esc(reference['hardware'])} · {esc(reference['collected_at_utc'])} · native macOS thread CPU time. "
            "Nine paired long-minus-short measurements remove fixed harness overhead; each difference covers approximately 500 million instructions. "
            "P10/P90 use nearest lower sample ranks. Core and operating frequency were unobserved. "
            "Estimated IPC assumes 3.49 GHz; it does not establish hardware issue width. "
            "Memory probes reuse a warmed 64-byte working set intended to fit L1.</p>"
            '<div class="table"><table><thead><tr><th>Probe</th><th>Native ns/instruction</th><th>P10–P90 ns/instruction</th>'
            '<th>Spread / median</th><th>Estimated native IPC</th>'
            + '<th>Simulated IPC</th>' + '</tr></thead><tbody>' + ''.join(rows)
            + '</tbody></table></div><p>Simulation uses the same assembly loops, differencing 1,024 and 2,048 iterations. '
            'All checksums are checked analytically. '
            'These cases are excluded from application accuracy statistics. <a href="throughput.json">Reference data</a></p>')


def forwarding_panels(reference, simulations, profiles, *, partial=False):
    """Render dependency probes only in the microbenchmark section."""
    forwarding.validate_reference(reference)
    forwarding.validate_comparison(simulations, reference["cases"], partial=partial)
    indexed = simulation_index(simulations)
    frequencies = {role: profiles[role]["configuration"]["freq"] for role in ("base", "head")}
    if any(type(hz) not in (int, float) or not math.isfinite(hz) or hz <= 0 for hz in frequencies.values()):
        raise ValueError("invalid forwarding model frequency")
    rows = {}
    for index, case in enumerate(reference["cases"]):
        updates = case["iterations"] * forwarding.UPDATES_PER_ITERATION
        native = {key: case[key + "_seconds"] * 1e9 / updates for key in ("p10", "median", "p90")}
        model = {role: indexed[role][case["id"]]["cycles"] * 1e9 / indexed[role][case["id"]]["updates"] / frequencies[role]
                 for role in ("base", "head") if case["id"] in indexed[role]}
        spread = 100 * (native["p90"] - native["p10"]) / native["median"]
        tooltip = (f'{case["id"]}: native P10/median/P90 {native["p10"]:.4f}/{native["median"]:.4f}/{native["p90"]:.4f} ns/update; '
                   f'spread {spread:.2f}%; base {prediction_text(model, "base", 4)}, head {prediction_text(model, "head", 4)} ns/update')
        rows[case["id"]] = dict(native=native, model=model, spread=spread, tooltip=tooltip)
    groups = (
        ("64-bit GPR dependencies", ("gpr64_register", "gpr64_same", "gpr64_disjoint", "gpr64_indexed", "gpr64_indexed_load", "gpr64_indexed_store"),
         ("reg", "direct", "apart", "idx", "idx ld", "idx st")),
        ("32-bit GPR dependencies", ("gpr32_register", "gpr32_same"), ("reg", "direct")),
        ("Compare/select dependencies", ("select_register", "select_same"), ("reg", "direct")),
        ("FP dependencies", ("fp_register", "fp_same", "fp_disjoint"), ("reg", "direct", "apart")),
    )
    graphs = '<h3>Store/load dependency probes</h3><p>Value-varying assembly updates; native macOS thread CPU time. '
    graphs += (f'{esc(reference["hardware"])} · {esc(reference["collected_at_utc"])}. '
               'Nine alternating long-minus-short pairs; 64 updates per loop iteration. '
               'Hover a point for P10/median/P90, spread and both simulator predictions. '
               'Core and instantaneous frequency were unobserved. '
               + 'Simulation converts cycles using the declared model frequency: '
               + ', '.join(f'{role} {hz/1e9:g} GHz' for role, hz in frequencies.items())
               + '. These probes are excluded from application accuracy statistics.</p><div class="charts">')
    for title, names, labels in groups:
        series = []
        for role, color in (("native", "#9a6a17"), ("base", "#71838d"), ("head", "#087e8b")):
            values = [(i+1, rows[name]["native"]["median"] if role == "native" else rows[name]["model"][role], rows[name]["tooltip"])
                      for i, name in enumerate(names) if role == "native" or role in rows[name]["model"]]
            series.append((role, values, color))
        graphs += chart(title, series, "Probe form", "ns/update", scatter=True,
                        x_ticks=[(i+1, label) for i, label in enumerate(labels)])
    graphs += ('</div><p>Reg is the same-class register chain; direct uses immediate-offset load/store; apart is a '
               'nonoverlap throughput control, not a latency baseline. Idx uses register offsets on both sides, '
               'idx ld/st on one side. Compare/select increments and wraps modulo 65,536. '
               'Matched memory-minus-register differences estimate effective dependency cost including instruction/address overhead, '
               'not physical bypass latency. P10/P90 are variability summaries, not confidence intervals.</p>')
    table = (f'<details id="forwarding-table"><summary>Store/load dependency table · {len(rows)} probes</summary>'
             '<div class="table"><table><thead><tr><th>Probe</th><th>Native ns/update</th><th>P10–P90 ns/update</th>'
             '<th>Spread / median</th><th>Base ns/update</th><th>Head ns/update</th><th>Native memory − register (ns/update)</th>'
             '</tr></thead><tbody>')
    for name, row in rows.items():
        baseline = next(names[0] for _, names, _ in groups if name in names)
        difference = '—' if name == baseline or name.endswith('_disjoint') else f'{row["native"]["median"] - rows[baseline]["native"]["median"]:.4f}'
        n = row["native"]
        table += (f'<tr><td>{esc(name)}</td><td>{n["median"]:.4f}</td><td>{n["p10"]:.4f}–{n["p90"]:.4f}</td>'
                  f'<td>{row["spread"]:.2f}%</td><td>{prediction_text(row["model"], "base")}</td><td>{prediction_text(row["model"], "head")}</td><td>{difference}</td></tr>')
    table += ('</tbody></table></div><p>Simulation differences use the same assembly at 1,024 and 2,048 iterations, '
              'with exact instruction counts and analytic checksums. <a href="forwarding.json">Native reference data</a></p></details>')
    return graphs, table


def scheduling_panels(reference, simulations, profiles, *, partial=False):
    """Compare additional independent work around the same dependent chain."""
    scheduling.validate_reference(reference)
    scheduling.validate_comparison(simulations, reference["cases"], partial=partial)
    indexed = simulation_index(simulations)
    frequencies = {role: profiles[role]["configuration"]["freq"] for role in ("base", "head")}
    if any(type(hz) not in (int, float) or not math.isfinite(hz) or hz <= 0 for hz in frequencies.values()):
        raise ValueError("invalid scheduling model frequency")
    rows = {}
    for index, case in enumerate(reference["cases"]):
        native = {key: case[key + "_seconds"] * 1e9 / case["iterations"] for key in ("p10", "median", "p90")}
        modeled = {role: indexed[role][case["id"]]["cycles"] * 1e9 / (scheduling.ITERATIONS[1] - scheduling.ITERATIONS[0]) / frequencies[role]
                   for role in ("base", "head") if case["id"] in indexed[role]}
        tooltip = (f'{case["id"]}: native P10/median/P90 {native["p10"]:.3f}/{native["median"]:.3f}/{native["p90"]:.3f} ns/iteration; '
                   f'base {prediction_text(modeled, "base", 3)}, head {prediction_text(modeled, "head", 3)} ns/iteration; {case["instructions_per_iteration"]} instructions/iteration')
        rows[case["id"]] = dict(case=case, native=native, modeled=modeled, tooltip=tooltip)
    groups = (("Integer dependency and independent work", ("int_chain", "int_interleaved")),
              ("FP dependency and independent work", ("fp_chain", "fp_interleaved")))
    if set(rows) != {name for _, names in groups for name in names}:
        raise ValueError("scheduling panel requires the four declared dependency patterns")
    counts = ", ".join(f'{esc(case["id"])}: {case["instructions_per_iteration"]}' for case in reference["cases"])
    graphs = ('<h3>Execution-unit scheduling probes</h3><p>Each iteration has eight blocks. Each block runs eight dependent '
              'MADD or FMADD updates, a dependent fan-out to six integer or four FP registers, '
              'then an optional independent eight-update ADD or FADD chain. A final join feeds both paths into the next block. '
              'The independent accumulator resets before each block. '
              f'Instructions per iteration: {counts}. '
              'Compare the incremental cost of independent work per iteration, not equal-instruction throughput. '
              f'Native {esc(reference["hardware"])} · {esc(reference["collected_at_utc"])}. '
              'Nine alternating long-minus-short native macOS thread CPU-time pairs. '
              'Core and instantaneous frequency were unobserved; simulation uses declared frequencies: '
              + ", ".join(f'{role} {hz/1e9:g} GHz' for role, hz in frequencies.items())
              + '. These tested operand patterns do not establish physical M2 scheduling dimensions or arbitrary instruction latencies '
              'and are excluded from application accuracy statistics.</p><div class="charts">')
    for title, names in groups:
        selected = [rows[name] for name in names]
        series = []
        for role, color in (("native", "#9a6a17"), ("base", "#71838d"), ("head", "#087e8b")):
            points = [(i + 1, row["native"]["median"] if role == "native" else row["modeled"][role], row["tooltip"]) for i, row in enumerate(selected) if role == "native" or role in row["modeled"]]
            series.append((role, points, color))
        graphs += chart(title, series, "Instruction arrangement", "ns/iteration", scatter=True,
                        x_ticks=[(1, "chain"), (2, "interleaved")])
    graphs += '</div>'
    table = ('<details id="scheduling-table"><summary>Execution-unit scheduling table · 4 probes</summary>'
             '<div class="table"><table><thead><tr><th>Probe</th><th>Native ns/iteration</th><th>P10–P90</th>'
             '<th>Base ns/iteration</th><th>Head ns/iteration</th><th>Native ns/instruction</th></tr></thead><tbody>')
    for row in rows.values():
        n, m, case = row["native"], row["modeled"], row["case"]
        table += (f'<tr><td>{esc(case["id"])}</td><td>{n["median"]:.3f}</td><td>{n["p10"]:.3f}–{n["p90"]:.3f}</td>'
                  f'<td>{prediction_text(m, "base", 3)}</td><td>{prediction_text(m, "head", 3)}</td><td>{n["median"]/case["instructions_per_iteration"]:.4f}</td></tr>')
    table += ('</tbody></table></div><p>P10/P90 summarize variability, not confidence intervals. '
              'Simulation subtracts 1,024 from 2,048 iterations, with checked instruction counts and analytic checksums. '
              '<a href="scheduling.json">Native reference data</a></p></details>')
    return graphs, table



RECURRENCE_GROUPS = (
    ("Compare/select pair latency",
     ("sel32_rr_adj", "sel32_rr_sep", "sel32_rr_adj_p8", "sel32_rr_sep_p8", "sel32_rr_adj_p4", "sel64_rr_adj", "sel64_rr_sep"),
     ("Wa", "Ws", "Wa8", "Ws8", "Wa4", "Xa", "Xs")),
    ("Compare/select operand forms",
     ("sel32_rr_swap_adj", "sel32_rr_dst_adj", "sel32_rr_other_adj", "sel32_ri_same_adj", "sel32_ri_adj", "sel32_ri_sep",
      "sel32_ri16_adj", "sel32_shift_adj", "sel32_csinc_adj"),
     ("swap", "dst", "other", "15r", "15a", "15s", "16a", "shl", "inc")),
    ("Cycle scale and issue throughput", ("clock_add", "clock_fadd", "thr32_mix_adj", "thr32_mix_sep"),
     ("add", "fadd", "mix a", "mix s")),
    ("Same-address store/load recurrences",
     ("mem64_add", "mem64_add_alias", "mem64_add_disp", "mem64_partial", "mem64_independent", "mem_fadd",
      "fwd_near", "fwd_mid", "fwd_far", "mem64_addr_far"),
     ("same", "alias", "disp", "part", "indep", "fp", "near", "mid", "far", "addr")),
    ("Memory-carried maximum and Nussinov loop",
     ("mem32_max_const", "mem32_max_chg", "mem32_max_const_sep", "tight_add_mem", "tight_max_mem", "tight_max_mem_same",
      "tight_max_reg", "nuss_const", "nuss_chg", "nuss_sep", "nuss_reg"),
     ("const", "chg", "sep", "t add", "t max", "t same", "t reg", "k con", "k chg", "k sep", "k reg")),
)


def recurrence_panels(reference, simulations, *, partial=False):
    """Compare/select and memory-recurrence probes, in cycles per update.

    Native and simulated values share one unit, so no frequency enters."""
    recurrence.validate_reference(reference)
    recurrence.validate_comparison(simulations, reference["cases"], partial=partial)
    indexed = simulation_index(simulations)
    rows = {}
    for case in reference["cases"]:
        native = {key: case[key + "_cycles_per_update"] for key in ("p10", "median", "p90")}
        model = {role: indexed[role][case["id"]]["cycles"] / indexed[role][case["id"]]["updates"]
                 for role in ("base", "head") if case["id"] in indexed[role]}
        tooltip = (f'{case["id"]}: native P10/median/P90 {native["p10"]:.4f}/{native["median"]:.4f}/{native["p90"]:.4f} cycles/update; '
                   f'spread {case["relative_spread"]:.2%}; {case["quality"]}; '
                   f'base {prediction_text(model, "base", 4)}, head {prediction_text(model, "head", 4)} cycles/update')
        rows[case["id"]] = dict(case=case, native=native, model=model, tooltip=tooltip)
    if sorted(rows) != sorted(name for _, names, _ in RECURRENCE_GROUPS for name in names):
        raise ValueError("recurrence panel requires exactly the declared probes")
    campaign, environment = reference["campaign"], reference["environment"]
    counted = "counter_cycles_per_cpu_second" in campaign
    graphs = ('<h3>Compare/select and memory-recurrence probes</h3>'
              f'<p>{esc(reference["hardware"])} · native {esc(reference["os"].title())} · {esc(reference["host"])} · '
              f'{esc(reference["core_identity"])} · {esc(reference["collected_at_utc"])}. '
              f'Cycles per update: {esc(reference["cycle_scale"])}. '
              + ('Counted instructions were ' + f'{campaign["counted_over_declared_instructions"]:.6f}'
                 + ' of the declared loops and the counter advanced ' + f'{campaign["counter_cycles_per_cpu_second"] / 1e9:.4f}'
                 + ' billion cycles per thread-CPU second. ' if counted else '')
              + ('The counter group was enabled and running for the same ' + f'{int(environment["counter_enabled_ns"]) / 1e9:.3f}'
                 + ' s, so it was never multiplexed. ' if environment.get("counters") == "perf_event_user" else '')
              + f'{campaign["blocks"]} mirrored blocks; each visit differences N and 2N trips in ABBA or BAAB order with intervals of about '
              f'{campaign["target_seconds"] * 1e3:g} ms or more. '
              f'{campaign["performance_core_interval_share"]:.1%} of main intervals stayed on the reference core; other visits are counted, not mixed in. '
              + (f'The operating system reported {campaign["reported_khz_minimum"]:,}–{campaign["reported_khz_maximum"]:,} kHz at block boundaries, '
                 'an observation rather than a measurement. ' if "reported_khz_maximum" in campaign else '')
              + 'Simulated values are model cycles per update, so native and simulated points share one unit and no frequency is assumed. '
              'Hover a point for P10/median/P90, spread, quality and both simulator predictions. '
              'These probes are excluded from application accuracy statistics.</p><div class="charts">')
    for title, names, labels in RECURRENCE_GROUPS:
        series = []
        for role, color in (("native", "#9a6a17"), ("base", "#71838d"), ("head", "#087e8b")):
            values = [(i + 1, rows[name]["native"]["median"] if role == "native" else rows[name]["model"][role], rows[name]["tooltip"])
                      for i, name in enumerate(names) if role == "native" or role in rows[name]["model"]]
            series.append((role, values, color))
        graphs += chart(title, series, "Probe", "cycles/update", scatter=True,
                        x_ticks=[(i + 1, label) for i, label in enumerate(labels)])
    graphs += ('</div><p>Pair latency: W and X are 32- and 64-bit register compares selecting the compared registers, '
               'a adjacent, s with one instruction between, 8 and 4 the loop shifted by that many bytes (at 4 every fourth pair '
               'straddles a 64-byte line). Operand forms, all 32-bit and adjacent except 15s: swap selects the compared registers '
               'in the other order, dst writes a third register, other selects two registers that were not compared; '
               '15r compares with immediate 15 and selects between the compared register and one other; 15a, 15s, 16a and shl '
               'compare with an immediate (15, 15, 16, 1 LSL 12) and select two registers that were not compared; inc uses CSINC. '
               'Mix is the independent compare/select/add/add stream. Store/load: same base register, a second base register '
               'holding the same address, displacement 8, a 32-bit load of a 64-bit store, no memory dependency, FP; near, mid and far '
               'reload 0, 4 and 8 instructions after the store; addr carries the recurrence through the load address. '
               'Maximum: constant or changing stored value and the separated pair; t is one static pair per taken branch '
               '(t same reloads into the register it stores); '
               'k is the eleven-instruction Nussinov loop replica. P10/P90 are variability summaries, not confidence intervals.</p>')
    table = (f'<details id="recurrence-table"><summary>Compare/select and memory-recurrence table · {len(rows)} probes</summary>'
             '<div class="table"><table><thead><tr><th>Probe</th><th>Native cycles/update</th><th>P10–P90</th><th>Spread / median</th>'
             '<th>Quality</th><th>Visits used / excluded</th><th>Base cycles/update</th><th>Head cycles/update</th>'
             '<th>Head / native</th><th>Native ns/update</th></tr></thead><tbody>')
    for name, row in rows.items():
        case, n, m = row["case"], row["native"], row["model"]
        ratio = f'{m["head"] / n["median"]:.3f}' if "head" in m and case["quality"] != "invalid" else "—"
        table += (f'<tr><td>{esc(name)}</td><td>{n["median"]:.4f}</td><td>{n["p10"]:.4f}–{n["p90"]:.4f}</td>'
                  f'<td>{case["relative_spread"]:.2%}</td><td>{esc(case["quality"])}</td><td>{case["blocks"]} / {case["excluded_blocks"]}</td>'
                  f'<td>{prediction_text(m, "base")}</td><td>{prediction_text(m, "head")}</td><td>{ratio}</td>'
                  f'<td>{case["median_ns_per_update"]:.4f}</td></tr>')
    table += ('</tbody></table></div><p>Simulation differences use the same assembly at 128 and 256 outer trips, '
              'with exact instruction counts and analytic checksums. Head / native is a ratio of cycles per update. '
              '<a href="recurrence.json">Native reference data</a></p></details>')
    return graphs, table


def capacity_panels(reference, simulations, profiles, *, partial=False):
    """Capacity controls and divider probes never enter application summaries."""
    capacity.validate_reference(reference)
    capacity.validate_comparison(simulations, reference["cases"], partial=partial)
    indexed = simulation_index(simulations)
    frequencies = {role: profiles[role]["configuration"]["freq"] for role in ("base", "head")}
    if any(type(hz) not in (int, float) or not math.isfinite(hz) or hz <= 0 for hz in frequencies.values()):
        raise ValueError("invalid capacity model frequency")
    rows = []
    for index, case in enumerate(reference["cases"]):
        native = {key: case[key + "_seconds"] * 1e9 / case["iterations"] for key in ("p10", "median", "p90")}
        modeled = {role: indexed[role][case["id"]]["cycles"] * 1e9 / (capacity.ITERATIONS[1] - capacity.ITERATIONS[0]) / frequencies[role] for role in ("base", "head") if case["id"] in indexed[role]}
        spread = 100 * (native["p90"] - native["p10"]) / native["median"]
        tip = f'{case["id"]}: native {native["median"]:.3f} ns/iteration; P10-P90 {native["p10"]:.3f}-{native["p90"]:.3f}; base {prediction_text(modeled, "base", 3)}, head {prediction_text(modeled, "head", 3)}'
        rows.append(dict(case=case, native=native, modeled=modeled, spread=spread, tooltip=tip))
    graphs = ('<h3>FP capacity and divider controls</h3><p>Two independent integer dependency gates surround independent FP destinations. '
              'Gate lengths, NOP fillers and ungated FP throughput distinguish rename pressure from ROB, dependency and execution-port limits. '
              'Native ns are thread CPU-time differences; core and instantaneous frequency were unobserved. '
              'Model ns use each profile’s assumed frequency to convert simulated cycles. '
              'The model FP destination cap is separate from its ROB capacity and is not a physical register-file size. '
              'Divider controls include trivial fixed points and normal reciprocal / sqrt(x+1) recurrences. '
              'All results remain separate from application accuracy statistics.</p><div class="charts">')
    groups = [(f"FP destinations, gate {gate}", [r for r in rows if r["case"]["id"].startswith(f"fp_g{gate}_")], "Independent FP destinations", "_n") for gate in (64, 96)]
    groups += [(f"NOP / ROB control, gate {gate}", [r for r in rows if r["case"]["id"].startswith(f"nop_g{gate}_")], "NOP fillers", "_n") for gate in (64, 96)]
    groups += [("Ungated FP ports", [r for r in rows if r["case"]["kind"] == "ports"], "Independent FP destinations", "_n")]
    for op in ("fdiv", "fsqrt"):
        for normal in (False, True):
            groups.append((op.upper() + (" normal operands" if normal else " trivial operand control"),
                           [r for r in rows if r["case"]["kind"] == op and ("normal" in r["case"]["id"]) == normal],
                           "Independent active lanes", "lanes"))
    for title, selected, xlabel, separator in groups:
        selected.sort(key=lambda r: int(r["case"]["id"].split(separator)[1]))
        series = []
        for role, color in (("native", "#9a6a17"), ("base", "#71838d"), ("head", "#087e8b")):
            points = [(int(r["case"]["id"].split(separator)[1]),
                       r["native"]["median"] if role == "native" else r["modeled"][role], r["tooltip"]) for r in selected if role == "native" or role in r["modeled"]]
            series.append((role, points, color))
        graphs += chart(title, series, xlabel, "ns/iteration", scatter=True,
                        x_ticks=[(n, str(n)) for n in (1, 4, 16)] if separator == "lanes" else None)
    inference = capacity.infer_transition(reference["boundary"])
    boundary_series, boundary_ranges = [], []
    boundary_table = ('<details id="capacity-boundary-table"><summary>Paired capacity boundary · 34 rows</summary>'
                      '<div class="table"><table><thead><tr><th>Gate</th><th>FP destinations</th><th>Median ratio</th>'
                      '<th>P10–P90</th><th>Quality</th></tr></thead><tbody>')
    brackets = []
    for gate, color in ((64, "#9a6a17"), (96, "#087e8b")):
        boundary_rows = sorted((row for row in reference["boundary"]["cases"] if row["id"].startswith(f"fp_g{gate}_")),
                               key=lambda row: int(row["id"].split("_n")[1]))
        points = []
        for row in boundary_rows:
            count = int(row["id"].split("_n")[1])
            quality = "qualified" if row["qualified"] else "noisy (retained)"
            tip = f'gate {gate}, {count} destinations: {row["median_ratio"]:.4f}; P10–P90 {row["p10_ratio"]:.4f}–{row["p90_ratio"]:.4f}; {quality}'
            points.append((count, row["median_ratio"], tip))
            boundary_ranges.append((count, row["p10_ratio"], row["p90_ratio"], color))
            boundary_table += (f'<tr><td>{gate}</td><td>{count}</td><td>{row["median_ratio"]:.4f}</td>'
                               f'<td>{row["p10_ratio"]:.4f}–{row["p90_ratio"]:.4f}</td><td>{quality}</td></tr>')
        boundary_series.append((f"gate {gate}", points, color))
        inferred = inference[gate]
        bracket = inferred["bracket"]
        brackets.append(f'gate {gate}: ({bracket[0]}, {bracket[1]}]' if bracket else f'gate {gate}: no qualified bracket')
    graphs += chart("Paired capacity boundary (ABBA)", boundary_series, "Independent FP destinations",
                    "Target / matching NOP time", scatter=True, y_ranges=boundary_ranges)
    graphs += ('</div><p>Post-acquisition deterministic analysis: ' + esc("; ".join(brackets)) +
               '. Qualified counts ≤356 define each plateau: median of medians and maximum P90. '
               'A rise requires >15% above that median and P10 above the plateau maximum P90. '
               'Brackets run from the last qualified non-rise to the first qualified rise; noisy rows remain visible. '
               'The initial survey informed a dense confirmation sweep; quality and rise criteria were fixed before that second acquisition. '
               'Bars show sample P10–P90 dispersion, not confidence intervals.</p>')
    boundary_table += '</tbody></table></div></details>'
    table = (f'<details id="capacity-table"><summary>FP capacity and divider table · {len(rows)} probes</summary>'
             '<div class="table"><table><thead><tr><th>Probe</th><th>Native ns/iteration</th><th>P10–P90</th><th>Spread</th><th>Quality</th>'
             '<th>Base ns/iteration</th><th>Head ns/iteration</th></tr></thead><tbody>')
    for r in rows:
        n, m = r["native"], r["modeled"]
        table += (f'<tr><td>{esc(r["case"]["id"])}</td><td>{n["median"]:.3f}</td><td>{n["p10"]:.3f}–{n["p90"]:.3f}</td>'
                  f'<td>{r["spread"]:.1f}%</td><td>{"noisy" if r["spread"] > 20 else "usable"}</td><td>{prediction_text(m, "base", 3)}</td><td>{prediction_text(m, "head", 3)}</td></tr>')
    table += ('</tbody></table></div><p>Raw spread is not a confidence interval. Capacity inference uses additional interleaved ABBA controls, '
              'retained with the <a href="capacity.json">native reference and provenance</a>. No samples are dropped.</p></details>')
    return graphs, table + boundary_table


def memory_panels(reference, simulations, profiles, base_large_image_support, *, partial=False):
    """One canonical renderer for CI reports, live snapshots and diagnostic previews."""
    memory.validate_reference(reference)
    memory.validate_comparison(simulations, reference["cases"], base_large_image_support, partial=partial)
    indexed = simulation_index(simulations)
    frequencies = {role: profiles[role]["configuration"]["freq"] for role in ("base", "head")}
    if any(not memory.finite(hz) for hz in frequencies.values()):
        raise ValueError("invalid memory model frequency")
    rows = []
    for index, case in enumerate(reference["cases"]):
        chase = case["kind"] <= 5
        native_seconds = {k: case[k + "_seconds"] / case["native_passes"] for k in ("p10", "median", "p90")}
        predicted = {role: indexed[role][case["id"]]["cycles"] / frequencies[role] / case["passes"]
                     for role in frequencies if case["id"] in indexed[role] and "cycles" in indexed[role][case["id"]]}
        def metric(seconds):
            return seconds * 1e9 / case["operations_per_pass"] if chase else case["effective_bytes_per_pass"] / seconds / 1e9
        native = {k: metric(v) for k, v in native_seconds.items()}
        modeled = {k: metric(v) for k, v in predicted.items()}
        unit = "ns/load" if chase else "GB/s"
        size = case["working_set_bytes"]
        label = f"{size // 1048576} MiB" if size >= 1048576 else f"{size // 1024} KiB"
        low, high = sorted((native["p10"], native["p90"]))
        tooltip = (f'{case["pattern"]} · {label} ({size:,} total resident bytes)\n'
                   f'Native {native["median"]:.4f} {unit}; P10–P90 time-derived range {low:.4f}–{high:.4f} {unit}\n'
                   f'Base {prediction_text(modeled, "base", missing="unsupported image capacity" if case["id"] in indexed["base"] else "pending")}; head {prediction_text(modeled, "head", 4)} {unit}\n'
                   f'Native quality {case["quality"]}; paired-time spread {case["relative_spread"]:.2%}')
        rows.append(dict(case=case, native=native, modeled=modeled, unit=unit, size_label=label, low=low, high=high, tooltip=tooltip))
    usable = sum(row["case"]["quality"] == "usable" for row in rows)
    graphs = ('<h3 id="memory-diagnostics">Memory latency, parallelism and bandwidth</h3>'
              f'<p>{len(rows)} diagnostic points; {usable} usable native references. Single-core scalar assembly; '
              'the same function instruction bytes run natively as Mach-O and in simulated ELF. '
              'Each short and long execution follows initialization and two full warmup passes. '
              'Native values use nine alternating long-minus-short thread CPU-time pairs. '
              'Core identity and instantaneous frequency were unobserved. '
              'The total working set stays fixed across chain counts; Copy includes both buffers. '
              'These points do not enter application accuracy statistics.</p>'
              '<p>One random chain measures dependent-load loop latency; multiple chains show aggregate time per load, '
              'not individual access latency. Streaming plots show effective scalar bytes/s, not cache-line or DRAM-bus traffic '
              'and not peak vector bandwidth. Sizes describe working sets, not verified physical cache levels. '
              'Noisy native points remain visible and are identified in every tooltip. '
              'Vertical ranges show native P10–P90 repeat variability, not confidence intervals or seed sensitivity. '
              'Base points absent because of image capacity are explicitly marked in the table.</p><div class="charts">')
    for pattern in memory.PATTERNS:
        selected = [r for r in rows if r["case"]["pattern"] == pattern["id"]]
        series = []
        for role, color in (("native", "#9a6a17"), ("base", "#71838d"), ("head", "#087e8b")):
            points = [(math.log2(r["case"]["working_set_bytes"]), r["native"]["median"] if role == "native" else r["modeled"][role], r["tooltip"]) for r in selected if role == 'native' or role in r['modeled']]
            series.append((role, points, color))
        # Anchor the full logarithmic range rather than indexing into a matrix
        # whose transition samples may become denser over time.
        tick_sizes = {selected[0]['case']['working_set_bytes'], selected[-1]['case']['working_set_bytes'],
                      128 * 1024, 1024 * 1024, 16 * 1024 * 1024}
        ticks = [(math.log2(r['case']['working_set_bytes']), r['size_label']) for r in selected
                 if r['case']['working_set_bytes'] in tick_sizes]
        title = (f'Random pointer chase · {pattern["chains"]} chain' + ('' if pattern['chains'] == 1 else 's')) if pattern['kind'] <= 5 else f'Sequential {pattern["id"].title()}'
        ylabel = ('Dependent ns/load' if pattern['kind'] == 1 else 'Aggregate ns/load') if pattern['kind'] <= 5 else 'Effective GB/s'
        ranges = [(math.log2(r['case']['working_set_bytes']), r['low'], r['high'], '#9a6a17') for r in selected]
        graphs += chart(title, series, 'Total working set (log₂ spacing)', ylabel, x_ticks=ticks, y_ranges=ranges)
    graphs += '</div>'
    table = (f'<details id="memory-diagnostics-table"><summary>Memory diagnostics table · {len(rows)} points</summary>'
             '<div class="table"><table><thead><tr><th>Pattern</th><th>Total working set</th><th>Unit</th><th>Native</th>'
             '<th>P10–P90 time-derived range</th><th>Base</th><th>Head</th><th>Quality</th><th>Time spread</th></tr></thead><tbody>')
    for row in rows:
        c, n, m = row['case'], row['native'], row['modeled']
        table += (f'<tr data-row-key="{esc(c["id"])}"><td>{esc(c["pattern"])}</td><td>{esc(row["size_label"])}</td><td>{row["unit"]}</td>'
                  f'<td>{n["median"]:.4f}</td><td>{row["low"]:.4f}–{row["high"]:.4f}</td>'
                  f'<td>{prediction_text(m, "base", missing="Unsupported image capacity" if c["id"] in indexed["base"] else "pending")}</td><td>{prediction_text(m, "head", 4)}</td><td>{esc(c["quality"])}</td><td>{c["relative_spread"]:.2%}</td></tr>')
    table += ('</tbody></table></div><p>P10/P90 summarize variability, not confidence intervals. '
              'Native quality requires a ≥20 ms differential and (P90−P10)/median ≤10%. '
              'Simulation differences cover complete passes with exact instruction counts and checked outputs. '
              '<a href="memory.json">Native memory reference</a></p></details>')
    return graphs, table


def memory_preview(report_path, source, output):
    """Render complete memory-only evidence without relabeling historical application data."""
    report, source, output = read(report_path), Path(source), Path(output)
    if (report.get("kind") != "poros-memory-diagnostics-v1" or report.get("schema") != 1
            or report.get("artifact_status") != "pass" or report.get("accuracy_status") != "unverified"
            or report.get("head", {}).get("dirty") is not False):
        raise ValueError("invalid memory-only artifact")
    sha(report['head']['commit']); sha(report['base_commit'])
    for name in ('head_binary_sha256', 'base_binary_sha256', 'plan_id'):
        if not memory.sha(report.get(name)):
            raise ValueError("invalid memory-only execution identity")
    reference_path = source / memory.REFERENCE
    if digest(reference_path) != report.get('reference_sha256') or digest(reference_path) != report['head']['files'].get(memory.REFERENCE):
        raise ValueError("memory-only reference identity mismatch")
    reference = memory.load_reference(source)
    memory.validate_reference(reference, report['head']['files'])
    # A source that declares devices names the artifact's device at both
    # revisions, and that device must own this reference. Artifacts of sources
    # without a declaration are the default device, as they always were.
    devices.verify_source(source, report)
    if not devices.measured(report, "memory"):
        raise ValueError("memory-only artifact is for a device without a native memory reference")
    devices.bind_reference(report, "memory", memory.REFERENCE, reference)
    profiles = model_profiles.verify_source(source, report)
    if report.get("memory_base_large_image_support") is not memory.supports_large_images(source, report["base_commit"]):
        raise ValueError("memory-only base capability differs from source")
    graphs, table = memory_panels(reference, report['memory_simulation'], profiles, report.get('memory_base_large_image_support'))
    output.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(reference_path, output / 'memory.json')
    shutil.copyfile(report_path, output / 'memory-report.json')
    body = ('<header><div class="eyebrow">Poros / Memory diagnostics</div><h1>Memory microbenchmarks</h1>'
            '<p>Local diagnostic preview · application accuracy was not rerun.</p>'
            f'<p>Base <code>{esc(report["base_commit"])}</code><br>Head <code>{esc(report["head"]["commit"])}</code></p></header>'
            '<div class="notice"><strong>Hardware accuracy: unverified.</strong> '
            'This artifact covers the complete declared memory diagnostic set, not the application accuracy campaign.</div>'
            '<section id="microbenchmarks">' + graphs + table + '</section>'
            '<p><a href="memory-report.json">Simulation evidence</a> · <a href="memory.json">Native plotting reference</a></p>')
    (output / 'index.html').write_text(page('Poros memory diagnostics', body, POINT_TIPS))


def timing_curves(groups, cases, drawn=("reference", "base", "head")):
    """Time against input size. Only series that can exist for the report's device are drawn or named."""
    body = '<div class="charts">'
    for group in groups:
        workload = group["id"]
        axis = group["axis"]
        divisor, xlabel = {
            "iterations": (1e6, "Iterations (millions)"),
            "working_set_bytes": (1048576, "Working set (MiB)"),
            "dimension": (1, "Matrix dimension n"),
            "input_point": (1, "Input point (dimensions shown on hover)"),
        }[axis]
        rows = [c for c in cases if c["id"].startswith(workload + "-")]
        series = []
        for name, color in (
            ("reference", "#9a6a17"),
            ("base", "#71838d"),
            ("head", "#087e8b"),
        ):
            if name not in drawn:
                continue
            values = []
            for c in rows:
                seconds = (
                    c.get("reference_user_seconds")
                    if name == "reference"
                    else c.get(name, {}).get("predicted_user_seconds")
                )
                if seconds is not None:
                    point = int(c["id"].split("-")[1])
                    x = (
                        math.log2(point / group["points"][0])
                        if axis == "working_set_bytes"
                        else point / divisor
                    )
                    values.append(
                        (
                            x,
                            seconds * 1000,
                            f'{c["id"]}: {seconds*1000:.6f} ms ({c["reference_quality"]})',
                        )
                    )
            series.append((name, values, color))
        ticks = None
        if axis == "working_set_bytes":
            ticks = [
                (
                    math.log2(n / group["points"][0]),
                    f"{n/1048576:g} MiB" if n >= 1048576 else f"{n/1024:g} KiB",
                )
                for n in group["points"]
            ]
            xlabel = "Working set (log₂ spacing)"
        if axis == "input_point":
            ticks = [(n, str(n)) for n in group["points"]]
            for _, in_rows, _ in series:
                for i, (x, y, label) in enumerate(in_rows):
                    in_rows[i] = (x, y, label + "; " + group["labels"][int(x)-1])
        legacy_progress = workload in ("integer", "branch", "memory") and "points" in group
        if legacy_progress:
            body += "<div>"
            state = "complete" if len(rows) == len(group["points"]) else "partial" if rows else "pending"
            body += f'<p id="legacy-{esc(workload)}-progress">{esc(workload.title())}: {state} · {len(rows)} / {len(group["points"])} whole-program points</p>'
        body += chart(
            workload.upper() if workload == "mvt" else workload.title(),
            series,
            xlabel,
            "User time (ms)",
            x_ticks=ticks,
            **({} if "reference" in drawn else {"empty": "No predictions yet."}),
        )
        if legacy_progress:
            body += "</div>"
    body += "</div>"
    return body


def prediction_table(cases, ids):
    """Per-case predictions of a device without a native reference.

    There are no reference, error or direction columns to leave empty or fill."""
    body = ('<p class="muted">No native reference exists for this device. Values are simulator predictions with checked '
            'outputs; times are milliseconds.</p><div class="table"><table><thead><tr><th>Case</th><th>Functional</th>'
            '<th>Base</th><th>Head</th><th>Head instructions</th><th>Head cycles</th><th>Head cycles/instruction</th>'
            '<th>Head / base time</th></tr></thead><tbody>')
    by_id = {c["id"]: c for c in cases}
    for case in ids:
        c = by_id.get(case)
        if not c:
            body += "<tr><td>" + esc(case) + '</td><td colspan="7">Pending — not simulated yet</td></tr>'
            continue
        base, head = c.get("base", {}), c["head"]
        values = [
            case,
            c.get("functional_status", "not separately recorded"),
            number(base["predicted_user_seconds"] * 1000) if base else c.get("base_status", "unavailable"),
            number(head["predicted_user_seconds"] * 1000),
            f'{head["instructions"]:,}',
            f'{head["cycles"]:,}',
            f'{head["cycles"] / head["instructions"]:.4f}',
            f'{head["predicted_user_seconds"] / base["predicted_user_seconds"]:.4f}' if base else "—",
        ]
        body += "<tr>" + "".join("<td>" + esc(v) + "</td>" for v in values) + "</tr>"
    return body + '</tbody></table></div>'


def timing_table(cases, ids, referenced=True):
    if not referenced:
        return prediction_table(cases, ids)
    body = '<p class="muted">Error = 100 × |sim − reference| / min(sim, reference). Positive Δ means a larger error. No offset fitting or anchor removal. Times are milliseconds.</p><div class="table"><table><thead><tr><th>Case</th><th>Functional</th><th>Reference</th><th>Quality</th><th>Base</th><th>Head</th><th>Base error</th><th>Head error</th><th>Direction</th><th>Δ pp</th></tr></thead><tbody>'
    by_id = {c["id"]: c for c in cases}
    for case in ids:
        c = by_id.get(case)
        if not c:
            body += (
                "<tr><td>"
                + esc(case)
                + '</td><td colspan="8">Missing — report incomplete</td></tr>'
            )
            continue
        base, head = c.get("base", {}), c.get("head", {})
        values = [
            case,
            c.get("functional_status", "not separately recorded"),
            number(c.get("reference_user_seconds", 0) * 1000),
            c["reference_quality"],
            number(base.get("predicted_user_seconds", 0) * 1000) if base else c.get("base_status", "unavailable"),
            number(head.get("predicted_user_seconds", 0) * 1000) if head else "—",
            number(base.get("error_percent"), "%"),
            number(head.get("error_percent"), "%"),
            head.get("direction", "—"),
            number(c.get("error_change_percentage_points")),
        ]
        body += "<tr>" + "".join("<td>" + esc(v) + "</td>" for v in values) + "</tr>"
    body += '</tbody></table></div>'
    return body


def overall_progress(report, references, live=None):
    """Count measurement points, resolving a paired diagnostic only once.

    Whole-program points already include their functional/head/base evidence.
    Diagnostic IDs are namespaced by suite; legacy18 are not counted again.
    A half-finished pair is in progress even between active execution callbacks.
    """
    declared = report_manifest(report)["cases"]
    covered_cases(report["cases"], declared, partial=report["artifact_status"] == "partial")
    known = {case["id"] for case in declared}
    completed = {("whole-program", row["id"]) for row in report["cases"]}
    required = {("whole-program", ident) for ident in known}
    active_ids = (live or {}).get("active_cases", [])
    if len(set(active_ids)) != len(active_ids) or any(ident not in known or ("whole-program", ident) in completed for ident in active_ids):
        raise ValueError("invalid overall active cases")
    active = {("whole-program", ident) for ident in active_ids}
    for name in declared_diagnostics(report):
        simulations = report[name + "_simulation"]
        rows = {"head": simulations} if name == "throughput" else simulations
        if not isinstance(rows, dict) or set(rows) != ({"head"} if name == "throughput" else {"head", "base"}):
            raise ValueError("overall progress requires every diagnostic role")
        native = references.get(name)
        if native is not None:
            cases = native["cases"]
        elif not devices.measured(report, name):
            cases = definition_cases(report, name)
        else:
            # Complete historical callers may omit reference assets. Their rows
            # have already passed strict coverage validation, so are the inventory.
            if report["artifact_status"] != "pass":
                raise ValueError("overall progress needs the declared " + name + " reference")
            cases = rows["head"]
        ids = {case["id"] for case in cases}
        if len(ids) != len(cases):
            raise ValueError("duplicate overall diagnostic inventory")
        done = {}
        for role in diagnostic_roles(report, name):
            values = rows[role]
            covered_cases(values, cases, partial=report["artifact_status"] == "partial")
            done[role] = {row["id"] for row in values}
        resolved = set.intersection(*done.values())
        started = set.union(*done.values())
        seen_active = set()
        for item in report.get("diagnostic_progress", {}).get(name, {}).get("active", []):
            if item["role"] not in done or item["id"] not in ids or item["id"] in done[item["role"]]:
                raise ValueError("invalid overall active diagnostic")
            key = (item["role"], item["id"])
            if key in seen_active:
                raise ValueError("duplicate overall active diagnostic")
            seen_active.add(key)
            started.add(item["id"])
        required.update((name, ident) for ident in ids)
        completed.update((name, ident) for ident in resolved)
        active.update((name, ident) for ident in started - resolved)
    if "throughput" not in declared_diagnostics(report) and probe.ELF in report["head"]["files"]:
        # The superseded two-loop frontend suite remains readable historically.
        required.update(("frontend", ident) for ident in ("nop", "add"))
        completed.update(("frontend", ident) for ident in report.get("frontend_simulation", {}))
    if completed & active or not (completed | active) <= required:
        raise ValueError("invalid overall progress coverage")
    return dict(required=len(required), completed=len(completed), active=len(active),
                remaining=len(required - completed - active))


def live_progress(live, totals=None):
    completed, required = live["completed_cases"], live["required_cases"]
    # Snapshot validation rejects unknown/completed active IDs. Count unique IDs
    # so repeated worker observations cannot inflate the in-progress segment.
    active = len(set(live["active_cases"]))
    if totals is not None:
        completed, required, active = (totals[key] for key in ("completed", "required", "active"))
    remaining = required - completed - active
    if required <= 0 or completed < 0 or remaining < 0:
        raise ValueError("invalid live progress counts")
    if totals is not None and live["state"] == "complete" and completed != required:
        raise ValueError("unfinished measurements marked complete")
    percentage = 100 * completed / required
    validated = live.get("validated_bundle", False)
    state = ("Complete" if validated else "Points finished · validation pending") if live["state"] == "complete" else live["state"].title()
    body = '<section class="notice" aria-labelledby="live-progress-label">'
    body += '<div id="live-progress-label"><strong>' + esc(state) + '</strong> · '
    body += f'{completed} / {required} points · {percentage:.1f}%</div>'
    segments = (("completed", "Completed", completed, "#168574"),
                ("active", "In progress", active, "#4e87d4"),
                ("remaining", "Not started", remaining, "#d7e1e4"))
    description = "; ".join(f"{label}: {count}" for _, label, count, _ in segments)
    body += f'<div id="case-progress" role="progressbar" aria-label="Overall measurement progress" aria-valuemin="0" aria-valuenow="{completed}" aria-valuemax="{required}" aria-valuetext="{description}" aria-describedby="live-progress-context" style="display:flex;height:18px;margin-top:10px;border-radius:4px;overflow:hidden">'
    for key, _, count, color in segments:
        body += f'<span data-progress-state="{key}" aria-hidden="true" style="width:{100 * count / required:.8f}%;background:{color}"></span>'
    body += '</div><p style="display:flex;gap:16px;flex-wrap:wrap">'
    for _, label, count, color in segments:
        body += f'<span><span aria-hidden="true" style="display:inline-block;width:10px;height:10px;background:{color};margin-right:5px"></span>{label}: {count}</span>'
    body += '</p>'
    body += '<p id="live-progress-context">'
    body += "A whole-program point includes its required checks and model roles; a paired diagnostic point completes when both roles are resolved. In progress includes partial pairs and points assigned to active workers. "
    if "required_shards" in live:
        body += f'{live["completed_cases"]} / {live["required_cases"]} whole-program cases; {live["completed_shards"]} / {live["required_shards"]} shards. '
    if live["completed_cases"] == live["required_cases"] and completed < required:
        body += "Whole-program cases complete; diagnostics remain. "
    if validated:
        body += "Complete validated bundle."
    if live.get("updated_at"):
        body += ' Updated ' + esc(live["updated_at"]) + "."
    body += "</p>"
    if live["errors"]:
        body += '<p>' + esc('; '.join(live["errors"])) + '</p>'
    return body + '</section>'


def device_name(value):
    if not isinstance(value, str) or not devices.NAME.fullmatch(value):
        raise ValueError("unsafe device name")
    return value


def report_device(report):
    """The device a report, including a failure, is filed under."""
    return device_name(devices.ident(report))


def device_panel(report, embedded=False):
    """Which device a report describes.

    A report shown on its own also lists the devices declared with it. Inside a
    commit report that list is the commit overview's, so it is not repeated."""
    if "device" not in report:
        return ('<p id="device" class="muted">Device: <code>' + devices.DEFAULT + '</code> · '
                + ('the default: this failure names no device' if report["artifact_status"] == "fail"
                   else 'implied: this report predates device selection') + '.</p>')
    described = device_summary(report)
    if described is None:  # A failure recorded before the declarations were read.
        return '<p id="device">Device: <code>' + esc(report_device(report)) + '</code></p>'
    body = (f'<section id="device" data-device="{esc(described["id"])}"><p><strong>Device: {esc(described["hardware"])}</strong> '
            f'(<code>{esc(described["id"])}</code>) · Parameters: {esc(described["parameters"])} · '
            f'Native references: {esc(described["references"])}</p>')
    if embedded:
        return body + '</section>'
    body += ('<div class="table"><table><thead><tr><th>Device</th><th>Hardware</th><th>Parameter source</th>'
             '<th>Native references</th><th>Shown</th></tr></thead><tbody>')
    declaration = report["device"]["head"]["declaration"]
    for declared in declaration["devices"]:
        where = "on this page" if declared["id"] == described["id"] else "in its own view of the commit's report"
        body += (f'<tr data-row-key="{esc(declared["id"])}"><td><code>{esc(declared["id"])}</code></td><td>{esc(declared["hardware"])}</td>'
                 f'<td>{esc(devices.parameter_text(declared, declaration))}</td>'
                 f'<td>{esc(reference_availability(report, declared))}</td><td>{where}</td></tr>')
    return body + '</tbody></table></div></section>'


def unmeasured_overview(report, benchmarks, required):
    """What stands in for accuracy charts when a device has no native reference: nothing numeric."""
    declared = devices.entry(report)
    paired = sum("base" in c for c in benchmarks)
    return ('<h2 id="overall">Benchmark predictions</h2>'
            f'<div class="notice" id="unmeasured"><strong>No native {esc(declared["hardware"])} reference.</strong> '
            'Error, correlation and accuracy statistics do not exist for this device, and none are shown. '
            f'Each case was simulated as <code>{esc(declared["id"])}</code> and its complete output checked; '
            'times, cycles and instruction counts are predictions.</div>'
            f'<p>Application cases simulated with checked output: {len(benchmarks)} / {required}. '
            f'Also predicted by the base revision: {paired}.</p>')


def detail(report, entry, preview=False, **references):
    """One device's report as a page of its own: bundle previews and the single-device live view."""
    body, identity, state = detail_body(report, entry, preview, **references)
    body = (f'<div id="accuracy-report" data-report-identity="{identity}" data-live-state="{state}">'
            + body + f'<span hidden data-report-end="{identity}"></span></div>')
    refresh = LIVE_REFRESH if state == 'polling' else ''
    return page("Poros accuracy · " + entry["commit"][:12], body, POINT_TIPS + refresh)


def detail_body(report, entry, preview=False, frontend=None, throughput_reference=None, live=None, forwarding_reference=None, scheduling_reference=None, memory_reference=None, capacity_reference=None, recurrence_reference=None, embedded=False):
    """Render one device's report. Returns the body, its identity and its live state.

    `embedded` is the same rendering placed inside a commit report, which
    supplies the page header and the list of declared devices itself."""
    commit = entry["commit"]
    full_inventory = report["artifact_status"] == "pass" or live is not None
    body = ""
    if not embedded:
        body = (
            '<header><div class="eyebrow">Poros / Accuracy evidence</div><h1>Whole-program user time</h1><p class="muted"><code>'
            + esc(commit)
            + "</code></p>"
        )
        body += (
            "<p>PR / local preview · Not a published main result</p>"
            if preview
            else '<p><a href="../../index.html">← Report history</a></p>'
        ) + "</header>"
    if report.get("reporting"):
        rendering = report["reporting"]
        body += '<p class="muted">Measured simulator: <code>' + esc(commit) + '</code>; reporting implementation: <code>' + esc(rendering["commit"]) + '</code>' + (" (working tree edits; file hashes recorded)" if rendering["dirty"] else "") + ".</p>"
    if live is not None or (report.get("schema") == 4 and report["artifact_status"] == "pass"):
        references = dict(throughput=throughput_reference, forwarding=forwarding_reference,
                          scheduling=scheduling_reference, capacity=capacity_reference,
                          recurrence=recurrence_reference, memory=memory_reference)
        totals = overall_progress(report, references, live)
        progress = live if live is not None else dict(state="complete", validated_bundle=True,
            completed_cases=len(report["cases"]), required_cases=len(report_manifest(report)["cases"]),
            active_cases=[], errors=[])
        body += live_progress(progress, totals)
    native_reference = report.get("reference", {}).get("protocol") == performance.NATIVE_PROTOCOL
    cases = report.get("cases", [])
    benchmarks = [c for c in cases if not is_micro_case(c)]
    micros = [c for c in cases if is_micro_case(c)]
    declared = report_manifest(report)["cases"] if full_inventory else cases
    benchmark_ids = [c["id"] for c in declared if not is_micro_case(c)]
    micro_ids = [c["id"] for c in declared if is_micro_case(c)]
    if report["artifact_status"] == "fail" and not report.get("head", {}).get(
        "commit", report.get("head_commit")
    ):
        body += '<p class="notice">This failure occurred before source identity was recorded. The bundle identifies the requested target; it does not certify an executed binary or measured source.</p>'
    if report.get("error"):
        body += '<p class="notice">' + esc(report["error"]) + "</p>"
    body += device_panel(report, embedded)
    # Accuracy exists only against a native reference of the report's own device.
    referenced = devices.measured(report, "application")
    body += ('<nav class="downloads"><a href="#overall">' + ("Benchmark accuracy" if referenced else "Benchmark predictions")
             + '</a><a href="#microbenchmarks">Microbenchmarks</a></nav>')
    if referenced:
        body += overview_charts(benchmarks)
        body += comparison_table(benchmarks)
        summary = (compute.case_summary([c for c in benchmarks if c.get("head")], performance=report.get("schema") == 4)
                   if full_inventory else {})
        if summary.get("head"):
            mean = number(summary["head"]["mean_error_percent"], "%")
            maximum = number(summary["head"]["maximum_error_percent"], "%")
            body += f"<p>Application timing error across {summary['comparable']} qualified cases: mean {mean}, maximum {maximum}.</p>"
    else:
        body += unmeasured_overview(report, benchmarks, len(benchmark_ids))
    body += '<h2>Benchmark timing curves</h2>'
    groups = (
        report_manifest(report).get(
            "workloads",
            [dict(id=w, axis="iterations") for w in ("integer", "memory", "branch")],
        )
        if full_inventory
        else []
    )
    drawn = tuple(name for name, exists in (("reference", referenced), ("base", devices.available(report, "base")), ("head", True)) if exists)
    body += timing_curves([g for g in groups if g["id"] not in MICRO_WORKLOADS], benchmarks, drawn)
    body += (f'<details id="benchmark-table"><summary>Benchmark timing table · {len(benchmark_ids)} cases</summary>'
             + timing_table(benchmarks, benchmark_ids, referenced) + '</details>')
    body += '<section id="microbenchmarks"><h2>Microbenchmarks</h2>'
    body += '<p>Hardware diagnostics are reported separately from benchmark accuracy.</p>'
    if micro_ids and referenced:
        micro_usable = sum(c.get("reference_quality") == "usable" for c in micros)
        micro_required = len(micro_ids) if full_inventory else "—"
        body += f"<p>Usable microbenchmark references: {micro_usable} / {micro_required}</p>"
    body += timing_curves([g for g in groups if g["id"] in MICRO_WORKLOADS], micros, drawn)
    if report.get("schema") == 1:
        body += "<p>Historical memory sweep: 64 KiB fixed working set, varying iterations.</p>"
    elif full_inventory:
        accesses = next(g["iterations"] for g in report_manifest(report)["workloads"] if g["id"] == "memory")
        body += f"<p>Memory x axis uses log₂ spacing with actual byte-size labels; times are untransformed. Memory: {accesses:,} accesses per point, plus initialization of the declared working set. Cache-boundary positions describe the configured model, not validated hardware capacities.</p>"
    memory_table = ""
    if memory_reference is not None:
        memory_charts, memory_table = memory_panels(memory_reference, report.get("memory_simulation"), model_profiles.validate(report), report.get("memory_base_large_image_support"), partial=report["artifact_status"] == "partial")
        body += diagnostic_progress(report, "memory", memory_reference) + memory_charts
    elif full_inventory and memory.declared(report.get("head", {}).get("files", {})) and devices.measured(report, "memory"):
        raise ValueError("memory reference required by tested source")
    scheduling_table = ""
    if scheduling_reference:
        scheduling_charts, scheduling_table = scheduling_panels(
            scheduling_reference, report.get("scheduling_simulation"), model_profiles.validate(report), partial=report["artifact_status"] == "partial")
        body += diagnostic_progress(report, "scheduling", scheduling_reference) + scheduling_charts
    capacity_table = ""
    if capacity_reference:
        capacity_charts, capacity_table = capacity_panels(capacity_reference, report.get("capacity_simulation"), model_profiles.validate(report), partial=report["artifact_status"] == "partial")
        body += diagnostic_progress(report, "capacity", capacity_reference) + capacity_charts
    forwarding_table = ""
    if forwarding_reference:
        forwarding_charts, forwarding_table = forwarding_panels(
            forwarding_reference, report.get("forwarding_simulation"), model_profiles.validate(report), partial=report["artifact_status"] == "partial")
        body += diagnostic_progress(report, "forwarding", forwarding_reference) + forwarding_charts
    recurrence_table = ""
    if recurrence_reference:
        recurrence_charts, recurrence_table = recurrence_panels(
            recurrence_reference, report.get("recurrence_simulation"), partial=report["artifact_status"] == "partial")
        body += diagnostic_progress(report, "recurrence", recurrence_reference) + recurrence_charts
    if throughput_reference:
        body += diagnostic_progress(report, "throughput", throughput_reference)
        body += (f'<details id="throughput-table"><summary>Instruction throughput table · {len(throughput_reference["cases"])} probes</summary>'
                 + throughput_table(throughput_reference, report.get("throughput_simulation"), partial=report["artifact_status"] == "partial") + '</details>')
    elif frontend:
        body += (f'<details id="frontend-table"><summary>Frontend throughput table · {len(frontend["loops"])} probes</summary>'
                 + frontend_table(frontend, report.get("frontend_simulation")) + '</details>')
    body += memory_table + scheduling_table + forwarding_table + recurrence_table + capacity_table
    if full_inventory and report.get("schema") == 4:
        # Suites run for every device. Without a native reference of the
        # device's own there are only simulated counters to show.
        for name in declared_diagnostics(report):
            if not devices.measured(report, name):
                body += diagnostic_progress(report, name) + unmeasured_diagnostic_table(report, name)
    body += (f'<details id="microbenchmark-table"><summary>Whole-program microbenchmark timing table · {len(micro_ids)} cases</summary>'
             + timing_table(micros, micro_ids, referenced) + '</details></section>')
    replacement = report.get("reference", {}).get("streamcluster_replacement")
    if replacement:
        body += ('<p>Streamcluster uses 16 complete initialization/computation/output passes per process. '
                 'Its six-point native acquisition is recorded separately in the plotting reference; '
                 'the other 258 reference points retain their original acquisition.</p>')
    body += (
        "<h2>Evidence and provenance</h2><p>Measurement completed (UTC): "
        + esc(entry.get("measurement_completed_at") or ("unavailable" if referenced else "none; no native measurement exists for this device"))
        + '</p><div class="downloads">'
        + ('<a href="report.json">Report JSON</a><a href="report.md">Markdown report</a><a href="bundle.json">Evidence hashes</a>' if live is None or live.get("validated_bundle") else "")
    )
    if report["artifact_status"] == "pass":
        if report["schema"] == 4 and not referenced:
            pass  # No native reference exists for this device, so none is offered.
        elif report["schema"] == 4:
            body += '<a href="reference.json">Plotting data and native output identities</a>'
            collection = report["reference"]["collection_commit"]
            body += '<a href="https://github.com/' + REPO + '/tree/' + collection + '">' + ('Frozen ELF/computation source' if native_reference else 'Exact native collection source') + '</a>'
        elif report["schema"] == 3:
            body += '<a href="cohorts/original/reference.json">Original24 raw samples</a><a href="cohorts/compute/reference/reference.json">Compute240 raw samples</a><a href="cohorts/compute/reference/oracle.json">Independent output oracle</a><a href="cohorts/compute/reference/validation-report.json">Original validation report</a><a href="cohorts/compute/reference/validation-harness.py">Original validation harness</a><a href="cohorts/compute/source/scripts/compute_usertime.py">Compute measurement protocol</a>'
        else:
            body += '<a href="reference.json">Raw native samples</a><a href="manifest.json">Workloads and hardware profile</a><a href="native_usertime.py">Measurement harness</a>'
    if entry.get("run_url"):
        body += '<a href="' + esc(entry["run_url"]) + '">GitHub Actions run</a>'
    if entry.get("evaluation_run_url") and entry["evaluation_run_url"] != entry.get(
        "run_url"
    ):
        body += (
            '<a href="'
            + esc(entry["evaluation_run_url"])
            + '">Full Evaluation proof</a>'
        )
    if entry.get("evidence_run_url") and entry["evidence_run_url"] != entry.get(
        "run_url"
    ):
        body += (
            '<a href="' + esc(entry["evidence_run_url"]) + '">Original evidence run</a>'
        )
    body += (
        '<a href="https://github.com/'
        + REPO
        + "/commit/"
        + commit
        + '">Source commit</a></div>'
    )
    if report.get("output_oracle"):
        oracle = report["output_oracle"]
        body += ('<p id="output-oracle">Outputs were checked against <code>' + esc(oracle["path"]) + '</code> (SHA-256 <code>'
                 + esc(oracle["sha256"]) + '</code>): ' + esc(oracle["scope"]) + '.</p>')
    inputs = [
        name for name in entry.get("evidence_files", []) if name.startswith(("inputs/", "cohorts/compute/source/", "cohorts/compute/reference/outputs/"))
    ]
    if inputs:
        body += (
            '<details><summary>Download workload sources and ELF binaries</summary><p class="downloads">'
            + " ".join(
                '<a href="' + esc(name) + '">' + esc(name[7:] if name.startswith("inputs/") else name) + "</a>"
                for name in inputs
            )
            + "</p></details>"
        )
    body += (
        "<details><summary>Reference environment and exact configuration</summary><pre>"
        + esc(
            json.dumps(
                {
                    "base_commit": report.get("base_commit"),
                    "reporting": report.get("reporting"),
                    "formula": report.get("formula"),
                    "reference": report.get("reference"),
                    "compute_reference": report.get("compute_reference"),
                    "compute_base_status": report.get("compute_base_status"),
                    "configuration": cases[0].get("head", {}) if cases else None,
                    "model_profiles": report.get("model_profiles"),
                    # The commit report shows the declaration once, in its overview.
                    **({"device": dict(id=report["device"]["id"], declaration_sha256=(report["device"].get("head") or {}).get("sha256"))
                                  if embedded else report["device"],
                        "output_oracle": report.get("output_oracle")} if "device" in report else {}),
                },
                indent=2,
            )
        )
        + "</pre></details>"
    )
    if live is not None:
        body += '<p><a href="snapshot.json">Live snapshot JSON</a></p>'
    identity = hashlib.sha256(json.dumps(dict(commit=commit, **{name: report.get(name) for name in (
        'base_commit', 'reference_sha256', 'workload_manifest', 'model_profiles',
        'head_binary_sha256', 'base_binary_sha256')}, **({"device": report["device"]["id"]} if "device" in report else {})),
        sort_keys=True).encode()).hexdigest()
    state = 'static' if live is None else 'validated' if live.get('validated_bundle') else 'polling'
    return body, identity, state


def series_key(report):
    if report["artifact_status"] != "pass":
        return None
    profiles = [
        {k: c["head"][k] for k in ("configuration", "cache_config")}
        for c in report["cases"]
    ]
    return hashlib.sha256(
        json.dumps(
            [report["formula"], report.get("reference_sha256"), profiles], sort_keys=True
        ).encode()
    ).hexdigest()


def validate_run_urls(record):
    for key in (
        "run_url",
        "evidence_run_url",
        "evaluation_run_url",
        "publication_run_url",
    ):
        if key in record and (
            not isinstance(record[key], str) or not RUN_URL.fullmatch(record[key])
        ):
            raise ValueError("invalid provenance URL: " + key)
    for release in record.get("releases", []):
        validate_run_urls(release)
    if "release" in record:
        validate_run_urls(record["release"])


def merge_evaluation(entry, incoming):
    """Order proof by Evaluation identity, independently of publication jobs."""
    conclusion = incoming.get("evaluation_conclusion", "not checked")
    if conclusion == "not checked":
        entry.setdefault("evaluation_conclusion", "not checked")
        return
    if conclusion not in TERMINAL | {
        "queued",
        "in_progress",
        "waiting",
        "pending",
        "requested",
    }:
        raise ValueError("invalid Evaluation state")
    key = tuple(incoming.get(k) for k in EVALUATION_FIELDS[:2])
    if not all(type(value) is int and value > 0 for value in key):
        raise ValueError("Evaluation proof requires its own run and attempt identity")
    expected_url = f"https://github.com/{REPO}/actions/runs/{key[0]}/attempts/{key[1]}"
    if incoming.get("evaluation_run_url") != expected_url:
        raise ValueError("Evaluation proof URL does not match its identity")
    previous = tuple(entry.get(k, 0) for k in EVALUATION_FIELDS[:2])
    if key < previous:
        return
    if key == previous:
        old = entry.get("evaluation_conclusion", "not checked")
        if old in TERMINAL:
            if conclusion not in TERMINAL:
                return
            if old != conclusion:
                raise ValueError(
                    "contradictory outcomes for the same Evaluation attempt"
                )
    for field in EVALUATION_FIELDS:
        entry[field] = incoming[field]


INDEX_SCHEMA = 2
# What a schema-1 index entry said about the commit, not about its one report.
COMMIT_FIELDS = ("commit", "main", "releases", *EVALUATION_FIELDS)
# Names in a commit directory that belong to the commit, not to the default device's bundle.
COMMIT_FILES = {"devices", "commit.json", "devices.json", "index.html"}
# A revision without a declaration is the default device alone; nothing more is known of it.
LEGACY_DEVICE = dict(id=devices.DEFAULT, hardware=None, parameter_source=devices.DEFAULT, references=None)
REFERENCE_ARGUMENTS = dict(frontend="frontend", throughput="throughput_reference", forwarding="forwarding_reference",
                           scheduling="scheduling_reference", capacity="capacity_reference",
                           recurrence="recurrence_reference", memory="memory_reference")


def commit_dir(site, commit):
    return Path(site) / "calibration" / "runs" / sha(commit)


def bundle_dir(site, commit, device):
    """Where a commit keeps one device's bundle.

    The default device's files stay directly in the commit directory, where
    every published report already has them. Other devices sit beside them."""
    folder = commit_dir(site, commit)
    return folder if device == devices.DEFAULT else folder / "devices" / device_name(device)


def clear_bundle(folder, device):
    """Remove one device's retained bundle and nothing that belongs to its siblings."""
    if device != devices.DEFAULT:
        if folder.exists():
            shutil.rmtree(folder)
    elif folder.exists():
        for child in folder.iterdir():
            if child.name not in COMMIT_FILES:
                shutil.rmtree(child) if child.is_dir() else child.unlink()
    folder.mkdir(parents=True, exist_ok=True)


def copy_bundle(folder, device, destination):
    """Copy one device's retained bundle without the commit's report or its sibling devices."""
    top = str(folder)
    shutil.copytree(folder, destination, ignore=lambda directory, names: (
        COMMIT_FILES & set(names) if device == devices.DEFAULT and directory == top else ()))


def migrate_index(index):
    """Schema 1 held one default-device report per commit. Give each entry its device level."""
    if index.get("schema") == INDEX_SCHEMA:
        return index
    if index.get("schema") != 1:
        raise ValueError("unsupported archive schema")
    entries = []
    for old in index["entries"]:
        entry = {key: old[key] for key in COMMIT_FIELDS if key in old}
        # The Evaluation recorded for the one report becomes that device's own,
        # so a later Evaluation of the commit no longer replaces it.
        entry["devices"] = {devices.DEFAULT: {key: value for key, value in old.items()
                                               if (key not in COMMIT_FIELDS or key in EVALUATION_FIELDS)
                                               and key not in ("release", "device")}}
        entries.append(entry)
    return dict(index, schema=INDEX_SCHEMA, entries=entries)


def report_declaration(report):
    """What one report proves about the devices declared at its commit; None if nothing."""
    chosen = report.get("device")
    if isinstance(chosen, dict) and isinstance(chosen.get("head"), dict):
        if report["artifact_status"] != "pass":
            # Validation stops early for a failure. Its declaration counts only
            # if it is bound to the failure's own source inventory.
            try:
                devices.validate(report)
            except (KeyError, TypeError, ValueError):
                return None
        head = chosen["head"]
        return dict(status="declared", origin="report", path=head["path"], sha256=head["sha256"], declaration=head["declaration"])
    # A passing report without a declaration in its source inventory, or from
    # before reports had an inventory, predates devices.
    files = report.get("head", {}).get("files", {})
    if report["artifact_status"] != "fail" and "device" not in report and isinstance(files, dict) and devices.PATH not in files:
        return dict(status="legacy", origin="report")
    return None


def source_declaration(source, commit):
    """The declaration read from Git at exactly this commit; None when that source cannot be read."""
    if source is None:
        return None
    try:
        found = devices.revision(source, commit)
    except (subprocess.CalledProcessError, OSError, ValueError):
        return None
    if found is None:
        return dict(status="legacy", origin="source")
    return dict(status="declared", origin="source", path=found["path"], sha256=found["sha256"], declaration=found["declaration"])


def commit_declaration(stored, found):
    """One declaration per commit, from verified evidence of that commit only.

    A retained record, a report's source-bound copy and the repository at the
    commit must agree. With none of them, support is unknown: it is never taken
    from another revision, and never from the reporting checkout."""
    known = [record for record in [stored, *found] if record and record["status"] != "unknown"]
    for record in known:
        if record["status"] == "declared" and (
                record.get("path") != devices.PATH
                or hashlib.sha256(devices.encode(devices.check(record["declaration"]))).hexdigest() != record["sha256"]):
            raise ValueError("retained device declaration does not match its recorded digest")
    if not known:
        return dict(status="unknown")
    first = known[0]
    if any((other["status"], other.get("sha256")) != (first["status"], first.get("sha256")) for other in known[1:]):
        raise ValueError("conflicting device declarations for one commit")
    return first


def declared_devices(record):
    """A commit's devices in declared order; the default device alone for a legacy revision; none if unknown."""
    if record["status"] == "declared":
        return record["declaration"]["devices"]
    return [LEGACY_DEVICE] if record["status"] == "legacy" else []


def require_supported(record, device):
    if record["status"] == "declared":
        devices.select(record["declaration"], device)
    elif record["status"] == "legacy" and device != devices.DEFAULT:
        raise ValueError(f"this revision predates device declarations and models only {devices.DEFAULT}, not {device!r}")


def device_counts(view):
    """Application-case counts of one device's report; None without a usable report, which is unknown and not zero."""
    report = view.get("report")
    if view["state"] not in ("report", "running") or report is None:
        return None
    rows = [row for row in report["cases"] if not is_micro_case(row)]
    referenced = devices.measured(report, "application")
    return dict(required=sum(not is_micro_case(case) for case in report_manifest(report)["cases"]),
                reported=len(rows), referenced=referenced,
                qualified=sum(qualified(row) for row in rows) if referenced else None)


def pooled_observations(views):
    """Every application row of every device that is compared with its own native reference.

    One observation is one device and one application input, so the same input
    on two devices is two observations, each named by its device. Devices
    without a report, with failed evidence or without a native application
    reference contribute nothing."""
    rows = []
    for view in views:
        counts = device_counts(view)
        if counts is not None and counts["referenced"]:
            rows += [dict(row, id=view["id"] + "/" + row["id"], device=view["id"])
                     for row in view["report"]["cases"] if not is_micro_case(row)]
    return rows


def pooled_summary(views):
    """The commit's application summary over pooled observations, in the shape one report's summary has."""
    rows = pooled_observations(views)
    contributing = [view for view in views if (device_counts(view) or {}).get("referenced")]
    summary = compute.case_summary(rows, performance=all(view["report"].get("schema") == 4 for view in contributing)) if rows else {}
    errors = [row["head"]["error_percent"] for row in rows if "error_percent" in row["head"]]
    overview = dict(devices={view["id"]: dict(state=view["state"], **(device_counts(view) or {})) for view in views},
                    contributors=[view["id"] for view in contributing],
                    median_error_percent=statistics.median(errors) if errors else None)
    return rows, summary, overview


def reference_text(view, record):
    """Native reference availability of one device at this commit, in words."""
    declared = view["declared"]
    if declared is None:
        return "unknown"
    if declared is LEGACY_DEVICE or declared.get("references") is None:
        return "measured (legacy evidence)" if view["state"] == "report" else "unknown without a report"
    report = view.get("report")
    kinds = ["application", *declared_diagnostics(report)] if report is not None and report.get("schema") == 4 and "files" in report.get("head", {}) else list(devices.KINDS)
    status = devices.reference_status(declared, kinds)
    if status == "partial":
        status += ": " + ", ".join(kind for kind in kinds if kind in declared["references"])
    return status


def commit_overview(record, views, *, retained=True):
    """The commit level of a report: its declared devices and the accuracy pooled over them."""
    rows, summary, overview = pooled_summary(views)
    if record["status"] == "declared":
        body = ('<p id="declaration">Devices declared at this commit by <code>' + esc(record["path"]) + '</code>, SHA-256 <code>'
                + esc(record["sha256"]) + '</code>'
                + (' (<a href="devices.json">retained copy</a>)' if retained else '') + ', read from '
                + ("the repository at this commit" if record["origin"] == "source" else "a report's source inventory of this commit") + '.</p>')
    elif record["status"] == "legacy":
        body = ('<p id="declaration">This revision predates device declarations. Its evidence is the default device, <code>'
                + devices.DEFAULT + '</code>; no other device is assumed.</p>')
    else:
        body = ('<div class="notice" id="declaration"><strong>Supported devices are unknown for this commit.</strong> '
                'Only failed evidence exists, and the device declaration of this revision could not be read from its source. '
                'The devices listed are those a run was attempted for; no support is inferred from any other revision.</div>')
    body += ('<div class="table"><table id="commit-devices"><thead><tr><th>Device</th><th>Hardware</th><th>Parameter source</th>'
             '<th>Native references</th><th>Report</th><th>Application cases</th></tr></thead><tbody>')
    for view in views:
        declared, counts = view["declared"], device_counts(view)
        report = view.get("report")
        if declared is None:
            hardware, parameters = "unknown", "unknown"
        elif declared["hardware"] is None:
            reference = report.get("reference") if report is not None else None
            hardware = reference.get("hardware", "not recorded") if isinstance(reference, dict) else "not recorded"
            parameters = devices.DEFAULT + " (implied)"
        else:
            hardware, parameters = declared["hardware"], devices.parameter_text(declared, record["declaration"])
        if counts is None:
            cases = "—"
        elif counts["referenced"]:
            cases = f'{counts["qualified"]} qualified of {counts["required"]}'
            if counts["reported"] != counts["required"]:
                cases += f' ({counts["reported"]} reported)'
        else:
            cases = f'{counts["reported"]} of {counts["required"]} predicted; no native reference'
        badge = {"report": "pass", "failed": "fail"}.get(view["state"], "")
        body += (f'<tr data-row-key="{esc(view["id"])}"><td><a href="#device-{esc(view["id"])}"><code>{esc(view["id"])}</code></a></td>'
                 f'<td>{esc(hardware)}</td><td>{esc(parameters)}</td><td>{esc(reference_text(view, record))}</td>'
                 f'<td><span class="badge {badge}">{esc(view["state"])}</span></td><td>{esc(cases)}</td></tr>')
    body += '</tbody></table></div>'
    included = [view for view in views if view["id"] in overview["contributors"]]
    reasons = {"pending": "no report yet", "failed": "failed evidence"}
    excluded = [f'{view["id"]} ({reasons.get(view["state"], "no native application reference: predictions only")})'
                for view in views if view not in included]
    count = sum(qualified(row) for row in rows)
    body += '<h3 id="pooled-accuracy">Pooled application accuracy</h3>'
    scope = ('Each observation is one device and one application input, compared with that device\'s own native reference; '
             'the same input on two devices is two observations. Statistics are computed over the pooled observations, '
             'never by averaging per-device results.')
    if not count:
        body += ('<div class="notice" id="pooled-empty"><strong>No qualified observation at this commit.</strong> '
                 'Pooled error and correlation statistics are undefined here, not zero. '
                 + ('Not pooled: ' + esc('; '.join(excluded)) + '. ' if excluded else '')
                 + ('The reported devices that have a native application reference have no qualified application case. '
                    if included else '') + '</div>')
        return body, summary, overview
    body += ('<p id="pooled-scope">' + f'{count} qualified observations from {len(included)} of {len(views)} devices: '
             + esc(', '.join(f'{view["id"]} {device_counts(view)["qualified"]} of {device_counts(view)["required"]}' for view in included))
             + '. ' + scope + (' Not pooled: ' + esc('; '.join(excluded)) + '.' if excluded else '') + '</p>')
    if any(view["state"] in ("pending", "failed", "running") for view in views):
        body += ('<p class="notice" id="pooled-partial">Partial: not every declared device has a complete report, '
                 'so these statistics describe only the devices listed above.</p>')
    body += (f'<p id="pooled-error">Absolute error |predicted − measured| / min(predicted, measured) across {summary["comparable"]} '
             f'qualified observations: mean {number(summary["head"]["mean_error_percent"], "%")}, '
             f'median {number(overview["median_error_percent"], "%")}, maximum {number(summary["head"]["maximum_error_percent"], "%")}.</p>')
    body += overview_charts(rows, heading='')
    measures = correlation_statistics([(row["reference_user_seconds"], row["head"]["predicted_user_seconds"], row["id"])
                                       for row in rows if qualified(row)])
    undefined = [label for key, label in (("pearson", "Pearson r"), ("tau", "Kendall τ-b"), ("pair", "the share of correctly ordered pairs"))
                 if measures[key] == "—"]
    if undefined:
        body += ('<p class="notice" id="pooled-undefined">Undefined for these observations, shown as —: ' + esc(', '.join(undefined))
                 + '. Each needs at least two observations that differ in the compared times.</p>')
    return body + comparison_table(rows, "pooled observations"), summary, overview


def embed(body, device):
    """Place one device's rendered report inside the commit report.

    The default device keeps its element names, so links into reports published
    before commits had several devices still land. Every other device gets its
    own names and its own bundle directory, so no two sections share either.
    A device name has no underscore, so `device-a_b` is element `b` of device
    `a` and never the view of a device called `a-b`."""
    if device == devices.DEFAULT:
        return body
    prefix, folder = f"device-{device}_", f"devices/{device}/"
    body = re.sub(r' (id|aria-labelledby|aria-describedby)="([^"]+)"', lambda m: f' {m.group(1)}="{prefix}{m.group(2)}"', body)
    body = re.sub(r' href="#([^"]+)"', lambda m: f' href="#{prefix}{m.group(1)}"', body)
    return re.sub(r' href="(?!#|https?://|\.\./|/)([^"]+)"', lambda m: f' href="{folder}{m.group(1)}"', body)


VIEW_SCRIPT = """<script>
(() => {
  // Navigation is plain fragment links; this only tells assistive technology which view is shown.
  const links = [...document.querySelectorAll('nav.views a')];
  function mark() {
    let target = null;
    try { target = location.hash.length > 1 ? document.getElementById(decodeURIComponent(location.hash.slice(1))) : null; } catch (_) {}
    const view = (target && target.closest('.view')) || document.getElementById('overview');
    for (const link of links) {
      if (link.getAttribute('href') === '#' + view.id) link.setAttribute('aria-current', 'true');
      else link.removeAttribute('aria-current');
    }
  }
  addEventListener('hashchange', mark);
  mark();
})();
</script>"""


def commit_report(commit, record, views, *, releases=(), live=False, retained=True):
    """The one report of a commit: its overview and every declared device, as views of one document.

    A view is a section reached by a fragment link, so the address names the
    view, and back, forward and a copied link all return to it. Where the
    browser supports it only the addressed view is shown; otherwise the views
    simply follow each other. Nothing depends on scripts."""
    overview, summary, pooled = commit_overview(record, views, retained=retained)
    selected = '.report:has(.view:target,.view :target)'
    style = ('<style>@supports selector(:has(*)){'
             + selected + '>.view:not(:target):not(:has(:target)){display:none}'
             '.report:not(:has(.view:target,.view :target))>.view:not(#overview){display:none}'
             + ','.join(['.report:not(:has(.view:target,.view :target)) .views a[href="#overview"]']
                        + [f'.report:has(#{name}:target,#{name} :target) .views a[href="#{name}"]'
                           for name in ["overview", *("device-" + view["id"] for view in views)]])
             + '{background:#18343e;color:white;border-color:#18343e}}</style>')
    body = (style + '<header><div class="eyebrow">Poros / Accuracy evidence</div><h1>Whole-program user time</h1><p class="muted"><code>'
            + esc(commit) + '</code></p>'
            + ('<p>Live commit view · Not a published result</p>' if live else '<p><a href="../../index.html">← Report history</a></p>')
            + (('<p>Pinned by release: ' + esc(', '.join(release["tag"] for release in releases)) + '</p>') if releases else '')
            + '</header><div class="report"><nav class="views" aria-label="Views of this report"><a href="#overview">Commit overview</a>')
    for view in views:
        body += f'<a href="#device-{esc(view["id"])}">{esc(view["id"])} · {esc(view["state"])}</a>'
    body += ('</nav><section class="view" id="overview" aria-labelledby="overview-title"><h2 id="overview-title">Commit overview</h2>'
             + overview
             + ('<p class="downloads"><a href="commit.json">Commit record: declaration, device bundles and digests</a></p>' if retained else '')
             + '</section>')
    for view in views:
        declared = view["declared"]
        name = declared["hardware"] if declared is not None and declared["hardware"] else "device"
        body += (f'<section class="view" id="device-{esc(view["id"])}" aria-labelledby="view-{esc(view["id"])}-title" '
                 f'data-device="{esc(view["id"])}" data-state="{esc(view["state"])}">'
                 f'<h2 id="view-{esc(view["id"])}-title">{esc(name)} <code>{esc(view["id"])}</code> · {esc(view["state"])}</h2>'
                 '<p><a href="#overview">← Commit overview</a></p>')
        if view["state"] == "pending":
            body += ('<p class="notice">No report has been published for this device at this commit. '
                     'It is declared here, so one can still arrive; nothing is assumed about its results.</p>')
        else:
            inner, _, _ = detail_body(view["report"], view["entry"], embedded=True, live=view.get("live"), **view["references"])
            body += embed(inner, view["id"])
        body += '</section>'
    # A live view is re-read while any device is still running. The address keeps its fragment.
    reload = '<script>setTimeout(() => location.reload(), 15000);</script>' if live and any(view["state"] == "running" for view in views) else ''
    return page("Poros accuracy · " + commit[:12], body + '</div>', POINT_TIPS + VIEW_SCRIPT + reload), summary, pooled


def archived_views(site, entry, bundles):
    """Every device of an archived commit: its state and, where a bundle is retained, its validated report."""
    views = [dict(id=declared["id"], declared=declared) for declared in declared_devices(entry["declaration"])]
    listed = {view["id"] for view in views}
    # Without a verified declaration only what was attempted can be shown.
    views += [dict(id=device, declared=None) for device in sorted(entry["devices"]) if device not in listed]
    for view in views:
        stored = entry["devices"].get(view["id"])
        if stored is None:
            view.update(state="pending", report=None)
            continue
        folder = bundle_dir(site, entry["commit"], view["id"])
        report, manifest = bundles[view["id"]]
        references = {argument: read(folder / (name + ".json")) if name + ".json" in manifest["files"] else None
                      for name, argument in REFERENCE_ARGUMENTS.items()}
        view.update(state="report" if report["artifact_status"] == "pass" else "failed", report=report, references=references,
                    entry=dict({key: entry[key] for key in COMMIT_FIELDS if key in entry}, **stored),
                    bundle_sha256=digest(folder / "bundle.json"))
    return views


def render_commit(site, entry, source=None):
    """Rewrite one commit's report and record from its retained device bundles."""
    commit, folder = entry["commit"], commit_dir(site, entry["commit"])
    bundles = {device: validate_bundle(bundle_dir(site, commit, device), commit) for device in entry["devices"]}
    for device, (report, _) in bundles.items():
        if report_device(report) != device:
            raise ValueError("retained report belongs to another device")
    stored = entry.get("declaration")
    # The repository is consulted only while the retained evidence says nothing.
    found = [report_declaration(report) for report, _ in bundles.values()]
    if not any(found) and (stored is None or stored["status"] == "unknown"):
        found.append(source_declaration(source, commit))
    entry["declaration"] = record = commit_declaration(stored, found)
    for device in list(entry["devices"]):
        try:
            require_supported(record, device)
        except ValueError:
            if bundles[device][0]["artifact_status"] != "fail":
                raise
            # A failure filed while the declaration was unknown, for a device the
            # commit turns out not to declare, is not evidence about this commit.
            shutil.rmtree(bundle_dir(site, commit, device))
            del entry["devices"][device], bundles[device]
    views = archived_views(site, entry, bundles)
    html, summary, pooled = commit_report(commit, record, views, releases=entry.get("releases", ()))
    (folder / "index.html").write_text(html)
    if record["status"] == "declared":
        (folder / "devices.json").write_bytes(devices.encode(record["declaration"]))
    else:
        (folder / "devices.json").unlink(missing_ok=True)
    states = [view["state"] for view in views]
    contributors = [(view["id"], series_key(view["report"])) for view in views if view["id"] in pooled["contributors"]]
    entry.update(
        # Any failed device fails the commit; a declared device without a report leaves it partial.
        artifact_status="fail" if "failed" in states else "partial" if "pending" in states else "pass",
        benchmark_summary=summary, overview=pooled,
        series=None if not contributors else contributors[0][1] if [name for name, _ in contributors] == [devices.DEFAULT]
        else hashlib.sha256(json.dumps(contributors, sort_keys=True).encode()).hexdigest())
    for view in views:
        if view["state"] != "pending":
            saved = entry["devices"][view["id"]]
            saved.update(benchmark_summary=benchmark_summary(view["report"]), bundle_sha256=view["bundle_sha256"])
            write(bundle_dir(site, commit, view["id"]) / "metadata.json",
                  dict({key: entry[key] for key in COMMIT_FIELDS if key in entry}, device=view["id"], **saved))
    write(folder / "commit.json", dict(
        schema=1, commit=commit, main=entry["main"], releases=entry["releases"], declaration=record,
        artifact_status=entry["artifact_status"], benchmark_summary=summary, overview=pooled,
        devices={view["id"]: dict(state=view["state"], **({} if view["state"] == "pending" else dict(
            artifact_status=entry["devices"][view["id"]]["artifact_status"],
            bundle=("" if view["id"] == devices.DEFAULT else f'devices/{view["id"]}/') + "bundle.json",
            bundle_sha256=view["bundle_sha256"]))) for view in views}))


def archive(site, bundle_path, metadata, main_order, keep=20, source=None):
    """Update only our subtree. Call in a disposable checkout, then commit atomically.

    A commit has one report. Each device's bundle joins it, under the device its
    validated report names, without touching the bundles of its siblings;
    publication metadata naming another device is rejected rather than refiled.
    `source` is a repository that has the commits, used to read a commit's
    device declaration when no retained report carries it."""
    site = Path(site)
    root = site / "calibration"
    if any(
        (site / name).is_symlink()
        for name in ("calibration", "index.html", ".nojekyll")
    ) or any(p.is_symlink() for p in root.rglob("*")):
        raise ValueError("archive must not contain symlinks")
    commit = sha(metadata["commit"])
    report, bundle_manifest = validate_bundle(bundle_path, commit)
    device = report_device(report)
    if metadata.get("device", devices.DEFAULT) != device:
        raise ValueError(f"publication names device {metadata.get('device', devices.DEFAULT)!r} but the report is for {device!r}")
    if metadata["kind"] not in ("main", "release") or metadata["conclusion"] not in (
        "success",
        "failure",
        "cancelled",
        "timed_out",
        "action_required",
        "startup_failure",
        "skipped",
        "stale",
        "neutral",
    ):
        raise ValueError("invalid publication state")
    if metadata["kind"] == "main" and commit not in main_order:
        raise ValueError("main report is not on current first-parent history")
    validate_run_urls(metadata)
    if "run_url" not in metadata:
        raise ValueError("missing publication URL")
    index_path = root / "index.json"
    index = migrate_index(
        read(index_path) if index_path.exists() else {"schema": INDEX_SCHEMA, "entries": []}
    )
    entries = {sha(e["commit"]): e for e in index["entries"]}
    for e in entries.values():
        validate_run_urls(e)
        for saved in e["devices"].values():
            validate_run_urls(saved)
            if saved["artifact_status"] not in ("pass", "fail") or "run_url" not in saved:
                raise ValueError("invalid retained entry")
    if metadata["kind"] == "release":
        for e in entries.values():
            if e["commit"] != commit and any(
                r["id"] == metadata["release"]["id"] for r in e["releases"]
            ):
                raise ValueError(
                    "a permanently pinned release cannot move to another commit"
                )
    entry = entries.get(commit) or dict(commit=commit, main=False, releases=[], devices={})
    # Refuse a device this commit does not declare before anything is stored.
    record = commit_declaration(entry.get("declaration"), [report_declaration(report), source_declaration(source, commit)])
    require_supported(record, device)
    entry["declaration"] = record
    entries[commit] = entry
    existing = entry["devices"].get(device)
    if existing and existing["artifact_status"] == "pass":
        # Reuse the original measured report, including its original comparison base.
        saved = existing
    else:
        destination = bundle_dir(site, commit, device)
        clear_bundle(destination, device)
        for name in bundle_manifest["files"]:
            target = destination / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(Path(bundle_path) / name, target)
        shutil.copyfile(Path(bundle_path) / "bundle.json", destination / "bundle.json")
        saved = entry["devices"][device] = {
            **{key: value for key, value in metadata.items()
               if key not in COMMIT_FIELDS and key not in ("device", "release")},
            "artifact_status": report["artifact_status"],
            "accuracy_status": report["accuracy_status"],
            "series": series_key(report),
            "summary": report.get("summary", {}),
            "evidence_run_url": metadata.get("evidence_run_url", metadata["run_url"]),
            "evidence_files": sorted(bundle_manifest["files"]),
            "measurement_completed_at": (
                measurement_completed(report, Path(bundle_path))
                if report["artifact_status"] == "pass"
                else None
            ),
        }
        # The Evaluation that produced this evidence stays with it. A later
        # publication that reuses the evidence does not replace its proof.
        merge_evaluation(saved, metadata)
    entry["main"] = entry["main"] or metadata["kind"] == "main"
    entry.pop("successful_evaluation", None)
    # The commit shows its newest Evaluation, whichever device it was for.
    merge_evaluation(entry, metadata)
    if metadata["kind"] == "main":
        if (int(metadata.get("run_id", 0)), int(metadata.get("run_attempt", 0))) >= (
            int(saved.get("run_id", 0)),
            int(saved.get("run_attempt", 0)),
        ):
            for key in ("conclusion", "run_id", "run_attempt", "run_url"):
                if key in metadata:
                    saved[key] = metadata[key]
    if metadata["kind"] == "release":
        release = metadata["release"]
        if (
            type(release["id"]) is not int
            or release["id"] <= 0
            or not isinstance(release["tag"], str)
            or len(release["tag"]) > 256
        ):
            raise ValueError("invalid release identity")
        if not any(r["id"] == release["id"] for r in entry["releases"]):
            entry["releases"].append(release)
    positions = {sha(c): i for i, c in enumerate(main_order)}
    normal = sorted(
        (e for e in entries.values() if e["main"] and not e["releases"]),
        key=lambda e: positions.get(e["commit"], len(positions)),
    )
    retain = {e["commit"] for e in normal[:keep]} | {
        e["commit"] for e in entries.values() if e["releases"]
    }
    for old in list(entries):
        if old not in retain:
            # A commit is retained or dropped whole, with every device's bundle.
            shutil.rmtree(commit_dir(site, old))
            del entries[old]
    ordered = sorted(
        entries.values(),
        key=lambda e: (positions.get(e["commit"], len(positions)), e["commit"]),
    )
    for e in ordered:
        render_commit(site, e, source)
    success = [
        e
        for e in ordered
        if e["main"]
        and e["commit"] in positions
        and e["artifact_status"] in ("pass", "partial")
        and e.get("evaluation_conclusion") == "success"
    ]
    index = {
        "schema": INDEX_SCHEMA,
        "main_head": main_order[0] if main_order else None,
        "latest_success": success[0]["commit"] if success else None,
        "entries": ordered,
    }
    write(index_path, index)
    (root / "index.html").write_text(home(index))
    (site / ".nojekyll").touch()
    if not (site / "index.html").exists():
        (site / "index.html").write_text(
            page(
                "Poros reports",
                '<h1>Poros accuracy reports</h1><p><a href="calibration/">Open calibration dashboard →</a></p>',
            )
        )
    return index


STATUS_SCRIPT = """<script>
(async()=>{const status=document.getElementById('current-main');try{
 const response=await fetch('https://api.github.com/repos/sarchlab/poros/commits/main',{headers:{Accept:'application/vnd.github+json'}});if(!response.ok)throw Error();const main=await response.json();
 const runsResponse=await fetch('https://api.github.com/repos/sarchlab/poros/actions/workflows/evaluation.yml/runs?branch=main&event=push&per_page=10');if(!runsResponse.ok)throw Error();const runs=await runsResponse.json();
 const run=runs.workflow_runs.find(r=>r.head_sha===main.sha);status.textContent='Current main '+main.sha.slice(0,12)+': '+(run?(run.conclusion||run.status):'evaluation pending or unavailable')+'. Latest successful report is listed separately below.';
}catch(_){status.append(' Live GitHub status unavailable; showing the last published snapshot.');}})();
</script>"""


def device_coverage(entry):
    """How many of a commit's own devices have reported, in words; no device is named or assumed."""
    record = entry.get("declaration") or {"status": "unknown"}
    reported = sum(saved["artifact_status"] == "pass" for saved in entry["devices"].values())
    failed = sum(saved["artifact_status"] == "fail" for saved in entry["devices"].values())
    if record["status"] == "unknown":
        return f"declaration unknown; {failed} failed"
    text = f"{reported} of {len(declared_devices(record))} reported"
    return text + (f", {failed} failed" if failed else "")


def home(index):
    """The archive landing: one row per commit, whatever devices that commit has."""
    index = migrate_index(index)
    body = '<header><div class="eyebrow">Poros / Calibration dashboard</div><h1>Accuracy over time</h1><p class="muted">Whole-program user CPU time · Raw predictions against retained hardware observations</p></header><div class="notice"><strong>Hardware accuracy remains unverified.</strong> Report success describes evidence completeness, not an accuracy threshold. Overall accuracy covers application benchmarks. Microbenchmarks have a separate section.</div>'
    body += (
        '<p id="current-main">Published snapshot of main: <code>'
        + esc(index["main_head"] or "unknown")
        + "</code>.</p>"
    )
    latest = index["latest_success"]
    body += (
        "<p><strong>Latest successful main report:</strong> "
        + (
            '<a href="runs/' + latest + '/">' + latest[:12] + "</a>"
            if latest
            else "None yet"
        )
        + '</p><h2>Report history</h2><p class="muted">One report per commit, covering the devices that commit declares. Latest 20 ordinary main commits; release reports are retained permanently outside that limit. Failed and incomplete results stay visible.</p><div class="table"><table><thead><tr><th>Commit</th><th>Full Evaluation</th><th>Report</th><th>Devices at this commit</th><th>Mean application error (qualified/total)</th><th>Permanent releases</th></tr></thead><tbody>'
    )
    for e in index["entries"]:
        # Before its first rendering an entry has only its devices' own results.
        status = e.get("artifact_status") or ("fail" if any(d["artifact_status"] == "fail" for d in e["devices"].values()) else "pass")
        body += (
            '<tr><td><a href="runs/'
            + e["commit"]
            + '/">'
            + e["commit"][:12]
            + "</a></td><td>"
            + esc(e.get("evaluation_conclusion", "not checked"))
            + '</td><td><span class="badge '
            + esc(status)
            + '">'
            + esc(status)
            + "</span></td><td>"
            + esc(device_coverage(e))
            + "</td>"
        )
        summary = e.get("benchmark_summary", {})
        coverage = f'{summary.get("comparable", "—")}/{summary.get("required", "—")}'
        body += "<td>" + number(summary.get("head", {}).get("mean_error_percent"), "%") + " (" + coverage + ")</td>"
        body += (
            "<td>"
            + esc(", ".join(r["tag"] for r in e["releases"]) or "—")
            + "</td></tr>"
        )
    body += '</tbody></table></div><h2>Benchmark accuracy trends</h2><p class="muted">Each chart uses one set of contributing devices, references, formula and hardware profiles. The mean pools every qualified device and application input of a commit. Points are ordered by main ancestry, oldest first; gaps and coverage must be read alongside the reports.</p><div class="charts">'
    groups = {}
    for e in reversed(index["entries"]):
        if e.get("series") and e["main"]:
            groups.setdefault(e["series"], []).append(e)
    for identity, entries in groups.items():
        points = []
        for i, entry in enumerate(entries, 1):
            mean = entry.get("benchmark_summary", {}).get("head", {}).get("mean_error_percent")
            if mean is not None:
                points.append((i, mean, f"{entry['commit'][:12]}: {mean:.4f}%"))
        series = [("Applications", points, "#087e8b")]
        body += chart(
            "Series " + identity[:12], series, "Main report order", "Mean error (%)"
        )
    body += (
        '</div><p><a href="index.json">Download history index JSON</a> · <a href="https://github.com/'
        + REPO
        + '/actions/workflows/evaluation.yml">All evaluation runs</a></p>'
    )
    return page("Poros calibration dashboard", body, POINT_TIPS + STATUS_SCRIPT)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    sub = p.add_subparsers(dest="command", required=True)
    b = sub.add_parser("bundle")
    b.add_argument("--report", required=True)
    b.add_argument("--source", default=".")
    b.add_argument("--output", required=True)
    b.add_argument("--commit", required=True)
    m = sub.add_parser("memory-preview")
    m.add_argument("--report", required=True)
    m.add_argument("--source", default=".")
    m.add_argument("--output", required=True)
    f = sub.add_parser("failure")
    f.add_argument("--output", required=True)
    f.add_argument("--commit", required=True)
    f.add_argument("--message", required=True)
    f.add_argument("--device", help="modeled device the failed run was for (default: the default device)")
    f.add_argument("--base-commit", help="base revision the failed run had resolved, if any")
    reuse = sub.add_parser("reusable")
    reuse.add_argument("--bundle", required=True)
    reuse.add_argument("--commit", required=True)
    serve = sub.add_parser("serve", help="Serve a live snapshot with the canonical report renderer")
    serve.add_argument("--snapshot", required=True, action="append",
                       help="one device's snapshot; repeat with other devices of the same commit for the live commit report")
    serve.add_argument("--bundle", help="Optional complete validated bundle to transition to (single snapshot only)")
    serve.add_argument("--port", type=int, default=18763)
    a = p.parse_args()
    if a.command == "memory-preview":
        memory_preview(a.report, a.source, a.output)
        return
    if a.command == "serve":
        import accuracy_live
        accuracy_live.serve(a.snapshot, a.bundle, a.port)
        return
    if a.command == "reusable":
        raise SystemExit(0 if reusable_bundle(a.bundle, a.commit) else 1)
    if a.command == "bundle":
        bundle(a.report, a.source, a.output, a.commit)
    else:
        failure(a.output, a.commit, a.message, a.device, a.base_commit)


if __name__ == "__main__":
    main()
