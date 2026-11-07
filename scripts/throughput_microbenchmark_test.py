#!/usr/bin/env python3
"""Throughput evidence fails closed; real execution smoke runs separately in CI."""
import base64
import copy
import json
from pathlib import Path
import shutil
import struct
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import throughput_microbenchmark as suite
import accuracy_site as site
import accuracy_test_support as fixtures
import compute_usertime as compute
import frontend_microbenchmark as frontend


class ThroughputTests(unittest.TestCase):
    def setUp(self):
        self.reference = suite.load_reference(suite.ROOT)
        self.cases = suite.cases()
        self.rows = [dict(id=c["id"], instructions=c["instructions_per_iteration"]*1024, cycles=100000) for c in self.cases]

    def test_reference_requires_full_manifest_and_source_identity(self):
        mutations = [lambda r: r["cases"].pop(), lambda r: r["cases"].reverse(),
                     lambda r: r["cases"][0].update(median_seconds=float("nan")),
                     lambda r: r["cases"][0].update(p10_seconds=-1),
                     lambda r: r.update(definition=r["definition"] + "\n"),
                     lambda r: r["sources"].pop(suite.ELF),
                     lambda r: r.update(timer="wall time"), lambda r: r.update(trials=1)]
        for mutate in mutations:
            bad = copy.deepcopy(self.reference)
            mutate(bad)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                suite.validate_reference(bad)
        with self.assertRaises(ValueError):
            suite.validate_reference(self.reference, {})

    def test_native_inputs_are_live_bound_but_collector_provenance_is_historical(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            for name in [*suite.INPUTS, suite.REFERENCE]:
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(suite.ROOT / name, target)
            for name in suite.COLLECTOR_SOURCES:
                (root / name).write_bytes(b"a changed collector or rebuilt ELF")
            self.assertEqual(suite.load_reference(root), self.reference)
            suite.validate_reference(self.reference, {name: suite.digest(root / name) for name in suite.INPUTS})
            for name in suite.MEASUREMENT_INPUTS:
                original = (root / name).read_bytes()
                (root / name).write_bytes(original + b"\n")
                with self.subTest(name=name), self.assertRaises(ValueError):
                    suite.load_reference(root)
                (root / name).write_bytes(original)
            # The saved collector/ELF digests are never rewritten to today's files.
            self.assertEqual(json.loads((root / suite.REFERENCE).read_text()), self.reference)

    def test_simulation_requires_exact_case_order_and_instruction_differences(self):
        suite.validate_simulation(self.rows, self.cases)
        for mutate in (lambda r: r.pop(), lambda r: r.reverse(), lambda r: r[0].update(cycles=True),
                       lambda r: r[0].update(cycles=0), lambda r: r[0].update(instructions=1)):
            bad = copy.deepcopy(self.rows)
            mutate(bad)
            with self.assertRaises(ValueError):
                suite.validate_simulation(bad, self.cases)

    def test_execution_checks_checksum_profile_and_exact_build(self):
        case = next(c for c in self.cases if c["kind"] == "store")
        profile = dict(configuration={"FetchWidth": 3}, cache_config={})
        row = dict(termination="exit", exit_status=0, stderr_base64="", env=[],
                   binary_sha256=suite.digest(suite.ROOT / suite.ELF), args=["program", "14", "8"],
                   stdout_base64=base64.b64encode(struct.pack("<Q", 164)).decode(), instructions=3000,
                   build={"vcs.revision": "a"*40, "vcs.modified": "false"},
                   timing=dict(retired_instructions=3000, user_cycles=2000, **profile))
        mutations = [None, lambda r: r.update(stdout_base64=base64.b64encode(struct.pack("<Q", 36)).decode()),
                     lambda r: r["build"].update({"vcs.modified": "true"}),
                     lambda r: r["timing"].update(configuration={"FetchWidth": 8}),
                     lambda r: r.update(binary_sha256="0"*64), lambda r: r.update(args=["wrong"])]
        for mutate in mutations:
            result = copy.deepcopy(row)
            if mutate:
                mutate(result)
            with patch.object(suite.subprocess, "run", return_value=SimpleNamespace(stdout=json.dumps(result), stderr=b"")):
                if mutate:
                    with self.assertRaises(ValueError):
                        suite.execute("poros", suite.ROOT, case, 14, 8, "ooo", "a"*40, profile)
                else:
                    suite.execute("poros", suite.ROOT, case, 14, 8, "ooo", "a"*40, profile)

    def test_report_renders_all_cases_without_hardware_width_claim(self):
        page = site.throughput_table(self.reference, self.rows)
        for case in self.cases:
            self.assertIn(case["id"], page)
        self.assertIn("does not establish hardware issue width", page)
        self.assertIn("P10", page)
        self.assertNotIn("NOP currently", page)

    def test_bundle_roundtrip_requires_new_evidence_and_preserves_application_stats(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            fixture = fixtures.Evidence(suite.ROOT, root / "fixture")
            report = fixtures.compact_report(fixture)
            before = site.benchmark_summary(report)
            for name in [*suite.INPUTS, suite.REFERENCE, frontend.ELF, site.FRONTEND_REFERENCE]:
                target = fixture.source / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(suite.ROOT / name, target)
                report["head"]["files"][name] = suite.digest(target)
            report["throughput_simulation"] = self.rows
            raw = root / "report.json"
            compute.write(raw, report)
            raw.with_suffix(".md").write_text("Synthetic application unit data")
            bundle = root / "bundle"
            site.bundle(raw, fixture.source, bundle, fixtures.SHA)
            valid, manifest = site.validate_bundle(bundle, fixtures.SHA)
            self.assertEqual(site.benchmark_summary(valid), before)
            page = (bundle / "index.html").read_text()
            overall, micro = page.split('<section id="microbenchmarks">')
            self.assertNotIn("Instruction throughput suite", overall)
            self.assertIn("Instruction throughput suite", micro)
            self.assertIn("throughput.json", manifest["files"])
            self.assertNotIn("frontend.json", manifest["files"])
            self.assertNotIn("Front-end throughput on", micro)
            for mutate in (lambda r: r.pop("throughput_simulation"),
                           lambda r: r["throughput_simulation"].pop(),
                           lambda r: r["head"]["files"].update({suite.DIRECTORY + "loops.S": "0"*64}),
                           lambda r: r.update(frontend_simulation={})):
                bad = copy.deepcopy(report)
                mutate(bad)
                compute.write(bundle / "report.json", bad)
                manifest["files"]["report.json"] = suite.digest(bundle / "report.json")
                compute.write(bundle / "bundle.json", manifest)
                with self.assertRaises(ValueError):
                    site.validate_bundle(bundle, fixtures.SHA)

    def test_historical_frontend_only_bundle_renders_and_archives(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            fixture = fixtures.Evidence(suite.ROOT, root / "fixture")
            report = fixtures.compact_report(fixture)
            for name in (frontend.ELF, site.FRONTEND_REFERENCE):
                target = fixture.source / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(suite.ROOT / name, target)
                report["head"]["files"][name] = suite.digest(target)
            report["frontend_simulation"] = {name: dict(instructions=frontend.LOOP_INSTRUCTIONS, cycles=352256)
                                             for name in ("nop", "add")}
            raw = root / "report.json"
            compute.write(raw, report)
            raw.with_suffix(".md").write_text("Synthetic historical application unit data")
            bundle = root / "bundle"
            site.bundle(raw, fixture.source, bundle, fixtures.SHA)
            valid, manifest = site.validate_bundle(bundle, fixtures.SHA)
            self.assertEqual(valid, report)
            self.assertEqual(set(manifest["files"]), {"report.json", "report.md", "reference.json", "frontend.json"})
            url = "https://github.com/sarchlab/poros/actions/runs/1/attempts/1"
            metadata = dict(commit=fixtures.SHA, kind="main", conclusion="success", run_id=1, run_attempt=1,
                            run_url=url, evaluation_conclusion="success", evaluation_run_id=1,
                            evaluation_run_attempt=1, evaluation_run_url=url)
            pages = root / "pages"
            site.archive(pages, bundle, metadata, [fixtures.SHA])
            for page in (bundle / "index.html", pages / "calibration/runs" / fixtures.SHA / "index.html"):
                text = page.read_text()
                self.assertIn("Front-end throughput on Apple M2", text)
                self.assertIn("Simulated IPC", text)
                self.assertNotIn("Instruction throughput suite", text)


if __name__ == "__main__":
    unittest.main()
