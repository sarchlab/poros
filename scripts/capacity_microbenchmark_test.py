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

    def test_committed_binaries_match_acquisition_provenance(self):
        self.assertEqual(suite.digest(suite.ROOT / suite.ELF), self.reference["sources"][suite.ELF])
        self.assertEqual(self.reference["boundary"]["native_binary_sha256"], self.reference["native_binary_sha256"])

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
        with patch.object(site, "chart", return_value="chart") as chart, patch.object(suite, "ITERATIONS", (128, 384)):
            site.capacity_panels(self.reference, self.comparison, profiles)
        self.assertEqual(chart.call_count, 10)
        self.assertAlmostEqual(chart.call_args_list[0].args[1][1][1][0][1], 100000 / 256 / 3.49)
        for call in chart.call_args_list:
            xs = [p[0] for p in call.args[1][0][1]]
            self.assertEqual(xs, sorted(xs))
        for call in chart.call_args_list[5:9]:
            self.assertEqual([p[0] for p in call.args[1][0][1]], [1, 4, 16])
        boundary = chart.call_args_list[-1]
        self.assertEqual(len(boundary.kwargs["y_ranges"]), 34)
        self.assertEqual([series[0] for series in boundary.args[1]], ["gate 64", "gate 96"])
        for series in boundary.args[1]:
            self.assertEqual(len(series[1]), 17)  # Noisy rows remain plotted.

    def test_live_update_keeps_an_interior_boundary_row_in_place(self):
        """The page's own scripts, run in Node. Every boundary row of a gate
        starts with that gate, so a row is known by its case: a reader with a
        middle row at the top of the window stays there across an update that
        changes nothing."""
        profiles = {role: {"configuration": {"freq": 3490000000}} for role in ("base", "head")}
        graphs, tables = site.capacity_panels(self.reference, self.comparison, profiles)
        # A running report around the capacity panels: what the update script asks of a document.
        page = site.live_document("Capacity", '<header><h1>Capacity</h1></header><div role="progressbar"></div>' + graphs + tables,
                                  dict(report="capacity"), True)
        gate = sorted((row["id"] for row in self.reference["boundary"]["cases"] if row["id"].startswith("fp_g64_")),
                      key=lambda ident: int(ident.split("_n")[1]))
        self.assertEqual((len(gate), page.count(' data-row-key="fp_g')), (17, 34))
        middle = gate[8]

        def across_an_update(document, attribute):
            row = f'#capacity-boundary-table tr[{attribute}="{middle}"]'
            steps = [["open", "#capacity-boundary-table"], ["anchor", row]]
            observed = fixtures.live_dom([document, document], steps + [["observe", None]])
            if observed is None:
                self.skipTest("Node is not installed")
            before, after = fixtures.live_dom([document, document], steps + [["scroll", observed[0]["anchorTop"]], ["observe", None], ["refresh", None]])
            self.assertEqual((before["anchorTop"], after["fetches"], after["reloads"], after["open"]), (0, 1, 0, ["capacity-boundary-table"]))
            return after["anchorTop"], after["scrollY"] - before["scrollY"]

        self.assertEqual(across_an_update(page, "data-row-key"), (0, 0))
        # Control: the same rows known only by their first cell, as they were. All 17 of a gate are then one
        # row to the update, which puts the last of them where the reader's row was.
        unkeyed = page.replace(' data-row-key="fp_g', ' data-case="fp_g')
        moved, scrolled = across_an_update(unkeyed, "data-case")
        self.assertLess(moved, 0)
        self.assertEqual(scrolled, -moved)


    def test_frozen_boundary_inference_is_explicit_and_deterministic(self):
        result = suite.infer_transition(self.reference["boundary"])
        self.assertEqual(result[64]["bracket"], [366, 370])
        self.assertEqual(result[96]["bracket"], [366, 368])
        self.assertEqual(result[64]["plateau_counts"], [336, 344, 348, 350, 352, 354, 356])
        self.assertEqual(result[96]["plateau_counts"], [320, 336, 344, 348, 352, 354, 356])
        self.assertAlmostEqual(result[64]["plateau_median"], 1.099645720442374)
        self.assertAlmostEqual(result[96]["plateau_p90"], 1.1619777848371406)
        # Analysis is order independent and must not mutate frozen evidence.
        original = copy.deepcopy(self.reference["boundary"])
        shuffled = copy.deepcopy(original)
        shuffled["cases"].reverse()
        self.assertEqual(suite.infer_transition(shuffled), result)
        self.assertEqual(self.reference["boundary"], original)
        # An unqualified apparent early rise cannot narrow the bracket.
        for row in shuffled["cases"]:
            if row["id"] == "fp_g64_n368":
                row.update(median_ratio=10, p10_ratio=9, p90_ratio=13, qualified=False)
        self.assertEqual(suite.infer_transition(shuffled)[64]["bracket"], [366, 370])
        # Without a qualified rise or plateau, report no bracket.
        for row in shuffled["cases"]:
            if row["id"].startswith("fp_g64_") and int(row["id"].split("_n")[1]) > 356:
                row["qualified"] = False
            if row["id"].startswith("fp_g96_") and int(row["id"].split("_n")[1]) <= 356:
                row["qualified"] = False
        self.assertIsNone(suite.infer_transition(shuffled)[64]["bracket"])
        self.assertIsNone(suite.infer_transition(shuffled)[96]["bracket"])

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
            self.assertIn('Paired capacity boundary (ABBA)', micro)
            self.assertIn('<details id="capacity-boundary-table">', micro)
            self.assertIn('gate 64: (366, 370]', micro)
            self.assertIn('gate 96: (366, 368]', micro)
            self.assertNotIn('Paired capacity boundary', overall)
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
