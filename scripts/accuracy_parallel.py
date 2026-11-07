#!/usr/bin/env python3
"""Prepare, execute and aggregate source-bound accuracy shards.

Shards transport full simulator results, not precomputed report rows. Aggregation
reuses the serial comparison and validates every result before consuming it.
"""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
from types import SimpleNamespace

import accuracy_site as site
import performance_reference as reference
import compute_usertime as compute
import usertime as user
import model_profiles

ROOT = user.ROOT
REFERENCE = reference.PATH
SCHEMA = 1


def read(path):
    return site.read(path)


def write(path, value):
    site.write(path, value)


def identity(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()).hexdigest()


def positive(value):
    if type(value) is not int or value < 1:
        raise ValueError("expected positive integer")
    return value


def git(*args):
    return subprocess.check_output(["git", "-C", str(ROOT), *args], text=True).strip()


def inputs():
    declared, measured = reference.load(ROOT)
    cases = declared["cases"]
    identities = {r["id"]: user.OutputIdentity(r["output_bytes"], r["output_sha256"]) for r in measured["cases"]}
    return cases, identities


def load_witness(identities, ident):
    return identities[ident]


def partition(cases, count):
    """Interleave six-point groups rather than concentrating them in one job."""
    if not 1 <= positive(count) <= len(cases):
        raise ValueError("invalid shard count")
    return [[c["id"] for c in cases[i::count]] for i in range(count)]


def prepare(base, directory, run_id, attempt, shards):
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=False)
    state = user.source_state()
    if state["dirty"]:
        raise ValueError("prepare requires clean committed source")
    base = git("rev-parse", base + "^{commit}")
    cases, _ = inputs()
    reference.require_collection_ancestor(ROOT, read(ROOT / REFERENCE)["collection_commit"], state["commit"])
    compute_base = compute.base_declares_cohort(base)
    if not compute_base:
        raise ValueError("performance plan requires a compute-ELF-capable base")
    subprocess.run(["git", "-C", str(ROOT), "cat-file", "-e", base + ":pkg/elfsim/run.go"], check=True)
    head_binary, base_binary = directory / "head", directory / "base"
    subprocess.run(["go", "build", "-o", str(head_binary), "./cmd/poros"], cwd=ROOT, check=True)
    with tempfile.TemporaryDirectory(prefix="poros-accuracy-base-") as temporary:
        tree = Path(temporary) / "source"
        subprocess.run(["git", "-C", str(ROOT), "worktree", "add", "--detach", str(tree), base], check=True)
        try:
            subprocess.run(["go", "build", "-o", str(base_binary), "./cmd/poros"], cwd=tree, check=True)
        finally:
            subprocess.run(["git", "-C", str(ROOT), "worktree", "remove", "--force", str(tree)], check=True)
    if user.source_state() != state:
        raise ValueError("source changed during prepare")
    plan = dict(schema=SCHEMA, run_id=str(run_id), prepared_attempt=positive(attempt),
                head=state, base_commit=base, compute_base_available=compute_base, base_capability=compute.base_capability(base),
                binaries={"head": user.digest(head_binary), "base": user.digest(base_binary)},
                references={"performance264": user.digest(ROOT / REFERENCE)},
                cases=cases, shards=partition(cases, shards))
    declarations = model_profiles.resolve(ROOT, state, base)
    if declarations is not None:
        plan["model_profiles"] = declarations
    if not plan["run_id"]:
        raise ValueError("run identity is required")
    write(directory / "plan.json", plan)
    return identity(plan)


def load_plan(directory, run_id, attempt):
    directory = Path(directory)
    plan = read(directory / "plan.json")
    if plan["schema"] != SCHEMA or plan["run_id"] != str(run_id):
        raise ValueError("plan schema/run mismatch")
    if positive(plan["prepared_attempt"]) > positive(attempt):
        raise ValueError("plan belongs to a future attempt")
    state = user.source_state()
    if state["dirty"] or plan["head"] != state:
        raise ValueError("plan source mismatch")
    site.sha(plan["base_commit"])
    if plan.get("model_profiles") != model_profiles.resolve(ROOT, state, plan["base_commit"]):
        raise ValueError("plan model declarations differ from source")
    cases, witnesses = inputs()
    if plan["cases"] != cases or plan["shards"] != partition(cases, len(plan["shards"])):
        raise ValueError("plan coverage/partition mismatch")
    if plan["base_capability"] != compute.base_capability(plan["base_commit"]):
        raise ValueError("plan base image capability mismatch")
    if plan["compute_base_available"] is not compute.base_declares_cohort(plan["base_commit"]):
        raise ValueError("plan base capability mismatch")
    if plan["references"] != {"performance264": user.digest(ROOT / REFERENCE)}:
        raise ValueError("plan reference mismatch")
    if set(plan["binaries"]) != {"head", "base"}:
        raise ValueError("plan binaries missing")
    for label, digest in plan["binaries"].items():
        path = site.regular(directory / label)
        if user.digest(path) != digest:
            raise ValueError("plan binary mismatch: " + label)
    return plan, witnesses


