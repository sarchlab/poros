#!/usr/bin/env python3
"""Archive retention, provenance, failure visibility and publisher boundary tests."""

import copy
import accuracy_test_support as fixtures
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
        self.fixture_source = self.root / "fixture-source"
        manifest = site.read(ROOT/"tests/usertime/manifest.json")
        names = {"tests/usertime/manifest.json", "scripts/native_usertime.py", "scripts/workload_contract.py"} | {"tests/usertime/"+n for n in manifest["sources"]} | {"tests/usertime/"+c["file"] for c in manifest["cases"]}
        for name in names:
            destination=self.fixture_source/name
            destination.parent.mkdir(parents=True,exist_ok=True)
            shutil.copyfile(ROOT/name,destination)
        reference = fixtures.synthetic_micro_reference(self.fixture_source,manifest)
        for row in reference["cases"]:
            if row["workload"] != "branch" or row["point"] < 6000000:
                for sample,value in zip(row["samples"],[.0005]*10+[.001]+[.002]*10):sample["user_seconds"]=value
                row["quality"]="noisy"
        site.write(self.fixture_source/"evaluation/usertime/m2-linuxvm-user-v2.json",reference)

    def bundle(self, commit, successful=True, error=10):
        self.counter += 1
        raw, output = (
            self.root / f"raw{self.counter}",
            self.root / f"bundle{self.counter}",
        )
        raw.mkdir()
        if not successful:
            site.failure(raw, commit, 'Build failed <script>alert("bad")</script>')
        else:
            manifest = site.read(self.fixture_source / "tests/usertime/manifest.json")
            reference = site.read(self.fixture_source / "evaluation/usertime/m2-linuxvm-user-v2.json")
            paths = [
                "scripts/native_usertime.py",
                "scripts/workload_contract.py",
                "tests/usertime/manifest.json",
                "evaluation/usertime/m2-linuxvm-user-v2.json",
            ]
            paths += [
                "tests/usertime/" + n
                for n in set(manifest["sources"])
                | {c["file"] for c in manifest["cases"]}
            ]
            cases = []
            for c, ref in zip(manifest["cases"], reference["cases"]):
                prediction = {
                    "predicted_user_seconds": ref["median_user_seconds"] * 1.1,
                    "instructions": 100,
                    "cycles": 110,
                    **manifest["profile"],
                }
                if ref["quality"] == "usable":
                    prediction.update(error_percent=error, direction="slower")
                cases.append(
                    {
                        "id": c["id"],
                        "functional_status": "pass",
                        "split": c["split"],
                        "reference_user_seconds": ref["median_user_seconds"],
                        "reference_quality": ref["quality"],
                        "head": prediction,
                        "base": copy.deepcopy(prediction),
                        **(
                            {"error_change_percentage_points": 0}
                            if ref["quality"] == "usable"
                            else {}
                        ),
                    }
                )
            summary = {}
            for split in ("calibration", "held_out"):
                rows = [c for c in cases if c["split"] == split]
                count = sum("error_percent" in c["head"] for c in rows)
                summary[split] = {"required": len(rows), "comparable": count}
                if count:
                    summary[split].update(
                        {
                            label: {
                                "mean_error_percent": error,
                                "maximum_error_percent": error,
                            }
                            for label in ("head", "base")
                        }
                    )
            report = {
                "schema": 2,
                "workload_manifest": manifest,
                "artifact_status": "pass",
                "accuracy_status": "unverified",
                "formula": site.FORMULA,
                "head": {
                    "commit": commit,
                    "dirty": False,
                    "files": {p: site.digest(self.fixture_source / p) for p in paths},
                },
                "base_commit": revision(0),
                "reference_sha256": site.digest(
                    self.fixture_source / "evaluation/usertime/m2-linuxvm-user-v2.json"
                ),
                "reference": {k:v for k,v in reference.items() if k != "cases"},
                "cases": cases,
                "summary": summary,
            }
            site.write(raw / "report.json", report)
            (raw / "report.md").write_text("# Test report\n")
        site.bundle(raw / "report.json", self.fixture_source, output, commit)
        return output

    def metadata(self, commit, kind="main", conclusion="success", run=1, attempt=1):
        return {
            "commit": commit,
            "kind": kind,
            "conclusion": conclusion,
            "evaluation_conclusion": conclusion,
            "evaluation_run_id": run,
            "evaluation_run_attempt": attempt,
            "evaluation_run_url": f"https://github.com/sarchlab/poros/actions/runs/{run}/attempts/{attempt}",
            "run_id": run,
            "run_attempt": attempt,
            "run_url": f"https://github.com/sarchlab/poros/actions/runs/{run}/attempts/{attempt}",
            **({"release": {"id": run, "tag": "v1.0"}} if kind == "release" else {}),
        }

    def test_keep_twenty_plus_releases_and_preserve_unrelated(self):
        order = [revision(n) for n in range(25, 0, -1)]
        bundles = {}
        for n in range(1, 26):
            commit = revision(n)
            bundles[n] = self.bundle(commit)
            site.archive(self.pages, bundles[n], self.metadata(commit, run=n), order)
            if n == 1:
                site.archive(
                    self.pages,
                    bundles[n],
                    self.metadata(commit, "release", run=101),
                    order,
                )
        index = site.read(self.pages / "calibration/index.json")
        self.assertEqual(len(index["entries"]), 21)
        self.assertEqual(index["latest_success"], revision(25))
        self.assertTrue(
            (self.pages / "calibration/runs" / revision(1) / "reference.json").exists()
        )
        self.assertFalse((self.pages / "calibration/runs" / revision(2)).exists())
        self.assertEqual(
            (self.pages / "README.md").read_text(), "unrelated source is preserved"
        )

    def test_out_of_order_does_not_move_latest_back(self):
        order = [revision(3), revision(2), revision(1)]
        for n in (3, 1, 2):
            site.archive(
                self.pages,
                self.bundle(revision(n)),
                self.metadata(revision(n), run=n),
                order,
            )
        self.assertEqual(
            site.read(self.pages / "calibration/index.json")["latest_success"],
            revision(3),
        )

    def test_failed_ci_with_valid_artifact_can_recover_without_rewriting_report(self):
        commit = revision(1)
        bundle = self.bundle(commit)
        site.archive(
            self.pages, bundle, self.metadata(commit, conclusion="failure"), [commit]
        )
        original = (
            self.pages / "calibration/runs" / commit / "report.json"
        ).read_bytes()
        self.assertIsNone(
            site.read(self.pages / "calibration/index.json")["latest_success"]
        )
        changed = self.bundle(commit, error=20)
        site.archive(self.pages, changed, self.metadata(commit, attempt=2), [commit])
        index = site.archive(
            self.pages, bundle, self.metadata(commit, conclusion="failure"), [commit]
        )
        self.assertEqual(index["latest_success"], commit)
        self.assertEqual(index["entries"][0]["conclusion"], "success")
        self.assertEqual(
            (self.pages / "calibration/runs" / commit / "report.json").read_bytes(),
            original,
        )

    def test_failed_report_recovery_and_release_pin_idempotent(self):
        commit = revision(1)
        site.archive(
            self.pages,
            self.bundle(commit, False),
            self.metadata(commit, conclusion="failure"),
            [commit],
        )
        bundle = self.bundle(commit)
        site.archive(self.pages, bundle, self.metadata(commit, attempt=2), [commit])
        metadata = self.metadata(commit, "release", run=20)
        metadata["release"]["publication_run_url"] = metadata["run_url"]
        site.archive(self.pages, bundle, metadata, [commit])
        retried = copy.deepcopy(metadata)
        retried["release"][
            "publication_run_url"
        ] = "https://github.com/sarchlab/poros/actions/runs/20/attempts/2"
        index = site.archive(self.pages, bundle, retried, [commit])
        self.assertEqual(index["entries"][0]["releases"], [metadata["release"]])
        self.assertEqual(index["latest_success"], commit)

    def test_zero_calibration_coverage_and_four_sweep_axes(self):
        commit = revision(10)
        report, _ = site.validate_bundle(self.bundle(commit), commit)
        self.assertEqual(
            report["summary"]["calibration"], {"required": 12, "comparable": 0}
        )
        page = site.detail(report, self.metadata(commit))
        self.assertIn("calibration<strong>0 / 6", page)
        self.assertIn("Mean head error: —", page)
        self.assertIn("Matrix dimension n", page)
        self.assertIn("Working set (log₂ spacing)", page)
        self.assertIn("8 KiB", page)
        self.assertIn("32 MiB", page)
        self.assertIn("4,194,304 accesses", page)
        self.assertIn("MVT", page)
        self.assertIn("<th>Functional</th>", page)
        self.assertEqual(len(report["cases"]), 24)

    def test_schema2_cannot_embed_legacy_six_case_manifest(self):
        with self.assertRaisesRegex(ValueError, "24-case"):
            site.report_manifest(dict(schema=2, workload_manifest=dict(schema=1)))

    def test_schema1_bundle_needs_no_new_support_file(self):
        # Synthetic format fixture, not claimed measured evidence. Exercise the
        # original six-case bundle shape without a contract-module dependency.
        commit = revision(11)
        directory = self.bundle(commit)
        report = site.read(directory / "report.json")
        inventory = site.read(directory / "manifest.json")
        reference = site.read(directory / "reference.json")
        rows, declarations, measurements = [], [], []
        for i, name in enumerate(site.REQUIRED):
            workload, n = name.split("-")
            j = next(
                j for j, c in enumerate(inventory["cases"]) if c["workload"] == workload
            )
            declaration = copy.deepcopy(inventory["cases"][j])
            for key in ("workload", "point", "axis"):
                declaration.pop(key)
            declaration.update(id=name, args=["program", n])
            measured = copy.deepcopy(reference["cases"][j])
            measured.update(declaration)
            row = copy.deepcopy(report["cases"][j])
            row.update(id=name)
            row.pop("functional_status")
            rows.append(row)
            declarations.append(declaration)
            measurements.append(measured)
        inventory.update(schema=1, cases=declarations)
        for key in ("workloads", "suite", "reference_file"):
            inventory.pop(key)
        reference.update(schema=1, cases=measurements)
        reference.pop("support_sha256")
        report.update(schema=1, cases=rows)
        report.pop("workload_manifest")
        report["summary"] = {
            split: {"required": sum(c["split"] == split for c in rows), "comparable": 0}
            for split in ("calibration", "held_out")
        }
        site.write(directory / "manifest.json", inventory)
        reference["manifest_sha256"] = site.digest(directory / "manifest.json")
        site.write(directory / "reference.json", reference)
        report["reference"] = reference
        report["reference_sha256"] = site.digest(directory / "reference.json")
        site.write(directory / "report.json", report)
        (directory / "workload_contract.py").unlink()
        bundle = site.read(directory / "bundle.json")
        bundle["files"].pop("workload_contract.py")
        bundle["files"] = {
            name: site.digest(directory / name) for name in bundle["files"]
        }
        site.write(directory / "bundle.json", bundle)
        retained, _ = site.validate_bundle(directory, commit)
        rendered = site.detail(retained, self.metadata(commit))
        self.assertIn("Historical memory sweep: 64 KiB", rendered)
        self.assertIn("not separately recorded", rendered)

    def test_failure_missing_and_noisy_are_visible_and_escaped(self):
        commit = revision(1)
        index = site.archive(
            self.pages,
            self.bundle(commit, False),
            self.metadata(commit, conclusion="failure"),
            [commit],
        )
        page = (self.pages / "calibration/runs" / commit / "index.html").read_text()
        self.assertIn("Build failed", page)
        self.assertIn("&lt;script&gt;", page)
        self.assertNotIn("<script>alert", page)
        self.assertIsNone(index["latest_success"])
        report, _ = site.validate_bundle(self.bundle(revision(2)), revision(2))
        detail = site.detail(report, self.metadata(revision(2)))
        self.assertIn("Usable benchmark references<strong>0 / 6", detail)
        self.assertIn("Usable microbenchmark references: 2 / 18", detail)
        self.assertIn("noisy", detail)
        self.assertIn("Hardware accuracy: unverified", detail)
        index = site.archive(
            self.pages,
            self.bundle(revision(2)),
            self.metadata(revision(2), run=2),
            [revision(2), revision(1)],
        )
        self.assertNotIn("10.0000% (2/12)", site.home(index))
        self.assertIn("— (0/6)", site.home(index))

    def test_failure_coverage_never_invents_legacy_or_partial_totals(self):
        for partial in (False, True):
            with self.subTest(partial=partial):
                commit = revision(30 + int(partial))
                if partial:
                    report, _ = site.validate_bundle(self.bundle(commit), commit)
                    report.update(
                        artifact_status="fail", error="interrupted after one case"
                    )
                    report["cases"] = report["cases"][:1]
                    report.pop("summary")
                    raw = self.root / "partial.json"
                    site.write(raw, report)
                    raw.with_suffix(".md").write_text(report["error"])
                    bundle = self.root / "partial-bundle"
                    site.bundle(raw, ROOT, bundle, commit)
                else:
                    bundle = self.bundle(commit, False)
                index = site.archive(
                    self.pages,
                    bundle,
                    self.metadata(commit, conclusion="failure"),
                    [commit, revision(30)],
                )
                detail = (
                    self.pages / "calibration/runs" / commit / "index.html"
                ).read_text()
                self.assertIn("Usable benchmark references<strong>0 / —", detail)
                self.assertIn("calibration<strong>— / —", detail)
                self.assertIn("held_out<strong>— / —", detail)
                self.assertNotIn("0 / 2", detail)
                self.assertNotIn("0 / 4", detail)
                self.assertNotIn("0 / 1", detail)
                home = site.home(index)
                self.assertIn("— (—/—)", home)
                self.assertNotIn("(0/2)", home)
                self.assertNotIn("(0/4)", home)

    def test_zero_errors_plotted_and_scatter_not_connected(self):
        chart = site.chart(
            "zero", [("head", [(1, 0, "perfect")], "#000")], "x", "error"
        )
        self.assertIn("perfect", chart)
        self.assertNotIn("nan", chart)
        scatter = site.chart(
            "scatter",
            [("head", [(1, 2, "a"), (3, 4, "b")], "#000")],
            "x",
            "y",
            scatter=True,
        )
        self.assertNotIn("polyline", scatter)

    def test_overview_ties_quality_and_rank_direction(self):
        def row(name, x, y, quality="usable"):
            return dict(id=name, reference_quality=quality, reference_user_seconds=x,
                        head=dict(predicted_user_seconds=y))
        html = site.overview_charts([row("A",1,2), row("B",2,1),
                                     row("C",2,2), row("D",3,4), row("E",4,1,"noisy")])
        self.assertIn("60.00% (3/5)", html)
        self.assertIn("Kendall’s τ-b = 0.4000", html)
        self.assertIn("rank 1 · B", html)
        self.assertIn("rank 4 · A", html)
        self.assertNotIn("· E ·", html)
        self.assertIn("Pearson correlation (r) = —", site.overview_charts([row("A",1,1), row("B",1,2)]))
        self.assertIn("Kendall’s τ-b = —", site.overview_charts([row("A",1,1), row("B",2,1)]))
        self.assertIn("Pairs ordered correctly: —", site.overview_charts([row("A",1,1)]))

    def test_s_curve_inclusive_symmetric_error_bands(self):
        rows = [dict(id=str(i), reference_quality="usable", reference_user_seconds=x,
                     head=dict(predicted_user_seconds=y))
                for i, (x, y) in enumerate([(100,105),(120,100),(100,150),(200,100),(100,201)])]
        html = site.overview_charts(rows)
        for _, label, _ in site.ERROR_BANDS:
            self.assertIn(f"{site.esc(label)} <strong>1 (20.00%)</strong>", html)

    def test_bad_bundles_rejected(self):
        commit = revision(1)
        for mutation in ("commit", "hash", "traversal", "missing", "symlink"):
            with self.subTest(mutation=mutation):
                bundle = self.bundle(commit)
                metadata = site.read(bundle / "bundle.json")
                if mutation == "commit":
                    metadata["commit"] = revision(2)
                if mutation == "hash":
                    metadata["files"]["report.json"] = "0" * 64
                if mutation == "traversal":
                    metadata["files"]["inputs/../../secret"] = "0" * 64
                if mutation == "missing":
                    del metadata["files"]["report.md"]
                if mutation == "symlink":
                    (bundle / "report.md").unlink()
                    (bundle / "report.md").symlink_to(bundle / "report.json")
                site.write(bundle / "bundle.json", metadata)
                with self.assertRaises(ValueError):
                    site.validate_bundle(bundle, commit)

    def test_malformed_pass_reports_rejected(self):
        commit = revision(1)
        report, _ = site.validate_bundle(self.bundle(commit), commit)
        for mutate in (
            lambda r: r["cases"][0].pop("head"),
            lambda r: r["cases"][0].pop("functional_status"),
            lambda r: r["workload_manifest"]["cases"].pop(),
            lambda r: r["cases"][0].update(reference_user_seconds=-1),
            lambda r: r["summary"]["calibration"].update(comparable=99),
            lambda r: r.update(accuracy_status="pass"),
        ):
            candidate = copy.deepcopy(report)
            mutate(candidate)
            with self.assertRaises(ValueError):
                site.check_report(candidate, commit)

    def test_symlink_archive_and_foreign_main_rejected(self):
        commit = revision(1)
        bundle = self.bundle(commit)
        with self.assertRaises(ValueError):
            site.archive(self.pages, bundle, self.metadata(commit), [revision(2)])
        (self.pages / "calibration").symlink_to(self.root)
        with self.assertRaises(ValueError):
            site.archive(self.pages, bundle, self.metadata(commit), [commit])

    def test_wrong_workflow_pr_or_repository_rejected(self):
        workflow = {
            "event": "push",
            "head_branch": "main",
            "head_repository": {"full_name": site.REPO},
            "status": "completed",
            "workflow_id": 10,
            "id": 11,
            "run_attempt": 2,
            "head_sha": revision(1),
            "conclusion": "success",
            "html_url": "https://github.com/sarchlab/poros/actions/runs/11",
        }
        event = {"repository": {"full_name": site.REPO}, "workflow_run": workflow}
        with patch.object(
            publisher,
            "api",
            side_effect=[
                {"id": 10, "path": ".github/workflows/evaluation.yml"},
                workflow,
            ],
        ):
            self.assertEqual(
                publisher.assert_event(event, "workflow_run")["commit"], revision(1)
            )
        for key, value in (
            ("event", "pull_request"),
            ("head_branch", "feature"),
            ("head_repository", {"full_name": "other/repo"}),
        ):
            candidate = copy.deepcopy(event)
            candidate["workflow_run"][key] = value
            with self.assertRaises(ValueError):
                publisher.assert_event(candidate, "workflow_run")

    def test_success_bundle_matches_real_committed_source(self):
        original = self.bundle(revision(1))
        report = site.read(original / "report.json")
        source = self.root / "source"
        source.mkdir()
        for name in report["head"]["files"]:
            destination = source / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(self.fixture_source / name, destination)
        subprocess.run(["git", "init", str(source)], check=True, capture_output=True)
        for key, value in (("user.name", "Test"), ("user.email", "test@example.org")):
            subprocess.run(["git", "-C", str(source), "config", key, value], check=True)
        subprocess.run(["git", "-C", str(source), "add", "."], check=True)
        subprocess.run(
            ["git", "-C", str(source), "commit", "-m", "evidence source"],
            check=True,
            capture_output=True,
        )
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
        with self.assertRaises(ValueError):
            publisher.verify_source(source, output, commit)

    def test_workflow_contracts(self):
        # GitHub parses the full YAML on push; these lock our security/retention contract.
        evaluation = (ROOT / ".github/workflows/evaluation.yml").read_text()
        publishing = (ROOT / ".github/workflows/accuracy-pages.yml").read_text()
        parallel = (ROOT / ".github/workflows/accuracy-evaluate.yml").read_text()
        self.assertIn("permissions:\n  contents: read", evaluation)
        self.assertNotIn("contents: write", evaluation)
        self.assertIn("retention-days: 30", parallel)
        self.assertIn("name: user-time-accuracy-${{ github.run_attempt }}", parallel)
        self.assertIn(
            "github.event_name == 'pull_request' && github.ref || github.run_id",
            evaluation,
        )
        self.assertIn("types: [completed]", publishing)
        self.assertIn("branches: [main]", publishing)
        self.assertIn("types: [published]", publishing)
        self.assertIn("queue: max", publishing)
        self.assertNotIn("pull_request_target", publishing)
        self.assertNotIn("cancel-in-progress", publishing)
        self.assertIn("persist-credentials: false", publishing)
        self.assertIn("ref: ${{ steps.target.outputs.commit }}", publishing)
        self.assertIn("release_tag:", publishing)
        self.assertIn("--resolve-target", publishing)
        self.assertIn("accuracy_backfill.py route", publishing)
        self.assertIn("accuracy_backfill.py package", publishing)
        self.assertIn(
            "name: release-accuracy-evidence-${{ github.run_attempt }}", publishing
        )
        self.assertIn("pages: write", publishing)
        self.assertIn(
            "github.event.workflow_run.head_repository.full_name == github.repository",
            publishing,
        )

    def test_attempt_scoped_artifacts_and_failed_job_rerun(self):
        artifacts = [
            {"name": "user-time-accuracy-1", "expired": False},
            {"name": "user-time-accuracy-3", "expired": False},
            {"name": "user-time-accuracy-2", "expired": True},
        ]
        self.assertEqual(publisher.choose_artifact(artifacts, 2)[0], 1)
        self.assertEqual(publisher.choose_artifact(artifacts, 3)[0], 3)
        self.assertEqual(publisher.choose_artifact(artifacts, 1)[0], 1)
        self.assertEqual(publisher.choose_artifact([], 2), (None, None))
        release_artifacts = [
            {"name": "release-accuracy-evidence-1", "expired": False},
            {"name": "release-accuracy-evidence-3", "expired": False},
        ]
        self.assertEqual(
            publisher.choose_artifact(
                release_artifacts, 2, "release-accuracy-evidence"
            )[0],
            1,
        )

    def test_manual_report_success_does_not_imply_full_evaluation_success(self):
        commit = revision(1)
        metadata = self.metadata(commit)
        metadata["evaluation_conclusion"] = "failure"
        index = site.archive(self.pages, self.bundle(commit), metadata, [commit])
        self.assertIsNone(index["latest_success"])
        with patch.object(publisher, "api", return_value={"workflow_runs": []}):
            self.assertEqual(
                publisher.evaluation_for_commit(commit)["evaluation_conclusion"],
                "not checked",
            )
        wrong = {
            "head_sha": revision(2),
            "head_branch": "main",
            "event": "push",
            "head_repository": {"full_name": site.REPO},
        }
        with patch.object(publisher, "api", return_value={"workflow_runs": [wrong]}):
            with self.assertRaises(ValueError):
                publisher.evaluation_for_commit(commit)

    def test_pages_waits_past_stale_built_and_errored_results(self):
        expected = revision(2)
        for stale in ("built", "errored"):
            responses = [
                {"url": "build/latest"},
                {"status": stale, "commit": revision(1)},
                {"status": "built", "commit": expected},
            ]
            commands = [
                None,
                subprocess.CompletedProcess([], 1),
                None,
                subprocess.CompletedProcess([], 0),
            ]
            with patch.object(publisher, "api", side_effect=responses), patch.object(
                publisher, "run", side_effect=commands
            ), patch.object(publisher.time, "sleep"):
                result = publisher.wait_pages(expected, self.root)
                self.assertEqual(result["commit"], expected)
        with patch.object(
            publisher,
            "api",
            side_effect=[
                {"url": "build/latest"},
                {"status": "errored", "commit": expected, "error": "bad"},
            ],
        ), patch.object(
            publisher, "run", side_effect=[None, subprocess.CompletedProcess([], 0)]
        ):
            with self.assertRaises(ValueError):
                publisher.wait_pages(expected, self.root)

    def test_pages_unrelated_build_times_out_with_bounded_polling(self):
        responses = [
            {"url": "build/latest"},
            {"status": "built", "commit": revision(1)},
        ]
        with patch.object(publisher, "api", side_effect=responses), patch.object(
            publisher, "run", side_effect=[None, subprocess.CompletedProcess([], 1)]
        ), patch.object(publisher.time, "sleep"), patch.object(
            publisher.time, "monotonic", side_effect=[0, 0, 2]
        ):
            with self.assertRaises(TimeoutError):
                publisher.wait_pages(revision(2), self.root, timeout=1)

    def test_evaluation_identity_independent_of_publication_order(self):
        commit = revision(1)
        bundle = self.bundle(commit)
        failed = self.metadata(commit, conclusion="failure", run=10)
        site.archive(self.pages, bundle, failed, [commit])
        manual = self.metadata(commit, run=20)
        manual.update({k: failed[k] for k in site.EVALUATION_FIELDS})
        site.archive(self.pages, bundle, manual, [commit])
        passed = self.metadata(commit, run=10, attempt=2)
        index = site.archive(self.pages, bundle, passed, [commit])
        self.assertEqual(index["latest_success"], commit)
        self.assertEqual(index["entries"][0]["evaluation_run_attempt"], 2)
        self.assertEqual(index["entries"][0]["evaluation_conclusion"], "success")
        # Delayed old events and weaker metadata-only lookups cannot downgrade proof.
        unknown = self.metadata(commit, run=30)
        unknown = {k: v for k, v in unknown.items() if k not in site.EVALUATION_FIELDS}
        unknown["evaluation_conclusion"] = "not checked"
        for late in (failed, manual, unknown):
            index = site.archive(self.pages, bundle, late, [commit])
            self.assertEqual(index["latest_success"], commit)
            self.assertEqual(index["entries"][0]["evaluation_conclusion"], "success")
        # A genuinely newer failed attempt revokes eligibility rather than a sticky flag.
        newer_failure = self.metadata(commit, conclusion="failure", run=10, attempt=3)
        index = site.archive(self.pages, bundle, newer_failure, [commit])
        self.assertIsNone(index["latest_success"])
        self.assertEqual(index["entries"][0]["evaluation_conclusion"], "failure")

    def test_release_first_then_same_evaluation_completes(self):
        commit = revision(1)
        bundle = self.bundle(commit)
        release = self.metadata(commit, "release", run=200)
        proof = self.metadata(commit, run=100)
        release.update({k: proof[k] for k in site.EVALUATION_FIELDS})
        release["evaluation_conclusion"] = "in_progress"
        site.archive(self.pages, bundle, release, [commit])
        index = site.archive(self.pages, bundle, proof, [commit])
        self.assertEqual(index["latest_success"], commit)
        self.assertEqual(index["entries"][0]["evaluation_conclusion"], "success")
        # Same-attempt in-progress snapshots cannot replace a terminal proof.
        index = site.archive(self.pages, bundle, release, [commit])
        self.assertEqual(index["entries"][0]["evaluation_conclusion"], "success")

    def test_early_failure_keeps_original_diagnostic_without_claiming_provenance(self):
        commit = revision(1)
        raw = self.root / "early.json"
        report = {
            "schema": 1,
            "artifact_status": "fail",
            "accuracy_status": "unverified",
            "cases": [],
            "error": "accuracy acceptance requires clean committed source",
        }
        site.write(raw, report)
        raw.with_suffix(".md").write_text(report["error"])
        output = self.root / "early-bundle"
        site.bundle(raw, ROOT, output, commit)
        self.assertEqual(raw.read_bytes(), (output / "report.json").read_bytes())
        self.assertEqual(site.read(output / "bundle.json")["commit"], commit)
        self.assertNotIn("head_commit", site.read(output / "report.json"))
        preview = (output / "index.html").read_text()
        self.assertIn(report["error"], preview)
        self.assertIn("before source identity was recorded", preview)
        self.assertFalse(site.reusable_bundle(output, commit))

    def test_only_valid_successful_release_evidence_is_reused(self):
        commit = revision(1)
        valid = self.bundle(commit)
        self.assertTrue(site.reusable_bundle(valid, commit))
        self.assertFalse(site.reusable_bundle(self.bundle(commit, False), commit))
        self.assertFalse(site.reusable_bundle(self.root / "missing", commit))
        (valid / "reference.json").write_text("corrupt")
        self.assertFalse(site.reusable_bundle(valid, commit))

    def test_all_provenance_urls_reject_non_actions_links(self):
        commit = revision(1)
        bundle = self.bundle(commit)
        for key in ("run_url", "evidence_run_url", "evaluation_run_url"):
            metadata = self.metadata(commit)
            metadata[key] = "javascript:alert(1)"
            with self.assertRaises(ValueError):
                site.archive(self.pages, bundle, metadata, [commit])
        release = self.metadata(commit, "release")
        release["release"]["publication_run_url"] = "https://evil.example/run"
        with self.assertRaises(ValueError):
            site.archive(self.pages, bundle, release, [commit])

    def test_manual_release_backfill_resolves_published_id_and_annotated_tag(self):
        event = {
            "repository": {"full_name": site.REPO},
            "inputs": {"release_tag": "v1.0"},
        }
        release = {
            "id": 42,
            "draft": False,
            "published_at": "2026-10-29T00:00:00Z",
            "tag_name": "v1.0",
        }
        ref = {"ref": "refs/tags/v1.0", "object": {"type": "tag", "sha": revision(5)}}
        with patch.dict(os.environ, {"GITHUB_REF": "refs/heads/main"}), patch.object(
            publisher,
            "api",
            side_effect=[
                release,
                ref,
                {"object": {"type": "commit", "sha": revision(1)}},
            ],
        ):
            target = publisher.assert_event(event, "workflow_dispatch")
        self.assertEqual(
            target,
            {
                "kind": "release",
                "commit": revision(1),
                "release": {"id": 42, "tag": "v1.0"},
            },
        )
        with patch.dict(os.environ, {"GITHUB_REF": "refs/heads/feature"}):
            with self.assertRaises(ValueError):
                publisher.assert_event(event, "workflow_dispatch")
        with patch.object(publisher, "api", return_value={**release, "draft": True}):
            with self.assertRaises(ValueError):
                publisher.published_release_target("v1.0")
        with patch.object(
            publisher,
            "api",
            side_effect=subprocess.CalledProcessError(1, ["gh", "api"]),
        ):
            with self.assertRaises(subprocess.CalledProcessError):
                publisher.published_release_target("nonexistent")
        automatic = {
            "repository": {"full_name": site.REPO},
            "action": "published",
            "release": {"id": 999, "tag_name": "v1.0", "draft": False},
        }
        with patch.object(publisher, "published_release_target", return_value=target):
            with self.assertRaises(ValueError):
                publisher.assert_event(automatic, "release")
        automatic["release"]["id"] = 42
        with patch.dict(os.environ, {"GITHUB_SHA": revision(2)}), patch.object(
            publisher, "published_release_target", return_value=target
        ):
            with self.assertRaises(ValueError):
                publisher.assert_event(automatic, "release")
        with patch.dict(os.environ, {"GITHUB_SHA": revision(1)}), patch.object(
            publisher, "published_release_target", return_value=target
        ):
            self.assertEqual(publisher.assert_event(automatic, "release"), target)

    def test_publisher_local_bare_repository(self):
        # Real clone/commit/push; API and source provenance are separately tested above.
        work = self.root / "work"
        remote = self.root / "remote.git"
        subprocess.run(
            ["git", "init", "--bare", str(remote)], check=True, capture_output=True
        )
        subprocess.run(
            ["git", "clone", str(remote), str(work)], check=True, capture_output=True
        )
        for key, value in (("user.name", "Test"), ("user.email", "test@example.org")):
            subprocess.run(["git", "-C", str(work), "config", key, value], check=True)
        (work / "unrelated.txt").write_text("keep")
        subprocess.run(["git", "-C", str(work), "add", "."], check=True)
        subprocess.run(
            ["git", "-C", str(work), "commit", "-m", "initial"],
            check=True,
            capture_output=True,
        )
        subprocess.run(["git", "-C", str(work), "branch", "-M", "main"], check=True)
        subprocess.run(
            ["git", "-C", str(work), "push", "origin", "main:main", "main:gh-pages"],
            check=True,
            capture_output=True,
        )
        commit = site.git(work, "rev-parse", "HEAD")
        bundle = self.bundle(commit, False)
        result = publisher.publish(
            work,
            bundle,
            self.metadata(commit, conclusion="failure"),
            str(remote),
            build=False,
        )
        self.assertEqual(
            site.git(work, "ls-remote", str(remote), "gh-pages").split()[0], result
        )
        contents = subprocess.check_output(
            ["git", "--git-dir", str(remote), "show", "gh-pages:unrelated.txt"],
            text=True,
        )
        self.assertEqual(contents, "keep")


if __name__ == "__main__":
    unittest.main()
