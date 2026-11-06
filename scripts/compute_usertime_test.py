#!/usr/bin/env python3
"""Expanded evidence gates; synthetic fixtures here are never timing references."""
import base64
import copy
import hashlib
import gzip
import accuracy_test_support as fixtures
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import accuracy_site as site
import compute_usertime as compute
import usertime as gate
import publish_accuracy as publisher

ROOT = Path(__file__).resolve().parents[1]
SHA = "a" * 40


class ComputeTimeTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.source = self.directory / "source"
        self.fixture = fixtures.Evidence(ROOT, self.directory)
        self.source = self.fixture.source
        self.folder = self.fixture.folder
        self.declared = self.fixture.declared
        self.reference = self.fixture.reference
        self.report = self.fixture.report
        original_validate = compute.validate_reference
        validation = patch.object(compute, "validate_reference", side_effect=lambda folder, root=None: original_validate(folder, self.source if root is None else root))
        validation.start()
        self.addCleanup(validation.stop)

    def test_exact_contract_and_frozen_splits(self):
        compute.validate_reference(self.folder, self.source)
        compute.validate_expanded(self.report["workload_manifest"])
        self.assertEqual({k: v["required"] for k, v in self.report["summary"].items()}, dict(calibration=24, held_out=12, **{"validation-legacy": 228}))
        for mutate in (
            lambda m: m["cases"].pop(),
            lambda m: m["compute"]["cases"][0]["args"].append("changed"),
            lambda m: m["compute"]["cases"][0].update(split="held_out"),
            lambda m: m["compute"]["cases"][0].update(file="other.elf"),
            lambda m: m["compute"]["cases"][0]["outputs"][0].update(count=999),
        ):
            changed = copy.deepcopy(self.report["workload_manifest"])
            mutate(changed)
            with self.assertRaises(ValueError):
                compute.validate_expanded(changed)

    def test_mapped_bytes_and_gzip_bounds_are_not_trusted_metadata(self):
        manifest = compute.read(self.source / compute.PERFORMANCE_MANIFEST)
        manifest["workloads"][0]["mapped_image_bytes"] = 4096
        with self.assertRaisesRegex(ValueError, "mapped image"):
            compute.differential.validate_inventory(self.source, manifest, self.declared["build"])
        case, row = self.declared["cases"][0], self.reference["cases"][0]
        path = self.folder / compute.output_name(row, compute.PROTOCOL)
        path.write_bytes(gzip.compress(bytes(1024*1024), mtime=0))
        with self.assertRaises(ValueError):
            compute.witness(self.folder, case, row)

    def test_raw_evidence_rejects_corruption_and_incomplete_protocol(self):
        for mutate in (
            lambda r: r.pop("completed_at_utc"),
            lambda r: r.update(precision_floor_seconds=0),
            lambda r: r["cases"].pop(),
            lambda r: r["cases"][0]["samples"].pop(),
            lambda r: r["cases"][0]["samples"][20].update(stdout_sha256="0" * 64),
            lambda r: r["cases"][0]["warmup_sample"].update(stdout_bytes=0),
            lambda r: r["cases"][0]["samples"][0].update(user_seconds=0),
            lambda r: r["cases"][0].update(quality="noisy"),
        ):
            reference = copy.deepcopy(self.reference)
            mutate(reference)
            compute.write(self.folder / "reference.json", reference)
            with self.assertRaises(ValueError):
                compute.validate_reference(self.folder, self.source)
        compute.write(self.folder / "reference.json", self.reference)
        path = self.folder / compute.output_name(self.reference["cases"][0],compute.PROTOCOL)
        path.write_bytes(path.read_bytes()[:-1])
        with self.assertRaises((ValueError,EOFError,OSError)):
            compute.validate_reference(self.folder, self.source)

    def test_zero_user_samples_are_retained_not_errors_or_retried(self):
        changed = copy.deepcopy(self.reference)
        changed["cases"][0]["samples"][0].update(user_seconds=0)
        changed["cases"][0]["quality"] = "invalid_nonpositive"
        compute.write(self.folder / "reference.json", changed)
        validated = compute.validate_reference(self.folder, self.source)
        self.assertEqual(validated["cases"][0]["samples"][0]["user_seconds"], 0)

    def test_bundle_render_roundtrip_and_path_protection(self):
        report_path = self.directory / "report.json"
        compute.write(report_path, self.report)
        report_path.with_suffix(".md").write_text("# Synthetic expanded test report\n")
        bundle = self.directory / "bundle"
        site.bundle(report_path, self.source, bundle, SHA)
        result, manifest = site.validate_bundle(bundle, SHA)
        self.assertEqual(len(result["cases"]), 264)
        html = (bundle / "index.html").read_text()
        self.assertTrue("246 / 246" in html and "18 / 18" in html, "qualified coverage must retain old24 quality")
        self.assertTrue("validation-legacy" in html, "missing third split")
        self.assertTrue("m=1, n=1" in html, "missing actual dimension labels")
        self.assertEqual(html.count("<figcaption>"), 46)  # 44 workload curves + two overall charts
        self.assertIn("cohorts/compute/source/parsec/kernels/bodytrack.c", manifest["files"])
        self.assertIn("cohorts/compute/source/tests/benchmarks/bodytrack.c", manifest["files"])
        path = bundle / "cohorts/compute/reference" / compute.output_name(self.reference["cases"][0],compute.PROTOCOL)
        path.unlink()
        path.symlink_to(self.folder / "outputs" / path.name)
        with self.assertRaises(ValueError):
            site.validate_bundle(bundle, SHA)
        for bad in ("../escape", "a/./b", "/absolute", "a//b", "a\\b"):
            with self.assertRaises(ValueError):
                compute.relative(bad)

    def test_summary_reference_identity_and_missing_baseline_rejected(self):
        site.check_report(self.report, SHA)
        for mutate in (
            lambda r: r["cases"].pop(),
            lambda r: r["summary"]["validation-legacy"].update(required=0),
            lambda r: r["reference_files"].update(compute240="d" * 64),
            lambda r: r.update(compute_base_status="available"),
        ):
            report = copy.deepcopy(self.report)
            mutate(report)
            with self.assertRaises((ValueError, KeyError)):
                site.check_report(report, SHA)
        other = copy.deepcopy(self.report)
        other["reference_sha256"] = "e" * 64
        self.assertNotEqual(site.series_key(other), site.series_key(self.report))

    def test_base_capability_comes_from_programs_not_measurement_file(self):
        repository = self.directory / "history"
        repository.mkdir()
        def git(*args):
            return subprocess.check_output(["git", "-C", str(repository), *args], stderr=subprocess.DEVNULL).decode().strip()
        git("init", "-q")
        git("config", "user.name", "test")
        git("config", "user.email", "test@example.invalid")
        git("commit", "--allow-empty", "-qm", "no compute declaration")
        absent = git("rev-parse", "HEAD")
        with self.assertRaisesRegex(ValueError, "unrecognized base"):
            compute.base_declares_cohort(absent, repository)
        for name in ("tests/benchmarks/manifest.json", "tests/benchmarks/provenance.json"):
            target = repository / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, target)
        git("add", "tests")
        git("commit", "-qm", "declare complete compute programs without timing measurements")
        self.assertTrue(compute.base_declares_cohort(git("rev-parse", "HEAD"), repository))
        # Isolate the one explicitly supported old boundary; no real repository
        # history is needed in shallow CI checkouts.
        with patch.object(compute.subprocess, "run", side_effect=[SimpleNamespace(returncode=0), SimpleNamespace(returncode=1)]):
            self.assertFalse(compute.base_declares_cohort("cb965b4d41fc7d52281652c3bce7803cc893cae0", repository))

    def test_compute_comparison_checks_all_rows_and_capable_base_failures(self):
        head, base = self.directory / "head", self.directory / "base"
        head.write_bytes(b"head test executable identity")
        base.write_bytes(b"base test executable identity")
        original = copy.deepcopy(self.report["original_report"])
        original.update(head_binary_sha256=compute.legacy.digest(head), base_binary_sha256=compute.legacy.digest(base), base_commit="cae89ddaf49e7150058098c81de415a22bdab6cc")
        args = SimpleNamespace(head=str(head), base=str(base), base_commit=original["base_commit"],
                               compute_reference=str(self.folder), reference=str(self.source / compute.legacy.PERFORMANCE_REFERENCE))
        calls = []
        def execute(binary, case, mode=None, expected_output=None):
            calls.append((binary, case["id"], mode))
            return dict(instructions=100, stdout_base64=base64.b64encode(expected_output).decode(),
                        build={"vcs.revision": SHA if binary == str(head) else original["base_commit"], "vcs.modified": "false"},
                        timing=dict(user_cycles=6980000, predicted_user_seconds=.002, cpi=69800, retired_instructions=100,
                                    cache={k: 0 for k in ("L1Hits", "L1Misses", "L2Hits", "L2Misses")},
                                    branch_predictions=0, branch_direction_hits=0, **self.declared["profile"]))
        with patch.object(compute, "base_declares_cohort", return_value=True), patch("builtins.print"):
            report = compute.extend_report(original, args, execute, gate.validate_timing, gate.error_percent, lambda: original["head"])
        self.assertEqual(report["artifact_status"], "pass", report.get("error"))
        self.assertEqual(len(report["cases"]), 264)
        self.assertEqual(len(calls), 480 + sum(compute.case_base_status(c, original["base_capability"]) == "available" for c in self.declared["cases"]))
        self.assertTrue(all(r["error_change_percentage_points"] == 0 for r in report["cases"][24:] if "base" in r))
        def failed(binary, *a, **kw):
            if binary == str(base):
                raise ValueError("capable base regressed")
            return execute(binary, *a, **kw)
        with patch.object(compute, "base_declares_cohort", return_value=True), patch("builtins.print"):
            report = compute.extend_report(original, args, failed, gate.validate_timing, gate.error_percent, lambda: original["head"])
        self.assertEqual(report["artifact_status"], "fail")
        self.assertIn("capable base regressed", report["error"])

    def test_bundle_rejects_rehashed_metadata_and_timing_tampering(self):
        report_path = self.directory / "report.json"
        compute.write(report_path, self.report)
        report_path.with_suffix(".md").write_text("# Synthetic test report\n")
        bundle = self.directory / "bundle"
        site.bundle(report_path, self.source, bundle, SHA)
        altered = copy.deepcopy(self.report)
        altered["head"]["files"]["scripts/compute_usertime.py"] = "0" * 64
        with self.assertRaisesRegex(ValueError, "does not match tested commit"):
            site.check_compute_source_identity(altered, bundle, ["cohorts/compute/source/scripts/compute_usertime.py"])
        for mutate in (
            lambda r: r.update(base_commit="f" * 40),
            lambda r: r.update(head_binary_sha256="f" * 64),
            lambda r: r["compute_reference"].update(hardware="different machine"),
            lambda r: r["cases"][24]["head"].update(cycles=1),
            lambda r: r["cases"][24]["head"].update(direction="faster"),
            lambda r: r["cases"][24].update(error_change_percentage_points=99),
        ):
            report = copy.deepcopy(self.report)
            mutate(report)
            compute.write(bundle / "report.json", report)
            manifest = compute.read(bundle / "bundle.json")
            manifest["files"]["report.json"] = compute.legacy.digest(bundle / "report.json")
            compute.write(bundle / "bundle.json", manifest)
            with self.assertRaises(ValueError):
                site.validate_bundle(bundle, SHA)


    def committed_bundle(self):
        # A real commit includes every declared source/reference byte. The
        # publisher must verify Git objects, not merely a self-consistent bundle.
        (self.source / "marker.txt").write_text("preserve unrelated content\n")
        def git(*args):
            return subprocess.check_output(["git", "-C", str(self.source), *args], stderr=subprocess.DEVNULL).decode().strip()
        git("init", "-q")
        git("config", "user.name", "test")
        git("config", "user.email", "test@example.invalid")
        for name, field in (("pkg/loader/elf.go", "loader_source"), ("pkg/linuxuser/run.go", "runtime_source")):
            destination = self.source/name
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_text(self.report["base_capability"][field])
        git("add", "pkg")
        git("commit", "-qm", "synthetic legacy base capability")
        base_commit = git("rev-parse", "HEAD")
        for name in ("pkg/loader/elf.go", "pkg/linuxuser/run.go"):
            shutil.copyfile(ROOT/name,self.source/name)
        git("add", ".")
        git("commit", "-qm", "synthetic committed evidence fixture")
        git("branch", "-M", "main")
        commit = git("rev-parse", "HEAD")
        report = copy.deepcopy(self.report)
        report["base_commit"] = report["original_report"]["base_commit"] = base_commit
        state = dict(commit=commit, dirty=False, files={name: compute.legacy.digest(self.source / name) for name in git("ls-tree", "-r", "--name-only", commit).splitlines()})
        report["head"] = state
        report["original_report"]["head"] = state
        path = self.directory / "committed-report.json"
        compute.write(path, report)
        path.with_suffix(".md").write_text("# Synthetic committed evidence test\n")
        bundle = self.directory / "committed-bundle"
        site.bundle(path, self.source, bundle, commit)
        return commit, report, bundle, git

    @staticmethod
    def metadata(commit, kind="main", run=1):
        url = f"https://github.com/sarchlab/poros/actions/runs/{run}/attempts/1"
        result = dict(commit=commit, kind=kind, conclusion="success", run_id=run, run_attempt=1, run_url=url,
                      evaluation_conclusion="success", evaluation_run_id=run, evaluation_run_attempt=1, evaluation_run_url=url)
        if kind == "release":
            result["release"] = dict(id=run, tag="v-test")
        return result

    def test_committed_schema3_publisher_verification_and_rehashed_tamper(self):
        commit, report, bundle, git = self.committed_bundle()
        publisher.verify_source(self.source, bundle, commit)
        tampered = self.directory / "rehashed-bundle"
        shutil.copytree(bundle, tampered)
        changed = copy.deepcopy(report)
        changed["head"]["files"]["marker.txt"] = "0" * 64
        changed["original_report"]["head"] = changed["head"]
        compute.write(tampered / "report.json", changed)
        compute.write(tampered / "cohorts/original/report.json", changed["original_report"])
        for folder in (tampered / "cohorts/original", tampered):
            manifest = compute.read(folder / "bundle.json")
            manifest["files"] = {name: compute.legacy.digest(folder / name) for name in manifest["files"]}
            compute.write(folder / "bundle.json", manifest)
        # Internal consistency alone is insufficient: a rewritten source hash
        # must still be rejected against the actual recorded Git commit.
        site.validate_bundle(tampered, commit)
        with self.assertRaisesRegex(ValueError, "source hash differs from commit"):
            publisher.verify_source(self.source, tampered, commit)

        forged = self.directory / "forged-base"
        shutil.copytree(bundle, forged)
        changed = copy.deepcopy(report)
        capability = copy.deepcopy(changed["base_capability"])
        capability["loader_source"] = capability["loader_source"].replace("= 64 << 20", "= 256 << 20")
        capability["runtime_source"] = capability["runtime_source"].replace("= 16 << 20", "= 64 << 20")
        capability.update(image_limit_bytes=256 << 20, output_limit_bytes=64 << 20,
                          loader_source_sha256=hashlib.sha256(capability["loader_source"].encode()).hexdigest(),
                          runtime_source_sha256=hashlib.sha256(capability["runtime_source"].encode()).hexdigest())
        compute.validate_base_capability(capability)
        for part in (changed, changed["original_report"]):
            part["base_capability"] = capability
            for row in part["cases"]:
                row["base_status"]="available"
                row["base"]=copy.deepcopy(row["head"])
                if "error_percent" in row["head"]:row["error_change_percentage_points"]=0
            part["summary"]={k:v for k,v in compute.summaries(part["cases"],True).items() if part["schema"]==3 or k!="validation-legacy"}
        compute.write(forged/"report.json",changed)
        compute.write(forged/"cohorts/original/report.json",changed["original_report"])
        for folder in (forged/"cohorts/original",forged):
            metadata=compute.read(folder/"bundle.json")
            metadata["files"]={name:compute.legacy.digest(folder/name) for name in metadata["files"]}
            compute.write(folder/"bundle.json",metadata)
        site.validate_bundle(forged,commit)
        with self.assertRaisesRegex(ValueError,"base ELF capability differs"):
            publisher.verify_source(self.source,forged,commit)

        remote = self.directory / "remote.git"
        subprocess.run(["git", "init", "--bare", str(remote)], check=True, capture_output=True)
        git("remote", "add", "origin", str(remote))
        git("push", "origin", "main:main", "main:gh-pages")
        publication = publisher.publish(self.source, bundle, self.metadata(commit), str(remote), build=False)
        self.assertEqual(git("ls-remote", str(remote), "gh-pages").split()[0], publication)
        index = json.loads(subprocess.check_output(["git", "--git-dir", str(remote), "show", "gh-pages:calibration/index.json"]))
        self.assertEqual(index["entries"][0]["summary"]["validation-legacy"]["required"], 228)
        marker = subprocess.check_output(["git", "--git-dir", str(remote), "show", "gh-pages:marker.txt"], text=True)
        self.assertEqual(marker, "preserve unrelated content\n")

    def test_schema3_archive_reuse_release_retention_and_index(self):
        commit, report, bundle, _ = self.committed_bundle()
        pages = self.directory / "pages"
        order = ["d" * 40, "c" * 40, commit]
        site.archive(pages, bundle, self.metadata(commit), order, keep=1)
        site.archive(pages, bundle, self.metadata(commit, "release"), order, keep=1)
        archived = pages / "calibration/runs" / commit
        self.assertTrue(site.reusable_bundle(archived, commit))
        for i, revision in enumerate(("c" * 40, "d" * 40), 2):
            next_report = copy.deepcopy(report)
            next_report["head"]["commit"] = revision
            next_report["original_report"]["head"]["commit"] = revision
            path = self.directory / (revision + ".json")
            compute.write(path, next_report)
            path.with_suffix(".md").write_text("# Synthetic retention test\n")
            next_bundle = self.directory / revision
            site.bundle(path, self.source, next_bundle, revision)
            index = site.archive(pages, next_bundle, self.metadata(revision, run=i), order, keep=1)
        self.assertEqual({e["commit"] for e in index["entries"]}, {commit, "d" * 40})
        self.assertTrue(site.reusable_bundle(archived, commit))
        self.assertFalse((pages / "calibration/runs" / ("c" * 40)).exists())
        home = (pages / "calibration/index.html").read_text()
        self.assertTrue("Validation-legacy mean" in home, "third split missing from history")
        self.assertTrue("v-test" in home, "release pin missing from history")
        retained, _ = site.validate_bundle(archived, commit)
        self.assertEqual(len(retained["cases"]), 264)
        self.assertEqual(len(site.report_manifest(retained)["workloads"]), 44)
        self.assertEqual(index["latest_success"], "d" * 40)

    def test_public_destination_never_leaves_partial_pass(self):
        output = self.directory / "partial.json"
        args = SimpleNamespace(output=str(output), compute_reference=str(self.folder))
        def partial(old_args, executor=None):
            compute.write(old_args.output, self.report["original_report"])
            return 0
        with patch.object(gate, "compare24", side_effect=partial), patch.object(compute, "extend_report", side_effect=RuntimeError("interruption")):
            with self.assertRaises(RuntimeError):
                gate.compare_legacy(args)
        self.assertEqual(compute.read(output)["artifact_status"], "fail")


if __name__ == "__main__":
    unittest.main()
