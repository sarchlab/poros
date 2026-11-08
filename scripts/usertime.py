#!/usr/bin/env python3
"""Compare whole-program user-time predictions with frozen native measurements."""

from dataclasses import dataclass
import hashlib
import argparse
import copy
import tempfile
import base64
import json
from pathlib import Path
import math
import statistics
import subprocess
import native_usertime as native
from workload_contract import case_ids, check_output, instruction_budget, execution_timeout_seconds
from native_usertime import (
    ROOT,
    digest,
    inventory,
    expected,
    quality,
    child_usage,
    capture,
    measure,
)

FORMULA = "absolute-difference-over-min-v2"


def error_percent(simulated, reference):
    if not all((math.isfinite(x) and x > 0 for x in (simulated, reference))):
        raise ValueError("accuracy needs finite positive times")
    value = 100 * abs(simulated - reference) / min(simulated, reference)
    if not math.isfinite(value):
        raise ValueError("accuracy overflow")
    return value


@dataclass(frozen=True)
class OutputIdentity:
    length: int
    sha256: str

    def check(self, actual):
        if len(actual) != self.length or hashlib.sha256(actual).hexdigest() != self.sha256:
            raise ValueError("complete output identity mismatch")


def execute(poros, case, mode=None, expected_output=None, expected_identity=None, expected_profile=None):
    timeout_seconds = execution_timeout_seconds(case)
    argv = [
        str(poros),
        "elf",
        "--mode",
        mode or case["model"],
        "--frequency-hz",
        str(case["frequency_hz"]),
        "--timeout",
        f"{timeout_seconds // 60}m",
        "--max-instructions",
        str(instruction_budget(case)),
        "--max-output",
        str(max(1 << 20, expected_identity.length if expected_identity is not None else len(expected_output) if expected_output is not None else 16 * int(case["args"][1]) if case.get("workload") == "mvt" else 8)),
        "--argv0",
        case["args"][0],
    ]
    for entry in case["env"]:
        argv += ["--env", entry]
    argv += [str(ROOT / case["file"] if "outputs" in case else ROOT / "tests/usertime" / case["file"])] + case["args"][1:]
    p = subprocess.run(argv, capture_output=True, timeout=timeout_seconds + 50)
    if p.returncode != 0 or p.stderr:
        raise ValueError(
            "timing execution failed: "
            + case["id"]
            + ": "
            + p.stderr.decode(errors="replace")
        )
    r = json.loads(p.stdout)
    return validate_execution(r, case, mode, expected_output, expected_identity, expected_profile)


def validate_execution(r, case, mode=None, expected_output=None, expected_identity=None, expected_profile=None):
    """Apply identical checks to live and recorded simulator executions."""
    if (
        r["binary_sha256"] != case["sha256"]
        or r["args"] != case["args"]
        or r["env"] != case["env"]
    ):
        raise ValueError("executed input provenance mismatch")
    if r["termination"] != "exit" or r["exit_status"] != 0 or r["stderr_base64"]:
        raise ValueError("timing correctness failed: " + case["id"])
    if type(r["instructions"]) is not int or not 0 < r["instructions"] <= instruction_budget(case):
        raise ValueError("instruction count must be a positive integer")
    actual = base64.b64decode(r["stdout_base64"], validate=True)
    if expected_output is not None and expected_identity is not None:
        raise ValueError("ambiguous output contract")
    if "outputs" in case:
        import compute_usertime as compute
        compute.differential.check_output(case, actual)
        if expected_output is None and expected_identity is None:
            raise ValueError("compute execution requires complete-output identity")
    else:
        check_output(case, actual)
    if expected_output is not None and actual != expected_output:
        raise ValueError("compute complete output mismatch: " + case["id"])
    if expected_identity is not None:
        expected_identity.check(actual)
    if mode == "functional":
        if "timing" in r:
            raise ValueError("functional execution unexpectedly contains timing")
        return r
    t = r["timing"]
    profile = inventory()["profile"] if expected_profile is None else expected_profile
    if (
        t["configuration"] != profile["configuration"]
        or t["cache_config"] != profile["cache_config"]
    ):
        raise ValueError("declared model profile mismatch")
    if (
        t["model"] != case["model"]
        or t["assumed_hz"] != case["frequency_hz"]
        or t["retired_instructions"] != r["instructions"]
    ):
        raise ValueError("timing profile mismatch")
    return r


