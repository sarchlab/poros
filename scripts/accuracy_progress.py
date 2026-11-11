"""Canonical partial reports from completed shards; never executes a simulator."""
import argparse
import copy
from datetime import datetime, timezone
import gzip
import hashlib
import json
import os
import tempfile
import sys
from pathlib import Path
import time

import accuracy_live as live
import accuracy_parallel as parallel
import accuracy_site as site
import compute_usertime as compute
import performance_compare as comparison
import performance_reference as reference
import usertime as user


def atomic_write(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, prefix="." + path.name, delete=False) as stream:
        temporary = Path(stream.name)
        json.dump(value, stream, allow_nan=False)
        stream.write("\n")
    try:
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def reporting_identity():
    state = user.source_state()
    files = {name: digest for name, digest in state["files"].items() if name.startswith("scripts/")}
    # Include new untracked modules in an explicitly dirty local preview.
    for path in (Path(user.ROOT) / "scripts").glob("*.py"):
        files[path.relative_to(user.ROOT).as_posix()] = user.digest(path)
    return {"commit": state["commit"], "dirty": state["dirty"], "files": files}


def report_header(plan, source):
    declared, measured = reference.load(source, require_native=True)
    report = dict(schema=4, artifact_status="partial", accuracy_status="unverified", formula=user.FORMULA,
                  cases=[], workload_manifest=declared, head=plan["head"], base_commit=plan["base_commit"],
                  head_binary_sha256=plan["binaries"]["head"], base_binary_sha256=plan["binaries"]["base"],
                  base_capability=plan["base_capability"], base_status="per_case_resource_limit",
                  model_profiles=plan["model_profiles"], reference_sha256=plan["references"]["performance264"],
                  reference={k: v for k, v in measured.items() if k not in ("cases", "input_sources", "collector_sources")},
                  reporting=reporting_identity())
    for name in site.declared_diagnostics(report):
        report[name + "_simulation"] = [] if name == "throughput" else {"head": [], "base": []}
    if "memory" in plan:
        report["memory_base_large_image_support"] = plan["memory"]["base_large_image_support"]
    report["summary"] = compute.summaries([], True)
    return report, measured


def snapshot(report, source, progress):
    result = dict(kind=live.KIND, report=copy.deepcopy(report), progress=progress,
                  reference_text=(Path(source) / reference.PATH).read_bytes().decode("utf-8"))
    result["report"]["artifact_status"] = "partial"
    provenance = {key: report[key] for key in ("base_commit", "head_binary_sha256", "base_binary_sha256", "model_profiles")}
    provenance["head_commit"] = report["head"]["commit"]
    for name, suite in site.declared_diagnostics(report).items():
        result[name + "_reference_text"] = (Path(source) / suite.REFERENCE).read_bytes().decode("utf-8")
        result[name + "_provenance"] = provenance
    if "throughput" not in site.declared_diagnostics(report) and site.FRONTEND_REFERENCE in report["head"]["files"]:
        result["frontend_reference_text"] = (Path(source) / site.FRONTEND_REFERENCE).read_bytes().decode("utf-8")
    return live.validate_snapshot(result)


def active_memory_progress(plan, report, active_shards):
    """Expose memory points assigned to active workers, without new measurements."""
    if "memory" not in plan:
        return
    rows = report["memory_simulation"]
    done = {role: {row["id"] for row in values} for role, values in rows.items()}
    cases = {case["id"]: case for case in plan["memory"]["cases"]}
    active = []
    for index in sorted(set(active_shards)):
        if index >= len(plan["memory"]["shards"]):
            continue
        for ident in plan["memory"]["shards"][index]:
            for role in parallel.memory_roles(plan, cases[ident]):
                if ident not in done[role]:
                    active.append(dict(role=role, id=ident))
    complete = all(len(values) == len(cases) for values in done.values())
    report.setdefault("diagnostic_progress", {})["memory"] = dict(
        state="running" if active else "complete" if complete else "partial" if any(done.values()) else "pending",
        active=active, errors=[])