def roles(plan, case, witnesses):
    result = ["functional", "head"]
    if plan["compute_base_available"] and compute.case_base_status(case, plan["base_capability"]) == "available":
        result.append("base")
    return result


def validate_result(result, plan, case, role, golden):
    mode = "functional" if role == "functional" else None
    profile = plan.get("model_profiles", {}).get("base" if role == "base" else "head", {}).get("profile")
    user.validate_execution(result, case, mode, expected_identity=golden, expected_profile=profile)
    revision = plan["base_commit"] if role == "base" else plan["head"]["commit"]
    if result["build"].get("vcs.revision") != revision or result["build"].get("vcs.modified") != "false":
        raise ValueError("execution build provenance mismatch")
    if role != "functional":
        user.validate_timing(result, case)


def run_shard(directory, run_id, attempt, index, output):
    plan, witnesses = load_plan(directory, run_id, attempt)
    if type(index) is not int or not 0 <= index < len(plan["shards"]):
        raise ValueError("unknown shard")
    report = dict(schema=SCHEMA, plan_id=identity(plan), run_id=str(run_id),
                  attempt=attempt, shard=index, status="fail", records=[], errors=[],
                  started_at=time.time())
    write(output, report)
    cases = {c["id"]: c for c in plan["cases"]}
    for ident in plan["shards"][index]:
        case = cases[ident]
        record = dict(case_id=ident, executions={})
        report["records"].append(record)
        try:
            golden = load_witness(witnesses, ident)
            for role in roles(plan, case, witnesses):
                label = "base" if role == "base" else "head"
                options = {"expected_profile": plan["model_profiles"][label]["profile"]} if "model_profiles" in plan else {}
                result = user.execute(Path(directory) / label, case,
                                      mode="functional" if role == "functional" else None,
                                      expected_identity=golden, **options)
                validate_result(result, plan, case, role, golden)
                record["executions"][role] = result
            print("accuracy shard", index, ident, "pass", flush=True)
        except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError) as exc:
            report["errors"].append(dict(case_id=ident, error=str(exc)))
            print("accuracy shard", index, ident, "FAIL", str(exc), flush=True)
        write(output, report)
    # Repeat identity checks after execution; a changed binary/source invalidates
    # the whole shard even when its individual executions appeared successful.
    load_plan(directory, run_id, attempt)
    report["status"] = "fail" if report["errors"] else "pass"
    report["completed_at"] = time.time()
    write(output, report)
    return 0 if report["status"] == "pass" else 1


def select_shards(plan, reports, attempt):
    """A failed-job rerun can reuse older successful shards of this exact plan.

    A newer failure supersedes an older success. Whole reruns create a new plan
    and workflow download patterns isolate it from older prepared artifacts.
    """
    selected, seen = {}, set()
    for report in reports:
        if report["schema"] != SCHEMA or report["plan_id"] != identity(plan) or report["run_id"] != plan["run_id"]:
            raise ValueError("foreign shard identity")
        index, recorded_attempt = report["shard"], report["attempt"]
        if type(index) is not int or not 0 <= index < len(plan["shards"]):
            raise ValueError("unexpected shard")
        if not plan["prepared_attempt"] <= positive(recorded_attempt) <= positive(attempt):
            raise ValueError("shard attempt outside current plan/run")
        if (index, recorded_attempt) in seen:
            raise ValueError("duplicate shard attempt")
        seen.add((index, recorded_attempt))
        if index not in selected or selected[index]["attempt"] < recorded_attempt:
            selected[index] = report
    if set(selected) != set(range(len(plan["shards"]))):
        raise ValueError("missing required shards")
    for index, report in selected.items():
        if report["status"] != "pass" or report["errors"] or not report.get("completed_at"):
            raise ValueError("shard failed or incomplete: " + str(index))
        if [r["case_id"] for r in report["records"]] != plan["shards"][index]:
            raise ValueError("missing/duplicate/unexpected shard cases")
    return selected


