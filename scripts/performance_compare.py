"""The active 264-point comparison; serial and sharded runs use this same loop."""
import json
from pathlib import Path
import subprocess

import compute_usertime as compute
import performance_reference as reference


def compare(args, executor):
    import usertime as user
    output = Path(args.output)
    report = dict(schema=4, artifact_status="fail", accuracy_status="unverified", formula=user.FORMULA, cases=[])
    compute.write(output, report)
    output.with_suffix(".md").write_text("# User-time accuracy report\nArtifact: **fail**; comparison incomplete.\n")
    try:
        if Path(args.reference).resolve() != (Path(user.ROOT) / reference.PATH).resolve():
            raise ValueError("active comparison requires the committed compact reference")
        declared, measured = reference.load(user.ROOT)
        state = user.source_state()
        if state["dirty"]:
            raise ValueError("accuracy comparison requires clean committed source")
        if not args.base or args.base_unavailable or not compute.base_declares_cohort(args.base_commit):
            raise ValueError("performance comparison requires a compute-ELF-capable base")
        report.update(workload_manifest=declared, head=state, base_commit=args.base_commit,
                      head_binary_sha256=user.digest(args.head), base_binary_sha256=user.digest(args.base),
                      base_capability=compute.base_capability(args.base_commit), base_status="per_case_resource_limit",
                      reference_sha256=user.digest(Path(user.ROOT) / reference.PATH),
                      reference={key: value for key, value in measured.items() if key not in ("cases", "input_sources", "collector_sources")})
        for case, native in zip(declared["cases"], measured["cases"]):
            expected = user.OutputIdentity(native["output_bytes"], native["output_sha256"])
            row = dict(id=case["id"], split=case["split"], status="unverified", mapped_image_bytes=case["mapped_image_bytes"],
                       base_status=compute.case_base_status(case, report["base_capability"]),
                       reference_user_seconds=native["median_user_seconds"], reference_quality=native["quality"],
                       reference_p10_seconds=native["p10_user_seconds"], reference_p90_seconds=native["p90_user_seconds"],
                       reason="Linux VM user accounting; core/frequency unknown; no accuracy threshold approved")
            functional = executor(args.head, case, mode="functional", expected_identity=expected)
            for label, binary, revision in (("head", args.head, state["commit"]), ("base", args.base, args.base_commit)):
                if label == "base" and row["base_status"] != "available":
                    continue
                result = executor(binary, case, expected_identity=expected)
                user.validate_timing(result, case)
                if result["build"].get("vcs.revision") != revision or result["build"].get("vcs.modified") != "false":
                    raise ValueError(label + " binary provenance mismatch")
                if label == "head" and any(result[key] != functional[key] for key in ("instructions", "stdout_base64", "build")):
                    raise ValueError("functional/timing architectural mismatch")
                timing = result["timing"]
                row[label] = dict(predicted_user_seconds=timing["predicted_user_seconds"], instructions=result["instructions"], cycles=timing["user_cycles"], configuration=timing["configuration"], cache_config=timing["cache_config"])
                if native["quality"] == "usable":
                    seconds, real = timing["predicted_user_seconds"], native["median_user_seconds"]
                    row[label].update(error_percent=user.error_percent(seconds, real), direction="slower" if seconds > real else "faster" if seconds < real else "equal")
            if "base" in row:
                if any(row["base"][key] != row["head"][key] for key in ("configuration", "cache_config")):
                    raise ValueError("base/head hardware profile differs")
                if native["quality"] == "usable":
                    row["error_change_percentage_points"] = row["head"]["error_percent"] - row["base"]["error_percent"]
            row["functional_status"] = "pass"
            report["cases"].append(row)
            print("accuracy", case["id"], native["quality"], flush=True)
        report["summary"] = compute.summaries(report["cases"], True)
        if user.source_state() != state or user.digest(args.head) != report["head_binary_sha256"] or user.digest(args.base) != report["base_binary_sha256"]:
            raise ValueError("source or simulator changed during comparison")
        if user.digest(Path(user.ROOT) / reference.PATH) != report["reference_sha256"]:
            raise ValueError("reference changed during comparison")
        report["artifact_status"] = "pass"
    except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError) as error:
        report["error"] = str(error)
    compute.write(output, report)
    lines = ["# Whole-program user-time accuracy report", "", f"Artifact: **{report['artifact_status']}**; hardware accuracy: **unverified**", f"Formula: `{user.FORMULA}`. All 264 required performance points.", "", "| Case / split | Reference (s) | Quality | Base (s) | Head (s) | Head error % |", "|---|---:|---|---:|---:|---:|"]
    for row in report["cases"]:
        lines.append(f"| {row['id']} / {row['split']} | {row['reference_user_seconds']:.9g} | {row['reference_quality']} | {row.get('base', {}).get('predicted_user_seconds', 'unavailable')} | {row['head']['predicted_user_seconds']:.9g} | {row['head'].get('error_percent', 'unverified')} |")
    for split, summary in report.get("summary", {}).items():
        lines.append(f"{split}: `{json.dumps(summary, sort_keys=True)}`")
    if "error" in report:
        lines.append("Failure: " + report["error"])
    output.with_suffix(".md").write_text("\n".join(lines) + "\n")
    return 0 if report["artifact_status"] == "pass" else 1
