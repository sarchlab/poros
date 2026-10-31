#!/usr/bin/env python3
"""Publish trusted main/release accuracy artifacts to the configured Pages branch."""

import argparse
import json
import os
import re
from pathlib import Path
import hashlib
import subprocess
import tempfile
import time
from urllib.parse import quote

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
        data = subprocess.check_output(
            ["git", "-C", str(source), "show", commit + ":" + path]
        )
        if hashlib.sha256(data).hexdigest() != report["head"]["files"][path]:
            raise ValueError("report source hash differs from commit: " + path)
    original = "cohorts/original/" if report["schema"] == 3 else ""
    inputs = site.read(Path(bundle) / (original + "manifest.json"))
    bindings = {
        original + "reference.json": "evaluation/usertime/"
        + site.safe_name(inputs.get("reference_file", "m2-linuxvm-user-v1.json")),
        original + "manifest.json": "tests/usertime/manifest.json",
        original + "native_usertime.py": "scripts/native_usertime.py",
    }
    if inputs["schema"] == 2:
        bindings[original + "workload_contract.py"] = "scripts/workload_contract.py"
    input_prefix = original + "inputs/"
    for name in manifest["files"]:
        if name.startswith(input_prefix):
            bindings[name] = "tests/usertime/" + name[len(input_prefix):]
        elif report["schema"] == 3 and name.startswith("cohorts/compute/source/"):
            bindings[name] = name[len("cohorts/compute/source/"):]
        elif report["schema"] == 3 and name.startswith("cohorts/compute/reference/"):
            bindings[name] = site.compute.REFERENCE + "/" + name[len("cohorts/compute/reference/"):]
    for artifact, path in bindings.items():
        if site.digest(Path(bundle) / artifact) != report["head"]["files"][path]:
            raise ValueError("bundled evidence differs from source commit")


def evaluation_metadata(workflow):
    identity = (workflow["id"], workflow["run_attempt"])
    return {
        "evaluation_run_id": identity[0],
        "evaluation_run_attempt": identity[1],
        "evaluation_conclusion": workflow.get("conclusion") or workflow["status"],
        "evaluation_run_url": f"https://github.com/{REPO}/actions/runs/{identity[0]}/attempts/{identity[1]}",
    }


def published_release_target(tag):
    if not isinstance(tag, str) or not tag or len(tag) > 256:
        raise ValueError("invalid release tag")
    release = api(f"repos/{REPO}/releases/tags/{quote(tag, safe='')}")
    if (
        release["draft"]
        or not release.get("published_at")
        or release["tag_name"] != tag
    ):
        raise ValueError("release backfill requires an existing published release")
    ref = api(f"repos/{REPO}/git/ref/tags/{quote(tag, safe='')}")
    if ref["ref"] != "refs/tags/" + tag:
        raise ValueError("release tag ref mismatch")
    target = ref["object"]
    for _ in range(10):
        commit = site.sha(target["sha"])
        if target["type"] == "commit":
            return {
                "kind": "release",
                "commit": commit,
                "release": {"id": release["id"], "tag": tag},
            }
        if target["type"] != "tag":
            break
        target = api(f"repos/{REPO}/git/tags/{commit}")["object"]
    raise ValueError("release tag does not resolve to a commit within ten tag objects")


def assert_event(event, event_name):
    """Do not trust arbitrary workflow artifacts or PR heads in a write job."""
    if event.get("repository", {}).get("full_name") != REPO:
        raise ValueError("unexpected repository")
    if event_name == "workflow_run":
        workflow = event["workflow_run"]
        if (
            workflow["event"] != "push"
            or workflow["head_branch"] != "main"
            or workflow["head_repository"]["full_name"] != REPO
            or workflow["status"] != "completed"
        ):
            raise ValueError("only completed main push evaluations may publish")
        canonical = api(f"repos/{REPO}/actions/workflows/evaluation.yml")
        if (
            workflow["workflow_id"] != canonical["id"]
            or canonical["path"] != ".github/workflows/evaluation.yml"
        ):
            raise ValueError("unexpected source workflow")
        actual = api(
            f'repos/{REPO}/actions/runs/{workflow["id"]}/attempts/{workflow["run_attempt"]}'
        )
        for key in (
            "head_sha",
            "head_branch",
            "event",
            "workflow_id",
            "conclusion",
            "run_attempt",
        ):
            if actual[key] != workflow[key]:
                raise ValueError("workflow attempt identity changed")
        return {
            "commit": site.sha(workflow["head_sha"]),
            "kind": "main",
            "conclusion": workflow["conclusion"],
            "run_id": workflow["id"],
            "run_attempt": workflow["run_attempt"],
            "run_url": workflow["html_url"]
            + "/attempts/"
            + str(workflow["run_attempt"]),
            **evaluation_metadata(workflow),
        }
    if event_name == "release":
        release = event["release"]
        if event["action"] != "published" or release["draft"]:
            raise ValueError("only published releases may be pinned")
        target = published_release_target(release["tag_name"])
        if target["release"]["id"] != release["id"]:
            raise ValueError("release identity changed since the publication event")
        if target["commit"] != site.sha(os.environ["GITHUB_SHA"]):
            raise ValueError("release tag moved since the publication event")
        return target
    if event_name == "workflow_dispatch":
        if os.environ.get("GITHUB_REF") != "refs/heads/main":
            raise ValueError("manual publication must run from main")
        tag = event.get("inputs", {}).get("release_tag", "")
        if tag:
            return published_release_target(tag)
        return {"kind": "main", "commit": site.sha(os.environ["GITHUB_SHA"])}
    raise ValueError("unsupported publication event")


