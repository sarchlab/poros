"""The active 264-point comparison; serial and sharded runs use this same loop."""
from pathlib import Path
import subprocess

import compute_usertime as compute
import performance_reference as reference
import frontend_microbenchmark as frontend
import throughput_microbenchmark as throughput
import forwarding_microbenchmark as forwarding
import scheduling_microbenchmark as scheduling
import capacity_microbenchmark as capacity
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


def application_row(case, native, measured, report, head, base, executor):
    """Construct the same complete application row from live or recorded runs."""
    import usertime as user
    profiles = model_profiles.validate(report)
    expected = user.OutputIdentity(native["output_bytes"], native["output_sha256"])
    row = dict(id=case["id"], split=case["split"], status="unverified", mapped_image_bytes=case["mapped_image_bytes"],
               base_status=compute.case_base_status(case, report["base_capability"]),
               reference_user_seconds=native["median_user_seconds"], reference_quality=native["quality"],
               reference_p10_seconds=native["p10_user_seconds"], reference_p90_seconds=native["p90_user_seconds"],
               reason="Native macOS Mach-O user accounting; startup included; core/frequency unknown; no accuracy threshold approved")
    if "streamcluster_replacement" in measured:
        row["reference_origin"] = reference.reference_origin(measured, case)
    functional = executor(head, case, mode="functional", expected_identity=expected)
    for label, binary, revision in (("head", head, report["head"]["commit"]), ("base", base, report["base_commit"])):
        if label == "base" and row["base_status"] != "available":
            continue
        options = {"expected_profile": profiles[label]} if "model_profiles" in report else {}
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
    return row