def validate_timing(result, case):
    """Validate predictions even when the native reference is noisy or missing."""
    timing = result["timing"]
    instructions = result["instructions"]
    cycles = timing["user_cycles"]
    retired = timing["retired_instructions"]
    for value in (instructions, cycles, retired):
        if type(value) is not int or value <= 0:
            raise ValueError("timing counts must be positive integers")
    if retired != instructions:
        raise ValueError("retired instruction count mismatch")
    seconds, cpi = (timing["predicted_user_seconds"], timing["cpi"])
    for value in (seconds, cpi):
        if type(value) not in (int, float) or not math.isfinite(value) or value <= 0:
            raise ValueError("timing metrics must be finite and positive")
    if not math.isclose(seconds, cycles / case["frequency_hz"], rel_tol=1e-12):
        raise ValueError("predicted seconds disagree with cycles/frequency")
    if not math.isclose(cpi, cycles / instructions, rel_tol=1e-12):
        raise ValueError("CPI disagrees with cycles/instructions")
    for name in ("L1Hits", "L1Misses", "L2Hits", "L2Misses"):
        value = timing["cache"][name]
        if type(value) is not int or value < 0:
            raise ValueError("invalid cache counter")
    for name in ("branch_predictions", "branch_direction_hits"):
        if type(timing[name]) is not int or timing[name] < 0:
            raise ValueError("invalid branch counter")
    if timing["branch_direction_hits"] > timing["branch_predictions"]:
        raise ValueError("branch hits exceed predictions")


def source_state():
    paths = (
        subprocess.check_output(["git", "ls-files", "-z"], cwd=ROOT)
        .decode()
        .split("\x00")
    )
    return dict(
        commit=subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True
        ).strip(),
        dirty=bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT)),
        files={p: digest(ROOT / p) for p in paths if p},
    )


