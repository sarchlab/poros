#!/usr/bin/env python3
"""Capacity evidence shares transport checks without joining application stats."""
import copy
import hashlib
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

import capacity_microbenchmark as suite
import throughput_microbenchmark as throughput
import accuracy_site as site
import accuracy_test_support as fixtures
import compute_usertime as compute


class CapacityTests(unittest.TestCase):
    def setUp(self):
        self.reference = suite.load_reference(suite.ROOT)
        self.cases = suite.cases()
        self.rows = [dict(id=c["id"], instructions=c["instructions_per_iteration"] * 1024, cycles=100000) for c in self.cases]
        self.comparison = {role: copy.deepcopy(self.rows) for role in ("base", "head")}

    def test_suite_is_explicit_and_does_not_change_old_reference(self):
        old = throughput.load_reference(suite.ROOT)
        self.assertEqual(len(old["cases"]), 17)
        self.assertEqual(len(self.cases), 73)
        self.assertNotEqual(suite.SUITE.protocol, throughput.THROUGHPUT.protocol)
        self.assertEqual(suite.expected(dict(kind="window", id="fp_g64_n64"), 2), 5)
        self.assertEqual(suite.expected(dict(kind="fdiv", id="fdiv_normal_lanes16"), 2), 24)
        self.assertEqual(suite.expected(dict(kind="fsqrt", id="fsqrt_normal_lanes16"), 16), 25)
        with self.assertRaises(ValueError):
            throughput.validate_reference(self.reference)
        for mutation in (lambda r: r["cases"].pop(), lambda r: r.update(protocol=throughput.PROTOCOL),
                         lambda r: r["sources"].pop(suite.ELF), lambda r: r["cases"][0].update(median_seconds=float("nan"))):
            bad = copy.deepcopy(self.reference)
            mutation(bad)
            with self.assertRaises(ValueError):
                suite.validate_reference(bad)

    def test_comparison_requires_both_roles_and_exact_instruction_counts(self):
        suite.validate_comparison(self.comparison, self.cases)
        for mutation in (lambda r: r.pop("base"), lambda r: r["base"].pop(),
                         lambda r: r["head"][0].update(instructions=1)):
            bad = copy.deepcopy(self.comparison)
            mutation(bad)
            with self.assertRaises(ValueError):
                suite.validate_comparison(bad, self.cases)

    def test_boundary_provenance_quality_and_full_sample_coverage(self):
        for mutation in (lambda r: r["boundary"]["cases"][0]["samples"].pop(),
                         lambda r: r["boundary"]["cases"][0].update(qualified=True, median_ratio=0),
                         lambda r: r["boundary"]["sources"].update({suite.MEASUREMENT_INPUTS[0]: "0"*64}),
                         lambda r: r["boundary"]["cases"][0]["samples"][0].update(target_ns=float("inf")),
                         lambda r: r.pop("boundary")):
            bad = copy.deepcopy(self.reference)
            mutation(bad)
            with self.assertRaises(ValueError):
                suite.validate_reference(bad)

    def test_panels_sort_actual_filler_counts_and_active_lanes(self):
        profiles = {role: {"configuration": {"freq": 3490000000}} for role in ("base", "head")}
        with patch.object(site, "chart", return_value="chart") as chart:
            site.capacity_panels(self.reference, self.comparison, profiles)
        self.assertEqual(chart.call_count, 9)
        for call in chart.call_args_list:
            xs = [p[0] for p in call.args[1][0][1]]
            self.assertEqual(xs, sorted(xs))
        for call in chart.call_args_list[-4:]:
            self.assertEqual([p[0] for p in call.args[1][0][1]], [1, 4, 16])

    def test_bundle_and_archive_keep_capacity_separate_and_fail_closed(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            fixture = fixtures.Evidence(suite.ROOT, root / "fixture")
            report = fixtures.compact_report(fixture)
            before = site.benchmark_summary(report)
            for name in [*suite.INPUTS, suite.REFERENCE]:
                target = fixture.source / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(suite.ROOT / name, target)
                report["head"]["files"][name] = suite.digest(target)
            report["capacity_simulation"] = self.comparison
            raw = root / "report.json"
            compute.write(raw, report)
            raw.with_suffix(".md").write_text("Synthetic unit application data")
            bundle = root / "bundle"
            site.bundle(raw, fixture.source, bundle, fixtures.SHA)
            valid, manifest = site.validate_bundle(bundle, fixtures.SHA)
            self.assertEqual(before, site.benchmark_summary(valid))
            page = (bundle / "index.html").read_text()
            overall, micro = page.split('<section id="microbenchmarks">')
            self.assertNotIn("FP capacity and divider controls", overall)
            self.assertIn("FP capacity and divider controls", micro)
            self.assertIn("ns/iteration", micro)
            self.assertIn('<details id="capacity-table">', micro)
            self.assertLess(micro.index("FP capacity and divider controls"), micro.index('id="capacity-table"'))
            self.assertIn("capacity.json", manifest["files"])
            url = "https://github.com/sarchlab/poros/actions/runs/1/attempts/1"
            metadata = dict(commit=fixtures.SHA, kind="main", conclusion="success", run_id=1, run_attempt=1,
                            run_url=url, evaluation_conclusion="success", evaluation_run_id=1,
                            evaluation_run_attempt=1, evaluation_run_url=url)
            pages = root / "pages"
            site.archive(pages, bundle, metadata, [fixtures.SHA])
            archived = (pages / "calibration/runs" / fixtures.SHA / "index.html").read_text()
            self.assertIn("FP capacity and divider controls", archived)
            self.assertIn('<details id="capacity-table">', archived)

            for mutate in (lambda r: r.pop("capacity_simulation"),
                           lambda r: r["capacity_simulation"].pop("base"),
                           lambda r: r["head"]["files"].update({suite.DIRECTORY + "loops.S": "0" * 64})):
                bad = copy.deepcopy(report)
                mutate(bad)
                compute.write(bundle / "report.json", bad)
                manifest["files"]["report.json"] = suite.digest(bundle / "report.json")
                compute.write(bundle / "bundle.json", manifest)
                with self.assertRaises(ValueError):
                    site.validate_bundle(bundle, fixtures.SHA)


if __name__ == "__main__":
    unittest.main()
