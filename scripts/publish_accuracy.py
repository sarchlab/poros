#!/usr/bin/env python3
"""Publish trusted main/release accuracy artifacts to the configured Pages branch."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time

import accuracy_site as site

REPO = site.REPO


def run(*args, cwd=None, check=True):
    return subprocess.run(args, cwd=cwd, text=True, capture_output=True, check=check)


def api(path, method="GET"):
    return json.loads(run("gh", "api", "--method", method, path).stdout)


def verify_source(source, bundle, commit):
    report, manifest = site.validate_bundle(bundle, commit)
    if report["artifact_status"] != "pass":
        return
    paths = site.git(source, "ls-tree", "-r", "--name-only", commit).splitlines()
    if set(report["head"]["files"]) != set(paths):
        raise ValueError("report source inventory differs from the recorded commit")
    for path in paths:
        data = subprocess.check_output(["git", "-C", str(source), "show", commit + ":" + path])
        if site.hashlib.sha256(data).hexdigest() != report["head"]["files"][path]:
            raise ValueError("report source hash differs from commit: " + path)
    bindings = {"reference.json": "evaluation/usertime/m2-linuxvm-user-v1.json", "manifest.json": "tests/usertime/manifest.json", "native_usertime.py": "scripts/native_usertime.py"}
    bindings.update({n: "tests/usertime/" + n[7:] for n in manifest["files"] if n.startswith("inputs/")})
    for artifact, path in bindings.items():
        if site.digest(Path(bundle) / artifact) != report["head"]["files"][path]:
            raise ValueError("bundled evidence differs from source commit")


def assert_event(event, event_name):
    """Do not trust arbitrary workflow artifacts or PR heads in a write job."""
    if event.get("repository", {}).get("full_name") != REPO:
        raise ValueError("unexpected repository")
    if event_name == "workflow_run":
        workflow = event["workflow_run"]
        if workflow["event"] != "push" or workflow["head_branch"] != "main" or workflow["head_repository"]["full_name"] != REPO or workflow["status"] != "completed":
            raise ValueError("only completed main push evaluations may publish")
        canonical = api(f'repos/{REPO}/actions/workflows/evaluation.yml')
        if workflow["workflow_id"] != canonical["id"] or canonical["path"] != ".github/workflows/evaluation.yml":
            raise ValueError("unexpected source workflow")
        actual = api(f'repos/{REPO}/actions/runs/{workflow["id"]}/attempts/{workflow["run_attempt"]}')
        for key in ("head_sha", "head_branch", "event", "workflow_id", "conclusion", "run_attempt"):
            if actual[key] != workflow[key]:
                raise ValueError("workflow attempt identity changed")
        return {"commit": site.sha(workflow["head_sha"]), "kind": "main", "conclusion": workflow["conclusion"], "run_id": workflow["id"], "run_attempt": workflow["run_attempt"], "run_url": workflow["html_url"] + '/attempts/' + str(workflow["run_attempt"])}
    if event_name == "release":
        release = event["release"]
        if event["action"] != "published" or release["draft"]:
            raise ValueError("only published releases may be pinned")
        return {"kind": "release", "release": {"id": release["id"], "tag": release["tag_name"]}}
    if event_name == "workflow_dispatch":
        if os.environ.get("GITHUB_REF") != "refs/heads/main":
            raise ValueError("manual publication must run from main")
        return {"kind": "main"}
    raise ValueError("unsupported publication event")


def wait_pages(expected, checkout, timeout=600):
    response = api(f"repos/{REPO}/pages/builds", "POST")
    path = response["url"]
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        build = api(path)
        if build["status"] == "errored":
            raise ValueError("Pages build failed: " + str(build.get("error")))
        if build["status"] == "built":
            # A queued build may include a newer publication; it must include ours.
            run("git", "fetch", "origin", "gh-pages", cwd=checkout)
            if run("git", "merge-base", "--is-ancestor", expected, build["commit"], cwd=checkout, check=False).returncode:
                raise ValueError("Pages built an unrelated or older commit")
            return build
        time.sleep(10)
    raise TimeoutError("Pages build did not finish within 10 minutes")


def publish(source, bundle, metadata, destination=None, build=True):
    source = Path(source).resolve()
    verify_source(source, bundle, metadata["commit"])
    if build:
        configuration = api(f"repos/{REPO}/pages")
        if configuration["build_type"] != "legacy" or configuration["source"] != {"branch": "gh-pages", "path": "/"}:
            raise ValueError("Pages must already be configured for gh-pages / (root)")
    run("git", "fetch", "origin", "main", "gh-pages", cwd=source)
    remote = destination or site.git(source, "remote", "get-url", "origin")
    with tempfile.TemporaryDirectory(prefix="poros-pages-") as temporary:
        checkout = Path(temporary)
        run("git", "clone", "--single-branch", "--branch", "gh-pages", remote, str(checkout))
        run("git", "config", "user.name", "github-actions[bot]", cwd=checkout)
        run("git", "config", "user.email", "41898282+github-actions[bot]@users.noreply.github.com", cwd=checkout)
        # queue:max serializes normal runs. Optimistic retries also protect manual writers.
        for attempt in range(4):
            run("git", "fetch", "origin", "main", cwd=source)
            main_order = site.git(source, "rev-list", "--first-parent", "origin/main").splitlines()
            site.archive(checkout, bundle, metadata, main_order)
            run("git", "add", "--", "calibration", ".nojekyll", "index.html", cwd=checkout)
            if run("git", "diff", "--cached", "--quiet", cwd=checkout, check=False).returncode:
                run("git", "commit", "-m", "Publish accuracy evidence for " + metadata["commit"][:12], cwd=checkout)
            pushed = run("git", "push", "origin", "HEAD:gh-pages", cwd=checkout, check=False)
            if pushed.returncode == 0:
                break
            if attempt == 3:
                raise RuntimeError("Pages branch push failed: " + pushed.stderr)
            run("git", "fetch", "origin", "gh-pages", cwd=checkout)
            run("git", "reset", "--hard", "origin/gh-pages", cwd=checkout)
        commit = site.git(checkout, "rev-parse", "HEAD")
        if build:
            wait_pages(commit, checkout)
        return commit


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", default=".")
    parser.add_argument("--bundle", required=True)
    parser.add_argument("--event", default=os.environ.get("GITHUB_EVENT_PATH"))
    parser.add_argument("--event-name", default=os.environ.get("GITHUB_EVENT_NAME"))
    parser.add_argument("--build-conclusion", default="failure")
    args = parser.parse_args()
    event = site.read(args.event)
    metadata = assert_event(event, args.event_name)
    source, bundle = Path(args.source).resolve(), Path(args.bundle).resolve()
    if args.event_name == "workflow_run":
        downloaded = run("gh", "run", "download", str(metadata["run_id"]), "--repo", REPO, "--name", "user-time-accuracy", "--dir", str(bundle), check=False)
        if downloaded.returncode or not (bundle / "bundle.json").is_file():
            bundle.mkdir(parents=True, exist_ok=True)
            site.failure(bundle, metadata["commit"], "Evaluation did not produce an accuracy bundle; inspect its run logs.")
            site.bundle(bundle / "report.json", source, bundle / "missing", metadata["commit"])
            bundle = bundle / "missing"
        else:
            identity = site.read(bundle / "bundle.json")
            if str(identity.get("run_id")) != str(metadata["run_id"]) or str(identity.get("run_attempt")) != str(metadata["run_attempt"]):
                raise ValueError("artifact came from a different workflow attempt")
    else:
        run("git", "fetch", "origin", "main", "--tags", cwd=source)
        ref = 'refs/tags/' + metadata["release"]["tag"] if metadata["kind"] == "release" else os.environ["GITHUB_SHA"]
        metadata["commit"] = site.sha(site.git(source, "rev-parse", "--verify", ref + "^{commit}"))
        metadata.update(conclusion=args.build_conclusion, run_url=f'https://github.com/{REPO}/actions/runs/{os.environ["GITHUB_RUN_ID"]}/attempts/{os.environ["GITHUB_RUN_ATTEMPT"]}')
        if metadata["kind"] == "release":
            metadata["release"]["publication_run_url"] = metadata["run_url"]
        # The build job records its exact target; a moving tag/main must not mislabel it.
        if site.read(bundle / "bundle.json")["commit"] != metadata["commit"]:
            raise ValueError("target moved while its report was built; rerun publication")
    result = publish(source, bundle, metadata)
    print("Published gh-pages commit " + result)
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write(f'## Accuracy dashboard\n\n[View report](https://sarchlab.github.io/poros/calibration/runs/{metadata["commit"]}/)\n\nPages commit: `{result}`\n')


if __name__ == "__main__":
    main()
