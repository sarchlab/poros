#!/usr/bin/env python3
"""Frozen 240-case compute measurement cohort, shared by capture/report/publication.

This adds evidence without changing the original 24-case collector or reference.
Native timings are direct-child Linux ARM64 user time; QEMU supplies correctness
hashes only. Output witnesses are stored once, outside the timed child interval.
"""
import argparse
import base64
import copy
from datetime import datetime
import re
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path
import platform
import statistics
import shutil
import subprocess
import time

import native_usertime as native
import workload_contract as legacy

ROOT = Path(__file__).resolve().parents[1]
REFERENCE = "evaluation/usertime/compute-linuxvm-v1"
SPLITS = ("calibration", "held_out", "validation-legacy")
PROTOCOL = "compute-child-user-v1"
QUALITY_POLICY = {"nonpositive": "reject-any", "precision_floor_seconds": .0001, "relative_p10_p90_limit": .2}
SUPPORT = ("scripts/compute_usertime.py", "scripts/native_usertime.py",
           "scripts/workload_contract.py", "scripts/benchmark-differential.py")
spec = importlib.util.spec_from_file_location("benchmark_differential", Path(__file__).with_name("benchmark-differential.py"))
differential = importlib.util.module_from_spec(spec)
spec.loader.exec_module(differential)


def read(path):
    return json.loads(Path(path).read_text())


def write(path, value):
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    Path(path).write_text(json.dumps(value, indent=2, allow_nan=False) + "\n")


def object_digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()).hexdigest()


def relative(name):
    path = Path(name)
    if not isinstance(name, str) or path.is_absolute() or not path.parts or str(path) != name or any(x in ("..", ".") for x in name.split("/")) or "\\" in name:
        raise ValueError("unsafe evidence path")
    return path


def contract(root=ROOT):
    root = Path(root)
    manifest = read(root / "tests/benchmarks/manifest.json")
    build = read(root / "tests/benchmarks/provenance.json")
    workloads = differential.validate_inventory(root, manifest, build)
    old = legacy.load_manifest(root)
    roles = read(root / "evaluation/workloads.json")
    if roles["split_version"] != "m0-v1":
        raise ValueError("unexpected compute calibration split")
    roles = {w["id"]: w["role"] for w in roles["workloads"]}
    for w in workloads:
        if roles[w["name"]] != ("calibration" if w["name"] in ("atax", "gemm") else "validation-legacy"):
            raise ValueError("compute migration must preserve frozen calibration roles")
    return contract_data(manifest, build, old["profile"])


def contract_data(manifest, build, profile):
    workloads = differential.validate_inventory(None, manifest, build, verify_files=False)
    cases, groups = [], []
    for w in workloads:
        name = w["name"]
        role = "calibration" if name in ("atax", "gemm") else "validation-legacy"
        groups.append(dict(id=name, axis="input_point", split=role, points=list(range(1, 7)),
                           labels=[", ".join(f'{d["name"]}={a}' for d, a in zip(w["dimensions"], c["args"])) for c in w["cases"]]))
        for c in w["cases"]:
            cases.append(dict(id=name + "-" + c["id"], oracle_id=name + "/" + c["id"],
                              workload=name, point=int(c["id"]), axis="input_point", split=role,
                              dimensions={d["name"]: int(a) for d, a in zip(w["dimensions"], c["args"])},
                              args=["program", *c["args"]], env=[], file=w["binary"],
                              sha256=build["files"][w["binary"]], outputs=copy.deepcopy(c["outputs"]),
                              model="ooo", frequency_hz=3490000000))
    return dict(schema=1, suite=PROTOCOL, profile=profile, build=build, program_manifest=manifest,
                program_manifest_sha256=build["files"]["tests/benchmarks/manifest.json"],
                workloads=groups, cases=cases)


def validate_oracle(oracle, declared):
    if oracle.get("schema") != 1 or oracle.get("build") != declared["build"] or oracle.get("manifest_sha256") != declared["program_manifest_sha256"]:
        raise ValueError("independent oracle program provenance mismatch")
    if not re.fullmatch(r"[0-9a-f]{40}", oracle.get("validation_commit", "")) or oracle.get("validation_clean") is not True or not legacy.SHA256.fullmatch(oracle.get("validation_report_sha256", "")) or not legacy.SHA256.fullmatch(oracle.get("validation_harness_sha256", "")):
        raise ValueError("incomplete independent oracle validation identity")
    rows = oracle["cases"]
    if [r["id"] for r in rows] != [c["oracle_id"] for c in declared["cases"]]:
        raise ValueError("independent oracle needs all 240 ordered cases")
    for row, case in zip(rows, declared["cases"]):
        if row["status"] != "pass" or row["args"] != case["args"][1:] or row["binary_sha256"] != case["sha256"]:
            raise ValueError("independent oracle input/result mismatch")
        if not legacy.SHA256.fullmatch(row["output_sha256"]) or row["native_sha256"] != row["output_sha256"] or row["qemu_sha256"] != row["output_sha256"]:
            raise ValueError("native/QEMU oracle disagreement")
        length = sum(o["count"] * (8 if o["type"] == "double" else 4) for o in case["outputs"])
        if row["output_bytes"] != length:
            raise ValueError("independent oracle output size mismatch")


