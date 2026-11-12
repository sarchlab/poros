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
import devices


MICRO_WORKLOADS = {"integer", "branch", "memory"}
OUTPUT_ORACLE_SCOPE = ("the committed file is validated as a whole, native timing fields included; this device uses only its "
                       "complete-output identities, which are what each program must print on any device, and none "
                       "of its timings is shown or compared")


def case_base_status(value, case):
    """Why a plan's or report's case has or lacks a base prediction."""
    status = compute.case_base_status(case, value["base_capability"])
    return status if devices.available(value, "base") else devices.BASE_UNAVAILABLE


def application_reference(value):
    """The native application reference of a plan's or report's device.

    Returns (path, hardware it was measured on), or None for a device that
    declares none. A value without a device declaration is the default device
    with the original file."""
    declared = devices.entry(value)
    if declared is None:
        return reference.PATH, reference.HARDWARE
    path = declared["references"].get("application")
    return None if path is None else (path, declared["hardware"])


def application_source(value):
    """The validated file a device's campaign takes its output identities from.

    That is its own reference when it has one. Otherwise it is the default
    device's file, used as an output oracle only."""
    return application_reference(value) or (reference.PATH, reference.HARDWARE)


def output_oracle(root):
    """The program results a device without a native reference is checked against."""
    import usertime as user
    return dict(path=reference.PATH, sha256=user.digest(Path(root) / reference.PATH), scope=OUTPUT_ORACLE_SCOPE)


def is_micro_case(case):
    return case["id"].partition("-")[0] in MICRO_WORKLOADS


def benchmark_summary(report):
    if report["artifact_status"] != "pass":
        return {}
    return compute.case_summary([c for c in report["cases"] if not is_micro_case(c)],
                             performance=report.get("schema") == 4)


def application_row(case, native, measured, report, head, base, executor):
    """Construct the same complete application row from live or recorded runs.

    A device without a native reference gets the same executions and checks. Of
    `native` only the output identity is then read: the row carries no reference
    time, quality or error."""
    import usertime as user
    profiles = model_profiles.validate(report)
    expected = user.OutputIdentity(native["output_bytes"], native["output_sha256"])
    referenced = devices.measured(report, "application")
    row = dict(id=case["id"], split=case["split"], status="unverified", mapped_image_bytes=case["mapped_image_bytes"],
               base_status=case_base_status(report, case))
    if referenced:
        row.update(reference_user_seconds=native["median_user_seconds"], reference_quality=native["quality"],
                   reference_p10_seconds=native["p10_user_seconds"], reference_p90_seconds=native["p90_user_seconds"],
                   reason="Native macOS Mach-O user accounting; startup included; core/frequency unknown; no accuracy threshold approved")
        if "streamcluster_replacement" in measured:
            row["reference_origin"] = reference.reference_origin(measured, case)
    else:
        row.update(reference_quality=devices.UNMEASURED,
                   reason="No native " + devices.entry(report)["hardware"] + " reference; simulator prediction only, no accuracy claim")
    comparable = referenced and native["quality"] == "usable"
    functional = executor(head, case, mode="functional", expected_identity=expected, **devices.options(report, "head"))
    for label, binary, revision in (("head", head, report["head"]["commit"]), ("base", base, report["base_commit"])):
        if label == "base" and row["base_status"] != "available":
            continue
        options = {"expected_profile": profiles[label]} if "model_profiles" in report else {}
        options.update(devices.options(report, label))
        result = executor(binary, case, expected_identity=expected, **options)
        user.validate_execution(result, case, expected_identity=expected, expected_profile=profiles[label], **devices.options(report, label))
        user.validate_timing(result, case)
        if result["build"].get("vcs.revision") != revision or result["build"].get("vcs.modified") != "false":
            raise ValueError(label + " binary provenance mismatch")
        if label == "head" and any(result[key] != functional[key] for key in ("instructions", "stdout_base64", "build")):
            raise ValueError("functional/timing architectural mismatch")
        timing = result["timing"]
        row[label] = dict(predicted_user_seconds=timing["predicted_user_seconds"], instructions=result["instructions"], cycles=timing["user_cycles"], configuration=timing["configuration"], cache_config=timing["cache_config"])
        if comparable:
            seconds, real = timing["predicted_user_seconds"], native["median_user_seconds"]
            row[label].update(error_percent=user.error_percent(seconds, real), direction="slower" if seconds > real else "faster" if seconds < real else "equal")
    if "base" in row:
        if comparable:
            row["error_change_percentage_points"] = row["head"]["error_percent"] - row["base"]["error_percent"]
    row["functional_status"] = "pass"
    return row