def choose_artifact(artifacts, attempt, prefix="user-time-accuracy"):
    """A failed-jobs rerun may legitimately reuse an earlier same-run capture."""
    candidates = []
    for artifact in artifacts:
        match = re.fullmatch(re.escape(prefix) + r"-([1-9][0-9]*)", artifact["name"])
        if match and not artifact["expired"] and int(match.group(1)) <= attempt:
            candidates.append((int(match.group(1)), artifact))
    return max(candidates, key=lambda item: item[0], default=(None, None))


def download_attempt_bundle(run_id, attempt, prefix, destination):
    artifacts = []
    page = 1
    while True:
        batch = api(
            f"repos/{REPO}/actions/runs/{run_id}/artifacts?per_page=100&page={page}"
        )
        artifacts.extend(batch["artifacts"])
        if len(batch["artifacts"]) < 100:
            break
        page += 1
    evidence_attempt, artifact = choose_artifact(artifacts, attempt, prefix)
    if artifact:
        downloaded = run(
            "gh",
            "run",
            "download",
            str(run_id),
            "--repo",
            REPO,
            "--name",
            artifact["name"],
            "--dir",
            str(destination),
            check=False,
        )
        if downloaded.returncode == 0 and (destination / "bundle.json").is_file():
            return evidence_attempt
    return None


def evaluation_for_commit(commit):
    response = api(
        f"repos/{REPO}/actions/workflows/evaluation.yml/runs?branch=main&event=push&head_sha={commit}&per_page=1"
    )
    runs = response["workflow_runs"]
    if not runs:
        return {"evaluation_conclusion": "not checked"}
    workflow = runs[0]
    if (
        workflow["head_sha"] != commit
        or workflow["head_branch"] != "main"
        or workflow["event"] != "push"
        or workflow["head_repository"]["full_name"] != REPO
    ):
        raise ValueError("full Evaluation proof does not match the target")
    return evaluation_metadata(workflow)


def wait_pages(expected, checkout, timeout=600):
    response = api(f"repos/{REPO}/pages/builds", "POST")
    path = response["url"]
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        build = api(path)
        if build["status"] in ("built", "errored") and build.get("commit"):
            # /builds/latest may still expose the previous build immediately
            # after POST. Only a build containing our commit can settle this run.
            run("git", "fetch", "origin", "gh-pages", cwd=checkout)
            contains = (
                run(
                    "git",
                    "merge-base",
                    "--is-ancestor",
                    expected,
                    site.sha(build["commit"]),
                    cwd=checkout,
                    check=False,
                ).returncode
                == 0
            )
            if contains:
                if build["status"] == "errored":
                    raise ValueError("Pages build failed: " + str(build.get("error")))
                return build
        time.sleep(10)
    raise TimeoutError(f"Pages build did not finish within {timeout} seconds")