def compact_oracle(report, declared):
    if report["acceptance_status"] != "pass" or report["status"] != "pass" or report["expected_cases"] != 240 or report["provenance"]["git_dirty"]:
        raise ValueError("oracle must be complete independently validated clean evidence")
    if any(report["provenance"]["sources"].get(name) != sha for name, sha in declared["build"]["files"].items()):
        raise ValueError("oracle source/build identities disagree")
    oracle = dict(schema=1, validation_commit=report["provenance"]["git_sha"], validation_clean=True,
                  validation_report_sha256=hashlib.sha256((json.dumps(report, indent=2, allow_nan=False) + "\n").encode()).hexdigest(), validation_harness_sha256=report["provenance"]["sources"]["scripts/benchmark-differential.py"],
                  qemu=report["qemu"], platform=report["platform"], build=report["build"],
                  manifest_sha256=report["manifest_sha256"], cases=report["cases"])
    validate_oracle(oracle, declared)
    return oracle


def witness(folder, case, row):
    path = Path(folder) / "outputs" / (row["output_sha256"] + ".bin")
    raw = path.read_bytes()
    differential.check_output(case, raw)
    if len(raw) != row["output_bytes"] or hashlib.sha256(raw).hexdigest() != row["output_sha256"]:
        raise ValueError("full output witness mismatch")
    return raw


def validate_reference(folder, root=ROOT):
    folder, root = Path(folder), Path(root)
    reference, declared, oracle = read(folder / "reference.json"), contract(root), read(folder / "oracle.json")
    validate_oracle(oracle, declared)
    if oracle != compact_oracle(read(folder / "validation-report.json"), declared) or legacy.digest(folder / "validation-report.json") != oracle["validation_report_sha256"] or legacy.digest(folder / "validation-harness.py") != oracle["validation_harness_sha256"]:
        raise ValueError("retained independent validation report/harness mismatch")
    try:
        started = datetime.strptime(reference["started_at_utc"], "%Y-%m-%dT%H:%M:%SZ")
        completed = datetime.strptime(reference["completed_at_utc"], "%Y-%m-%dT%H:%M:%SZ")
    except (KeyError, ValueError):
        raise ValueError("incomplete/invalid measurement timestamps") from None
    if reference.get("quality_policy") != QUALITY_POLICY or not re.fullmatch(r"sha256:[0-9a-f]{64}", reference.get("container_image", "")) or not re.fullmatch(r"[0-9a-f]{40}", reference.get("collection_commit", "")) or reference.get("collection_dirty") is not False:
        raise ValueError("measurement protocol/environment identity mismatch")
    if completed < started or reference.get("precision_floor_seconds") != .0001 or reference.get("retained_samples_per_case") != 21 or reference.get("warmup") != "one complete process per case":
        raise ValueError("measurement quality protocol mismatch")
    if reference.get("schema") != 1 or reference.get("protocol") != PROTOCOL or reference["contract"] != declared:
        raise ValueError("compute reference contract mismatch")
    if reference["oracle_sha256"] != legacy.digest(folder / "oracle.json"):
        raise ValueError("compute oracle evidence mismatch")
    if reference["support_sha256"] != {name: legacy.digest(root / name) for name in SUPPORT}:
        raise ValueError("compute measurement protocol source mismatch")
    if reference["metric"] != "child_user_cpu_seconds" or reference["timer"] != "wait4.ru_utime" or reference["environment"] != "virtualized" or reference["machine"] not in ("aarch64", "arm64"):
        raise ValueError("unexpected measurement scope")
    if [r["id"] for r in reference["cases"]] != [c["id"] for c in declared["cases"]]:
        raise ValueError("missing/reordered compute measurements")
    for case, row, independent in zip(declared["cases"], reference["cases"], oracle["cases"]):
        if row["output_sha256"] != independent["output_sha256"] or row["output_bytes"] != independent["output_bytes"]:
            raise ValueError("reference output differs from independent oracle")
        witness(folder, case, row)
        if len(row["samples"]) != 21:
            raise ValueError("exactly 21 retained samples required")
        for sample in [row["warmup_sample"], *row["samples"]]:
            if sample["exit_status"] != 0 or sample["stderr_bytes"] != 0 or sample["stderr_sha256"] != hashlib.sha256(b"").hexdigest():
                raise ValueError("invalid native process outcome")
            for metric in ("user_seconds", "system_seconds"):
                if type(sample[metric]) not in (int, float) or not math.isfinite(sample[metric]) or sample[metric] < 0:
                    raise ValueError("invalid raw CPU time")
            if sample["stdout_sha256"] != row["output_sha256"] or sample["stdout_bytes"] != row["output_bytes"]:
                raise ValueError("raw sample output mismatch")
        if row["quality"] != native.quality(row["samples"]) or row["median_user_seconds"] != statistics.median(s["user_seconds"] for s in row["samples"]):
            raise ValueError("compute reference quality/statistics mismatch")
    return reference