def compare24(args, executor=None):
    executor = executor or execute
    report = dict(
        schema=2,
        formula=FORMULA,
        artifact_status="fail",
        accuracy_status="unverified",
        cases=[],
    )
    try:
        m = inventory()
        if m.get("schema") != 2:
            raise ValueError(
                "current acceptance requires the schema-2 workload manifest"
            )
        state = source_state()
        if state["dirty"]:
            raise ValueError("accuracy acceptance requires clean committed source")
        report.update(
            workload_manifest=m,
            head=state,
            base_commit=args.base_commit,
            head_binary_sha256=digest(args.head),
            reference_sha256=digest(args.reference),
        )
        reference = json.loads(Path(args.reference).read_text())
        native.validate_capture_metadata(reference)
        report["reference"] = {
            k: reference[k]
            for k in (
                "measurement_id",
                "environment",
                "hardware",
                "platform",
                "core_identity",
                "instantaneous_frequency",
                "harness_sha256",
                "manifest_sha256",
            )
        }
        if reference["harness_sha256"] != digest(native.__file__):
            raise ValueError("measurement harness provenance mismatch")
        if reference["manifest_sha256"] != digest(
            ROOT / native.manifest_path(m)
        ) or tuple((c["id"] for c in reference["cases"])) != case_ids(m):
            raise ValueError("reference inventory/provenance mismatch")
        if reference.get("support_sha256") != {
            "workload_contract.py": digest(ROOT / "scripts/workload_contract.py")
        }:
            raise ValueError("measurement support provenance mismatch")
        if bool(args.base) == bool(args.base_unavailable):
            raise ValueError("provide base binary or explicit unavailable revision")
        if args.base_unavailable:
            # Only this known pre-timing revision may omit timing support.
            # A build/run failure in a capable base never becomes unavailable.
            if args.base_commit != "38031146ec743887070532f237b715fac6847512":
                raise ValueError("unrecognized unavailable base")
            report["base_status"] = "feature_unavailable"
        else:
            report["base_binary_sha256"] = digest(args.base)
            report["base_status"] = "available"
        import compute_usertime as compute
        report["base_capability"] = compute.base_capability(args.base_commit)
        for c, ref in zip(m["cases"], reference["cases"]):
            golden = expected(c)
            if len(ref["samples"]) != 21 or "warmup_sample" not in ref:
                raise ValueError("performance reference needs exactly 21 samples and retained warmup")
            for sample in [ref["warmup_sample"], *ref["samples"]]:
                if any(type(sample[k]) not in (int, float) or not math.isfinite(sample[k]) or sample[k] < 0 for k in ("user_seconds", "system_seconds")):
                    raise ValueError("invalid raw reference CPU time")
                if sample["exit_status"] != 0 or sample["stderr_base64"]:
                    raise ValueError("invalid reference correctness")
                check_output(c, base64.b64decode(sample["stdout_base64"], validate=True), golden)
            for key in c:
                if ref[key] != c[key]:
                    raise ValueError("reference inputs changed: " + key)
            if ref["quality"] != quality(ref["samples"]) or ref[
                "median_user_seconds"
            ] != statistics.median((s["user_seconds"] for s in ref["samples"])):
                raise ValueError("reference statistics mismatch")
            row = dict(
                id=c["id"],
                mapped_image_bytes=c["mapped_image_bytes"],
                base_status=compute.case_base_status(c, report["base_capability"]),
                split=c["split"],
                status="unverified",
                reference_user_seconds=ref["median_user_seconds"],
                reference_quality=ref["quality"],
            )
            functional = executor(args.head, c, mode="functional")
            for label, binary, revision in (
                ("head", args.head, state["commit"]),
                ("base", args.base if row["base_status"] == "available" else None, args.base_commit),
            ):
                if not binary:
                    continue
                r = executor(binary, c)
                validate_timing(r, c)
                if label == "head" and (
                    functional["instructions"] != r["instructions"]
                    or functional["stdout_base64"] != r["stdout_base64"]
                    or functional["build"] != r["build"]
                ):
                    raise ValueError("functional/timing architectural result mismatch")
                if (
                    r["build"].get("vcs.revision") != revision
                    or r["build"].get("vcs.modified") != "false"
                ):
                    raise ValueError(label + " binary provenance mismatch")
                seconds = r["timing"]["predicted_user_seconds"]
                row[label] = dict(
                    predicted_user_seconds=seconds,
                    instructions=r["instructions"],
                    cycles=r["timing"]["user_cycles"],
                    configuration=r["timing"]["configuration"],
                    cache_config=r["timing"]["cache_config"],
                )
                if ref["quality"] == "usable":
                    row[label]["error_percent"] = error_percent(
                        seconds, ref["median_user_seconds"]
                    )
                    row[label]["direction"] = (
                        "slower"
                        if seconds > ref["median_user_seconds"]
                        else (
                            "faster"
                            if seconds < ref["median_user_seconds"]
                            else "equal"
                        )
                    )
            if "base" in row:
                for key in ("configuration", "cache_config"):
                    if row["base"][key] != row["head"][key]:
                        raise ValueError("base/head hardware profile differs: " + key)
                if "error_percent" in row["head"]:
                    row["error_change_percentage_points"] = (
                        row["head"]["error_percent"] - row["base"]["error_percent"]
                    )
            row["functional_status"] = "pass"
            row["reason"] = (
                "reference environment/core/frequency not hardware-validated; no accuracy threshold approved"
            )
            report["cases"].append(row)
        report["summary"] = {
            split: dict(
                required=sum((c["split"] == split for c in m["cases"])),
                comparable=sum(
                    (
                        c["split"] == split and "error_percent" in c["head"]
                        for c in report["cases"]
                    )
                ),
            )
            for split in ("calibration", "held_out")
        }
        for split, summary in report["summary"].items():
            rows = [r for r in report["cases"] if r["split"] == split]
            summary.update(head_comparable=sum("error_percent" in r["head"] for r in rows), base_comparable=sum("error_percent" in r.get("base", {}) for r in rows))
            for label in ("base", "head"):
                values = [
                    c[label]["error_percent"]
                    for c in report["cases"]
                    if c["split"] == split
                    and label in c
                    and ("error_percent" in c[label])
                ]
                if values:
                    summary[label] = dict(
                        mean_error_percent=statistics.mean(values),
                        maximum_error_percent=max(values),
                    )
        if source_state() != state:
            raise ValueError("source changed during accuracy run")
        if digest(args.head) != report["head_binary_sha256"] or (
            args.base and digest(args.base) != report["base_binary_sha256"]
        ):
            raise ValueError("binary changed during accuracy run")
        if digest(args.reference) != report["reference_sha256"]:
            raise ValueError("reference changed during accuracy run")
        report["artifact_status"] = "pass"
    except (
        ValueError,
        KeyError,
        TypeError,
        OSError,
        subprocess.SubprocessError,
    ) as exc:
        report["error"] = str(exc)
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n")
    lines = [
        "# User-time accuracy report",
        f"Formula: `{FORMULA}`",
        f"Artifact: **{report['artifact_status']}**; hardware accuracy: **{report['accuracy_status']}**",
        "",
        "| Case | Reference (s) | Quality | Base (s) | Head (s) | Base error % | Head error % | Change (pp) | Direction | Status / reason |",
        "|---|---:|---|---:|---:|---:|---:|---:|---|---|",
    ]
    for c in report["cases"]:
        base, head = (c.get("base", {}), c["head"])
        lines.append(
            f"| {c['id']} | {c['reference_user_seconds']:.9g} | {c['reference_quality']} | {base.get('predicted_user_seconds', 'unavailable')} | {head['predicted_user_seconds']:.9g} | {base.get('error_percent', 'unavailable')} | {head.get('error_percent', 'unverified')} | {c.get('error_change_percentage_points', 'unavailable')} | {head.get('direction', 'unverified')} | {c['status']}: {c['reason']} |"
        )
    for split, summary in report.get("summary", {}).items():
        lines.append(f"{split}: `{json.dumps(summary, sort_keys=True)}`")
    if "error" in report:
        lines.append("Failure: " + report["error"])
    Path(args.output).with_suffix(".md").write_text("\n".join(lines) + "\n")
    return 0 if report["artifact_status"] == "pass" else 1