def recorded_executor(plan, selected, witnesses, directory):
    cases = {c["id"]: c for c in plan["cases"]}
    records = {}
    for report in selected.values():
        for record in report["records"]:
            case = cases[record["case_id"]]
            expected_roles = roles(plan, case, witnesses)
            if set(record["executions"]) != set(expected_roles):
                raise ValueError("missing/extra execution roles")
            golden = load_witness(witnesses, case["id"])
            for role, result in record["executions"].items():
                validate_result(result, plan, case, role, golden)
                records[(case["id"], role)] = result
    remaining = set(records)

    def execute(binary, case, mode=None, expected_output=None, expected_identity=None, expected_profile=None):
        label = "head" if Path(binary) == Path(directory) / "head" else "base" if Path(binary) == Path(directory) / "base" else None
        role = "functional" if mode == "functional" and label == "head" else label
        key = (case["id"], role)
        witness = witnesses.get(case["id"])
        output_matches = expected_output is None and expected_identity == witness
        if label is None or case != cases[case["id"]] or not output_matches:
            raise ValueError("recorded execution request mismatch")
        if expected_profile != plan.get("model_profiles", {}).get(label, {}).get("profile") and mode != "functional":
            raise ValueError("recorded model profile request mismatch")
        if key not in remaining:
            raise ValueError("missing or reused execution")
        remaining.remove(key)
        return copy.deepcopy(records[key])

    return execute, remaining


def aggregate(directory, results, run_id, attempt, output, shard_status="success"):
    output = Path(output)
    # Create a valid failure artifact even when prepare/download was unavailable.
    site.failure(output.parent, git("rev-parse", "HEAD"), "Parallel accuracy aggregation incomplete")
    try:
        if shard_status != "success":
            raise ValueError("upstream shard jobs did not succeed: " + shard_status)
        plan, witnesses = load_plan(directory, run_id, attempt)
        paths = sorted(Path(results).rglob("shard.json"))
        selected = select_shards(plan, [read(path) for path in paths], attempt)
        executor, remaining = recorded_executor(plan, selected, witnesses, directory)
        args = SimpleNamespace(head=str(Path(directory) / "head"), base=str(Path(directory) / "base"),
                               base_unavailable=False, base_commit=plan["base_commit"],
                               reference=str(ROOT / REFERENCE),
                               output=str(output))
        status = user.compare(args, executor)
        if status:
            # The shared comparator already wrote its detailed failure report.
            return status
        if remaining:
            raise ValueError("executions remained unused")
        load_plan(directory, run_id, attempt)
        write(output.parent / "shard-summary.json", dict(plan_id=identity(plan), run_id=run_id, attempt=attempt,
              shards=[dict(shard=i, attempt=r["attempt"], cases=len(r["records"]),
                           elapsed_seconds=r["completed_at"] - r["started_at"])
                      for i, r in sorted(selected.items())]))
        return 0
    except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError) as exc:
        site.failure(output.parent, git("rev-parse", "HEAD"), "Parallel accuracy aggregation failed: " + str(exc))
        return 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    for name in ("prepare", "shard", "aggregate"):
        p = sub.add_parser(name)
        p.add_argument("--prepared", required=True)
        p.add_argument("--run-id", required=True)
        p.add_argument("--attempt", type=int, required=True)
        if name == "prepare":
            p.add_argument("--base", required=True)
            p.add_argument("--shards", type=int, default=132)
        else:
            p.add_argument("--output", required=True)
        if name == "shard":
            p.add_argument("--index", type=int, required=True)
        if name == "aggregate":
            p.add_argument("--results", required=True)
            p.add_argument("--shard-status", default="success")
    args = parser.parse_args()
    if args.command == "prepare":
        plan_id = prepare(args.base, args.prepared, args.run_id, args.attempt, args.shards)
        if os.environ.get("GITHUB_OUTPUT"):
            with open(os.environ["GITHUB_OUTPUT"], "a") as out:
                out.write("plan_id=" + plan_id + "\n")
        print(plan_id)
        return 0
    if args.command == "shard":
        return run_shard(args.prepared, args.run_id, args.attempt, args.index, args.output)
    return aggregate(args.prepared, args.results, args.run_id, args.attempt, args.output, args.shard_status)


if __name__ == "__main__":
    raise SystemExit(main())
