#!/usr/bin/env python3
"""Expanded evidence gates; synthetic fixtures here are never timing references."""
import base64
import copy
import hashlib
import json
from pathlib import Path
import shutil
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import accuracy_site as site
import compute_usertime as compute
import usertime as gate

ROOT = Path(__file__).resolve().parents[1]
SHA = "a" * 40


class ComputeTimeTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.source = self.directory / "source"
        for name in compute.source_paths(ROOT) | {"evaluation/usertime/m2-linuxvm-user-v2.json"}:
            target = self.source / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, target)
        self.folder = self.source / compute.REFERENCE
        self.declared = compute.contract(self.source)
        self.reference = self.make_reference()
        self.report = self.make_report()

    def make_reference(self):
        rows, oracle = [], []
        for case in self.declared["cases"]:
            length = sum(x["count"] * (8 if x["type"] == "double" else 4) for x in case["outputs"])
            data = bytes(length)  # Deliberately synthetic finite test output.
            sha = hashlib.sha256(data).hexdigest()
            target = self.folder / "outputs" / (sha + ".bin")
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
            sample = dict(user_seconds=.001, system_seconds=.0001, exit_status=0,
                          stdout_sha256=sha, stdout_bytes=length, stderr_bytes=0,
                          stderr_sha256=hashlib.sha256(b"").hexdigest())
            rows.append(dict(id=case["id"], samples=[copy.deepcopy(sample) for _ in range(21)],
                             warmup_sample=sample, median_user_seconds=.001, quality="usable",
                             output_sha256=sha, output_bytes=length))
            oracle.append(dict(id=case["oracle_id"], args=case["args"][1:], binary_sha256=case["sha256"],
                               status="pass", output_sha256=sha, output_bytes=length, native_sha256=sha, qemu_sha256=sha))
        harness = (self.source / "scripts/benchmark-differential.py").read_bytes()
        validation = dict(schema=1, acceptance_status="pass", status="pass", expected_cases=240, qemu="synthetic unit oracle", platform="synthetic",
                          provenance=dict(git_dirty=False, git_sha=SHA, sources={**self.declared["build"]["files"], "scripts/benchmark-differential.py": hashlib.sha256(harness).hexdigest()}),
                          build=self.declared["build"], manifest_sha256=self.declared["program_manifest_sha256"], cases=oracle)
        compute.write(self.folder / "validation-report.json", validation)
        (self.folder / "validation-harness.py").write_bytes(harness)
        compute.write(self.folder / "oracle.json", compute.compact_oracle(validation, self.declared))
        reference = dict(schema=1, protocol=compute.PROTOCOL, measurement_id="synthetic-unit-only", contract=self.declared,
                         collection_commit=SHA, collection_dirty=False, quality_policy=compute.QUALITY_POLICY, container_image="sha256:" + "f" * 64,
                         started_at_utc="2026-01-01T00:00:00Z", completed_at_utc="2026-01-01T00:05:00Z",
                         precision_floor_seconds=.0001, retained_samples_per_case=21, warmup="one complete process per case",
                         metric="child_user_cpu_seconds", timer="wait4.ru_utime", environment="virtualized", machine="aarch64",
                         hardware="synthetic", platform="synthetic", core_identity="unknown", instantaneous_frequency="unknown",
                         oracle_sha256=compute.legacy.digest(self.folder / "oracle.json"),
                         support_sha256={name: compute.legacy.digest(self.source / name) for name in compute.SUPPORT}, cases=rows)
        compute.write(self.folder / "reference.json", reference)
        return reference

    def make_report(self):
        old_manifest = compute.legacy.load_manifest(self.source)
        old_reference = compute.read(self.source / "evaluation/usertime/m2-linuxvm-user-v2.json")
        old_rows = []
        for case, reference in zip(old_manifest["cases"], old_reference["cases"]):
            seconds = max(reference["median_user_seconds"], .001)
            head = dict(predicted_user_seconds=seconds, instructions=100, cycles=100, **old_manifest["profile"])
            if reference["quality"] == "usable":
                head.update(error_percent=gate.error_percent(seconds, reference["median_user_seconds"]), direction="equal" if seconds == reference["median_user_seconds"] else "slower")
            old_rows.append(dict(id=case["id"], split=case["split"], functional_status="pass", reference_quality=reference["quality"],
                                 reference_user_seconds=reference["median_user_seconds"], head=head, base=copy.deepcopy(head), error_change_percentage_points=0))
        old = dict(schema=2, artifact_status="pass", accuracy_status="unverified", formula=gate.FORMULA,
                   head=dict(commit=SHA, dirty=False, files={str(p.relative_to(self.source)): compute.legacy.digest(p) for p in self.source.rglob("*") if p.is_file()}),
                   base_commit="cb965b4d41fc7d52281652c3bce7803cc893cae0", base_status="available", head_binary_sha256="d" * 64, base_binary_sha256="e" * 64, workload_manifest=old_manifest, reference_sha256=compute.legacy.digest(self.source / "evaluation/usertime/m2-linuxvm-user-v2.json"),
                   reference=old_reference, cases=old_rows, summary={k: v for k, v in compute.summaries(old_rows).items() if k != "validation-legacy"})
        rows = copy.deepcopy(old_rows)
        for case in self.declared["cases"]:
            rows.append(dict(id=case["id"], split=case["split"], functional_status="pass", reference_quality="usable", reference_user_seconds=.001,
                             head=dict(predicted_user_seconds=.002, instructions=100, cycles=6980000, error_percent=100, direction="slower", **old_manifest["profile"])))
        refs = dict(original24=old["reference_sha256"], compute240=compute.legacy.digest(self.folder / "reference.json"))
        return dict(schema=3, artifact_status="pass", accuracy_status="unverified", formula=gate.FORMULA,
                    head=old["head"], base_commit=old["base_commit"], original_report=old, base_status=old["base_status"], head_binary_sha256=old["head_binary_sha256"], base_binary_sha256=old["base_binary_sha256"], reference=old["reference"],
                    workload_manifest=compute.expanded_manifest(old_manifest, self.declared), cases=rows,
                    reference_files=refs, reference_sha256=compute.reference_identity(refs["original24"], refs["compute240"]),
                    compute_base_status="compute_elf_unavailable", compute_reference={key: self.reference[key] for key in ("measurement_id", "environment", "hardware", "platform", "core_identity", "instantaneous_frequency", "completed_at_utc")},
                    summary=compute.summaries(rows))

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
        path = self.folder / "outputs" / (self.reference["cases"][0]["output_sha256"] + ".bin")
        path.write_bytes(path.read_bytes()[:-1])
        with self.assertRaises(ValueError):
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
        self.assertTrue("242 / 264" in html, "qualified coverage must retain old24 quality")
        self.assertTrue("validation-legacy" in html, "missing third split")
        self.assertTrue("m=1, n=1" in html, "missing actual dimension labels")
        self.assertEqual(html.count("<figcaption>"), 45)  # 44 workload curves + comparison scatter
        self.assertIn("cohorts/compute/source/parsec/kernels/bodytrack.c", manifest["files"])
        self.assertIn("cohorts/compute/source/tests/benchmarks/bodytrack.c", manifest["files"])
        path = bundle / "cohorts/compute/reference/outputs" / (self.reference["cases"][0]["output_sha256"] + ".bin")
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
        self.assertTrue(compute.base_declares_cohort("cae89ddaf49e7150058098c81de415a22bdab6cc"))
        self.assertFalse(compute.base_declares_cohort("cb965b4d41fc7d52281652c3bce7803cc893cae0"))

    def test_compute_comparison_checks_all_rows_and_capable_base_failures(self):
        head, base = self.directory / "head", self.directory / "base"
        head.write_bytes(b"head test executable identity")
        base.write_bytes(b"base test executable identity")
        original = copy.deepcopy(self.report["original_report"])
        original.update(head_binary_sha256=compute.legacy.digest(head), base_binary_sha256=compute.legacy.digest(base), base_commit="cae89ddaf49e7150058098c81de415a22bdab6cc")
        args = SimpleNamespace(head=str(head), base=str(base), base_commit=original["base_commit"],
                               compute_reference=str(self.folder), reference=str(self.source / "evaluation/usertime/m2-linuxvm-user-v2.json"))
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
        self.assertEqual(len(calls), 720)
        self.assertEqual(report["cases"][-1]["error_change_percentage_points"], 0)
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


    def test_public_destination_never_leaves_partial_pass(self):
        output = self.directory / "partial.json"
        args = SimpleNamespace(output=str(output), compute_reference=str(self.folder))
        def partial(old_args):
            compute.write(old_args.output, self.report["original_report"])
            return 0
        with patch.object(gate, "compare24", side_effect=partial), patch.object(compute, "extend_report", side_effect=RuntimeError("interruption")):
            with self.assertRaises(RuntimeError):
                gate.compare(args)
        self.assertEqual(compute.read(output)["artifact_status"], "fail")


if __name__ == "__main__":
    unittest.main()