def capture(args):
    if platform.system() != "Linux" or platform.machine() not in ("aarch64", "arm64"):
        raise ValueError("capture requires direct native Linux ARM64 execution")
    folder = Path(args.output)
    if folder.exists() and any(folder.iterdir()):
        raise ValueError("capture destination must be empty; never overwrite raw evidence")
    declared = contract()
    git = ["git", "-c", "safe.directory=" + str(ROOT), "-C", str(ROOT)]
    commit = subprocess.check_output([*git, "rev-parse", "HEAD"], text=True).strip()
    if subprocess.check_output([*git, "status", "--porcelain"]):
        raise ValueError("formal native capture requires clean committed collector source")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", os.environ.get("POROS_REFERENCE_IMAGE", "")):
        raise ValueError("formal capture requires exact native container image identity")
    validation = read(args.oracle)
    oracle = compact_oracle(validation, declared)
    harness = subprocess.check_output([*git, "show", oracle["validation_commit"] + ":scripts/benchmark-differential.py"])
    if hashlib.sha256(harness).hexdigest() != oracle["validation_harness_sha256"]:
        raise ValueError("independent validation harness revision mismatch")
    folder.mkdir(parents=True, exist_ok=True)
    write(folder / "oracle.json", oracle)
    write(folder / "validation-report.json", validation)
    (folder / "validation-harness.py").write_bytes(harness)
    reference = dict(schema=1, protocol=PROTOCOL, measurement_id=args.measurement_id,
                     collection_commit=commit, collection_dirty=False, quality_policy=QUALITY_POLICY,
                     started_at_utc=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                     metric="child_user_cpu_seconds", timer="wait4.ru_utime", environment="virtualized",
                     hardware=args.hardware, container_image=os.environ.get("POROS_REFERENCE_IMAGE", "unknown"),
                     platform=platform.platform(), machine=platform.machine(), kernel=platform.release(),
                     core_identity="unknown", instantaneous_frequency="unknown", warmup="one complete process per case",
                     retained_samples_per_case=21, precision_floor_seconds=.0001, precision_floor_basis="conservative protocol policy, not measured timer resolution",
                     protocol_notes="21 predetermined samples; no reroll/drop; user/system VM accounting and host isolation unverified",
                     support_sha256={name: legacy.digest(ROOT / name) for name in SUPPORT},
                     oracle_sha256=legacy.digest(folder / "oracle.json"), contract=declared, cases=[])
    (folder / "outputs").mkdir()
    for case, expected in zip(declared["cases"], oracle["cases"]):
        time.sleep(1)
        samples = []
        for i in range(22):
            sample = native.child_usage(ROOT / case["file"], case["args"], {}, timeout=10)
            out, err = (base64.b64decode(sample[k], validate=True) for k in ("stdout_base64", "stderr_base64"))
            differential.check_output(case, out)
            sha = hashlib.sha256(out).hexdigest()
            if sample["exit_status"] != 0 or err or sha != expected["output_sha256"] or len(out) != expected["output_bytes"]:
                raise ValueError("native correctness failed: " + case["id"])
            output = folder / "outputs" / (sha + ".bin")
            if not output.exists():
                output.write_bytes(out)
            samples.append(dict(user_seconds=sample["user_seconds"], system_seconds=sample["system_seconds"],
                                exit_status=sample["exit_status"], stdout_sha256=sha, stdout_bytes=len(out),
                                stderr_sha256=hashlib.sha256(err).hexdigest(), stderr_bytes=len(err)))
        reference["cases"].append(dict(id=case["id"], output_sha256=sha, output_bytes=len(out), warmup_sample=samples[0],
                                       samples=samples[1:], quality=native.quality(samples[1:]),
                                       median_user_seconds=statistics.median(s["user_seconds"] for s in samples[1:])))
        # Partial evidence remains on failure, but cannot validate as complete.
        write(folder / "reference.json", reference)
        print(case["id"], reference["cases"][-1]["quality"], flush=True)
    reference["completed_at_utc"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    write(folder / "reference.json", reference)
    validate_reference(folder)


def expanded_manifest(old, compute):
    return dict(schema=3, suite="whole-program-264-v3", profile=old["profile"],
                workloads=old["workloads"] + compute["workloads"], cases=old["cases"] + compute["cases"],
                original=old, compute=compute)


def validate_expanded(manifest):
    if manifest.get("schema") != 3 or manifest.get("suite") != "whole-program-264-v3":
        raise ValueError("unsupported expanded workload manifest")
    old, compute = manifest["original"], manifest["compute"]
    legacy.validate_manifest(old)
    if compute != contract_data(compute["program_manifest"], compute["build"], old["profile"]):
        raise ValueError("expanded compute input/build contract mismatch")
    groups, cases = compute["workloads"], compute["cases"]
    if compute["suite"] != PROTOCOL or len(groups) != 40 or {g["id"] for g in groups} != differential.EXPECTED:
        raise ValueError("incomplete compute groups")
    expected = [(g, n) for g in groups for n in range(1, 7)]
    if len(cases) != 240 or compute["profile"] != old["profile"]:
        raise ValueError("incomplete compute cases/profile")
    for case, (group, n) in zip(cases, expected):
        role = "calibration" if group["id"] in ("atax", "gemm") else "validation-legacy"
        if group["points"] != list(range(1, 7)) or len(group["labels"]) != 6 or group["split"] != role or group["axis"] != "input_point":
            raise ValueError("invalid compute sweep")
        if case["id"] != f'{group["id"]}-{n}' or case["point"] != n or case["workload"] != group["id"] or case["split"] != role:
            raise ValueError("invalid compute row identity/split")
    if manifest != expanded_manifest(old, compute):
        raise ValueError("expanded inventory differs from cohorts")
    return manifest


def reference_identity(old_sha, compute_sha):
    return object_digest(dict(original24=old_sha, compute240=compute_sha))


def source_paths(root=ROOT):
    declared = contract(root)
    old = legacy.load_manifest(root)
    return set(declared["build"]["files"]) | set(SUPPORT) | {
        "tests/benchmarks/provenance.json", "evaluation/workloads.json", "tests/usertime/manifest.json"
    } | {"tests/usertime/" + name for name in old["sources"]} | {"tests/usertime/" + c["file"] for c in old["cases"]}


def evidence_paths(folder):
    folder = Path(folder)
    reference = validate_reference(folder / "reference", folder / "source")
    return {"source/" + name for name in source_paths(folder / "source")} | {
        "reference/reference.json", "reference/oracle.json", "reference/validation-report.json", "reference/validation-harness.py"
    } | {"reference/outputs/" + row["output_sha256"] + ".bin" for row in reference["cases"]}


def copy_evidence(source, destination):
    source, destination = Path(source), Path(destination)
    reference = validate_reference(source / REFERENCE, source)
    paths = {"source/" + name: source / relative(name) for name in source_paths(source)}
    for name in {"reference.json", "oracle.json", "validation-report.json", "validation-harness.py"} | {"outputs/" + row["output_sha256"] + ".bin" for row in reference["cases"]}:
        paths["reference/" + name] = source / REFERENCE / relative(name)
    for name, path in paths.items():
        target = destination / relative(name)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, target)
    if set(paths) != evidence_paths(destination):
        raise ValueError("incomplete compute evidence snapshot")
    return paths