class SnapshotExporter:
    """Keep only validated compact rows in memory, not multi-GB raw shard trees.

    Files are immutable campaign receipts. A changed stat causes full revalidation;
    unchanged files reuse canonical rows validated against this exact plan.
    """
    def __init__(self, prepared, source, results, run_id, attempt, probe_progress=None, status=None, checkpoints=None, diagnostic_status=None):
        self.prepared, self.source, self.results = map(Path, (prepared, source, results))
        self.run_id, self.attempt = run_id, attempt
        self.plan, self.witnesses = parallel.load_plan(prepared, run_id, attempt, source)
        self.header, self.native = report_header(self.plan, source)
        self.cache = {}
        self.probe_progress = Path(probe_progress) if probe_progress else None
        self.status = Path(status) if status else None
        self.checkpoints = Path(checkpoints) if checkpoints else None
        self.diagnostic_status = Path(diagnostic_status) if diagnostic_status else None
        if self.checkpoints and self.probe_progress:
            raise ValueError("choose raw diagnostic checkpoints or legacy probe progress")
        if self.diagnostic_status and not self.checkpoints:
            raise ValueError("diagnostic status requires raw checkpoints")

    def read_shard(self, path):
        stat = path.stat()
        receipt = path.parent / "receipt.json"
        receipt_bytes = receipt.read_bytes() if receipt.exists() else b""
        fingerprint = (stat.st_ino, stat.st_size, stat.st_mtime_ns, hashlib.sha256(receipt_bytes).hexdigest())
        if path in self.cache and self.cache[path][0] == fingerprint:
            return self.cache[path][1]
        opener = gzip.open if path.suffix == ".gz" else open
        with opener(path, "rb") as stream:
            raw = stream.read()
        if receipt_bytes:
            delivered = json.loads(receipt_bytes)
            if (delivered.get("shard_sha256") != hashlib.sha256(raw).hexdigest()
                    or delivered.get("head") != self.plan["head"]["commit"]
                    or delivered.get("binaries") != self.plan["binaries"] or delivered.get("returncode") != 0):
                raise ValueError("shard receipt digest mismatch")
        shard = json.loads(raw)
        selected = parallel.select_shards(self.plan, [shard], self.attempt, partial=True)
        execute, remaining = parallel.recorded_executor(self.plan, selected, self.witnesses, self.prepared)
        cases = {c["id"]: c for c in self.plan["cases"]}
        native = {c["id"]: c for c in self.native["cases"]}
        rows = [comparison.application_row(cases[r["case_id"]], native[r["case_id"]], self.native,
                self.header, self.prepared / "head", self.prepared / "base", execute) for r in shard["records"]]
        if remaining:
            raise ValueError("unused recorded executions")
        memory = {"head": [], "base": []}
        if "memory" in self.plan:
            if shard["shard"] < len(self.plan["memory"]["shards"]):
                memory = parallel.recorded_memory(self.plan, [shard], self.attempt, partial=True)
            elif shard.get("memory_records") != []:
                raise ValueError("unexpected memory records")
        compact = dict(shard=shard["shard"], attempt=shard["attempt"], rows=rows, memory=memory)
        self.cache[path] = (fingerprint, compact)
        return compact

    def export(self):
        report = copy.deepcopy(self.header)
        selected, seen = {}, set()
        paths = sorted([*self.results.rglob("shard.json"), *self.results.rglob("shard.json.gz")])
        status = site.read(self.status) if self.status else None
        if status and (status.get("head") != self.plan["head"]["commit"] or status.get("base") != self.plan["base_commit"] or status.get("run_id") != self.run_id):
            raise ValueError("campaign status identity mismatch")
        if status:
            count = len(self.plan["shards"])
            def index(value):
                if isinstance(value, str) and value.isascii() and value.isdigit():
                    value = int(value)
                if type(value) is not int or not 0 <= value < count:
                    raise ValueError("campaign status has unknown shard index")
                return value
            completed = status.get("completed")
            active = status.get("active", {})
            failed = status.get("failed", [])
            if not isinstance(completed, list) or not isinstance(active, dict) or not isinstance(failed, list):
                raise ValueError("invalid campaign status collections")
            completed_ids = [index(i) for i in completed]
            active_ids = [index(i) for i in active]
            if len(set(completed_ids)) != len(completed_ids) or set(completed_ids) & set(active_ids):
                raise ValueError("duplicate/overlapping campaign status indices")
            for failure in failed:
                if not isinstance(failure, dict) or not isinstance(failure.get("error"), str):
                    raise ValueError("invalid failed shard status")
                index(failure.get("index"))
        for path in paths:
            # Compressed downloads are not published until their receipt exists.
            if path.suffix == ".gz" and not (path.parent / "receipt.json").is_file():
                continue
            if status and int(path.parent.name) not in status["completed"]:
                continue
            shard = self.read_shard(path)
            key = shard["shard"], shard["attempt"]
            if key in seen:
                raise ValueError("duplicate shard attempt")
            seen.add(key)
            if key[0] not in selected or selected[key[0]]["attempt"] < key[1]:
                selected[key[0]] = shard
        rows = {r["id"]: r for shard in selected.values() for r in shard["rows"]}
        report["cases"] = [rows[c["id"]] for c in self.plan["cases"] if c["id"] in rows]
        if "memory" in self.plan:
            for role in ("head", "base"):
                rows = {r["id"]: r for shard in selected.values() for r in shard["memory"][role]}
                report["memory_simulation"][role] = [rows[c["id"]] for c in self.plan["memory"]["cases"] if c["id"] in rows]
        errors = []
        if self.checkpoints:
            simulations, diagnostics = parallel.recorded_diagnostics(
                self.plan, self.prepared, self.source, self.checkpoints, partial=True)
            for name, rows in simulations.items():
                report[name + "_simulation"] = rows
            report["diagnostic_progress"] = diagnostics
            if self.diagnostic_status:
                status_d = site.read(self.diagnostic_status)
                if (status_d.get("head") != self.plan["head"]["commit"]
                        or status_d.get("base") != self.plan["base_commit"] or status_d.get("run_id") != self.run_id):
                    raise ValueError("diagnostic status identity mismatch")
                tasks = parallel.diagnostic_tasks(self.plan, self.source)
                def task_index(value):
                    if isinstance(value, str) and value.isascii() and value.isdigit():
                        value = int(value)
                    if type(value) is not int or not 0 <= value < len(tasks):
                        raise ValueError("unknown diagnostic status index")
                    return value
                completed = status_d.get("completed")
                active_d, failed = status_d.get("active", {}), status_d.get("failed", [])
                if not isinstance(completed, list) or not isinstance(active_d, dict) or not isinstance(failed, list):
                    raise ValueError("invalid diagnostic status collections")
                completed_ids, active_ids = [task_index(i) for i in completed], [task_index(i) for i in active_d]
                if (len(set(completed_ids)) != len(completed_ids) or len(set(active_ids)) != len(active_ids)
                        or set(completed_ids) & set(active_ids)):
                    raise ValueError("duplicate/overlapping diagnostic status indices")
                for index in active_ids:
                    task = tasks[index]
                    name, ident = task["suite"], task["case"]["id"]
                    rows = {"head": simulations[name]} if name == "throughput" else simulations[name]
                    for role in task["roles"]:
                        entry = dict(role=role, id=ident)
                        if ident not in {r["id"] for r in rows[role]} and entry not in diagnostics[name]["active"]:
                            diagnostics[name]["active"].append(entry)
                            diagnostics[name]["state"] = "running"
                for failure in failed:
                    if not isinstance(failure, dict) or not isinstance(failure.get("error"), str):
                        raise ValueError("invalid failed diagnostic status")
                    task = tasks[task_index(failure.get("index"))]
                    message = task["suite"] + "/" + task["case"]["id"] + ": " + failure["error"]
                    diagnostics[task["suite"]]["state"] = "failed"
                    diagnostics[task["suite"]]["errors"].append(message)
                    errors.append(message)
        if self.probe_progress and self.probe_progress.exists():
            progress = site.read(self.probe_progress)
            # Validate the complete partial-report contract before copying any rows.
            live.validate_snapshot(progress)
            incoming = progress["report"]
            for key in ("head", "base_commit", "head_binary_sha256", "base_binary_sha256", "model_profiles"):
                if incoming[key] != report[key]:
                    raise ValueError("probe progress source identity mismatch")
            for name in site.declared_diagnostics(report):
                if name != "memory":
                    report[name + "_simulation"] = incoming[name + "_simulation"]
            report["diagnostic_progress"] = incoming.get("diagnostic_progress", {})
            errors = progress["progress"]["errors"]
        report["summary"] = compute.summaries(report["cases"], True)
        active = []
        if status:
            for index in status.get("active", {}):
                if int(index) not in selected:
                    active.extend(self.plan["shards"][int(index)])
            errors.extend(str(e) for e in status.get("errors", []))
            errors.extend("Shard " + str(item["index"]) + ": " + item["error"] for item in status.get("failed", []))
        active_memory_progress(self.plan, report, [int(index) for index in (status or {}).get("active", {}) if int(index) not in selected])
        progress = dict(state="failed" if errors else "running", completed_cases=len(report["cases"]),
                        required_cases=len(self.plan["cases"]), completed_shards=len(selected),
                        required_shards=len(self.plan["shards"]), errors=errors, active_cases=active,
                        updated_at=datetime.now(timezone.utc).isoformat())
        if status and status.get("phase") == "complete":
            progress["state"] = "complete"
        if status and status.get("aggregate_error"):
            progress.update(state="failed", errors=[*errors, str(status["aggregate_error"])])
        return snapshot(report, self.source, progress)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("prepared", "source", "results", "run-id", "output"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--attempt", type=int, required=True)
    parser.add_argument("--probe-progress")
    parser.add_argument("--status")
    parser.add_argument("--checkpoints")
    parser.add_argument("--diagnostic-status")
    parser.add_argument("--interval", type=float, default=0)
    args = parser.parse_args()
    exporter = SnapshotExporter(args.prepared, args.source, args.results, args.run_id, args.attempt, args.probe_progress, args.status, args.checkpoints, args.diagnostic_status)
    while True:
        try:
            atomic_write(args.output, exporter.export())
        except (OSError, ValueError, KeyError, TypeError) as error:
            if not args.interval:
                raise
            print("Snapshot unchanged; retrying: " + str(error), file=sys.stderr, flush=True)
        if not args.interval:
            return 0
        time.sleep(args.interval)


if __name__ == "__main__":
    raise SystemExit(main())
