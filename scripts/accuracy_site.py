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
import subprocess
from workload_contract import LEGACY_IDS, case_ids, validate_manifest, manifest_path
import compute_usertime as compute
import performance_reference as performance
from performance_compare import MICRO_WORKLOADS, is_micro_case, benchmark_summary

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


def check_report(report, commit):
    if report.get("schema") not in (1, 2, 3, 4) or report.get("artifact_status") not in (
        "pass",
        "fail",
    ):
        raise ValueError("unsupported report")
    if report.get("accuracy_status") != "unverified":
        raise ValueError("unsupported accuracy verdict")
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
    inventory = report_manifest(report)
    performance = report.get("schema") == 4 or inventory.get("suite") == "whole-program-performance-six-point-v1" or (report.get("schema") == 3 and inventory["compute"]["suite"] == compute.PROTOCOL)
    if performance:
        capability = compute.validate_base_capability(report["base_capability"])
        if report.get("schema") == 3 and report["original_report"]["base_capability"] != capability:
            raise ValueError("base capability differs between cohorts")
        for row, case in zip(report["cases"], inventory["cases"]):
            expected_status = compute.case_base_status(case, capability)
            if row.get("mapped_image_bytes") != case["mapped_image_bytes"] or row.get("base_status") != expected_status or ("base" in row) != (expected_status == "available"):
                raise ValueError("missing or unjustified per-case base result")
            if "base" not in row and "error_change_percentage_points" in row:
                raise ValueError("unpaired error change")
    if tuple(c["id"] for c in report["cases"]) != case_ids(inventory):
        raise ValueError("missing required cases")
    for row, declared in zip(report["cases"], inventory["cases"]):
        if (
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
        if row["reference_quality"] not in (
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
    return read(path / "reference.json").get("completed_at_utc")


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
        shutil.copyfile(source / performance.PATH, output / "reference.json")
        names.add("reference.json")
        if FRONTEND_REFERENCE in report["head"]["files"]:
            shutil.copyfile(source / FRONTEND_REFERENCE, output / "frontend.json")
            names.add("frontend.json")
    write(output / "bundle.json", dict(schema=SCHEMA, commit=commit,
          run_id=os.environ.get("GITHUB_RUN_ID"), run_attempt=os.environ.get("GITHUB_RUN_ATTEMPT"),
          files={name: digest(output / name) for name in names}))
    validate_bundle(output, commit)
    (output / "index.html").write_text(detail(report, dict(commit=commit, measurement_completed_at=measurement_completed(report, output) if report["artifact_status"] == "pass" else None), preview=True, frontend=read(output / "frontend.json") if "frontend.json" in names else None))


def validate_compact_bundle(path, commit, manifest):
    report = read(regular(path / "report.json"))
    names = {"report.json", "report.md"} | ({"reference.json"} if report["artifact_status"] == "pass" else set())
    if report["artifact_status"] == "pass" and FRONTEND_REFERENCE in report["head"]["files"]:
        if digest(regular(path / "frontend.json")) != report["head"]["files"][FRONTEND_REFERENCE]:
            raise ValueError("frontend diagnostic differs from tested source")
        names.add("frontend.json")
    if set(manifest["files"]) != names:
        raise ValueError("compact bundle membership mismatch")
    for name, expected in manifest["files"].items():
        if digest(regular(path / name)) != expected:
            raise ValueError("compact bundle hash mismatch")
    check_report(report, commit)
    if report["artifact_status"] != "pass":
        return report, manifest
    declared = report_manifest(report)
    measured = performance.validate(read(path / "reference.json"), declared)
    if digest(path / "reference.json") != report["reference_sha256"] or report["head"]["files"].get(performance.PATH) != report["reference_sha256"]:
        raise ValueError("compact reference differs from tested source")
    metadata = {key: value for key, value in measured.items() if key not in ("cases", "input_sources", "collector_sources")}
    if report["reference"] != metadata:
        raise ValueError("displayed compact provenance mismatch")
    if any(report["head"]["files"].get(name) != sha for name, sha in measured["input_sources"].items()):
        raise ValueError("compact input differs from tested source/ELF")
    for row, observed, case in zip(report["cases"], measured["cases"], declared["cases"]):
        if row["reference_quality"] != observed["quality"] or row["reference_user_seconds"] != observed["median_user_seconds"] or row.get("reference_p10_seconds") != observed["p10_user_seconds"] or row.get("reference_p90_seconds") != observed["p90_user_seconds"]:
            raise ValueError("compact plotted statistics mismatch")
        for label in ("head", "base"):
            if label not in row:
                continue
            value = row[label]
            if type(value["cycles"]) is not int or type(value["instructions"]) is not int or any(value[k] != declared["profile"][k] for k in ("configuration", "cache_config")):
                raise ValueError("compact report timing profile/count mismatch")
            seconds, real = value["predicted_user_seconds"], observed["median_user_seconds"]
            if not math.isclose(seconds, value["cycles"] / case["frequency_hz"], rel_tol=1e-12):
                raise ValueError("compact timing frequency mismatch")
            if observed["quality"] == "usable":
                error = 100 * abs(seconds-real) / min(seconds, real)
                if not math.isclose(value.get("error_percent", -1), error, rel_tol=1e-12, abs_tol=1e-12) or value.get("direction") != ("slower" if seconds > real else "faster" if seconds < real else "equal"):
                    raise ValueError("compact error formula/direction mismatch")
            elif "error_percent" in value or "direction" in value:
                raise ValueError("unqualified compact reference has accuracy claim")
        if "base" in row and observed["quality"] == "usable":
            if not math.isclose(row.get("error_change_percentage_points", float("nan")), row["head"]["error_percent"]-row["base"]["error_percent"], rel_tol=1e-12, abs_tol=1e-12):
                raise ValueError("compact paired error change mismatch")
        elif "error_change_percentage_points" in row:
            raise ValueError("unpaired compact error change")
    if report["summary"] != compute.summaries(report["cases"], True):
        raise ValueError("compact summary mismatch")
    return report, manifest



def reusable_bundle(path, commit):
    try:
        report, _ = validate_bundle(path, commit)
        return report["artifact_status"] == "pass"
    except (OSError, ValueError, KeyError, TypeError):
        return False


def failure(output, commit, message):
    output = Path(output)
    output.mkdir(parents=True, exist_ok=True)
    write(
        output / "report.json",
        {
            "schema": 1,
            "artifact_status": "fail",
            "accuracy_status": "unverified",
            "head_commit": sha(commit),
            "cases": [],
            "error": message,
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


def chart(title, series, xlabel, ylabel, scatter=False, x_ticks=None, y_ticks=None,
          ideal=False, note="", y_bands=()):
    """Plot the data domain; guide lines never contribute observations."""
    points = [(x, y) for _, values, _ in series for x, y, _ in values]
    if not points:
        return f"<figure><figcaption>{esc(title)}</figcaption><p>No comparable points available.</p></figure>"
    xmin, xmax = min(x for x, _ in points), max(x for x, _ in points)
    ymin, ymax = (y_ticks[0][0], y_ticks[-1][0]) if y_ticks else (0, max(y for _, y in points) or 1)

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
    marks = {}
    for name, values, color in series:
        if not scatter:
            coords = " ".join(f"{px(x):.2f},{py(y):.2f}" for x, y, _ in values)
            body += f'<polyline fill="none" stroke="{color}" points="{coords}"/>'
        for x, y, label in values:
            # Identical base/head observations must both be readable on hover.
            marks.setdefault((x, y), []).append((color, name + ": " + label))
    for (x, y), entries in marks.items():
        tip = esc(chr(10).join(label for _, label in entries))
        body += (f'<circle cx="{px(x):.3f}" cy="{py(y):.3f}" r="3.2" fill="{entries[-1][0]}" '
                 f'stroke="transparent" stroke-width="4" tabindex="0" aria-label="{tip}" data-tip="{tip}"/>')
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


def overview_charts(cases):
    # Both views and both statistics use exactly the same qualified head cases.
    points = [(c["reference_user_seconds"] * 1000, c["head"]["predicted_user_seconds"] * 1000, c["id"])
              for c in cases if c.get("reference_quality") == "usable" and c.get("head")
              and c["reference_user_seconds"] > 0 and c["head"]["predicted_user_seconds"] > 0]
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
    note = (f"<p><strong>Kendall’s τ-b = {tau}</strong><br>"
            f"<strong>Pairs ordered correctly: {pair}</strong><br>"
            f"Pearson correlation (r) = {pearson}</p>"
            f"<p class='muted'>{n} qualified cases. The percentage compares pairs with distinct measured times.</p>")
    comparison = chart("Qualified reference vs. prediction", [("head", scatter, "#087e8b")],
                       "Reference user time (ms)", "Predicted user time (ms)", scatter=True, ideal=True, note=note)
    return '<h2 id="overall">Overall benchmark accuracy</h2><div class="charts">' + s_curve + comparison + "</div>"


def frontend_table(reference):
    labels = {"nop": "NOP stream", "add": "Eight independent ADD chains"}
    rows = []
    for loop, label in labels.items():
        seconds = reference["loops"][loop]
        rate = reference["instructions_per_trial"] / seconds
        rows.append(f"<tr><td>{label}</td><td>{seconds*1000:.3f}</td>"
                    f"<td>{rate/1e9:.2f}</td><td>{rate/reference['assumed_hz']:.2f}</td></tr>")
    return (f'<h3>Front-end throughput on {esc(reference["hardware"])}</h3>'
            f'<p>{esc(reference["os"])} · {esc(reference["date"])} · median of {reference["trials"]} trials '
            f'of {reference["instructions_per_trial"]:,} loop instructions. Estimated IPC uses '
            f'{reference["assumed_hz"]/1e9:g} GHz and thread CPU time. Core and operating frequency were unobserved.</p>'
            '<div class="table"><table><thead><tr><th>Native loop</th><th>Thread time (ms)</th>'
            '<th>Billion instructions/s</th><th>Estimated IPC</th></tr></thead><tbody>'
            + "".join(rows) + '</tbody></table></div><p><a href="frontend.json">Native reference data</a></p>')


def timing_curves(groups, cases):
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
        body += chart(
            workload.upper() if workload == "mvt" else workload.title(),
            series,
            xlabel,
            "User time (ms)",
            x_ticks=ticks,
        )
    body += "</div>"
    return body


def timing_table(cases, ids):
    body = '<p class="muted">Error = 100 × |sim − reference| / min(sim, reference). Positive Δ means a larger error. No offset fitting or anchor removal. Times are milliseconds.</p><div class="table"><table><thead><tr><th>Case / split</th><th>Functional</th><th>Reference</th><th>Quality</th><th>Base</th><th>Head</th><th>Base error</th><th>Head error</th><th>Direction</th><th>Δ pp</th></tr></thead><tbody>'
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
            case + " / " + c["split"],
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


def detail(report, entry, preview=False, frontend=None):
    commit = entry["commit"]
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
    body += '<div class="notice"><strong>Hardware accuracy: unverified.</strong> Current reference is an Apple M2 host running a Linux ARM64 VM; core identity and instantaneous frequency are unknown. No accuracy threshold has been approved. A successful report is not a validated hardware model.</div>'
    cases = report.get("cases", [])
    benchmarks = [c for c in cases if not is_micro_case(c)]
    micros = [c for c in cases if is_micro_case(c)]
    declared = report_manifest(report)["cases"] if report["artifact_status"] == "pass" else cases
    benchmark_ids = [c["id"] for c in declared if not is_micro_case(c)]
    micro_ids = [c["id"] for c in declared if is_micro_case(c)]
    usable = sum(c.get("reference_quality") == "usable" for c in benchmarks)
    body += (
        '<div class="cards">'
        + "".join(
            '<div class="card">'
            + esc(label)
            + "<strong>"
            + esc(value)
            + "</strong></div>"
            for label, value in [
                ("Report artifact", report["artifact_status"]),
                ("Full Evaluation", entry.get("evaluation_conclusion", "not checked")),
                (
                    "Usable benchmark references",
                    f"{usable} / {len(benchmark_ids) if report.get('artifact_status') == 'pass' else '—'}",
                ),
                ("Hardware accuracy", report["accuracy_status"]),
            ]
        )
        + "</div>"
    )
    if report["artifact_status"] == "fail" and not report.get("head", {}).get(
        "commit", report.get("head_commit")
    ):
        body += '<p class="notice">This failure occurred before source identity was recorded. The bundle identifies the requested target; it does not certify an executed binary or measured source.</p>'
    if report.get("error"):
        body += '<p class="notice">' + esc(report["error"]) + "</p>"
    body += '<nav class="downloads"><a href="#overall">Benchmark accuracy</a><a href="#microbenchmarks">Microbenchmarks</a></nav>'
    body += overview_charts(benchmarks)
    benchmark_stats = benchmark_summary(report)
    body += '<h2>Calibration and held-out coverage</h2><div class="cards">'
    for split in (compute.SPLITS if report.get("schema") in (3, 4) else ("calibration", "held_out")):
        summary = benchmark_stats.get(split, {})
        body += (
            '<div class="card">'
            + esc(split)
            + "<strong>"
            + esc(f'{summary.get("comparable", "—")} / {summary.get("required", "—")}')
            + "</strong>"
            + ("<p>Head comparable: " + esc(summary["head_comparable"]) + "; base comparable: " + esc(summary["base_comparable"]) + ". Means use their respective coverage.</p>" if "base_comparable" in summary else "")
            + "<p>Mean head error: "
            + number(summary.get("head", {}).get("mean_error_percent"), "%")
            + "<br>Maximum: "
            + number(summary.get("head", {}).get("maximum_error_percent"), "%")
            + "</p></div>"
        )
    body += '</div><h2>Benchmark timing curves</h2>'
    groups = (
        report_manifest(report).get(
            "workloads",
            [dict(id=w, axis="iterations") for w in ("integer", "memory", "branch")],
        )
        if report["artifact_status"] == "pass"
        else []
    )
    body += timing_curves([g for g in groups if g["id"] not in MICRO_WORKLOADS], benchmarks)
    body += '<h2>Benchmark timing table</h2>' + timing_table(benchmarks, benchmark_ids)
    body += '<section id="microbenchmarks"><h2>Microbenchmarks</h2>'
    body += '<p>Hardware diagnostics are reported separately from benchmark accuracy.</p>'
    if micro_ids:
        micro_usable = sum(c.get("reference_quality") == "usable" for c in micros)
        micro_required = len(micro_ids) if report["artifact_status"] == "pass" else "—"
        body += f"<p>Usable microbenchmark references: {micro_usable} / {micro_required}</p>"
    if frontend:
        body += frontend_table(frontend)
    body += timing_curves([g for g in groups if g["id"] in MICRO_WORKLOADS], micros)
    if report.get("schema") == 1:
        body += "<p>Historical memory sweep: 64 KiB fixed working set, varying iterations.</p>"
    elif report["artifact_status"] == "pass":
        accesses = next(g["iterations"] for g in report_manifest(report)["workloads"] if g["id"] == "memory")
        body += f"<p>Memory x axis uses log₂ spacing with actual byte-size labels; times are untransformed. Memory: {accesses:,} accesses per point, plus initialization of the declared working set. Cache-boundary positions describe the configured model, not validated hardware capacities.</p>"
    body += timing_table(micros, micro_ids) + '</section>'
    body += (
        "<h2>Evidence and provenance</h2><p>Measurement completed (UTC): "
        + esc(entry.get("measurement_completed_at") or "unavailable")
        + '</p><div class="downloads"><a href="report.json">Report JSON</a><a href="report.md">Markdown report</a><a href="bundle.json">Evidence hashes</a>'
    )
    if report["artifact_status"] == "pass":
        if report["schema"] == 4:
            body += '<a href="reference.json">Plotting data and native output identities</a>'
            collection = report["reference"]["collection_commit"]
            body += '<a href="https://github.com/' + REPO + '/tree/' + collection + '">Exact native collection source</a>'
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
                    "formula": report.get("formula"),
                    "reference": report.get("reference"),
                    "compute_reference": report.get("compute_reference"),
                    "compute_base_status": report.get("compute_base_status"),
                    "configuration": cases[0].get("head", {}) if cases else None,
                },
                indent=2,
            )
        )
        + "</pre></details>"
    )
    return page("Poros accuracy · " + commit[:12], body, POINT_TIPS)