def base_declares_cohort(revision, root=ROOT):
    git = ["git", "-C", str(root)]
    subprocess.run([*git, "cat-file", "-e", revision + "^{commit}"], check=True, capture_output=True)
    result = subprocess.run([*git, "show", revision + ":tests/benchmarks/manifest.json"], capture_output=True)
    if result.returncode:
        if revision != "cb965b4d41fc7d52281652c3bce7803cc893cae0":
            raise ValueError("unrecognized base without declared compute ELF capability")
        return False
    provenance = subprocess.check_output([*git, "show", revision + ":tests/benchmarks/provenance.json"])
    differential.validate_inventory(None, json.loads(result.stdout), json.loads(provenance), verify_files=False)
    return True


def summaries(cases):
    result = {}
    for split in SPLITS:
        rows = [r for r in cases if r["split"] == split]
        summary = dict(required=len(rows), comparable=sum("error_percent" in r["head"] for r in rows))
        for label in ("base", "head"):
            values = [r[label]["error_percent"] for r in rows if "error_percent" in r.get(label, {})]
            if values:
                summary[label] = dict(mean_error_percent=statistics.mean(values), maximum_error_percent=max(values))
        result[split] = summary
    return result


def extend_report(original, args, execute, validate_timing, error_percent, source_state):
    report = copy.deepcopy(original)
    report.update(schema=3, artifact_status="fail", original_report=original)
    try:
        folder = Path(args.compute_reference)
        reference = validate_reference(folder)
        declared = reference["contract"]
        report["workload_manifest"] = expanded_manifest(original["workload_manifest"], declared)
        report["reference_files"] = dict(original24=original["reference_sha256"], compute240=legacy.digest(folder / "reference.json"))
        report["reference_sha256"] = reference_identity(**dict(old_sha=report["reference_files"]["original24"], compute_sha=report["reference_files"]["compute240"]))
        report["compute_reference"] = {key: reference[key] for key in ("measurement_id", "environment", "hardware", "platform", "core_identity", "instantaneous_frequency", "completed_at_utc")}
        available = base_declares_cohort(args.base_commit)
        if available and not args.base:
            raise ValueError("declared compute base cannot be unavailable")
        report["compute_base_status"] = "available" if available else "compute_elf_unavailable"
        for case, measured in zip(declared["cases"], reference["cases"]):
            golden = witness(folder, case, measured)
            functional = execute(args.head, case, mode="functional", expected_output=golden)
            row = dict(id=case["id"], split=case["split"], status="unverified", functional_status="pass",
                       reference_user_seconds=measured["median_user_seconds"], reference_quality=measured["quality"],
                       base_status=report["compute_base_status"],
                       reason="Linux VM user accounting; core/frequency unknown; no accuracy threshold approved; validation-legacy is not an independent historical holdout")
            for label, binary, revision in (("head", args.head, original["head"]["commit"]), ("base", args.base if available else None, args.base_commit)):
                if not binary:
                    continue
                result = execute(binary, case, expected_output=golden)
                validate_timing(result, case)
                if result["build"].get("vcs.revision") != revision or result["build"].get("vcs.modified") != "false":
                    raise ValueError(label + " binary provenance mismatch")
                if label == "head" and any(result[key] != functional[key] for key in ("instructions", "stdout_base64", "build")):
                    raise ValueError("compute functional/timing architectural mismatch")
                timing = result["timing"]
                row[label] = dict(predicted_user_seconds=timing["predicted_user_seconds"], instructions=result["instructions"],
                                  cycles=timing["user_cycles"], configuration=timing["configuration"], cache_config=timing["cache_config"])
                if measured["quality"] == "usable":
                    row[label]["error_percent"] = error_percent(timing["predicted_user_seconds"], measured["median_user_seconds"])
                    row[label]["direction"] = "slower" if timing["predicted_user_seconds"] > measured["median_user_seconds"] else "faster" if timing["predicted_user_seconds"] < measured["median_user_seconds"] else "equal"
            if "base" in row:
                if any(row["base"][key] != row["head"][key] for key in ("configuration", "cache_config")):
                    raise ValueError("compute base/head profile mismatch")
                if "error_percent" in row["head"]:
                    row["error_change_percentage_points"] = row["head"]["error_percent"] - row["base"]["error_percent"]
            report["cases"].append(row)
            print("accuracy", case["id"], measured["quality"], flush=True)
        report["summary"] = summaries(report["cases"])
        if source_state() != original["head"] or legacy.digest(args.head) != original["head_binary_sha256"]:
            raise ValueError("sources or simulator changed during expanded comparison")
        if args.base and legacy.digest(args.base) != original["base_binary_sha256"]:
            raise ValueError("base simulator changed during expanded comparison")
        if legacy.digest(folder / "reference.json") != report["reference_files"]["compute240"] or legacy.digest(args.reference) != report["reference_files"]["original24"]:
            raise ValueError("reference changed during expanded comparison")
        validate_reference(folder)
        report["artifact_status"] = "pass"
    except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError) as error:
        report["error"] = str(error)
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--oracle", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--measurement-id", required=True)
    parser.add_argument("--hardware", required=True)
    capture(parser.parse_args())