def publish(source, bundle, metadata, destination=None, build=True):
    source = Path(source).resolve()
    verify_source(source, bundle, metadata["commit"])
    if build:
        configuration = api(f"repos/{REPO}/pages")
        if configuration["build_type"] != "legacy" or configuration["source"] != {
            "branch": "gh-pages",
            "path": "/",
        }:
            raise ValueError("Pages must already be configured for gh-pages / (root)")
    run("git", "fetch", "origin", "main", "gh-pages", cwd=source)
    remote = destination or site.git(source, "remote", "get-url", "origin")
    with tempfile.TemporaryDirectory(prefix="poros-pages-") as temporary:
        checkout = Path(temporary)
        run(
            "git",
            "clone",
            "--single-branch",
            "--branch",
            "gh-pages",
            remote,
            str(checkout),
        )
        run("git", "config", "user.name", "github-actions[bot]", cwd=checkout)
        run(
            "git",
            "config",
            "user.email",
            "41898282+github-actions[bot]@users.noreply.github.com",
            cwd=checkout,
        )
        # queue:max serializes normal runs. Optimistic retries also protect manual writers.
        for attempt in range(4):
            run("git", "fetch", "origin", "main", cwd=source)
            main_order = site.git(
                source, "rev-list", "--first-parent", "origin/main"
            ).splitlines()
            site.archive(checkout, bundle, metadata, main_order)
            run(
                "git",
                "add",
                "--",
                "calibration",
                ".nojekyll",
                "index.html",
                cwd=checkout,
            )
            if run(
                "git", "diff", "--cached", "--quiet", cwd=checkout, check=False
            ).returncode:
                run(
                    "git",
                    "commit",
                    "-m",
                    "Publish accuracy evidence for " + metadata["commit"][:12],
                    cwd=checkout,
                )
            pushed = run(
                "git", "push", "origin", "HEAD:gh-pages", cwd=checkout, check=False
            )
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
    parser.add_argument("--bundle")
    parser.add_argument(
        "--resolve-target",
        help="Write a read-only release/main target resolution for the build job",
    )
    parser.add_argument("--event", default=os.environ.get("GITHUB_EVENT_PATH"))
    parser.add_argument("--event-name", default=os.environ.get("GITHUB_EVENT_NAME"))
    parser.add_argument("--build-conclusion", default="failure")
    args = parser.parse_args()
    event = site.read(args.event)
    metadata = assert_event(event, args.event_name)
    if args.resolve_target:
        if args.event_name == "workflow_run":
            raise ValueError("workflow_run uses its existing evaluation artifact")
        site.write(args.resolve_target, metadata)
        if os.environ.get("GITHUB_OUTPUT"):
            with open(os.environ["GITHUB_OUTPUT"], "a") as output:
                output.write(f'commit={metadata["commit"]}\nkind={metadata["kind"]}\n')
        return
    if not args.bundle:
        parser.error("--bundle is required for publication")
    source, bundle = Path(args.source).resolve(), Path(args.bundle).resolve()
    if args.event_name == "workflow_run":
        evidence_attempt = download_attempt_bundle(
            metadata["run_id"], metadata["run_attempt"], "user-time-accuracy", bundle
        )
        if evidence_attempt is None:
            bundle.mkdir(parents=True, exist_ok=True)
            site.failure(
                bundle,
                metadata["commit"],
                "Evaluation did not produce an accuracy bundle; inspect its run logs.",
            )
            site.bundle(
                bundle / "report.json", source, bundle / "missing", metadata["commit"]
            )
            bundle = bundle / "missing"
        else:
            identity = site.read(bundle / "bundle.json")
            if str(identity.get("run_id")) != str(metadata["run_id"]) or str(
                identity.get("run_attempt")
            ) != str(evidence_attempt):
                raise ValueError("artifact came from a different workflow attempt")
            metadata["evidence_run_url"] = (
                f'https://github.com/{REPO}/actions/runs/{metadata["run_id"]}/attempts/{evidence_attempt}'
            )
    else:
        if (
            download_attempt_bundle(
                int(os.environ["GITHUB_RUN_ID"]),
                int(os.environ["GITHUB_RUN_ATTEMPT"]),
                "release-accuracy-evidence",
                bundle,
            )
            is None
        ):
            raise ValueError(
                "release/manual build did not produce a retained evidence bundle"
            )
        run("git", "fetch", "origin", "main", "--tags", cwd=source)
        resolved = site.read(bundle / "publication-target.json")
        if resolved != metadata:
            raise ValueError("release/main target identity changed during the build")
        metadata.update(
            conclusion=args.build_conclusion,
            run_id=int(os.environ["GITHUB_RUN_ID"]),
            run_attempt=int(os.environ["GITHUB_RUN_ATTEMPT"]),
            run_url=f'https://github.com/{REPO}/actions/runs/{os.environ["GITHUB_RUN_ID"]}/attempts/{os.environ["GITHUB_RUN_ATTEMPT"]}',
        )
        metadata.update(evaluation_for_commit(metadata["commit"]))
        if metadata["kind"] == "release":
            metadata["release"]["publication_run_url"] = metadata["run_url"]
        # The build job records its exact target; a moving tag/main must not mislabel it.
        if site.read(bundle / "bundle.json")["commit"] != metadata["commit"]:
            raise ValueError(
                "target moved while its report was built; rerun publication"
            )
    result = publish(source, bundle, metadata)
    print("Published gh-pages commit " + result)
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write(
                f'## Accuracy dashboard\n\n[View report](https://sarchlab.github.io/poros/calibration/runs/{metadata["commit"]}/)\n\nPages commit: `{result}`\n'
            )


if __name__ == "__main__":
    main()
