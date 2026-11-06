#!/usr/bin/env python3
"""Route exact-target backfills and package one explicitly selected evidence path."""
import argparse
from pathlib import Path
import os
import shutil

import accuracy_site as site
import publish_accuracy as publisher


def route(source, pages, output, target):
    source, pages, output = Path(source), Path(pages), Path(output)
    commit = site.sha(target["commit"])
    if site.git(source, "rev-parse", "HEAD") != commit:
        raise ValueError("backfill source differs from resolved target")
    archived = pages / "calibration/runs" / commit
    if target["kind"] == "release" and archived.exists() and site.reusable_bundle(archived, commit):
        shutil.copytree(archived, output / "evidence")
        selected = "reuse"
    elif (source / "tests/usertime/performance.json").exists():
        performance = site.read(source / "tests/usertime/performance.json")
        if performance.get("suite") != "whole-program-performance-six-point-v1" or performance.get("schema") != 2:
            raise ValueError("unknown performance backfill protocol")
        if not (source / "scripts/accuracy_parallel.py").is_file():
            raise ValueError("performance target lacks parallel execution protocol")
        selected = "parallel"
    else:
        selected = "legacy"
    record = dict(schema=1, target=target, route=selected, base=site.git(source, "rev-parse", "HEAD^"))
    site.write(output / "backfill.json", record)
    return record


def package(source, resolved, evidence, output, expected_commit, route_status="success"):
    source, resolved, evidence, output = map(Path, (source, resolved, evidence, output))
    record = site.read(resolved / "backfill.json")
    if record.get("schema") != 1 or record.get("route") not in ("reuse", "parallel", "legacy") or record["target"]["commit"] != site.sha(expected_commit):
        raise ValueError("backfill route/target mismatch")
    if site.git(source, "rev-parse", "HEAD") != expected_commit:
        raise ValueError("package source differs from exact target")
    selected = resolved / "evidence" if record["route"] == "reuse" else evidence
    if route_status != "success" or not (selected / "bundle.json").is_file():
        # Preserve an explicit failed result; never rerun another execution route.
        failure = output.parent / "backfill-failure"
        site.failure(failure, expected_commit, record["route"] + " backfill (" + route_status + ") did not produce a complete evidence bundle; inspect workflow logs")
        site.bundle(failure / "report.json", source, output, expected_commit)
    else:
        publisher.verify_source(source, selected, expected_commit)
        shutil.copytree(selected, output)
    site.write(output / "publication-target.json", record["target"])
    return 0 if site.read(output / "report.json")["artifact_status"] == "pass" else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    p = sub.add_parser("route")
    p.add_argument("--source", required=True)
    p.add_argument("--pages", required=True)
    p.add_argument("--output", required=True)
    p.add_argument("--target", required=True)
    p = sub.add_parser("package")
    p.add_argument("--source", required=True)
    p.add_argument("--resolved", required=True)
    p.add_argument("--evidence", required=True)
    p.add_argument("--output", required=True)
    p.add_argument("--commit", required=True)
    p.add_argument("--run-id", type=int, required=True)
    p.add_argument("--attempt", type=int, required=True)
    p.add_argument("--route-status", required=True)
    args = parser.parse_args()
    if args.command == "route":
        result = route(args.source, args.pages, args.output, site.read(args.target))
        if os.environ.get("GITHUB_OUTPUT"):
            with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
                stream.write(f'route={result["route"]}\nbase={result["base"]}\n')
    else:
        attempt = publisher.download_attempt_bundle(args.run_id, args.attempt, "accuracy-backfill-resolved", Path(args.resolved), required="backfill.json")
        if attempt is None:
            raise ValueError("missing resolved backfill target artifact")
        record = site.read(Path(args.resolved) / "backfill.json")
        if record["route"] != "reuse":
            prefix = "user-time-accuracy" if record["route"] == "parallel" else "accuracy-backfill-legacy"
            publisher.download_attempt_bundle(args.run_id, args.attempt, prefix, Path(args.evidence))
        raise SystemExit(package(args.source, args.resolved, args.evidence, args.output, args.commit, args.route_status))


if __name__ == "__main__":
    main()