def series_key(report):
    if report["artifact_status"] != "pass":
        return None
    profiles = [
        {k: c["head"][k] for k in ("configuration", "cache_config")}
        for c in report["cases"]
    ]
    return hashlib.sha256(
        json.dumps(
            [report["formula"], report["reference_sha256"], profiles], sort_keys=True
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


def archive(site, bundle_path, metadata, main_order, keep=20):
    """Update only our subtree. Call in a disposable checkout, then commit atomically."""
    site = Path(site)
    root = site / "calibration"
    if any(
        (site / name).is_symlink()
        for name in ("calibration", "index.html", ".nojekyll")
    ) or any(p.is_symlink() for p in root.rglob("*")):
        raise ValueError("archive must not contain symlinks")
    commit = sha(metadata["commit"])
    report, bundle_manifest = validate_bundle(bundle_path, commit)
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
    index = (
        read(index_path) if index_path.exists() else {"schema": SCHEMA, "entries": []}
    )
    if index["schema"] != SCHEMA:
        raise ValueError("unsupported archive schema")
    entries = {sha(e["commit"]): e for e in index["entries"]}
    for e in entries.values():
        validate_run_urls(e)
        if e["artifact_status"] not in ("pass", "fail") or "run_url" not in e:
            raise ValueError("invalid retained entry")
    if metadata["kind"] == "release":
        for e in entries.values():
            if e["commit"] != commit and any(
                r["id"] == metadata["release"]["id"] for r in e["releases"]
            ):
                raise ValueError(
                    "a permanently pinned release cannot move to another commit"
                )
    existing = entries.get(commit)
    if existing and existing["artifact_status"] == "pass":
        # Reuse the original measured report, including its original comparison base.
        report = read(root / "runs" / commit / "report.json")
        entry = existing
        if metadata["kind"] == "main":
            entry["main"] = True
    else:
        destination = root / "runs" / commit
        if destination.exists():
            shutil.rmtree(destination)
        destination.mkdir(parents=True)
        for name in bundle_manifest["files"]:
            target = destination / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(Path(bundle_path) / name, target)
        shutil.copyfile(Path(bundle_path) / "bundle.json", destination / "bundle.json")
        entry = {
            **metadata,
            "main": metadata["kind"] == "main" or bool(existing and existing["main"]),
            "artifact_status": report["artifact_status"],
            "accuracy_status": report["accuracy_status"],
            "series": series_key(report),
            "summary": report.get("summary", {}),
            "releases": existing["releases"] if existing else [],
            "evidence_run_url": metadata.get("evidence_run_url", metadata["run_url"]),
            "evidence_files": sorted(bundle_manifest["files"]),
            "measurement_completed_at": (
                measurement_completed(report, Path(bundle_path))
                if report["artifact_status"] == "pass"
                else None
            ),
        }
        for field in EVALUATION_FIELDS:
            entry.pop(field, None)
            if existing and field in existing:
                entry[field] = existing[field]
        entries[commit] = entry
    entry.pop("successful_evaluation", None)
    merge_evaluation(entry, metadata)
    if metadata["kind"] == "main":
        if (int(metadata.get("run_id", 0)), int(metadata.get("run_attempt", 0))) >= (
            int(entry.get("run_id", 0)),
            int(entry.get("run_attempt", 0)),
        ):
            for key in ("conclusion", "run_id", "run_attempt", "run_url"):
                if key in metadata:
                    entry[key] = metadata[key]
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
            shutil.rmtree(root / "runs" / sha(old))
            del entries[old]
    ordered = sorted(
        entries.values(),
        key=lambda e: (positions.get(e["commit"], len(positions)), e["commit"]),
    )
    success = [
        e
        for e in ordered
        if e["main"]
        and e["commit"] in positions
        and e["artifact_status"] == "pass"
        and e.get("evaluation_conclusion") == "success"
    ]
    index = {
        "schema": SCHEMA,
        "main_head": main_order[0] if main_order else None,
        "latest_success": success[0]["commit"] if success else None,
        "entries": ordered,
    }
    for e in ordered:
        p = root / "runs" / e["commit"]
        retained_report, retained_manifest = validate_bundle(p, e["commit"])
        e["benchmark_summary"] = benchmark_summary(retained_report)
        write(p / "metadata.json", e)
        frontend = read(p / "frontend.json") if "frontend.json" in retained_manifest["files"] else None
        (p / "index.html").write_text(detail(retained_report, e, frontend=frontend))
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


def home(index):
    body = '<header><div class="eyebrow">Poros / Calibration dashboard</div><h1>Accuracy over time</h1><p class="muted">Whole-program user CPU time · Raw predictions against retained hardware observations</p></header><div class="notice"><strong>Hardware accuracy remains unverified.</strong> Report success describes evidence completeness, not an accuracy threshold. Calibration and held-out cases are reported separately.</div>'
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
        + '</p><h2>Report history</h2><p class="muted">Latest 20 ordinary main commits; release reports are retained permanently outside that limit. Failed and incomplete results stay visible.</p><div class="table"><table><thead><tr><th>Commit</th><th>Full Evaluation</th><th>Report</th><th>Calibration mean</th><th>Held-out mean</th><th>Validation-legacy mean</th><th>Permanent releases</th></tr></thead><tbody>'
    )
    for e in index["entries"]:
        body += (
            '<tr><td><a href="runs/'
            + e["commit"]
            + '/">'
            + e["commit"][:12]
            + "</a></td><td>"
            + esc(e.get("evaluation_conclusion", "not checked"))
            + '</td><td><span class="badge '
            + e["artifact_status"]
            + '">'
            + esc(e["artifact_status"])
            + "</span></td>"
        )
        for split in compute.SPLITS:
            summary = e.get("benchmark_summary", {}).get(split, {})
            coverage = (
                f'{summary.get("comparable", "—")}/{summary.get("required", "—")}'
            )
            body += (
                "<td>"
                + number(summary.get("head", {}).get("mean_error_percent"), "%")
                + " ("
                + coverage
                + ")</td>"
            )
        body += (
            "<td>"
            + esc(", ".join(r["tag"] for r in e["releases"]) or "—")
            + "</td></tr>"
        )
    body += '</tbody></table></div><h2>Benchmark accuracy trends</h2><p class="muted">Each chart uses one reference, formula and hardware profile. Points are ordered by main ancestry, oldest first; gaps and coverage must be read alongside the reports.</p><div class="charts">'
    groups = {}
    for e in reversed(index["entries"]):
        if e.get("series") and e["main"]:
            groups.setdefault(e["series"], []).append(e)
    for identity, entries in groups.items():
        series = []
        for split, color in (("calibration", "#087e8b"), ("held_out", "#b17e26"), ("validation-legacy", "#7657a1")):
            values = [
                (
                    i + 1,
                    e.get("benchmark_summary", {})
                    .get(split, {})
                    .get("head", {})
                    .get("mean_error_percent"),
                    e["commit"][:12],
                )
                for i, e in enumerate(entries)
            ]
            series.append(
                (
                    split,
                    [(x, y, f"{label}: {y:.4f}%") for x, y, label in values if y is not None],
                    color,
                )
            )
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
    f = sub.add_parser("failure")
    f.add_argument("--output", required=True)
    f.add_argument("--commit", required=True)
    f.add_argument("--message", required=True)
    reuse = sub.add_parser("reusable")
    reuse.add_argument("--bundle", required=True)
    reuse.add_argument("--commit", required=True)
    a = p.parse_args()
    if a.command == "reusable":
        raise SystemExit(0 if reusable_bundle(a.bundle, a.commit) else 1)
    if a.command == "bundle":
        bundle(a.report, a.source, a.output, a.commit)
    else:
        failure(a.output, a.commit, a.message)


if __name__ == "__main__":
    main()