def compare(args, executor, *, memory_simulation=None, memory_base_large_image_support=None, source=None, checkpoints=None, progress_path=None, diagnostic_simulation=None, replay_only=False, device=None):
    """Consume supplied rows; replay_only forbids all diagnostic execution fallbacks.

    `device` names the modeled device; None is the source's default. Every
    device runs every case. Native references are compared only for the device
    they were measured on; without them the report holds predictions alone."""
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
    if device is not None:
        # An early failure is still filed under the device that was asked for.
        report["device"] = dict(id=device)
    compute.write(output, report)
    output.with_suffix(".md").write_text("# User-time accuracy report\nArtifact: **fail**; comparison incomplete.\n")
    try:
        state = user.source_state(root)
        chosen = devices.resolve(root, state, args.base_commit, device)
        report.pop("device", None)
        if chosen is not None:
            report["device"] = chosen
        # The device's own reference, or the default device's file as output oracle.
        path, hardware = application_source(report)
        if Path(args.reference).resolve() != (root / path).resolve():
            raise ValueError("active comparison requires the committed compact reference")
        declared, measured = reference.load(root, require_native=True, path=path, hardware=hardware)
        if state["dirty"]:
            raise ValueError("accuracy comparison requires clean committed source")
        if not args.base or args.base_unavailable or not compute.base_declares_cohort(args.base_commit, root):
            raise ValueError("performance comparison requires a compute-ELF-capable base")
        report.update(workload_manifest=declared, head=state, base_commit=args.base_commit,
                      head_binary_sha256=user.digest(args.head), base_binary_sha256=user.digest(args.base),
                      base_capability=compute.base_capability(args.base_commit, root), base_status="per_case_resource_limit")
        if devices.measured(report, "application"):
            devices.require_reference(report, "application", measured)
            report.update(reference_sha256=user.digest(root / path),
                          reference={key: value for key, value in measured.items() if key not in ("cases", "input_sources", "collector_sources")})
        else:
            report["output_oracle"] = output_oracle(root)
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
        definitions = site.diagnostic_definitions(report, root)
        if definitions:
            report["diagnostic_definitions"] = definitions
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
            print("accuracy", case["id"], row["reference_quality"], flush=True)
        for name, suite in site.declared_diagnostics(report).items():
            if name == "memory":
                continue
            if devices.measured(report, name):
                devices.bind_reference(report, name, suite.REFERENCE, suite.load_reference(root))
            if diagnostic_simulation is not None:
                rows = diagnostic_simulation[name]
                site.validate_diagnostic_rows(report, name, rows, suite.cases(root))
                report[name + "_simulation"] = rows
                report["diagnostic_progress"][name] = dict(state="complete", active=[], errors=[])
                publish()
                continue
            for label in site.diagnostic_roles(report, name):
                binary = args.head if label == "head" else args.base
                revision = state["commit"] if label == "head" else args.base_commit
                checkpoint = None
                if checkpoints:
                    from probe_checkpoint import Checkpoint, digest
                    checkpoint = Checkpoint(checkpoints, name, label,
                        dict(commit=state["commit"], files_sha256=digest(state["files"])),
                        binary, revision, profiles[label], state["files"][suite.ELF], **devices.options(report, label))
                def update(rows, active, name=name, label=label, suite=suite):
                    if name == "throughput":
                        report[name + "_simulation"] = list(rows)
                        complete = len(rows) == len(suite.cases(root))
                    else:
                        report[name + "_simulation"][label] = list(rows)
                        complete = all(len(report[name + "_simulation"][role]) == len(suite.cases(root))
                                       for role in site.diagnostic_roles(report, name))
                    report["diagnostic_progress"][name] = dict(
                        state="running" if active else "complete" if complete else "partial",
                        active=[dict(role=label, id=active)] if active else [], errors=[])
                    publish()
                options = dict(checkpoint=checkpoint, progress=update) if (checkpoints or progress_path) else {}
                options.update(devices.options(report, label))
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
            if devices.measured(report, "memory"):
                native_memory = memory.load_reference(root)
                devices.bind_reference(report, "memory", memory.REFERENCE, native_memory)
                memory_cases = native_memory["cases"]
            else:
                memory_cases = memory.cases(root)
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
                    label: memory.measure(binary, root, revision, profiles[label], **devices.options(report, label))
                    if label in site.diagnostic_roles(report, "memory") else []
                    for label, binary, revision in (("head", args.head, state["commit"]),
                                                    ("base", args.base, args.base_commit))
                }
            site.validate_diagnostic_rows(report, "memory", report["memory_simulation"], memory_cases,
                                          base_large_image_support=capability)
        report["summary"] = compute.summaries(report["cases"], True)
        if (user.source_state(root)) != state or user.digest(args.head) != report["head_binary_sha256"] or user.digest(args.base) != report["base_binary_sha256"]:
            raise ValueError("source or simulator changed during comparison")
        if user.digest(root / path) != (report["reference_sha256"] if "reference_sha256" in report else report["output_oracle"]["sha256"]):
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
    if devices.entry(report) is not None:
        import accuracy_site as site
        described = site.device_summary(report)
        lines.append(f"Device: `{described['id']}` ({described['hardware']}); parameters: {described['parameters']}; "
                     f"native references: {described['references']}.")
    for section, micro in [("Benchmarks", False), ("Microbenchmarks", True)]:
        lines.extend(["", "## " + section, "", "| Case | Reference (s) | Quality | Base (s) | Head (s) | Head error % |", "|---|---:|---|---:|---:|---:|"])
        for row in report["cases"]:
            if is_micro_case(row) != micro:
                continue
            measured_seconds = f"{row['reference_user_seconds']:.9g}" if "reference_user_seconds" in row else devices.UNMEASURED
            lines.append(f"| {row['id']} | {measured_seconds} | {row['reference_quality']} | {row.get('base', {}).get('predicted_user_seconds', 'unavailable')} | {row['head']['predicted_user_seconds']:.9g} | {row['head'].get('error_percent', 'unverified')} |")
        if not micro and report["artifact_status"] == "pass":
            summary = benchmark_summary(report)
            lines.append(f"\nComparable application cases: {summary['comparable']} / {summary['required']}.")
    if "error" in report:
        lines.append("Failure: " + report["error"])
    output.with_suffix(".md").write_text("\n".join(lines) + "\n")
    return 0 if report["artifact_status"] == "pass" else 1
