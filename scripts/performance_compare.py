"""The active 264-point comparison; serial and sharded runs use this same loop."""
from pathlib import Path
import subprocess

import compute_usertime as compute
import performance_reference as reference
import frontend_microbenchmark as frontend
import throughput_microbenchmark as throughput
import forwarding_microbenchmark as forwarding
import scheduling_microbenchmark as scheduling
import memory_microbenchmark as memory
import model_profiles


MICRO_WORKLOADS = {"integer", "branch", "memory"}


def is_micro_case(case):
    return case["id"].partition("-")[0] in MICRO_WORKLOADS


def benchmark_summary(report):
    if report["artifact_status"] != "pass":
        return {}
    return compute.case_summary([c for c in report["cases"] if not is_micro_case(c)],
                             performance=report.get("schema") == 4)


def compare(args, executor):
    import usertime as user
    output = Path(args.output)
    report = dict(schema=4, artifact_status="fail", accuracy_status="unverified", formula=user.FORMULA, cases=[])
    compute.write(output, report)
    output.with_suffix(".md").write_text("# User-time accuracy report\nArtifact: **fail**; comparison incomplete.\n")
    try:
        if Path(args.reference).resolve() != (Path(user.ROOT) / reference.PATH).resolve():
            raise ValueError("active comparison requires the committed compact reference")
        declared, measured = reference.load(user.ROOT, require_native=True)
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
        declarations = model_profiles.resolve(user.ROOT, state, args.base_commit)
        if declarations is not None:
            report["model_profiles"] = declarations
        profiles = model_profiles.validate(report)
        for case, native in zip(declared["cases"], measured["cases"]):
            expected = user.OutputIdentity(native["output_bytes"], native["output_sha256"])
            row = dict(id=case["id"], split=case["split"], status="unverified", mapped_image_bytes=case["mapped_image_bytes"],
                       base_status=compute.case_base_status(case, report["base_capability"]),
                       reference_user_seconds=native["median_user_seconds"], reference_quality=native["quality"],
                       reference_p10_seconds=native["p10_user_seconds"], reference_p90_seconds=native["p90_user_seconds"],
                       reason="Native macOS Mach-O user accounting; startup included; core/frequency unknown; no accuracy threshold approved")
            if "streamcluster_replacement" in measured:
                row["reference_origin"] = reference.reference_origin(measured, case)
            functional = executor(args.head, case, mode="functional", expected_identity=expected)
            for label, binary, revision in (("head", args.head, state["commit"]), ("base", args.base, args.base_commit)):
                if label == "base" and row["base_status"] != "available":
                    continue
                options = {"expected_profile": profiles[label]} if declarations is not None else {}
                result = executor(binary, case, expected_identity=expected, **options)
                user.validate_execution(result, case, expected_identity=expected, expected_profile=profiles[label])
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
                if native["quality"] == "usable":
                    row["error_change_percentage_points"] = row["head"]["error_percent"] - row["base"]["error_percent"]
            row["functional_status"] = "pass"
            report["cases"].append(row)
            print("accuracy", case["id"], native["quality"], flush=True)
        if throughput.ELF in state["files"]:
            throughput.load_reference(user.ROOT)
            report["throughput_simulation"] = throughput.measure(args.head, user.ROOT, state["commit"], profiles["head"])
        elif frontend.ELF in state["files"]:
            report["frontend_simulation"] = frontend.measure(args.head, user.ROOT, state["commit"], profiles["head"])
        if forwarding.declared(state["files"]):
            forwarding.load_reference(user.ROOT)
            report["forwarding_simulation"] = {
                label: forwarding.measure(binary, user.ROOT, revision, profiles[label])
                for label, binary, revision in (("head", args.head, state["commit"]),
                                                ("base", args.base, args.base_commit))
            }
        if scheduling.declared(state["files"]):
            scheduling.load_reference(user.ROOT)
            report["scheduling_simulation"] = {
                label: scheduling.measure(binary, user.ROOT, revision, profiles[label])
                for label, binary, revision in (("head", args.head, state["commit"]),
                                                ("base", args.base, args.base_commit))
            }
        if memory.declared(state["files"]):
            native_memory = memory.load_reference(user.ROOT)
            if hasattr(args, "memory_simulation"):
                report["memory_simulation"] = args.memory_simulation
            else:
                report["memory_simulation"] = {
                    label: memory.measure(binary, user.ROOT, revision, profiles[label])
                    for label, binary, revision in (("head", args.head, state["commit"]),
                                                    ("base", args.base, args.base_commit))
                }
            memory.validate_comparison(report["memory_simulation"], native_memory["cases"])
        report["summary"] = compute.summaries(report["cases"], True)
        if user.source_state() != state or user.digest(args.head) != report["head_binary_sha256"] or user.digest(args.base) != report["base_binary_sha256"]:
            raise ValueError("source or simulator changed during comparison")
        if user.digest(Path(user.ROOT) / reference.PATH) != report["reference_sha256"]:
            raise ValueError("reference changed during comparison")
        report["artifact_status"] = "pass"
    except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError) as error:
        report["error"] = str(error)
    compute.write(output, report)
    lines = ["# Whole-program user-time accuracy report", "", f"Artifact: **{report['artifact_status']}**; hardware accuracy: **unverified**", f"Formula: `{user.FORMULA}`. All 264 required performance points."]
    for section, micro in [("Benchmarks", False), ("Microbenchmarks", True)]:
        lines.extend(["", "## " + section, "", "| Case | Reference (s) | Quality | Base (s) | Head (s) | Head error % |", "|---|---:|---|---:|---:|---:|"])
        for row in report["cases"]:
            if is_micro_case(row) != micro:
                continue
            lines.append(f"| {row['id']} | {row['reference_user_seconds']:.9g} | {row['reference_quality']} | {row.get('base', {}).get('predicted_user_seconds', 'unavailable')} | {row['head']['predicted_user_seconds']:.9g} | {row['head'].get('error_percent', 'unverified')} |")
        if not micro and report["artifact_status"] == "pass":
            summary = benchmark_summary(report)
            lines.append(f"\nComparable application cases: {summary['comparable']} / {summary['required']}.")
    if "error" in report:
        lines.append("Failure: " + report["error"])
    output.with_suffix(".md").write_text("\n".join(lines) + "\n")
    return 0 if report["artifact_status"] == "pass" else 1
