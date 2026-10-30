#!/usr/bin/env python3
"""Archive retention, provenance, failure visibility and publisher boundary tests."""
import copy
import json
import os
import shutil
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import accuracy_site as site
import publish_accuracy as publisher

ROOT = Path(__file__).resolve().parents[1]


def revision(n):
    return f"{n:040x}"


class AccuracySiteTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.pages = self.root / "pages"
        self.pages.mkdir()
        (self.pages / "README.md").write_text("unrelated source is preserved")
        self.counter = 0

    def bundle(self, commit, successful=True, error=10):
        self.counter += 1
        raw, output = self.root / f"raw{self.counter}", self.root / f"bundle{self.counter}"
        raw.mkdir()
        if not successful:
            site.failure(raw, commit, 'Build failed <script>alert("bad")</script>')
        else:
            manifest = site.read(ROOT / "tests/usertime/manifest.json")
            reference = site.read(ROOT / "evaluation/usertime/m2-linuxvm-user-v1.json")
            paths = ["scripts/native_usertime.py", "tests/usertime/manifest.json", "evaluation/usertime/m2-linuxvm-user-v1.json"]
            paths += ["tests/usertime/" + n for n in set(manifest["sources"]) | {c["file"] for c in manifest["cases"]}]
            cases = []
            for c, ref in zip(manifest["cases"], reference["cases"]):
                prediction = {"predicted_user_seconds": ref["median_user_seconds"] * 1.1, "instructions": 100, "cycles": 110, **manifest["profile"]}
                if ref["quality"] == "usable":
                    prediction.update(error_percent=error, direction="slower")
                cases.append({"id": c["id"], "split": c["split"], "reference_user_seconds": ref["median_user_seconds"], "reference_quality": ref["quality"], "head": prediction, "base": copy.deepcopy(prediction), **({"error_change_percentage_points": 0} if ref["quality"] == "usable" else {})})
            summary = {}
            for split in ("calibration", "held_out"):
                rows = [c for c in cases if c["split"] == split]
                count = sum("error_percent" in c["head"] for c in rows)
                summary[split] = {"required": len(rows), "comparable": count}
                if count:
                    summary[split].update({label: {"mean_error_percent": error, "maximum_error_percent": error} for label in ("head", "base")})
            report = {"schema": 1, "artifact_status": "pass", "accuracy_status": "unverified", "formula": site.FORMULA, "head": {"commit": commit, "dirty": False, "files": {p: site.digest(ROOT / p) for p in paths}}, "base_commit": revision(0), "reference_sha256": site.digest(ROOT / "evaluation/usertime/m2-linuxvm-user-v1.json"), "reference": reference, "cases": cases, "summary": summary}
            site.write(raw / "report.json", report)
            (raw / "report.md").write_text("# Test report\n")
        site.bundle(raw / "report.json", ROOT, output, commit)
        return output

    def metadata(self, commit, kind="main", conclusion="success", run=1, attempt=1):
        return {"commit": commit, "kind": kind, "conclusion": conclusion, "evaluation_conclusion": conclusion, "run_id": run, "run_attempt": attempt, "run_url": f"https://github.com/sarchlab/poros/actions/runs/{run}/attempts/{attempt}", **({"release": {"id": run, "tag": "v1.0"}} if kind == "release" else {})}

    def test_keep_twenty_plus_releases_and_preserve_unrelated(self):
        order = [revision(n) for n in range(25, 0, -1)]
        bundles = {}
        for n in range(1, 26):
            commit = revision(n)
            bundles[n] = self.bundle(commit)
            site.archive(self.pages, bundles[n], self.metadata(commit, run=n), order)
            if n == 1:
                site.archive(self.pages, bundles[n], self.metadata(commit, "release", run=101), order)
        index = site.read(self.pages / "calibration/index.json")
        self.assertEqual(len(index["entries"]), 21)
        self.assertEqual(index["latest_success"], revision(25))
        self.assertTrue((self.pages / "calibration/runs" / revision(1) / "reference.json").exists())
        self.assertFalse((self.pages / "calibration/runs" / revision(2)).exists())
        self.assertEqual((self.pages / "README.md").read_text(), "unrelated source is preserved")

    def test_out_of_order_does_not_move_latest_back(self):
        order = [revision(3), revision(2), revision(1)]
        for n in (3, 1, 2):
            site.archive(self.pages, self.bundle(revision(n)), self.metadata(revision(n), run=n), order)
        self.assertEqual(site.read(self.pages / "calibration/index.json")["latest_success"], revision(3))

    def test_failed_ci_with_valid_artifact_can_recover_without_rewriting_report(self):
        commit = revision(1)
        bundle = self.bundle(commit)
        site.archive(self.pages, bundle, self.metadata(commit, conclusion="failure"), [commit])
        original = (self.pages / "calibration/runs" / commit / "report.json").read_bytes()
        self.assertIsNone(site.read(self.pages / "calibration/index.json")["latest_success"])
        changed = self.bundle(commit, error=20)
        site.archive(self.pages, changed, self.metadata(commit, attempt=2), [commit])
        index = site.archive(self.pages, bundle, self.metadata(commit, conclusion="failure"), [commit])
        self.assertEqual(index["latest_success"], commit)
        self.assertEqual(index["entries"][0]["conclusion"], "success")
        self.assertEqual((self.pages / "calibration/runs" / commit / "report.json").read_bytes(), original)

    def test_failed_report_recovery_and_release_pin_idempotent(self):
        commit = revision(1)
        site.archive(self.pages, self.bundle(commit, False), self.metadata(commit, conclusion="failure"), [commit])
        bundle = self.bundle(commit)
        site.archive(self.pages, bundle, self.metadata(commit, attempt=2), [commit])
        metadata = self.metadata(commit, "release", run=20)
        site.archive(self.pages, bundle, metadata, [commit])
        index = site.archive(self.pages, bundle, metadata, [commit])
        self.assertEqual(index["entries"][0]["releases"], [metadata["release"]])
        self.assertEqual(index["latest_success"], commit)

    def test_failure_missing_and_noisy_are_visible_and_escaped(self):
        commit = revision(1)
        index = site.archive(self.pages, self.bundle(commit, False), self.metadata(commit, conclusion="failure"), [commit])
        page = (self.pages / "calibration/runs" / commit / "index.html").read_text()
        self.assertIn("Missing — report incomplete", page)
        self.assertIn("&lt;script&gt;", page)
        self.assertNotIn('<script>alert', page)
        self.assertIsNone(index["latest_success"])
        report, _ = site.validate_bundle(self.bundle(revision(2)), revision(2))
        detail = site.detail(report, self.metadata(revision(2)))
        self.assertIn("2 / 6", detail)
        self.assertIn("noisy", detail)
        self.assertIn("Hardware accuracy: unverified", detail)

    def test_zero_errors_plotted_and_scatter_not_connected(self):
        chart = site.chart("zero", [("head", [(1, 0, "perfect")], "#000")], "x", "error")
        self.assertIn("perfect", chart)
        self.assertNotIn("nan", chart)
        scatter = site.chart("scatter", [("head", [(1, 2, "a"), (3, 4, "b")], "#000")], "x", "y", scatter=True)
        self.assertNotIn("polyline", scatter)

    def test_bad_bundles_rejected(self):
        commit = revision(1)
        for mutation in ("commit", "hash", "traversal", "missing", "symlink"):
            with self.subTest(mutation=mutation):
                bundle = self.bundle(commit)
                metadata = site.read(bundle / "bundle.json")
                if mutation == "commit": metadata["commit"] = revision(2)
                if mutation == "hash": metadata["files"]["report.json"] = "0" * 64
                if mutation == "traversal": metadata["files"]["inputs/../../secret"] = "0" * 64
                if mutation == "missing": del metadata["files"]["report.md"]
                if mutation == "symlink":
                    (bundle / "report.md").unlink()
                    (bundle / "report.md").symlink_to(bundle / "report.json")
                site.write(bundle / "bundle.json", metadata)
                with self.assertRaises(ValueError): site.validate_bundle(bundle, commit)

    def test_malformed_pass_reports_rejected(self):
        commit = revision(1)
        report, _ = site.validate_bundle(self.bundle(commit), commit)
        for mutate in (lambda r: r["cases"][0].pop("head"), lambda r: r["cases"][0].update(reference_user_seconds=0), lambda r: r["summary"]["calibration"].update(comparable=99), lambda r: r.update(accuracy_status="pass")):
            candidate = copy.deepcopy(report)
            mutate(candidate)
            with self.assertRaises(ValueError): site.check_report(candidate, commit)

    def test_symlink_archive_and_foreign_main_rejected(self):
        commit = revision(1)
        bundle = self.bundle(commit)
        with self.assertRaises(ValueError): site.archive(self.pages, bundle, self.metadata(commit), [revision(2)])
        (self.pages / "calibration").symlink_to(self.root)
        with self.assertRaises(ValueError): site.archive(self.pages, bundle, self.metadata(commit), [commit])

    def test_wrong_workflow_pr_or_repository_rejected(self):
        workflow = {"event": "push", "head_branch": "main", "head_repository": {"full_name": site.REPO}, "status": "completed", "workflow_id": 10, "id": 11, "run_attempt": 2, "head_sha": revision(1), "conclusion": "success", "html_url": "https://github.com/sarchlab/poros/actions/runs/11"}
        event = {"repository": {"full_name": site.REPO}, "workflow_run": workflow}
        with patch.object(publisher, "api", side_effect=[{"id": 10, "path": ".github/workflows/evaluation.yml"}, workflow]):
            self.assertEqual(publisher.assert_event(event, "workflow_run")["commit"], revision(1))
        for key, value in (("event", "pull_request"), ("head_branch", "feature"), ("head_repository", {"full_name": "other/repo"})):
            candidate = copy.deepcopy(event)
            candidate["workflow_run"][key] = value
            with self.assertRaises(ValueError): publisher.assert_event(candidate, "workflow_run")

    def test_success_bundle_matches_real_committed_source(self):
        original = self.bundle(revision(1))
        report = site.read(original / "report.json")
        source = self.root / "source"
        source.mkdir()
        for name in report["head"]["files"]:
            destination = source / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, destination)
        subprocess.run(["git", "init", str(source)], check=True, capture_output=True)
        for key, value in (("user.name", "Test"), ("user.email", "test@example.org")):
            subprocess.run(["git", "-C", str(source), "config", key, value], check=True)
        subprocess.run(["git", "-C", str(source), "add", "."], check=True)
        subprocess.run(["git", "-C", str(source), "commit", "-m", "evidence source"], check=True, capture_output=True)
        commit = site.git(source, "rev-parse", "HEAD")
        report["head"]["commit"] = commit
        raw = self.root / "committed-report.json"
        site.write(raw, report)
        raw.with_suffix(".md").write_text("# Report\n")
        output = self.root / "committed-bundle"
        site.bundle(raw, source, output, commit)
        publisher.verify_source(source, output, commit)
        report["head"]["files"]["scripts/native_usertime.py"] = "0" * 64
        site.write(output / "report.json", report)
        index = site.read(output / "bundle.json")
        index["files"]["report.json"] = site.digest(output / "report.json")
        site.write(output / "bundle.json", index)
        with self.assertRaises(ValueError): publisher.verify_source(source, output, commit)

    def test_workflow_contracts(self):
        # GitHub parses the full YAML on push; these lock our security/retention contract.
        evaluation = (ROOT / ".github/workflows/evaluation.yml").read_text()
        publishing = (ROOT / ".github/workflows/accuracy-pages.yml").read_text()
        self.assertIn("permissions:\n  contents: read", evaluation)
        self.assertNotIn("contents: write", evaluation)
        self.assertIn("retention-days: 30", evaluation)
        self.assertIn("name: user-time-accuracy-${{ github.run_attempt }}", evaluation)
        self.assertIn("github.event_name == 'pull_request' && github.ref || github.run_id", evaluation)
        self.assertIn("types: [completed]", publishing)
        self.assertIn("branches: [main]", publishing)
        self.assertIn("types: [published]", publishing)
        self.assertIn("queue: max", publishing)
        self.assertNotIn("pull_request_target", publishing)
        self.assertNotIn("cancel-in-progress", publishing)
        self.assertIn("persist-credentials: false", publishing)
        self.assertIn("format('refs/tags/{0}', github.event.release.tag_name) || github.sha", publishing)
        self.assertIn("pages: write", publishing)
        self.assertIn("github.event.workflow_run.head_repository.full_name == github.repository", publishing)

    def test_attempt_scoped_artifacts_and_failed_job_rerun(self):
        artifacts = [{"name": "user-time-accuracy-1", "expired": False}, {"name": "user-time-accuracy-3", "expired": False}, {"name": "user-time-accuracy-2", "expired": True}]
        self.assertEqual(publisher.choose_artifact(artifacts, 2)[0], 1)
        self.assertEqual(publisher.choose_artifact(artifacts, 3)[0], 3)
        self.assertEqual(publisher.choose_artifact(artifacts, 1)[0], 1)
        self.assertEqual(publisher.choose_artifact([], 2), (None, None))

    def test_manual_report_success_does_not_imply_full_evaluation_success(self):
        commit = revision(1)
        metadata = self.metadata(commit)
        metadata["evaluation_conclusion"] = "failure"
        index = site.archive(self.pages, self.bundle(commit), metadata, [commit])
        self.assertIsNone(index["latest_success"])
        with patch.object(publisher, "api", return_value={"workflow_runs": []}):
            self.assertEqual(publisher.evaluation_for_commit(commit)["evaluation_conclusion"], "not checked")
        wrong = {"head_sha": revision(2), "head_branch": "main", "event": "push", "head_repository": {"full_name": site.REPO}}
        with patch.object(publisher, "api", return_value={"workflow_runs": [wrong]}):
            with self.assertRaises(ValueError): publisher.evaluation_for_commit(commit)

    def test_pages_build_must_include_published_commit(self):
        expected = revision(2)
        with patch.object(publisher, "api", side_effect=[{"url": "build/1"}, {"status": "built", "commit": revision(1)}]), patch.object(publisher, "run", side_effect=[None, subprocess.CompletedProcess([], 1)]):
            with self.assertRaises(ValueError): publisher.wait_pages(expected, self.root)
        with patch.object(publisher, "api", side_effect=[{"url": "build/1"}, {"status": "errored", "error": "bad"}]):
            with self.assertRaises(ValueError): publisher.wait_pages(expected, self.root)

    def test_publisher_local_bare_repository(self):
        # Real clone/commit/push; API and source provenance are separately tested above.
        work = self.root / "work"
        remote = self.root / "remote.git"
        subprocess.run(["git", "init", "--bare", str(remote)], check=True, capture_output=True)
        subprocess.run(["git", "clone", str(remote), str(work)], check=True, capture_output=True)
        for key, value in (("user.name", "Test"), ("user.email", "test@example.org")):
            subprocess.run(["git", "-C", str(work), "config", key, value], check=True)
        (work / "unrelated.txt").write_text("keep")
        subprocess.run(["git", "-C", str(work), "add", "."], check=True)
        subprocess.run(["git", "-C", str(work), "commit", "-m", "initial"], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(work), "branch", "-M", "main"], check=True)
        subprocess.run(["git", "-C", str(work), "push", "origin", "main:main", "main:gh-pages"], check=True, capture_output=True)
        commit = site.git(work, "rev-parse", "HEAD")
        bundle = self.bundle(commit, False)
        result = publisher.publish(work, bundle, self.metadata(commit, conclusion="failure"), str(remote), build=False)
        self.assertEqual(site.git(work, "ls-remote", str(remote), "gh-pages").split()[0], result)
        contents = subprocess.check_output(["git", "--git-dir", str(remote), "show", "gh-pages:unrelated.txt"], text=True)
        self.assertEqual(contents, "keep")


if __name__ == "__main__":
    unittest.main()