def compare_legacy(args, executor=None):
    executor = executor or execute
    import compute_usertime as compute
    if not getattr(args, "compute_reference", None):
        args.compute_reference = str(ROOT / compute.REFERENCE)
    # A timeout, process kill or unexpected exception must never leave a 24-row
    # PASS at the public destination for this 264-case acceptance contract.
    output = Path(args.output)
    output.write_text(json.dumps(dict(schema=3, artifact_status="fail", accuracy_status="unverified", error="264-case comparison incomplete")) + "\n")
    output.with_suffix(".md").write_text("# User-time accuracy report\nArtifact: **fail**; 264-case comparison incomplete.\n")
    with tempfile.TemporaryDirectory(prefix="poros-original24-") as temporary:
        old_args = copy.copy(args)
        old_args.output = str(Path(temporary) / "report.json")
        status = compare24(old_args, executor)
        original = json.loads(Path(old_args.output).read_text())
        if status:
            original["schema"] = 3
            output.write_text(json.dumps(original, indent=2) + "\n")
            output.with_suffix(".md").write_text(Path(old_args.output).with_suffix(".md").read_text())
            return status
    report = compute.extend_report(original, args, executor, validate_timing, error_percent, source_state)
    Path(args.output).write_text(json.dumps(report, indent=2, allow_nan=False) + "\n")
    lines = ["# Whole-program user-time accuracy report", "",
             f"Artifact: **{report['artifact_status']}**; hardware accuracy: **unverified**",
             f"Formula: `{FORMULA}`. Original 24 + compute 240 required points.", "",
             "| Case / split | Reference (s) | Quality | Base (s) | Head (s) | Head error % |",
             "|---|---:|---|---:|---:|---:|"]
    for row in report["cases"]:
        lines.append(f"| {row['id']} / {row['split']} | {row['reference_user_seconds']:.9g} | {row['reference_quality']} | {row.get('base', {}).get('predicted_user_seconds', 'unavailable')} | {row['head']['predicted_user_seconds']:.9g} | {row['head'].get('error_percent', 'unverified')} |")
    for split, summary in report.get("summary", {}).items():
        lines.append(f"{split}: `{json.dumps(summary, sort_keys=True)}`")
    if "error" in report:
        lines.append("Failure: " + report["error"])
    Path(args.output).with_suffix(".md").write_text("\n".join(lines) + "\n")
    return 0 if report["artifact_status"] == "pass" else 1


def compare(args, executor=None, *, memory_simulation=None):
    """Compare applications and consume explicit memory rows, or measure them serially."""
    from performance_compare import compare as compare_performance
    return compare_performance(args, executor or execute, memory_simulation=memory_simulation)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    p = sub.add_parser("capture")
    p.add_argument("--output", required=True)
    p.add_argument("--measurement-id", required=True)
    p.add_argument("--environment", choices=["native", "virtualized"], required=True)
    p.add_argument("--hardware", required=True)
    p.add_argument("--samples", type=int, default=21)
    p = sub.add_parser("compare")
    p.add_argument("--head", required=True)
    p.add_argument("--base")
    p.add_argument("--base-unavailable", action="store_true")
    p.add_argument("--base-commit", required=True)
    p.add_argument("--reference", required=True)
    p.add_argument("--output", required=True)
    p = sub.add_parser("measure")
    p.add_argument("--measurement-id", required=True)
    p.add_argument("--build-info")
    p.add_argument("--output", required=True)
    p.add_argument("--environment", choices=["native", "virtualized"], required=True)
    p.add_argument("--hardware", required=True)
    p.add_argument("--samples", type=int, default=13)
    p.add_argument("program", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    if args.command == "capture":
        capture(args)
        return 0
    if args.command == "measure":
        measure(args)
        return 0
    return compare(args)


if __name__ == "__main__":
    raise SystemExit(main())