def compare(args, executor, *, memory_simulation=None, memory_base_large_image_support=None, source=None, checkpoints=None, progress_path=None, diagnostic_simulation=None, replay_only=False):
    """Consume supplied rows; replay_only forbids all diagnostic execution fallbacks."""
    import usertime as user
    root = Path(source) if source is not None else Path(user.ROOT)
    output = Path(args.output)
    progress_ready, last_snapshot = False, None
    def publish(error=None):
        nonlocal last_snapshot
        if not progress_path or not progress_ready:
            return
        from accuracy_progress import atomic_write, snapshot
        from datetime import datetime, timezone
        report["summary"] = compute.summaries(report["cases"], True)
        progress = dict(state="failed" if error else "running", completed_cases=len(report["cases"]),
                        required_cases=len(report["workload_manifest"]["cases"]),
                        completed_shards=1, required_shards=1, active_cases=[],
                        errors=[str(error)] if error else [], updated_at=datetime.now(timezone.utc).isoformat())
        try:
            value = snapshot(report, root, progress)
        except (ValueError, KeyError, TypeError, OSError):
            if error is None:
                raise
            if last_snapshot is None:
                return
            import copy
            value = copy.deepcopy(last_snapshot)
            value["progress"].update(state="failed", errors=[str(error)], updated_at=progress["updated_at"])
            for item in value["report"].get("diagnostic_progress", {}).values():
                if item["active"]:
                    item.update(state="failed", errors=[str(error)])
        atomic_write(progress_path, value)
        last_snapshot = value
    report = dict(schema=4, artifact_status="fail", accuracy_status="unverified", formula=user.FORMULA, cases=[])
    compute.write(output, report)
    output.with_suffix(".md").write_text("# User-time accuracy report\nArtifact: **fail**; comparison incomplete.\n")
    try:
        if Path(args.reference).resolve() != (root / reference.PATH).resolve():
            raise ValueError("active comparison requires the committed compact reference")
        declared, measured = reference.load(root, require_native=True)
        state = user.source_state(root)
        if state["dirty"]:
            raise ValueError("accuracy comparison requires clean committed source")
        if not args.base or args.base_unavailable or not compute.base_declares_cohort(args.base_commit, root):
            raise ValueError("performance comparison requires a compute-ELF-capable base")
        report.update(workload_manifest=declared, head=state, base_commit=args.base_commit,
                      head_binary_sha256=user.digest(args.head), base_binary_sha256=user.digest(args.base),
                      base_capability=compute.base_capability(args.base_commit, root), base_status="per_case_resource_limit",
                      reference_sha256=user.digest(root / reference.PATH),
                      reference={key: value for key, value in measured.items() if key not in ("cases", "input_sources", "collector_sources")})
        declarations = model_profiles.resolve(root, state, args.base_commit)
        if declarations is not None:
            report["model_profiles"] = declarations
        profiles = model_profiles.validate(report)
        from accuracy_progress import reporting_identity
        import accuracy_site as site
        report["reporting"] = reporting_identity()
        report["diagnostic_progress"] = {}
        diagnostics = {name: suite for name, suite in site.declared_diagnostics(report).items() if name != "memory"}
        if diagnostic_simulation is not None and set(diagnostic_simulation) != set(diagnostics):
            raise ValueError("recorded diagnostic suite coverage mismatch")
        if replay_only and diagnostic_simulation is None:
            raise ValueError("aggregation requires recorded diagnostic evidence")
        for name in site.declared_diagnostics(report):
            report[name + "_simulation"] = [] if name == "throughput" else {"head": [], "base": []}
        if memory.declared(state["files"]):
            report["memory_base_large_image_support"] = memory.supports_large_images(root, args.base_commit)
            if memory_simulation is not None:
                report["memory_simulation"] = memory_simulation
        progress_ready = True
        publish()
        for case, native in zip(declared["cases"], measured["cases"]):
            row = application_row(case, native, measured, report, args.head, args.base, executor)
            report["cases"].append(row)
            publish()
            print("accuracy", case["id"], native["quality"], flush=True)
        for name, suite in site.declared_diagnostics(report).items():
            if name == "memory":
                continue
            suite.load_reference(root)
            if diagnostic_simulation is not None:
                rows = diagnostic_simulation[name]
                if name == "throughput":
                    suite.validate_simulation(rows, suite.cases(root))
                else:
                    suite.validate_comparison(rows, suite.cases(root))
                report[name + "_simulation"] = rows
                report["diagnostic_progress"][name] = dict(state="complete", active=[], errors=[])
                publish()
                continue
            for label in (("head",) if name == "throughput" else ("head", "base")):
                binary = args.head if label == "head" else args.base
                revision = state["commit"] if label == "head" else args.base_commit
                checkpoint = None
                if checkpoints:
                    from probe_checkpoint import Checkpoint, digest
                    checkpoint = Checkpoint(checkpoints, name, label,
                        dict(commit=state["commit"], files_sha256=digest(state["files"])),
                        binary, revision, profiles[label], state["files"][suite.ELF])
                def update(rows, active, name=name, label=label, suite=suite):
                    if name == "throughput":
                        report[name + "_simulation"] = list(rows)
                        complete = len(rows) == len(suite.cases(root))
                    else:
                        report[name + "_simulation"][label] = list(rows)
                        complete = all(len(r) == len(suite.cases(root)) for r in report[name + "_simulation"].values())
                    report["diagnostic_progress"][name] = dict(
                        state="running" if active else "complete" if complete else "partial",
                        active=[dict(role=label, id=active)] if active else [], errors=[])
                    publish()
                options = dict(checkpoint=checkpoint, progress=update) if (checkpoints or progress_path) else {}
                rows = suite.measure(binary, root, revision, profiles[label], **options)
                if checkpoints or progress_path:
                    update(rows, None)
                elif name == "throughput":
                    report[name + "_simulation"] = rows
                else:
                    report[name + "_simulation"][label] = rows
        if throughput.ELF not in state["files"] and frontend.ELF in state["files"]:
            if replay_only:
                raise ValueError("aggregation requires recorded legacy frontend evidence")
            report["frontend_simulation"] = frontend.measure(args.head, root, state["commit"], profiles["head"])
        if (memory_simulation is not None or memory_base_large_image_support is not None) and not memory.declared(state["files"]):
            raise ValueError("recorded memory rows supplied without a declared suite")
        if memory.declared(state["files"]):
            native_memory = memory.load_reference(root)
            capability = memory.supports_large_images(root, args.base_commit)
            report["memory_base_large_image_support"] = capability
            if memory_simulation is not None:
                if type(memory_base_large_image_support) is not bool or memory_base_large_image_support != capability:
                    raise ValueError("recorded memory base capability differs from source")
                report["memory_simulation"] = memory_simulation
            else:
                if replay_only:
                    raise ValueError("aggregation requires recorded memory evidence")
                print("accuracy memory: measuring the full memory suite serially for both revisions; use accuracy_parallel.py for sharded execution", flush=True)
                report["memory_simulation"] = {
                    label: memory.measure(binary, root, revision, profiles[label])
                    for label, binary, revision in (("head", args.head, state["commit"]),
                                                    ("base", args.base, args.base_commit))
                }
            memory.validate_comparison(report["memory_simulation"], native_memory["cases"], capability)
        report["summary"] = compute.summaries(report["cases"], True)
        if (user.source_state(root)) != state or user.digest(args.head) != report["head_binary_sha256"] or user.digest(args.base) != report["base_binary_sha256"]:
            raise ValueError("source or simulator changed during comparison")
        if user.digest(root / reference.PATH) != report["reference_sha256"]:
            raise ValueError("reference changed during comparison")
        report["artifact_status"] = "pass"
        publish()
    except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError) as error:
        report["artifact_status"] = "fail"
        report["error"] = str(error)
        for item in report.get("diagnostic_progress", {}).values():
            if item["active"]:
                item.update(state="failed", errors=[str(error)])
        publish(error)
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
